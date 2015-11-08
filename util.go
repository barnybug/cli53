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

	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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

func getService(debug bool, profile string) *route53.Route53 {
	config := aws.Config{}
	if profile != "" {
		config.Credentials = credentials.NewSharedCredentials("", profile)
	}
	// ensures throttled requests are retried
	config.MaxRetries = aws.Int(100)
	if debug {
		config.LogLevel = aws.LogLevel(aws.LogDebug)
	}
	return route53.New(&config)
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

var reZoneId = regexp.MustCompile("^(/hostedzone/)?[A-Z0-9]{12,}$")

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
		req := route53.ListHostedZonesInput{}
		for {
			resp, err := r53.ListHostedZones(&req)
			fatalIfErr(err)
			for _, zone := range resp.HostedZones {
				if zoneName(*zone.Name) == zoneName(nameOrId) || *zone.Id == nameOrId {
					matches = append(matches, *zone)
				}
			}
			if *resp.IsTruncated {
				req.Marker = resp.NextMarker
			} else {
				break
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
	} else if strings.HasSuffix(name, origin) {
		return name[0 : len(name)-len(origin)-1]
	} else {
		return name
	}
}

var reOutsideQuotes = regexp.MustCompile(`"(.*)"`)
var reBackslashed = regexp.MustCompile(`\\(.)`)

func unquote(s string) string {
	// remove outside quotes
	s = reOutsideQuotes.ReplaceAllString(s, "$1")
	// unbackslash inside
	return reBackslashed.ReplaceAllString(s, "$1")
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
