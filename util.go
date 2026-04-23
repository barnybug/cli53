package cli53

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/urfave/cli/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
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

func getAWSConfig(c *cli.Context) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{}

	if profile := c.String("profile"); profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	if c.Bool("debug") {
		opts = append(opts, awsconfig.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody))
		opts = append(opts, awsconfig.WithLogger(logging.NewStandardLogger(os.Stderr)))
	}

	return awsconfig.LoadDefaultConfig(context.Background(), opts...)
}

func getService(c *cli.Context) (*route53.Client, error) {
	cfg, err := getAWSConfig(c)
	if err != nil {
		return nil, err
	}

	roleARN := c.String("role-arn")
	if roleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		cfg.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(stsClient, roleARN),
		)
	}

	r53opts := []func(*route53.Options){}
	if endpoint := c.String("endpoint-url"); endpoint != "" {
		region := os.Getenv("AWS_REGION")
		if region == "" {
			return nil, cli.NewExitError("AWS_REGION must be set when using --endpoint-url", 1)
		}
		r53opts = append(r53opts, func(o *route53.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	return route53.NewFromConfig(cfg, r53opts...), nil
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

func uniqueReference() string {
	return fmt.Sprintf("%0x", rand.Int())
}

var unescaper = strings.NewReplacer(`\057`, "/", `\052`, "*")

func zoneName(s string) string {
	return unescaper.Replace(strings.TrimRight(s, "."))
}

var reZoneId = regexp.MustCompile("^(/hostedzone/)?Z[A-Z0-9]{9,}$")

func isZoneId(s string) bool {
	return reZoneId.MatchString(s)
}

func lookupZone(ctx context.Context, nameOrId string) *types.HostedZone {
	if isZoneId(nameOrId) {
		// lookup by id
		id := nameOrId
		if !strings.HasPrefix(nameOrId, "/hostedzone/") {
			id = "/hostedzone/" + id
		}
		req := route53.GetHostedZoneInput{
			Id: aws.String(id),
		}
		resp, err := r53.GetHostedZone(ctx, &req)
		var nsz *types.NoSuchHostedZone
		if errors.As(err, &nsz) {
			errorAndExit(fmt.Sprintf("Zone '%s' not found", nameOrId))
		}
		fatalIfErr(err)
		return resp.HostedZone
	} else {
		// lookup by name
		matches := []types.HostedZone{}
		req := route53.ListHostedZonesByNameInput{
			DNSName: aws.String(nameOrId),
		}
		resp, err := r53.ListHostedZonesByName(ctx, &req)
		fatalIfErr(err)
		for _, zone := range resp.HostedZones {
			if zoneName(*zone.Name) == zoneName(nameOrId) {
				matches = append(matches, zone)
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

func waitForChange(ctx context.Context, change *types.ChangeInfo) {
	fmt.Printf("Waiting for sync")
	for {
		req := route53.GetChangeInput{Id: change.Id}
		resp, err := r53.GetChange(ctx, &req)
		fatalIfErr(err)
		if resp.ChangeInfo.Status == types.ChangeStatusInsync {
			fmt.Println("\nCompleted")
			break
		} else if resp.ChangeInfo.Status == types.ChangeStatusPending {
			fmt.Printf(".")
		} else {
			fmt.Printf("\nFailed: %s\n", resp.ChangeInfo.Status)
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

// parse a <character-string> per RFC 1035 Section 5.1
func parseCharacterString(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return reBackslashed.ReplaceAllString(s[1:len(s)-1], "$1")
	} else {
		return s
	}
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
