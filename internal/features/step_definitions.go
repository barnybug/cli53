package features

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/barnybug/cli53"

	. "github.com/gucumber/gucumber"
)

func getService() *route53.Route53 {
	config := aws.Config{
		Logger: aws.LoggerFunc(func(args ...interface{}) {
			fmt.Fprintln(os.Stderr, args...)
		}),
	}
	// ensures throttled requests are retried
	config.MaxRetries = aws.Int(100)
	return route53.New(session.New(), &config)
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatalf("Unexpected error: %s", err)
	}
}

var cleanupIds = []string{}
var cleanupDSIds = []string{}
var runOutput string
var retCode int
var backReferences []string

func domainExists(name string) bool {
	return domainId(name) != ""
}

func domainZone(name string) *route53.HostedZone {
	r53 := getService()
	zones, err := r53.ListHostedZones(nil)
	fatalIfErr(err)
	for _, zone := range zones.HostedZones {
		if *zone.Name == name+"." {
			return zone
		}
	}
	return nil
}

func reusableDelegationSet(id string) *route53.DelegationSet {
	r53 := getService()
	req := route53.GetReusableDelegationSetInput{Id: &id}
	resp, err := r53.GetReusableDelegationSet(&req)
	if err == nil {
		return resp.DelegationSet
	}
	return nil
}

func domainId(name string) string {
	if zone := domainZone(name); zone != nil {
		return *zone.Id
	}
	return ""
}

var seeded sync.Once

func uniqueReference() string {
	seeded.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
	return fmt.Sprintf("%0x", rand.Int())
}

func cleanupDomain(r53 *route53.Route53, id string) {
	// delete all non-default SOA/NS records
	rrsets, err := cli53.ListAllRecordSets(r53, id)
	fatalIfErr(err)
	changes := []*route53.Change{}
	for _, rrset := range rrsets {
		if *rrset.Type != "NS" && *rrset.Type != "SOA" {
			change := &route53.Change{
				Action:            aws.String("DELETE"),
				ResourceRecordSet: rrset,
			}
			changes = append(changes, change)
		}
	}

	if len(changes) > 0 {
		req2 := route53.ChangeResourceRecordSetsInput{
			HostedZoneId: &id,
			ChangeBatch: &route53.ChangeBatch{
				Changes: changes,
			},
		}
		_, err = r53.ChangeResourceRecordSets(&req2)
		if err != nil {
			fmt.Printf("Warning: cleanup failed - %s\n", err)
		}
	}

	req3 := route53.DeleteHostedZoneInput{Id: &id}
	_, err = r53.DeleteHostedZone(&req3)
	if err != nil {
		fmt.Printf("Warning: cleanup failed - %s\n", err)
	}
}

func cleanupReusableDelegationSet(r53 *route53.Route53, id string) {
	req := route53.DeleteReusableDelegationSetInput{Id: &id}
	_, err := r53.DeleteReusableDelegationSet(&req)
	if err != nil {
		fmt.Printf("Warning: cleanup failed - %s\n", err)
	}
}

// Split on whitespace, but leave quoted strings in tact
func safeSplit(s string) []string {
	split := strings.Split(s, " ")

	var result []string
	var inquote string
	var block string
	for _, i := range split {
		if inquote == "" {
			if strings.HasPrefix(i, "'") || strings.HasPrefix(i, "\"") {
				inquote = string(i[0])
				block = strings.TrimPrefix(i, inquote) + " "
			} else {
				result = append(result, i)
			}
		} else {
			if !strings.HasSuffix(i, inquote) {
				block += i + " "
			} else {
				block += strings.TrimSuffix(i, inquote)
				inquote = ""
				result = append(result, block)
				block = ""
			}
		}
	}

	return result
}

func domain(s string) string {
	domain := World["$domain"].(string)
	return strings.Replace(s, "$domain", domain, -1)
}

var unquoter = strings.NewReplacer(`\\`, `\`, `\"`, `"`)

func unquote(s string) string {
	return unquoter.Replace(s)
}

var reMagic = regexp.MustCompile(`\$\w+`)

func replaceMagics(s string) string {
	return reMagic.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := World[m]; ok {
			return v.(string)
		}
		if i, err := strconv.Atoi(m[1:]); err == nil {
			return backReferences[i]
		}
		return m
	})
}

func coverageArgs(args []string) []string {
	// add coverage parameters to command
	coverage := fmt.Sprintf("coverage/%d.txt", rand.Int())
	return append([]string{args[0], "-test.coverprofile", coverage}, args[1:]...)
}

func execute(cmd string, env ...string) {
	args := safeSplit(cmd)
	ps := exec.Command("./"+args[0], args[1:]...)
	ps.Env = append(os.Environ(), env...)
	out, err := ps.CombinedOutput()
	runOutput = string(out)
	if err, ok := err.(*exec.ExitError); ok {
		waitStatus := err.Sys().(syscall.WaitStatus)
		retCode = waitStatus.ExitStatus()
	} else if err != nil {
		T.Errorf("Error: %s Output: %s", err, out)
	}
}

func init() {
	Before("", func() {
		// randomize temporary test domain name
		World["$domain"] = fmt.Sprintf("example%s.com", uniqueReference())
	})

	After("", func() {
		delete(World, "$domain")
		delete(World, "$delegationSet")
		if len(cleanupIds) > 0 {
			// cleanup
			r53 := getService()
			for _, id := range cleanupIds {
				cleanupDomain(r53, id)
			}
			cleanupIds = []string{}
		}
		if len(cleanupDSIds) > 0 {
			// cleanup
			r53 := getService()
			for _, id := range cleanupDSIds {
				cleanupReusableDelegationSet(r53, id)
			}
			cleanupDSIds = []string{}
		}
	})

	Given(`^I have a domain "(.+?)"$`, func(name string) {
		name = domain(name)
		// create a test domain
		r53 := getService()
		callerReference := uniqueReference()
		req := route53.CreateHostedZoneInput{
			CallerReference: &callerReference,
			Name:            &name,
		}
		resp, err := r53.CreateHostedZone(&req)
		fatalIfErr(err)
		cleanupIds = append(cleanupIds, *resp.HostedZone.Id)
	})

	Given(`^I have a delegation set$`, func() {
		r53 := getService()
		callerReference := uniqueReference()
		req := route53.CreateReusableDelegationSetInput{
			CallerReference: &callerReference,
		}
		resp, err := r53.CreateReusableDelegationSet(&req)
		fatalIfErr(err)
		id := *resp.DelegationSet.Id
		World["$delegationSet"] = id
		cleanupDSIds = append(cleanupDSIds, id)
	})

	When(`^I run "(.+?)"$`, func(cmd string) {
		cmd = replaceMagics(cmd)
		args := safeSplit(cmd)
		if os.Getenv("COVERAGE") != "" {
			args = coverageArgs(args)
		}
		ps := exec.Command("./"+args[0], args[1:]...)
		out, err := ps.CombinedOutput()
		if err != nil {
			T.Errorf("Error: %s Output: %s", err, out)
		} else {
			runOutput = string(out)
		}
	})

	When(`^I execute "(.+?)"$`, func(cmd string) {
		execute(domain(cmd))

	})

	When(`^I execute "(.+?)" with var (.+?) as "(.+?)"$`, func(cmd, name, value string) {
		execute(domain(cmd), name + "=" + value)
	})

	Then(`^the domain "(.+?)" is created$`, func(name string) {
		name = domain(name)
		id := domainId(name)
		if id == "" {
			T.Errorf("Domain %s was not created", name)
		} else {
			cleanupIds = append(cleanupIds, id)
		}
	})

	Then(`^the domain "(.+?)" is deleted$`, func(name string) {
		name = domain(name)
		id := domainId(name)
		if id == "" {
			cleanupIds = []string{} // drop from cleanupIds
		} else {
			T.Errorf("Domain %s was not deleted", name)
			cleanupIds = append(cleanupIds, id)
		}
	})

	Then(`^the domain "(.+?)" has (\d+) records$`, func(name string, expected int) {
		name = domain(name)
		r53 := getService()
		id := domainId(name)
		rrsets, err := cli53.ListAllRecordSets(r53, id)
		fatalIfErr(err)
		actual := len(rrsets)
		if expected != actual {
			T.Errorf("Domain %s: Expected %d records, actually %d records ", name, expected, actual)
		}
	})

	Then(`^the domain "(.+?)" has record "(.+)"$`, func(name, record string) {
		name = domain(name)
		record = domain(record)
		if !hasRecord(name, record) {
			T.Errorf("Domain %s: missing record %s", name, record)
		}
	})

	Then(`^the domain "(.+?)" doesn't have record "(.+)"$`, func(name, record string) {
		name = domain(name)
		record = domain(record)
		if hasRecord(name, record) {
			T.Errorf("Domain %s: present record %s", name, record)
		}
	})

	Then(`^the domain "(.+?)" export matches file "(.+?)"( including auth)?$`, func(name, filename, auth string) {
		name = domain(name)
		r53 := getService()
		zone := domainZone(name)
		out := new(bytes.Buffer)
		cli53.ExportBindToWriter(r53, zone, false, out)
		actual := out.Bytes()
		rfile, err := os.Open(filename)
		fatalIfErr(err)
		defer rfile.Close()
		expected, err := ioutil.ReadAll(rfile)
		fatalIfErr(err)

		errors := compareDomains(expected, actual, auth != "")
		if len(errors) > 0 {
			T.Errorf(errors)
		}
	})

	Then(`^the output contains "(.+?)"$`, func(s string) {
		s = unquote(domain(s))
		if !strings.Contains(runOutput, s) {
			T.Errorf("Output did not contain \"%s\"\nactual: %s", s, runOutput)
		}
	})

	Then(`^the output file "(.+?)" contains "(.+?)"$`, func(outputFile string, s string) {
		outputFile = unquote(outputFile)
		s = unquote(domain(s))
		output, err := ioutil.ReadFile(outputFile)

		if err != nil {
			T.Errorf("Could not read %s", outputFile)
		}

		if !strings.Contains(string(output), s) {
			T.Errorf("Output did not contain \"%s\"\nactual: %s", s, runOutput)
		}
	})

	Then(`^the output matches "(.+?)"$`, func(s string) {
		re, err := regexp.Compile(s)
		fatalIfErr(err)
		match := re.FindStringSubmatch(runOutput)
		if match == nil {
			T.Errorf("Output did not match \"%s\"", s)
		}
		backReferences = match
	})

	Then(`^the exit code was (\d+)$`, func(code int) {
		if code != retCode {
			T.Errorf("Exit code expected: %d != actual: %d", code, retCode)
		}
	})

	Then(`^the delegation set "(.+?)" is created$`, func(id string) {
		id = replaceMagics(id)
		ds := reusableDelegationSet(id)
		if ds == nil {
			T.Errorf("Reusable delegation set %s was not created", id)
		} else {
			cleanupDSIds = append(cleanupDSIds, id)
		}
	})

	Then(`^the delegation set "(.+?)" is deleted$`, func(id string) {
		id = replaceMagics(id)
		ds := reusableDelegationSet(id)
		if ds == nil {
			cleanupDSIds = []string{}
		} else {
			T.Errorf("Reusable delegation set %s was not deleted", id)
			cleanupDSIds = append(cleanupDSIds, id)
		}
	})
}

func hasRecord(name, record string) bool {
	r53 := getService()
	zone := domainZone(name)
	rrsets, err := cli53.ListAllRecordSets(r53, *zone.Id)
	fatalIfErr(err)

	for _, rrset := range rrsets {
		rrs := cli53.ConvertRRSetToBind(rrset)
		cli53.UnexpandSelfAliases(rrs, zone, false)
		for _, rr := range rrs {
			line := rr.String()
			line = strings.Replace(line, "\t", " ", -1)
			if record == line {
				return true
			}
		}
	}
	return false
}

func prepareZoneFile(b []byte, includeAuth bool) map[string]bool {
	s := string(b)
	s = strings.Replace(s, "\t", " ", -1)
	lines := strings.Split(s, "\n")
	ret := map[string]bool{}
	for _, line := range lines {
		if strings.HasPrefix(line, "$ORIGIN") {
			continue
		}
		if !includeAuth && (strings.Contains(line, " NS ") || strings.Contains(line, " SOA ")) {
			continue
		}
		ret[line] = true
	}
	return ret
}

func compareDomains(expected, actual []byte, includeAuth bool) string {
	mexpected := prepareZoneFile(expected, includeAuth)
	mactual := prepareZoneFile(actual, includeAuth)

	var errors string
	for record := range mexpected {
		if _, ok := mactual[record]; ok {
			delete(mactual, record)
		} else {
			errors += fmt.Sprintf("Expected record '%s' missing\n", record)
		}
	}
	for record := range mactual {
		errors += fmt.Sprintf("Unexpected record '%s' present\n", record)
	}
	return errors
}
