package cli53

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
)

func parseComment(rr dns.RR, comment string) dns.RR {
	if strings.HasPrefix(comment, "; AWS ") {
		kvs, err := ParseKeyValues(comment[6:])
		if err == nil {
			routing := kvs.GetString("routing")
			if fn, ok := RoutingTypes[routing]; ok {
				route := fn()
				route.Parse(kvs)
				rr = &AWSRR{
					rr,
					route,
					kvs.GetOptString("healthCheckId"),
					kvs.GetString("identifier"),
				}
			} else {
				fmt.Printf("Warning: parse AWS extension - routing=\"%s\" not understood\n", routing)
			}
		} else {
			fmt.Println("Warning: parse AWS extension: %s", err)
		}
	}
	return rr
}

func parseBindFile(file, origin string) []dns.RR {
	rdr, err := os.Open(file)
	fatalIfErr(err)
	tokensch := dns.ParseZone(rdr, origin, file)
	records := []dns.RR{}
	for token := range tokensch {
		if token.Error != nil {
			fatalIfErr(token.Error)
		}
		record := parseComment(token.RR, token.Comment)
		records = append(records, record)
	}
	return records
}

// Convert a DNS record into a route53 ResourceRecord.
func ConvertBindToRR(record dns.RR) []*route53.ResourceRecord {
	switch record := record.(type) {
	case *dns.A:
		rr := &route53.ResourceRecord{
			Value: aws.String(record.A.String()),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.AAAA:
		rr := &route53.ResourceRecord{
			Value: aws.String(record.AAAA.String()),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.CNAME:
		rr := &route53.ResourceRecord{
			Value: aws.String(record.Target),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.MX:
		value := fmt.Sprintf("%d %s", record.Preference, record.Mx)
		rr := &route53.ResourceRecord{
			Value: aws.String(value),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.NS:
		rr := &route53.ResourceRecord{
			Value: aws.String(record.Ns),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.PTR:
		rr := &route53.ResourceRecord{
			Value: aws.String(record.Ptr),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.SOA:
		value := fmt.Sprintf("%s %s %d %d %d %d %d", record.Ns, record.Mbox, record.Serial, record.Refresh, record.Retry, record.Expire, record.Minttl)
		rr := &route53.ResourceRecord{
			Value: aws.String(value),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.SPF:
		rrs := []*route53.ResourceRecord{}
		for _, txt := range record.Txt {
			rr := &route53.ResourceRecord{
				Value: aws.String(txt),
			}
			rrs = append(rrs, rr)
		}
		return rrs
	case *dns.SRV:
		value := fmt.Sprintf("%d %d %d %s", record.Priority, record.Weight, record.Port, record.Target)
		rr := &route53.ResourceRecord{
			Value: aws.String(value),
		}
		return []*route53.ResourceRecord{rr}
	case *dns.TXT:
		rrs := []*route53.ResourceRecord{}
		for _, txt := range record.Txt {
			rr := &route53.ResourceRecord{
				Value: aws.String(`"` + txt + `"`),
			}
			rrs = append(rrs, rr)
		}
		return rrs
	default:
		errorAndExit(fmt.Sprintf("Unsupported resource record: %s", record))
	}
	return []*route53.ResourceRecord{}
}

func ConvertAliasToRRSet(alias *dns.PrivateRR) *route53.ResourceRecordSet {
	// AWS ALIAS extension record
	hdr := alias.Header()
	rdata := alias.Data.(*ALIAS)
	return &route53.ResourceRecordSet{
		Type: aws.String(rdata.Type),
		Name: aws.String(hdr.Name),
		AliasTarget: &route53.AliasTarget{
			DNSName:              aws.String(rdata.Target),
			HostedZoneId:         aws.String(rdata.ZoneId),
			EvaluateTargetHealth: aws.Bool(rdata.EvaluateTargetHealth),
		},
	}
}

// Convert some DNS records into a route53 ResourceRecordSet. The records should have been
// previously grouped by matching name, type and (if applicable) identifier.
func ConvertBindToRRSet(records []dns.RR) *route53.ResourceRecordSet {
	rrs := []*route53.ResourceRecord{}
	for _, record := range records {
		if rr, ok := record.(*dns.PrivateRR); ok {
			return ConvertAliasToRRSet(rr)
		} else if rec, ok := record.(*AWSRR); ok {
			rrs = append(rrs, ConvertBindToRR(rec.RR)...)
		} else {
			rrs = append(rrs, ConvertBindToRR(record)...)
		}
	}

	if len(rrs) > 0 {
		hdr := records[0].Header()
		rrset := &route53.ResourceRecordSet{
			Type:            aws.String(dns.Type(hdr.Rrtype).String()),
			Name:            aws.String(hdr.Name),
			ResourceRecords: rrs,
			TTL:             aws.Int64(int64(hdr.Ttl)),
		}
		if awsrr, ok := records[0].(*AWSRR); ok {
			switch route := awsrr.Route.(type) {
			case *FailoverRoute:
				rrset.Failover = aws.String(route.Failover)
			case *GeoLocationRoute:
				rrset.GeoLocation = &route53.GeoLocation{
					CountryCode:     route.CountryCode,
					ContinentCode:   route.ContinentCode,
					SubdivisionCode: route.SubdivisionCode,
				}
			case *LatencyRoute:
				rrset.Region = aws.String(route.Region)
			case *WeightedRoute:
				rrset.Weight = aws.Int64(route.Weight)
			}
			if awsrr.HealthCheckId != nil {
				rrset.HealthCheckId = awsrr.HealthCheckId
			}
			rrset.SetIdentifier = aws.String(awsrr.Identifier)
		}
		return rrset
	}
	return nil
}

func ConvertRRSetToBind(rrset *route53.ResourceRecordSet) []dns.RR {
	ret := []dns.RR{}

	// A record either has resource records or is an alias.
	// Optionally a routing policy can apply which will can be:
	// - failover
	// - geolocation
	// - latency
	// - weighted

	name := unescaper.Replace(*rrset.Name)

	// Only resource records without routing can be represented in vanilla bind.
	if rrset.AliasTarget != nil {
		alias := rrset.AliasTarget
		dnsrr := &dns.PrivateRR{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: TypeALIAS,
				Class:  ClassAWS,
				Ttl:    86400,
			},
			Data: &ALIAS{
				*rrset.Type,
				*alias.DNSName,
				*alias.HostedZoneId,
				*alias.EvaluateTargetHealth,
			},
		}
		ret = append(ret, dnsrr)
	} else {
		switch *rrset.Type {
		case "A":
			for _, rr := range rrset.ResourceRecords {
				dnsrr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					A: net.ParseIP(*rr.Value),
				}
				ret = append(ret, dnsrr)
			}
		case "AAAA":
			for _, rr := range rrset.ResourceRecords {
				dnsrr := &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					AAAA: net.ParseIP(*rr.Value),
				}
				ret = append(ret, dnsrr)
			}
		case "CNAME":
			for _, rr := range rrset.ResourceRecords {
				dnsrr := &dns.CNAME{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeCNAME,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Target: *rr.Value,
				}
				ret = append(ret, dnsrr)
			}
		case "MX":
			// parse value
			for _, rr := range rrset.ResourceRecords {
				var preference uint16
				var value string
				fmt.Sscanf(*rr.Value, "%d %s", &preference, &value)

				dnsrr := &dns.MX{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeMX,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Mx:         value,
					Preference: preference,
				}
				ret = append(ret, dnsrr)
			}
		case "NS":
			for _, rr := range rrset.ResourceRecords {
				dnsrr := &dns.NS{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeNS,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Ns: *rr.Value,
				}
				ret = append(ret, dnsrr)
			}
		case "PTR":
			for _, rr := range rrset.ResourceRecords {
				dnsrr := &dns.PTR{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypePTR,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Ptr: *rr.Value,
				}
				ret = append(ret, dnsrr)
			}
		case "SOA":
			for _, rr := range rrset.ResourceRecords {
				// parse value
				var ns, mbox string
				var serial, refresh, retry, expire, minttl uint32
				fmt.Sscanf(*rr.Value, "%s %s %d %d %d %d %d", &ns, &mbox, &serial, &refresh, &retry, &expire, &minttl)

				dnsrr := &dns.SOA{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeSOA,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Ns:      ns,
					Mbox:    mbox,
					Serial:  serial,
					Refresh: refresh,
					Retry:   retry,
					Expire:  expire,
					Minttl:  minttl,
				}
				ret = append(ret, dnsrr)
			}
		case "SPF":
			for _, rr := range rrset.ResourceRecords {
				dnsrr := &dns.SPF{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeSPF,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Txt: []string{*rr.Value},
				}
				ret = append(ret, dnsrr)
			}
		case "SRV":
			for _, rr := range rrset.ResourceRecords {
				// parse value
				var priority, weight, port uint16
				var target string
				fmt.Sscanf(*rr.Value, "%d %d %d %s", &priority, &weight, &port, &target)

				dnsrr := &dns.SRV{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeSRV,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Priority: priority,
					Weight:   weight,
					Port:     port,
					Target:   target,
				}
				ret = append(ret, dnsrr)
			}
		case "TXT":
			// TXT records are unusual in that multiple values are stored as a single bind record.
			txt := []string{}
			for _, rr := range rrset.ResourceRecords {
				txt = append(txt, unquote(*rr.Value))
			}
			dnsrr := &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    uint32(*rrset.TTL),
				},
				Txt: txt,
			}
			ret = append(ret, dnsrr)
		}
	}

	var route AWSRoute
	if rrset.Failover != nil {
		route = &FailoverRoute{*rrset.Failover}
	} else if rrset.Weight != nil {
		route = &WeightedRoute{*rrset.Weight}
	} else if rrset.Region != nil {
		route = &LatencyRoute{*rrset.Region}
	} else if rrset.GeoLocation != nil {
		route = &GeoLocationRoute{rrset.GeoLocation.CountryCode, rrset.GeoLocation.ContinentCode, rrset.GeoLocation.SubdivisionCode}
	}
	if route != nil {
		for i, rr := range ret {
			// convert any records with AWS extensions into an AWSRR record
			awsrr := &AWSRR{rr, route, rrset.HealthCheckId, *rrset.SetIdentifier}
			ret[i] = awsrr
		}
	}

	return ret
}
