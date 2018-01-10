package cli53

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/urfave/cli"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

// Qualify names, if relative
func qualifyName(name, origin string) string {
	if name == "" || name == "@" {
		// root
		return origin
	} else if !strings.HasSuffix(name, ".") {
		// unqualified
		return name + "." + origin
	} else {
		// qualified
		return name
	}
}

func getConfig(c *cli.Context) (*aws.Config, error) {
	debug := c.Bool("debug")
	endpoint := c.String("endpoint-url")
	region := ""
	// SDK requires region to be set when endpoint-url is set
	if rg := os.Getenv("AWS_REGION"); rg != "" {
		region = rg
	} else if endpoint != "" {
		return nil, cli.NewExitError("AWS_REGION must be set when using --endpoint-url", 4)
	}
	config := aws.Config{
		Endpoint: &endpoint,
		Region:   &region,
		Logger: aws.LoggerFunc(func(args ...interface{}) {
			fmt.Fprintln(os.Stderr, args...)
		}),
	}
	// ensures throttled requests are retried
	config.MaxRetries = aws.Int(100)
	if debug {
		config.LogLevel = aws.LogLevel(aws.LogDebug)
	}
	return &config, nil
}

func getService(c *cli.Context) (*route53.Route53, error) {
	config, err := getConfig(c)
	if err != nil {
		return nil, err
  }
  profile := c.String("profile")
  if profile != "" {
    sess := session.Must(session.NewSessionWithOptions(session.Options{
       Config: *config,
       SharedConfigState: session.SharedConfigEnable,
       Profile: profile,
    }))
    return route53.New(sess), nil
  } else {
    sess := session.Must(session.NewSessionWithOptions(session.Options{
       Config: *config,
       SharedConfigState: session.SharedConfigEnable,
    }))
    return route53.New(sess), nil
  }
}

func fatalIfErr(err error) {
	if err != nil {
		errorAndExit(fmt.Sprint(err))
	}
}

func errorAndExit(msg string) {
	fmt.Fprintln(os.Stderr, "Error: "+msg)
	os.Exit(1)
}

var seeded sync.Once

func uniqueReference() string {
	seeded.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
	return fmt.Sprintf("%0x", rand.Int())
}

var unescaper = strings.NewReplacer(`\057`, "/", `\052`, "*")

func zoneName(s string) string {
	return unescaper.Replace(strings.TrimRight(s, "."))
}

var reZoneId = regexp.MustCompile("^(/hostedzone/)?Z[A-Z0-9]{10,}$")

func isZoneId(s string) bool {
	return reZoneId.MatchString(s)
}

func lookupZone(nameOrId string) *route53.HostedZone {
	if isZoneId(nameOrId) {
		// lookup by id
		id := nameOrId
		if !strings.HasPrefix(nameOrId, "/hostedzone/") {
			id = "/hostedzone/" + id
		}
		req := route53.GetHostedZoneInput{
			Id: aws.String(id),
		}
		resp, err := r53.GetHostedZone(&req)
		if err, ok := err.(awserr.Error); ok && err.Code() == "NoSuchHostedZone" {
			errorAndExit(fmt.Sprintf("Zone '%s' not found", nameOrId))
		}
		fatalIfErr(err)
		return resp.HostedZone
	} else {
		// lookup by name
		matches := []route53.HostedZone{}
		req := route53.ListHostedZonesByNameInput{
			DNSName: aws.String(nameOrId),
		}
		resp, err := r53.ListHostedZonesByName(&req)
		fatalIfErr(err)
		for _, zone := range resp.HostedZones {
			if zoneName(*zone.Name) == zoneName(nameOrId) {
				matches = append(matches, *zone)
			}
		}
		switch len(matches) {
		case 0:
			errorAndExit(fmt.Sprintf("Zone '%s' not found", nameOrId))
		case 1:
			return &matches[0]
		default:
			errorAndExit("Multiple zones match - you will need to use Zone ID to uniquely identify the zone")
		}
	}
	return nil
}

func waitForChange(change *route53.ChangeInfo) {
	fmt.Printf("Waiting for sync")
	for {
		req := route53.GetChangeInput{Id: change.Id}
		resp, err := r53.GetChange(&req)
		fatalIfErr(err)
		if *resp.ChangeInfo.Status == "INSYNC" {
			fmt.Println("\nCompleted")
			break
		} else if *resp.ChangeInfo.Status == "PENDING" {
			fmt.Printf(".")
		} else {
			fmt.Printf("\nFailed: %s\n", *resp.ChangeInfo.Status)
			break
		}
		time.Sleep(1 * time.Second)
	}
}

// Use shortened form of name with origin removed/abbreviated.
func shortenName(name, origin string) string {
	if name == origin {
		return "@"
	}
	return strings.TrimSuffix(name, "."+origin)
}

var reQuotedValue = regexp.MustCompile(`"((?:\\"|[^"])*)"`)
var reBackslashed = regexp.MustCompile(`\\(.)`)

func splitValues(s string) []string {
	ret := []string{}
	for _, m := range reQuotedValue.FindAllStringSubmatch(s, -1) {
		val := reBackslashed.ReplaceAllString(m[1], "$1")
		ret = append(ret, val)
	}
	return ret
}

var quoter = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func quote(s string) string {
	return `"` + quoter.Replace(s) + `"`
}

type KeyValues []interface{}

func (kvs KeyValues) GetOptString(key string) *string {
	for i := 0; i < len(kvs); i += 2 {
		if kvs[i] == key {
			if value, ok := kvs[i+1].(string); ok {
				return &value
			}
		}
	}
	return nil
}

func (kvs KeyValues) GetString(key string) string {
	val := kvs.GetOptString(key)
	if val != nil {
		return *val
	}
	return ""
}

func (kvs KeyValues) GetInt(key string) int {
	for i := 0; i < len(kvs); i += 2 {
		if kvs[i] == key {
			if value, ok := kvs[i+1].(int); ok {
				return value
			}
		}
	}
	return 0
}

func (kvs KeyValues) String() string {
	var ret string
	for i := 0; i < len(kvs); i += 2 {
		key := kvs[i]
		value := kvs[i+1]
		if ret != "" {
			ret += " "
		}
		switch value := value.(type) {
		case string:
			ret += fmt.Sprintf("%s=%s", key, quote(value))
		case int64, int:
			ret += fmt.Sprintf("%s=%v", key, value)
		}
	}
	return ret
}

func ParseKeyValues(input string) (result KeyValues, err error) {
	// result = append(result, "a", 2)
	l := lex(input)

	for {
		// alpha key
		key := l.acceptRun(unicode.IsLetter)
		if key == "" {
			err = l.Error("Expected key")
			return
		}
		// equals separator
		if !l.accept("=") {
			err = l.Error("Expected =")
			return
		}
		// value (string or int)
		var value interface{}
		if l.accept(`"`) {
			// quoted string
			str := ""
			for {
				if l.eof() {
					err = l.Error("Unterminated quoted string")
					return
				} else if l.accept(`\`) {
					str += l.acceptAny()
				} else if l.accept(`"`) {
					break
				} else {
					str += l.acceptAny()
				}
			}
			value = str
		} else if num := l.acceptRun(unicode.IsDigit); num != "" {
			value, err = strconv.Atoi(num)
			if err != nil {
				return
			}
		} else {
			err = l.Error("Unexpected token")
			return
		}
		result = append(result, key, value)
		if l.eof() {
			break
		}
		// whitespace between multiple key values
		if l.acceptRun(unicode.IsSpace) == "" {
			err = l.Error("Expected whitespace")
			return
		}
	}

	return
}
