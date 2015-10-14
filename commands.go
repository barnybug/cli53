package cli53

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
)

const ChangeBatchSize = 100

func createZone(name, comment string) {
	callerReference := uniqueReference()
	req := route53.CreateHostedZoneInput{
		CallerReference: &callerReference,
		Name:            &name,
		HostedZoneConfig: &route53.HostedZoneConfig{
			Comment: &comment,
		},
	}
	resp, err := r53.CreateHostedZone(&req)
	fatalIfErr(err)
	fmt.Printf("Created zone: '%s' ID: '%s'\n", *resp.HostedZone.Name, *resp.HostedZone.Id)
}

func purgeZoneRecords(id string, wait bool) {
	rrsets, err := ListAllRecordSets(r53, id)
	fatalIfErr(err)

	// delete all non-default SOA/NS records
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
		resp, err := r53.ChangeResourceRecordSets(&req2)
		fatalIfErr(err)
		fmt.Printf("%d record sets deleted\n", len(changes))
		if wait {
			waitForChange(resp.ChangeInfo)
		}
	}
}

func deleteZone(name string, purge bool) {
	zone := lookupZone(name)
	if purge {
		purgeZoneRecords(*zone.Id, false)
	}
	req := route53.DeleteHostedZoneInput{Id: zone.Id}
	_, err := r53.DeleteHostedZone(&req)
	fatalIfErr(err)
	fmt.Printf("Deleted zone: '%s' ID: '%s'\n", *zone.Name, *zone.Id)
}

func listZones() {
	req := route53.ListHostedZonesInput{}
	for {
		// paginated
		resp, err := r53.ListHostedZones(&req)
		fatalIfErr(err)
		for _, zone := range resp.HostedZones {
			fmt.Printf("%+v\n", zone)
		}
		if *resp.IsTruncated {
			req.Marker = resp.NextMarker
		} else {
			break
		}
	}
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
	if alias, ok := record.(*dns.PrivateRR); ok {
		rdata := alias.Data.(*ALIAS)
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

func importBind(name string, file string, wait bool, editauth bool, replace bool) {
	zone := lookupZone(name)
	records := parseBindFile(file, *zone.Name)
	expandSelfAliases(records, zone)

	// group records by name+type and optionally identifier
	grouped := map[Key][]dns.RR{}
	for _, record := range records {
		var identifier string
		if aws, ok := record.(*AWSRR); ok {
			identifier = aws.Identifier
		}
		key := Key{record.Header().Name, record.Header().Rrtype, identifier}
		grouped[key] = append(grouped[key], record)
	}

	existing := map[string]*route53.ResourceRecordSet{}
	if replace {
		rrsets, err := ListAllRecordSets(r53, *zone.Id)
		fatalIfErr(err)
		for _, rrset := range rrsets {
			if editauth || !isAuthRecord(zone, rrset) {
				existing[rrset.String()] = rrset
			}
		}
	}

	additions := []*route53.Change{}
	for _, values := range grouped {
		rrset := ConvertBindToRRSet(values)
		if rrset != nil && (editauth || !isAuthRecord(zone, rrset)) {
			key := rrset.String()
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
	changes := append(deletions, additions...)

	// batch changes
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

	fmt.Printf("%d records imported (%d changes / %d additions / %d deletions)\n", len(records), len(changes), len(additions), len(deletions))
	if wait && resp != nil {
		waitForChange(resp.ChangeInfo)
	}
}

func UnexpandSelfAliases(records []dns.RR, zone *route53.HostedZone) {
	id := strings.Replace(*zone.Id, "/hostedzone/", "", 1)
	for _, rr := range records {
		if alias, ok := rr.(*dns.PrivateRR); ok {
			rdata := alias.Data.(*ALIAS)
			if rdata.ZoneId == id {
				rdata.ZoneId = "$self"
				rdata.Target = shortenName(rdata.Target, *zone.Name)
			}
		}
	}
}

func exportBind(name string, full bool) {
	zone := lookupZone(name)
	rrsets, err := ListAllRecordSets(r53, *zone.Id)
	fatalIfErr(err)

	dnsname := *zone.Name
	fmt.Printf("$ORIGIN %s\n", dnsname)
	for _, rrset := range rrsets {
		rrs := ConvertRRSetToBind(rrset)
		UnexpandSelfAliases(rrs, zone)
		for _, rr := range rrs {
			line := rr.String()
			if !full {
				parts := strings.SplitN(line, "\t", 2)
				line = strings.Join([]string{
					shortenName(parts[0], *zone.Name),
					parts[1],
				}, "\t")
				line = shortenName(line, dnsname)
			}
			fmt.Println(line)
		}
	}
}

type createArgs struct {
	name          string
	record        string
	wait          bool
	identifier    string
	failover      string
	healthCheckId string
	weight        *int
	region        string
	countryCode   string
	continentCode string
}

func createRecord(args createArgs) {
	zone := lookupZone(args.name)

	origin := fmt.Sprintf("$ORIGIN %s\n", *zone.Name)
	rr, err := dns.NewRR(origin + args.record)
	fatalIfErr(err)
	expandSelfAlias(rr, zone)
	rrset := ConvertBindToRRSet([]dns.RR{rr})
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
	if args.countryCode != "" {
		rrset.GeoLocation = &route53.GeoLocation{
			CountryCode: aws.String(args.countryCode),
		}
	}
	if args.continentCode != "" {
		rrset.GeoLocation = &route53.GeoLocation{
			ContinentCode: aws.String(args.continentCode),
		}
	}

	change := &route53.Change{
		Action:            aws.String("CREATE"),
		ResourceRecordSet: rrset,
	}
	changes := []*route53.Change{change}
	req := route53.ChangeResourceRecordSetsInput{
		HostedZoneId: zone.Id,
		ChangeBatch: &route53.ChangeBatch{
			Changes: changes,
		},
	}
	resp, err := r53.ChangeResourceRecordSets(&req)
	fatalIfErr(err)
	txt := strings.Replace(rr.String(), "\t", " ", -1)
	fmt.Printf("Created record: '%s'\n", txt)

	if args.wait {
		waitForChange(resp.ChangeInfo)
	}
}

// Paginate request to get all record sets.
func ListAllRecordSets(r53 *route53.Route53, id string) (rrsets []*route53.ResourceRecordSet, err error) {
	req := route53.ListResourceRecordSetsInput{
		HostedZoneId: &id,
	}

	for {
		var resp *route53.ListResourceRecordSetsOutput
		resp, err = r53.ListResourceRecordSets(&req)
		if err != nil {
			return
		} else {
			rrsets = append(rrsets, resp.ResourceRecordSets...)
			if *resp.IsTruncated {
				req.StartRecordName = resp.NextRecordName
				req.StartRecordType = resp.NextRecordType
				req.StartRecordIdentifier = resp.NextRecordIdentifier
			} else {
				break
			}
		}
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
	purgeZoneRecords(*zone.Id, wait)
}
