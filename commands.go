package cli53

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
)

const ChangeBatchSize = 100

func createZone(name, comment, vpcId, vpcRegion, delegationSetId string) {
	callerReference := uniqueReference()
	req := route53.CreateHostedZoneInput{
		CallerReference: &callerReference,
		Name:            &name,
		HostedZoneConfig: &route53.HostedZoneConfig{
			Comment: &comment,
		},
	}

	if vpcId != "" && vpcRegion != "" {
		req.VPC = &route53.VPC{
			VPCId:     aws.String(vpcId),
			VPCRegion: aws.String(vpcRegion),
		}
	}
	if delegationSetId != "" {
		delegationSetId = strings.Replace(delegationSetId, "/delegationset/", "", 1)
		req.DelegationSetId = aws.String(delegationSetId)
	}
	resp, err := r53.CreateHostedZone(&req)
	fatalIfErr(err)
	fmt.Printf("Created zone: '%s' ID: '%s'\n", *resp.HostedZone.Name, *resp.HostedZone.Id)
}

func createReusableDelegationSet(zoneId string) {
	callerReference := uniqueReference()
	req := route53.CreateReusableDelegationSetInput{
		CallerReference: &callerReference,
	}
	if zoneId != "" {
		req.HostedZoneId = &zoneId
	}
	resp, err := r53.CreateReusableDelegationSet(&req)
	fatalIfErr(err)
	ds := resp.DelegationSet
	fmt.Printf("Created reusable delegation set ID: '%s'\n", *ds.Id)
	for _, ns := range ds.NameServers {
		fmt.Printf("Nameserver: %s\n", *ns)
	}
}

func listReusableDelegationSets() {
	req := route53.ListReusableDelegationSetsInput{}
	resp, err := r53.ListReusableDelegationSets(&req)
	fatalIfErr(err)
	fmt.Printf("Reusable delegation sets:\n")
	if len(resp.DelegationSets) == 0 {
		fmt.Println("none")
		return
	}
	for _, ds := range resp.DelegationSets {
		var nameservers []string
		for _, ns := range ds.NameServers {
			nameservers = append(nameservers, *ns)
		}
		fmt.Printf("- ID: %s Nameservers: %s\n", *ds.Id, strings.Join(nameservers, ", "))
	}
}

func deleteReusableDelegationSet(id string) {
	if !strings.HasPrefix(id, "/delegationset/") {
		id = "/delegationset/" + id
	}
	req := route53.DeleteReusableDelegationSetInput{
		Id: &id,
	}
	_, err := r53.DeleteReusableDelegationSet(&req)
	fatalIfErr(err)
	fmt.Printf("Deleted reusable delegation set\n")
}

func deleteRecordSets(zone *route53.HostedZone, rrsets []*route53.ResourceRecordSet, wait bool) (int, error) {
	// delete all non-default SOA/NS records
	changes := []*route53.Change{}
	for _, rrset := range rrsets {
		if !isAuthRecord(zone, rrset) {
			change := &route53.Change{
				Action:            aws.String("DELETE"),
				ResourceRecordSet: rrset,
			}
			changes = append(changes, change)
		}
	}

	if len(changes) > 0 {
		req := route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zone.Id,
			ChangeBatch: &route53.ChangeBatch{
				Changes: changes,
			},
		}
		resp, err := r53.ChangeResourceRecordSets(&req)
		if err != nil {
			return 0, err
		}
		if wait {
			waitForChange(resp.ChangeInfo)
		}
	}
	return len(changes), nil
}

func purgeZoneRecords(zone *route53.HostedZone, wait bool) {
	total := 0
	err := batchListAllRecordSets(r53, *zone.Id, func(rrsets []*route53.ResourceRecordSet) {
		n, err := deleteRecordSets(zone, rrsets, wait)
		fatalIfErr(err)
		total += n
	})
	fatalIfErr(err)

	fmt.Printf("%d record sets deleted\n", total)
}

func deleteZone(name string, purge bool) {
	zone := lookupZone(name)
	if purge {
		purgeZoneRecords(zone, false)
	}
	req := route53.DeleteHostedZoneInput{Id: zone.Id}
	_, err := r53.DeleteHostedZone(&req)
	fatalIfErr(err)
	fmt.Printf("Deleted zone: '%s' ID: '%s'\n", *zone.Name, *zone.Id)
}

func listZones(formatter Formatter) {
	zones := make(chan *route53.HostedZone)
	go func() {
		req := route53.ListHostedZonesInput{}
		for {
			// paginated
			resp, err := r53.ListHostedZones(&req)
			fatalIfErr(err)
			for _, zone := range resp.HostedZones {
				zones <- zone
			}
			if *resp.IsTruncated {
				req.Marker = resp.NextMarker
			} else {
				break
			}
		}
		close(zones)
	}()
	formatter.formatZoneList(zones, os.Stdout)
}

func isAuthRecord(zone *route53.HostedZone, rrset *route53.ResourceRecordSet) bool {
	return (*rrset.Type == "SOA" || *rrset.Type == "NS") && *rrset.Name == *zone.Name
}

func expandSelfAliases(records []dns.RR, zone *route53.HostedZone) {
	for _, record := range records {
		expandSelfAlias(record, zone)
	}
}

func expandSelfAlias(record dns.RR, zone *route53.HostedZone) {
	if awsrr, ok := record.(*AWSRR); ok {
		record = awsrr.RR
	}
	if alias, ok := record.(*dns.PrivateRR); ok {
		rdata := alias.Data.(*ALIASRdata)
		if rdata.ZoneId == "$self" {
			rdata.ZoneId = strings.Replace(*zone.Id, "/hostedzone/", "", 1)
			rdata.Target = qualifyName(rdata.Target, *zone.Name)
		}
	}
}

type Key struct {
	Name       string
	Rrtype     uint16
	Identifier string
}

type changeSorter struct {
	changes []*route53.Change
}

func (r changeSorter) Len() int {
	return len(r.changes)
}

func (r changeSorter) Swap(i, j int) {
	r.changes[i], r.changes[j] = r.changes[j], r.changes[i]
}

func (r changeSorter) Less(i, j int) bool {
	// sort non-aliases first
	if r.changes[i].ResourceRecordSet.AliasTarget == nil {
		return true
	}
	if r.changes[j].ResourceRecordSet.AliasTarget == nil {
		return false
	}
	return *r.changes[i].ResourceRecordSet.Name < *r.changes[j].ResourceRecordSet.Name
}

func groupRecords(records []dns.RR) map[Key][]dns.RR {
	// group records by name+type and optionally identifier
	grouped := map[Key][]dns.RR{}
	for _, record := range records {
		var identifier string
		if aws, ok := record.(*AWSRR); ok {
			identifier = aws.Identifier
		}
		if alias, ok := record.(*dns.PrivateRR); ok {
			// issue #195: alias records need to be keyed by the type of the alias too
			rdata := alias.Data.(*ALIASRdata)
			identifier += "@" + rdata.Type
		}
		key := Key{record.Header().Name, record.Header().Rrtype, identifier}
		grouped[key] = append(grouped[key], record)
	}
	return grouped
}

type importArgs struct {
	name     string
	file     string
	wait     bool
	editauth bool
	replace  bool
	dryrun   bool
}

func rrsetKey(rrset *route53.ResourceRecordSet) string {
	key := fmt.Sprintf("%s %s", *rrset.Type, *rrset.Name)
	if rrset.TTL != nil {
		key += fmt.Sprintf(" %d", *rrset.TTL)
	}
	var rrs []string
	for _, rr := range rrset.ResourceRecords {
		rrs = append(rrs, rr.String())
	}
	if rrset.AliasTarget != nil {
		rrs = append(rrs, rrset.AliasTarget.String())
	}
	sort.Strings(rrs)
	for _, rr := range rrs {
		key += " " + rr
	}
	return key
}

func importBind(args importArgs) {
	zone := lookupZone(args.name)

	var reader io.Reader
	if args.file == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(args.file)
		fatalIfErr(err)
		defer f.Close()
		reader = f
	}

	records := parseBindFile(reader, args.file, *zone.Name)
	expandSelfAliases(records, zone)

	grouped := groupRecords(records)
	existing := map[string]*route53.ResourceRecordSet{}
	if args.replace {
		rrsets, err := ListAllRecordSets(r53, *zone.Id)
		fatalIfErr(err)
		for _, rrset := range rrsets {
			if args.editauth || !isAuthRecord(zone, rrset) {
				rrset.Name = aws.String(unescaper.Replace(*rrset.Name))
				existing[rrsetKey(rrset)] = rrset
			}
		}
	}

	additions := []*route53.Change{}
	for _, values := range grouped {
		rrset := ConvertBindToRRSet(values)
		if rrset != nil && (args.editauth || !isAuthRecord(zone, rrset)) {
			key := rrsetKey(rrset)
			if _, ok := existing[key]; ok {
				// no difference - leave it untouched
				delete(existing, key)
			} else {
				// new record, add
				change := route53.Change{
					Action:            aws.String("CREATE"),
					ResourceRecordSet: rrset,
				}
				additions = append(additions, &change)
			}
		}
	}

	// remaining records in existing should be deleted
	deletions := []*route53.Change{}
	for _, rrset := range existing {
		change := route53.Change{
			Action:            aws.String("DELETE"),
			ResourceRecordSet: rrset,
		}
		deletions = append(deletions, &change)
	}

	if args.dryrun {
		if len(additions)+len(deletions) == 0 {
			fmt.Println("Dry-run, but no changes would have been made.")
		} else {
			fmt.Println("Dry-run, changes that would be made:")
			for _, addition := range additions {
				rrs := ConvertRRSetToBind(addition.ResourceRecordSet)
				for _, rr := range rrs {
					fmt.Printf("+ %s\n", rr.String())
				}
			}
			for _, deletion := range deletions {
				rrs := ConvertRRSetToBind(deletion.ResourceRecordSet)
				for _, rr := range rrs {
					fmt.Printf("- %s\n", rr.String())
				}
			}
		}
	} else {
		resp := batchChanges(additions, deletions, zone)
		fmt.Printf("%d records imported (%d changes / %d additions / %d deletions)\n", len(records), len(additions)+len(deletions), len(additions), len(deletions))

		if args.wait && resp != nil {
			waitForChange(resp.ChangeInfo)
		}
	}
}

func batchChanges(additions, deletions []*route53.Change, zone *route53.HostedZone) *route53.ChangeResourceRecordSetsOutput {
	// sort additions so aliases are last
	sort.Sort(changeSorter{additions})

	changes := append(deletions, additions...)

	var resp *route53.ChangeResourceRecordSetsOutput
	for i := 0; i < len(changes); i += ChangeBatchSize {
		end := i + ChangeBatchSize
		if end > len(changes) {
			end = len(changes)
		}
		batch := route53.ChangeBatch{
			Changes: changes[i:end],
		}
		req := route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zone.Id,
			ChangeBatch:  &batch,
		}
		var err error
		resp, err = r53.ChangeResourceRecordSets(&req)
		fatalIfErr(err)
	}
	return resp
}

func UnexpandSelfAliases(records []dns.RR, zone *route53.HostedZone, full bool) {
	id := strings.Replace(*zone.Id, "/hostedzone/", "", 1)
	for _, rr := range records {
		if awsrr, ok := rr.(*AWSRR); ok {
			rr = awsrr.RR
		}
		if alias, ok := rr.(*dns.PrivateRR); ok {
			rdata := alias.Data.(*ALIASRdata)
			if rdata.ZoneId == id {
				rdata.ZoneId = "$self"
				if !full {
					rdata.Target = shortenName(rdata.Target, *zone.Name)
				}
			}
		}
	}
}

func exportBind(name string, full bool, writer io.Writer) {
	zone := lookupZone(name)
	ExportBindToWriter(r53, zone, full, writer)
}

type exportSorter struct {
	rrsets []*route53.ResourceRecordSet
	zone   string
}

func (r exportSorter) Len() int {
	return len(r.rrsets)
}

func (r exportSorter) Swap(i, j int) {
	r.rrsets[i], r.rrsets[j] = r.rrsets[j], r.rrsets[i]
}

func (r exportSorter) Less(i, j int) bool {
	if *r.rrsets[i].Name == *r.rrsets[j].Name {
		if *r.rrsets[i].Type == "SOA" {
			return true
		}
		return *r.rrsets[i].Type < *r.rrsets[j].Type
	}
	if *r.rrsets[i].Name == r.zone {
		return true
	}
	if *r.rrsets[j].Name == r.zone {
		return false
	}
	return *r.rrsets[i].Name < *r.rrsets[j].Name
}

func ExportBindToWriter(r53 *route53.Route53, zone *route53.HostedZone, full bool, out io.Writer) {
	rrsets, err := ListAllRecordSets(r53, *zone.Id)
	fatalIfErr(err)

	sort.Sort(exportSorter{rrsets, *zone.Name})
	dnsname := *zone.Name
	fmt.Fprintf(out, "$ORIGIN %s\n", dnsname)
	for _, rrset := range rrsets {
		rrs := ConvertRRSetToBind(rrset)
		UnexpandSelfAliases(rrs, zone, full)
		for _, rr := range rrs {
			line := rr.String()
			if !full {
				parts := strings.Split(line, "\t")
				parts[0] = shortenName(parts[0], dnsname)
				if parts[3] == "CNAME" {
					parts[4] = shortenName(parts[4], dnsname)
				}
				line = strings.Join(parts, "\t")
			}
			fmt.Fprintln(out, line)
		}
	}
}

type createArgs struct {
	name            string
	records         []string
	wait            bool
	append          bool
	replace         bool
	identifier      string
	failover        string
	healthCheckId   string
	weight          *int
	region          string
	countryCode     string
	continentCode   string
	subdivisionCode string
	multivalue      bool
}

func (args createArgs) validate() bool {
	if args.failover != "" && args.failover != "PRIMARY" && args.failover != "SECONDARY" {
		fmt.Println("failover must be PRIMARY or SECONDARY")
		return false
	}
	if args.replace && args.append {
		fmt.Println("you can only --append or --replace, not both at the same time")
		return false
	}
	extcount := 0
	if args.failover != "" {
		extcount += 1
	}
	if args.weight != nil {
		extcount += 1
	}
	if args.region != "" {
		extcount += 1
	}
	if args.countryCode != "" {
		extcount += 1
	}
	if args.continentCode != "" {
		extcount += 1
	}
	if args.multivalue {
		extcount += 1
	}
	if args.subdivisionCode != "" && args.countryCode == "" {
		fmt.Println("country-code must be specified if subdivision-code is specified")
		return false
	}
	if extcount > 0 && args.identifier == "" {
		fmt.Println("identifier must be set when creating an extended record")
		return false
	}
	if extcount == 0 && args.identifier != "" {
		fmt.Println("identifier should only be set when creating an extended record")
		return false
	}
	if extcount > 1 {
		fmt.Println("failover, weight, region, country-code and continent-code are mutually exclusive")
		return false
	}
	return true
}

func (args createArgs) applyRRSetParams(rrset *route53.ResourceRecordSet) {
	if args.identifier != "" {
		rrset.SetIdentifier = aws.String(args.identifier)
	}
	if args.failover != "" {
		rrset.Failover = aws.String(args.failover)
	}
	if args.healthCheckId != "" {
		rrset.HealthCheckId = aws.String(args.healthCheckId)
	}
	if args.weight != nil {
		rrset.Weight = aws.Int64(int64(*args.weight))
	}
	if args.region != "" {
		rrset.Region = aws.String(args.region)
	}
	if args.continentCode != "" {
		rrset.GeoLocation = &route53.GeoLocation{
			ContinentCode: aws.String(args.continentCode),
		}
	}
	if args.countryCode != "" {
		rrset.GeoLocation = &route53.GeoLocation{
			CountryCode: aws.String(args.countryCode),
		}
	}
	if args.countryCode != "" && args.subdivisionCode != "" {
		rrset.GeoLocation = &route53.GeoLocation{
			CountryCode:     aws.String(args.countryCode),
			SubdivisionCode: aws.String(args.subdivisionCode),
		}
	}
	if args.multivalue {
		rrset.MultiValueAnswer = aws.Bool(true)
	}
}

func equalStringPtrs(a, b *string) bool {
	if a == nil && b == nil {
		return true
	} else if a != nil && b != nil {
		return *a == *b
	} else {
		return false
	}
}

func equalCaseInsensitiveStringPtrs(a, b *string) bool {
	if a == nil && b == nil {
		return true
	} else if a != nil && b != nil {
		return strings.EqualFold(*a, *b)
	} else {
		return false
	}
}

func parseRecordList(args []string, zone *route53.HostedZone) []dns.RR {
	records := []dns.RR{}
	origin := fmt.Sprintf("$ORIGIN %s\n", *zone.Name)
	for _, text := range args {
		record, err := dns.NewRR(origin + text)
		fatalIfErr(err)
		records = append(records, record)
	}
	return records
}

func createRecords(args createArgs) {
	zone := lookupZone(args.name)
	records := parseRecordList(args.records, zone)
	expandSelfAliases(records, zone)

	grouped := groupRecords(records)

	var existing []*route53.ResourceRecordSet
	if args.replace || args.append {
		var err error
		existing, err = ListAllRecordSets(r53, *zone.Id)
		fatalIfErr(err)
	}

	additions := []*route53.Change{}
	deletions := []*route53.Change{}
	for _, values := range grouped {
		rrset := ConvertBindToRRSet(values)
		args.applyRRSetParams(rrset)

		addChange := &route53.Change{
			Action:            aws.String("CREATE"),
			ResourceRecordSet: rrset,
		}
		additions = append(additions, addChange)

		if args.replace || args.append {
			// add DELETE if there is an existing record
			for _, candidate := range existing {
				if equalCaseInsensitiveStringPtrs(rrset.Name, candidate.Name) &&
					equalStringPtrs(rrset.Type, candidate.Type) &&
					equalStringPtrs(rrset.SetIdentifier, candidate.SetIdentifier) {
					change := route53.Change{
						Action:            aws.String("DELETE"),
						ResourceRecordSet: candidate,
					}
					deletions = append(deletions, &change)

					if args.append {
						addChange.ResourceRecordSet.ResourceRecords = append(addChange.ResourceRecordSet.ResourceRecords, candidate.ResourceRecords...)
					}
					break
				}
			}
		}
	}

	resp := batchChanges(additions, deletions, zone)

	for _, record := range records {
		txt := strings.Replace(record.String(), "\t", " ", -1)
		fmt.Printf("Created record: '%s'\n", txt)
	}

	if args.wait {
		waitForChange(resp.ChangeInfo)
	}
}

func batchListAllRecordSets(r53 *route53.Route53, id string, callback func(rrsets []*route53.ResourceRecordSet)) error {
	req := route53.ListResourceRecordSetsInput{
		HostedZoneId: &id,
	}

	for {
		resp, err := r53.ListResourceRecordSets(&req)
		if err != nil {
			return err
		} else {
			callback(resp.ResourceRecordSets)
			if *resp.IsTruncated {
				req.StartRecordName = resp.NextRecordName
				req.StartRecordType = resp.NextRecordType
				req.StartRecordIdentifier = resp.NextRecordIdentifier
			} else {
				break
			}
		}
	}
	return nil
}

// Paginate request to get all record sets.
func ListAllRecordSets(r53 *route53.Route53, id string) (rrsets []*route53.ResourceRecordSet, err error) {
	err = batchListAllRecordSets(r53, id, func(results []*route53.ResourceRecordSet) {
		rrsets = append(rrsets, results...)
	})

	// unescape wildcards
	for _, rrset := range rrsets {
		rrset.Name = aws.String(unescaper.Replace(*rrset.Name))
	}

	return
}

func deleteRecord(name string, match string, rtype string, wait bool, identifier string) {
	zone := lookupZone(name)
	rrsets, err := ListAllRecordSets(r53, *zone.Id)
	fatalIfErr(err)

	match = qualifyName(match, *zone.Name)
	changes := []*route53.Change{}
	for _, rrset := range rrsets {
		if *rrset.Name == match && *rrset.Type == rtype && (identifier == "" || *rrset.SetIdentifier == identifier) {
			change := &route53.Change{
				Action:            aws.String("DELETE"),
				ResourceRecordSet: rrset,
			}
			changes = append(changes, change)
		}
	}

	if len(changes) > 0 {
		req2 := route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zone.Id,
			ChangeBatch: &route53.ChangeBatch{
				Changes: changes,
			},
		}
		resp, err := r53.ChangeResourceRecordSets(&req2)
		fatalIfErr(err)
		fmt.Printf("%d record sets deleted\n", len(changes))
		if wait {
			waitForChange(resp.ChangeInfo)
		}
	} else {
		fmt.Println("Warning: no records matched - nothing deleted")
	}
}

func purgeRecords(name string, wait bool) {
	zone := lookupZone(name)
	purgeZoneRecords(zone, wait)
}
