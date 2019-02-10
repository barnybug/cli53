package cli53

import (
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
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
			fmt.Printf("Warning: parse AWS extension: %s", err)
		}
	}
	return rr
}

func parseBindFile(reader io.Reader, filename, origin string) []dns.RR {
	tokensch := dns.ParseZone(reader, origin, filename)
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

func quoteValues(vals []string) string {
	var qvals []string
	for _, val := range vals {
		qvals = append(qvals, `"`+val+`"`)
	}
	return strings.Join(qvals, " ")
}

// ConvertBindToRR will convert a DNS record into a route53 ResourceRecord.
func ConvertBindToRR(record dns.RR) *route53.ResourceRecord {
	switch record := record.(type) {
	case *dns.A:
		return &route53.ResourceRecord{
			Value: aws.String(record.A.String()),
		}
	case *dns.AAAA:
		return &route53.ResourceRecord{
			Value: aws.String(record.AAAA.String()),
		}
	case *dns.CNAME:
		return &route53.ResourceRecord{
			Value: aws.String(record.Target),
		}
	case *dns.MX:
		value := fmt.Sprintf("%d %s", record.Preference, record.Mx)
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	case *dns.NAPTR:
		var value string
		if record.Replacement == "." {
			value = fmt.Sprintf("%d %d \"%s\" \"%s\" \"%s\" .", record.Order, record.Preference, record.Flags, record.Service, record.Regexp)
		} else {
			value = fmt.Sprintf("%d %d \"%s\" \"%s\" \"\" \"%s\"", record.Order, record.Preference, record.Flags, record.Service, record.Replacement)
		}
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	case *dns.NS:
		return &route53.ResourceRecord{
			Value: aws.String(record.Ns),
		}
	case *dns.PTR:
		return &route53.ResourceRecord{
			Value: aws.String(record.Ptr),
		}
	case *dns.SOA:
		value := fmt.Sprintf("%s %s %d %d %d %d %d", record.Ns, record.Mbox, record.Serial, record.Refresh, record.Retry, record.Expire, record.Minttl)
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	case *dns.SPF:
		value := quoteValues(record.Txt)
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	case *dns.SRV:
		value := fmt.Sprintf("%d %d %d %s", record.Priority, record.Weight, record.Port, record.Target)
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	case *dns.TXT:
		value := quoteValues(record.Txt)
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	case *dns.CAA:
		value := fmt.Sprintf("%d %s \"%s\"", record.Flag, record.Tag, record.Value)
		return &route53.ResourceRecord{
			Value: aws.String(value),
		}
	default:
		errorAndExit(fmt.Sprintf("Unsupported resource record: %s", record))
	}
	return nil
}

// ConvertAliasToRRSet will convert an alias to a ResourceRecordSet.
func ConvertAliasToRRSet(alias *dns.PrivateRR) *route53.ResourceRecordSet {
	// AWS ALIAS extension record
	hdr := alias.Header()
	rdata := alias.Data.(*ALIASRdata)
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

// ConvertBindToRRSet will convert some DNS records into a route53
// ResourceRecordSet. The records should have been previously grouped
// by matching name, type and (if applicable) identifier.
func ConvertBindToRRSet(records []dns.RR) *route53.ResourceRecordSet {
	if len(records) == 0 {
		return nil
	}
	hdr := records[0].Header()
	name := strings.ToLower(hdr.Name)
	rrset := &route53.ResourceRecordSet{
		Type: aws.String(dns.TypeToString[hdr.Rrtype]),
		Name: aws.String(name),
		TTL:  aws.Int64(int64(hdr.Ttl)),
	}

	for _, record := range records {
		if awsrr, ok := record.(*AWSRR); ok {
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
			case *MultiValueAnswerRoute:
				rrset.MultiValueAnswer = aws.Bool(true)
			}
			if awsrr.HealthCheckId != nil {
				rrset.HealthCheckId = awsrr.HealthCheckId
			}
			rrset.SetIdentifier = aws.String(awsrr.Identifier)
			record = awsrr.RR
		}

		if rr, ok := record.(*dns.PrivateRR); ok {
			// 'AWS ALIAS' records do not have ResourceRecords
			rdata := rr.Data.(*ALIASRdata)
			rrset.Type = aws.String(rdata.Type)
			rrset.AliasTarget = &route53.AliasTarget{
				DNSName:              aws.String(rdata.Target),
				HostedZoneId:         aws.String(rdata.ZoneId),
				EvaluateTargetHealth: aws.Bool(rdata.EvaluateTargetHealth),
			}
			rrset.TTL = nil
		} else {
			rr := ConvertBindToRR(record)
			rrset.ResourceRecords = append(rrset.ResourceRecords, rr)
		}
	}

	return rrset
}

func absolute(name string) string {
	// route53 always treats target names as absolute, even when they are
	// missing the ending period.
	if !strings.HasSuffix(name, ".") {
		return name + "."
	}
	return name
}

var reNaptr = regexp.MustCompile(`^([[:digit:]]+) ([[:digit:]]+) "([^"]*)" "([^"]*)" "([^"]*)" "?([^"]+)"?$`)

// ConvertRRSetToBind will convert a ResourceRecordSet to an array of RR entries
func ConvertRRSetToBind(rrset *route53.ResourceRecordSet) []dns.RR {
	ret := []dns.RR{}

	// A record either has resource records or is an alias.
	// Optionally a routing policy can apply which will can be:
	// - failover
	// - geolocation
	// - latency
	// - weighted

	name := *rrset.Name

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
			Data: &ALIASRdata{
				*rrset.Type,
				*alias.DNSName,
				*alias.HostedZoneId,
				*alias.EvaluateTargetHealth,
			},
		}
		ret = append(ret, dnsrr)
	} else if rrset.TrafficPolicyInstanceId != nil {
		// Warn and skip traffic policy records
		fmt.Fprintf(os.Stderr, "Warning: Skipping traffic policy record %s\n", name)
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
					Target: absolute(*rr.Value),
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
					Mx:         absolute(value),
					Preference: preference,
				}
				ret = append(ret, dnsrr)
			}
		case "NAPTR":
			for _, rr := range rrset.ResourceRecords {
				// parse value
				naptr := reNaptr.FindStringSubmatch(*rr.Value)
				order, _ := strconv.Atoi(naptr[1])
				preference, _ := strconv.Atoi(naptr[2])

				dnsrr := &dns.NAPTR{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeNAPTR,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Order:       uint16(order),
					Preference:  uint16(preference),
					Flags:       naptr[3],
					Service:     naptr[4],
					Regexp:      naptr[5],
					Replacement: naptr[6],
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
				txt := splitValues(*rr.Value)
				dnsrr := &dns.SPF{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeSPF,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Txt: txt,
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
					Target:   absolute(target),
				}
				ret = append(ret, dnsrr)
			}
		case "TXT":
			for _, rr := range rrset.ResourceRecords {
				txt := splitValues(*rr.Value)
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
		case "CAA":
			for _, rr := range rrset.ResourceRecords {
				var flag uint8
				var tag string
				var quotedValue string
				fmt.Sscanf(*rr.Value, "%d %s %s", &flag, &tag, &quotedValue)

				dnsrr := &dns.CAA{
					Hdr: dns.RR_Header{
						Name:   name,
						Rrtype: dns.TypeCAA,
						Class:  dns.ClassINET,
						Ttl:    uint32(*rrset.TTL),
					},
					Flag:  flag,
					Tag:   tag,
					Value: strings.Trim(quotedValue, `"`),
				}
				ret = append(ret, dnsrr)
			}
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
	} else if rrset.MultiValueAnswer != nil && *rrset.MultiValueAnswer {
		route = &MultiValueAnswerRoute{}
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
