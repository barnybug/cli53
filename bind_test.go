package cli53

import (
	"net"
	"testing"
)

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

var commonA = &dns.A{
	Hdr: dns.RR_Header{
		Name:   "a.",
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    uint32(300),
	},
	A: net.ParseIP("127.0.0.1"),
}

var testConvertRRSetToBindTable = []struct {
	Input  route53.ResourceRecordSet
	Output []dns.RR
}{
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				A: net.ParseIP("127.0.0.1"),
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("AAAA"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				AAAA: net.ParseIP("127.0.0.1"),
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("CNAME"),
			Name: aws.String("test.example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("www.example.com."),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "test.example.com.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				Target: "www.example.com.",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("MX"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("5 mail.example.com."),
				},
			},
			TTL: aws.Int64(3600),
		},
		Output: []dns.RR{
			&dns.MX{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeMX,
					Class:  dns.ClassINET,
					Ttl:    uint32(3600),
				},
				Preference: 5,
				Mx:         "mail.example.com.",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("NS"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("ns1.example.com."),
				},
			},
			TTL: aws.Int64(3600),
		},
		Output: []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    uint32(3600),
				},
				Ns: "ns1.example.com.",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("PTR"),
			Name: aws.String("98."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("foo.example.com."),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.PTR{
				Hdr: dns.RR_Header{
					Name:   "98.",
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				Ptr: "foo.example.com.",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("SOA"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("ns-2018.awsdns-60.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"),
				},
			},
			TTL: aws.Int64(900),
		},
		Output: []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    uint32(900),
				},
				Ns:      "ns-2018.awsdns-60.co.uk.",
				Mbox:    "awsdns-hostmaster.amazon.com.",
				Serial:  1,
				Refresh: 7200,
				Retry:   900,
				Expire:  1209600,
				Minttl:  86400,
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("SPF"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("\"~all\""),
				},
			},
			TTL: aws.Int64(900),
		},
		Output: []dns.RR{
			&dns.SPF{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSPF,
					Class:  dns.ClassINET,
					Ttl:    uint32(900),
				},
				Txt: []string{"~all"},
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("SRV"),
			Name: aws.String("_sip._tcp.example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("0 5 5060 sipserver.example.com."),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "_sip._tcp.example.com.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				Priority: 0,
				Weight:   5,
				Port:     5060,
				Target:   "sipserver.example.com.",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("TXT"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("\"hello\""),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				Txt: []string{"hello"},
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("CAA"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("0 issue \"example.net\""),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.CAA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeCAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				Flag:  0,
				Tag:   "issue",
				Value: "example.net",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("NAPTR"),
			Name: aws.String("example.com."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String(`100 10 "u" "sip+E2U" "!^.*$!sip:information@foo.se!i" .`),
				},
			},
			TTL: aws.Int64(86400),
		},
		Output: []dns.RR{
			&dns.NAPTR{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNAPTR,
					Class:  dns.ClassINET,
					Ttl:    uint32(86400),
				},
				Order:       100,
				Preference:  10,
				Flags:       "u",
				Service:     "sip+E2U",
				Regexp:      "!^.*$!sip:information@foo.se!i",
				Replacement: ".",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("example.com."),
			AliasTarget: &route53.AliasTarget{
				DNSName:              aws.String("target"),
				HostedZoneId:         aws.String("zoneid"),
				EvaluateTargetHealth: aws.Bool(false),
			},
		},
		Output: []dns.RR{
			&dns.PrivateRR{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: TypeALIAS,
					Class:  ClassAWS,
					Ttl:    uint32(86400),
				},
				Data: &ALIASRdata{
					Type:                 "A",
					Target:               "target",
					ZoneId:               "zoneid",
					EvaluateTargetHealth: false,
				},
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("a."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			Failover:      aws.String("PRIMARY"),
			HealthCheckId: aws.String("6bb57c41-879a-42d0-acdd-ed6472f08eb9"),
			SetIdentifier: aws.String("failover-Primary"),
			TTL:           aws.Int64(300),
		},
		Output: []dns.RR{
			&AWSRR{
				commonA,
				&FailoverRoute{"PRIMARY"},
				aws.String("6bb57c41-879a-42d0-acdd-ed6472f08eb9"),
				"failover-Primary",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("a."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			GeoLocation: &route53.GeoLocation{
				ContinentCode: aws.String("AF"),
			},
			SetIdentifier: aws.String("Africa"),
			TTL:           aws.Int64(300),
		},
		Output: []dns.RR{
			&AWSRR{
				commonA,
				&GeoLocationRoute{ContinentCode: aws.String("AF")},
				nil,
				"Africa",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("a."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			Region:        aws.String("us-west-1"),
			SetIdentifier: aws.String("USWest1"),
			TTL:           aws.Int64(300),
		},
		Output: []dns.RR{
			&AWSRR{
				commonA,
				&LatencyRoute{Region: "us-west-1"},
				nil,
				"USWest1",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("a."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			Weight:        aws.Int64(1),
			SetIdentifier: aws.String("One"),
			TTL:           aws.Int64(300),
		},
		Output: []dns.RR{
			&AWSRR{
				commonA,
				&WeightedRoute{Weight: 1},
				nil,
				"One",
			},
		},
	},
	{
		Input: route53.ResourceRecordSet{
			Type: aws.String("A"),
			Name: aws.String("a."),
			ResourceRecords: []*route53.ResourceRecord{
				&route53.ResourceRecord{
					Value: aws.String("127.0.0.1"),
				},
			},
			MultiValueAnswer: aws.Bool(true),
			SetIdentifier:    aws.String("One"),
			TTL:              aws.Int64(300),
		},
		Output: []dns.RR{
			&AWSRR{
				commonA,
				&MultiValueAnswerRoute{},
				nil,
				"One",
			},
		},
	},
}

func TestConvertRRSetToBind(t *testing.T) {
	for _, test := range testConvertRRSetToBindTable {
		result := ConvertRRSetToBind(&test.Input)
		if !assert.NotNil(t, result, "Record: %s", test.Output) {
			continue
		}
		if !assert.Equal(t, len(test.Output), len(result)) {
			continue
		}
		for n := range test.Output {
			expected := test.Output[n].String()
			if assert.NotNil(t, result[n]) {
				actual := result[n].String()
				assert.Equal(t, expected, actual)
			}
		}
	}
}

func TestConvertBindToRRSet(t *testing.T) {
	for _, test := range testConvertRRSetToBindTable {
		result := ConvertBindToRRSet(test.Output)
		if !assert.NotNil(t, result, "Record %s", test.Output) {
			continue
		}
		actual := result.String()
		expected := test.Input.String()
		if actual != expected {
			t.Errorf("Expected record %s, got %s", expected, actual)
		}
	}
}

func mustParseRR(s string) dns.RR {
	rr, err := dns.NewRR(s)
	if err != nil {
		panic(err)
	}
	return rr
}

var testParseCommentTable = []struct {
	Record  dns.RR
	Comment string
	Output  string
}{
	{
		Record:  mustParseRR("test 3600 IN A 127.0.0.1"),
		Comment: "",
		Output: "test.	3600	IN	A	127.0.0.1",
	},
	// {
	// 	Record:  mustParseRR("test 3600 IN A 127.0.0.1"),
	// 	Comment: `AWS routing="GEOLOCATION" countryCode="GB" identifier="UK"`,
	// 	Output: `test.	3600	IN	A	127.0.0.1 ; AWS routing="GEOLOCATION" countryCode="GB" identifier="UK"`,
	// },
}

func TestParseComment(t *testing.T) {
	for _, test := range testParseCommentTable {
		result := parseComment(test.Record, test.Comment)
		assert.Equal(t, test.Output, result.String())
	}
}
