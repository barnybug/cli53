package cli53

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/urfave/cli/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithylogging "github.com/aws/smithy-go/logging"
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

func getConfig(c *cli.Context) (aws.Config, error) {
	ctx := context.Background()
	debug := c.Bool("debug")
	endpoint := c.String("endpoint-url")
	profile := c.String("profile")

	options := []func(*config.LoadOptions) error{
		config.WithRetryMaxAttempts(100),
	}

	if profile != "" {
		options = append(options, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		fallbackCfg, handled, fallbackErr := loadConfigWithSourceProfileFallback(ctx, effectiveSharedConfigProfile(profile))
		if handled {
			if fallbackErr != nil {
				return aws.Config{}, fallbackErr
			}
			return fallbackCfg, nil
		}
		return aws.Config{}, err
	}

	if debug {
		cfg.Logger = smithylogging.NewStandardLogger(os.Stderr)
		cfg.ClientLogMode = aws.LogRetries | aws.LogRequestWithBody | aws.LogResponseWithBody
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	// SDK requires region to be set when endpoint-url is set.
	if endpoint != "" && cfg.Region == "" {
		return aws.Config{}, cli.NewExitError("AWS_REGION must be set when using --endpoint-url", 1)
	}

	return cfg, nil
}

func loadConfigWithSourceProfileFallback(ctx context.Context, profile string) (aws.Config, bool, error) {
	target, found, err := loadAWSProfileSettings(profile)
	if err != nil {
		return aws.Config{}, false, nil
	}
	if !found || target.RoleARN == "" || target.SourceProfileName == "" {
		return aws.Config{}, false, nil
	}

	source, found, err := loadAWSProfileSettings(target.SourceProfileName)
	if err != nil {
		return aws.Config{}, false, nil
	}
	if !found || source.LoginSession == "" {
		return aws.Config{}, false, nil
	}
	if target.MFASerial != "" {
		return aws.Config{}, true, fmt.Errorf("profile %q requires MFA serial %q; cli53 fallback does not support prompting for MFA", profile, target.MFASerial)
	}

	options := []func(*config.LoadOptions) error{
		config.WithRetryMaxAttempts(100),
		config.WithSharedConfigProfile(target.SourceProfileName),
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, true, err
	}

	if target.Region != "" {
		cfg.Region = target.Region
	}

	assumeRoleOptions := []func(*stscreds.AssumeRoleOptions){
		func(o *stscreds.AssumeRoleOptions) {
			if target.ExternalID != "" {
				o.ExternalID = aws.String(target.ExternalID)
			}
			if target.RoleSessionName != "" {
				o.RoleSessionName = target.RoleSessionName
			}
			if target.Duration > 0 {
				o.Duration = target.Duration
			}
		},
	}
	cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), target.RoleARN, assumeRoleOptions...))

	return cfg, true, nil
}

func effectiveSharedConfigProfile(profile string) string {
	switch {
	case profile != "":
		return profile
	case os.Getenv("AWS_PROFILE") != "":
		return os.Getenv("AWS_PROFILE")
	case os.Getenv("AWS_DEFAULT_PROFILE") != "":
		return os.Getenv("AWS_DEFAULT_PROFILE")
	default:
		return "default"
	}
}

func sharedConfigFilesForDebug() []string {
	if path := os.Getenv("AWS_CONFIG_FILE"); path != "" {
		return []string{path}
	}
	return append([]string(nil), config.DefaultSharedConfigFiles...)
}

func sharedCredentialsFilesForDebug() []string {
	if path := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); path != "" {
		return []string{path}
	}
	return append([]string(nil), config.DefaultSharedCredentialsFiles...)
}

type awsProfileSettings struct {
	Name              string
	Region            string
	RoleARN           string
	SourceProfileName string
	ExternalID        string
	RoleSessionName   string
	MFASerial         string
	LoginSession      string
	Duration          time.Duration
}

func loadAWSProfileSettings(profile string) (awsProfileSettings, bool, error) {
	settings := awsProfileSettings{Name: profile}
	found := false

	for _, path := range sharedConfigFilesForDebug() {
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return awsProfileSettings{}, false, err
		}

		fileFound, fileErr := parseAWSConfigProfile(file, profile, &settings)
		_ = file.Close()
		if fileErr != nil {
			return awsProfileSettings{}, false, fmt.Errorf("parse %s: %w", path, fileErr)
		}
		found = found || fileFound
	}

	return settings, found, nil
}

func parseAWSConfigProfile(file *os.File, profile string, settings *awsProfileSettings) (bool, error) {
	scanner := bufio.NewScanner(file)
	targetSections := map[string]struct{}{"profile " + profile: {}}
	if profile == "default" {
		targetSections = map[string]struct{}{"default": {}}
	}

	found := false
	inTarget := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSpace(line[1 : len(line)-1])
			_, inTarget = targetSections[section]
			found = found || inTarget
			continue
		}
		if !inTarget {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		value = strings.TrimSpace(value)

		switch key {
		case "region":
			settings.Region = value
		case "role_arn":
			settings.RoleARN = value
		case "source_profile":
			settings.SourceProfileName = value
		case "external_id":
			settings.ExternalID = value
		case "role_session_name":
			settings.RoleSessionName = value
		case "mfa_serial":
			settings.MFASerial = value
		case "login_session":
			settings.LoginSession = value
		case "duration_seconds":
			seconds, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("invalid duration_seconds %q", value)
			}
			settings.Duration = time.Duration(seconds) * time.Second
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return found, nil
}

func getService(c *cli.Context) (*route53.Client, error) {
	cfg, err := getConfig(c)
	if err != nil {
		return nil, err
	}

	roleARN := c.String("role-arn")
	if roleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), roleARN))
	}

	return route53.NewFromConfig(cfg, func(o *route53.Options) {
		if endpoint := c.String("endpoint-url"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	}), nil
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

var reZoneId = regexp.MustCompile("^(/hostedzone/)?Z[A-Z0-9]{9,}$")

func isZoneId(s string) bool {
	return reZoneId.MatchString(s)
}

func lookupZone(ctx context.Context, nameOrId string) *route53types.HostedZone {
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
		var notFound *route53types.NoSuchHostedZone
		if errors.As(err, &notFound) {
			errorAndExit(fmt.Sprintf("Zone '%s' not found", nameOrId))
		}
		fatalIfErr(err)
		return resp.HostedZone
	} else {
		// lookup by name
		matches := []route53types.HostedZone{}
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

func waitForChange(ctx context.Context, change *route53types.ChangeInfo) {
	fmt.Printf("Waiting for sync")
	for {
		req := route53.GetChangeInput{Id: change.Id}
		resp, err := r53.GetChange(ctx, &req)
		fatalIfErr(err)
		if resp.ChangeInfo.Status == route53types.ChangeStatusInsync {
			fmt.Println("\nCompleted")
			break
		} else if resp.ChangeInfo.Status == route53types.ChangeStatusPending {
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
