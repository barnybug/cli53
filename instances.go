package cli53

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
)

type instancesArgs struct {
	name     string
	off      string
	regions  []string
	wait     bool
	ttl      int
	match    string
	internal bool
	aRecord  bool
	dryRun   bool
}

type InstanceRecord struct {
	name  string
	value string
}

func instances(args instancesArgs, config *aws.Config) {
	zone := lookupZone(args.name)
	fmt.Println("Getting DNS records")

	describeInstancesInput := ec2.DescribeInstancesInput{}
	if args.off == "" {
		filter := ec2.Filter{
			Name:   aws.String("instance-state-name"),
			Values: []*string{aws.String("running")},
		}
		describeInstancesInput.Filters = []*ec2.Filter{&filter}
	}

	var reMatch *regexp.Regexp
	if args.match != "" {
		var err error
		reMatch, err = regexp.Compile(args.match)
		if err != nil {
			fatalIfErr(err)
		}
	}

	insts := map[string]*ec2.Instance{}
	for _, region := range args.regions {
		ec2conn := ec2.New(session.New(), config.WithRegion(region))
		output, err := ec2conn.DescribeInstances(&describeInstancesInput)
		fatalIfErr(err)
		for _, r := range output.Reservations {
			for _, i := range r.Instances {
				for _, tag := range i.Tags {
					// limit to instances with a Name tag
					if *tag.Key == "Name" {
						if reMatch != nil && !reMatch.MatchString(*tag.Value) {
							continue
						}
						insts[*tag.Value] = i
						continue
					}
				}
			}
		}
	}

	if len(insts) == 0 {
		fmt.Println("No instances found")
	}

	var rtype string
	if args.aRecord {
		rtype = "A"
	} else {
		rtype = "CNAME"
	}

	suffix := "." + *zone.Name
	suffix = strings.TrimSuffix(suffix, ".")

	upserts := []*route53.Change{}
	for name, instance := range insts {
		var value *string
		if *instance.State.Name != "running" {
			value = &args.off
		} else if args.aRecord {
			if args.internal {
				value = instance.PublicIpAddress
			} else {
				value = instance.PrivateIpAddress
			}
		} else {
			if args.internal {
				value = aws.String(*instance.PrivateDnsName + ".")
			} else {
				value = aws.String(*instance.PublicDnsName + ".")
			}
		}

		// add domain suffix if missing
		dnsname := name
		if !strings.HasSuffix(dnsname, suffix) {
			dnsname += suffix
		}
		rr := route53.ResourceRecord{
			Value: value,
		}
		rrset := route53.ResourceRecordSet{
			Name:            &dnsname,
			TTL:             aws.Int64(int64(args.ttl)),
			Type:            &rtype,
			ResourceRecords: []*route53.ResourceRecord{&rr},
		}
		change := route53.Change{
			Action:            aws.String("UPSERT"),
			ResourceRecordSet: &rrset,
		}
		upserts = append(upserts, &change)
	}

	if args.dryRun {
		fmt.Println("Dry-run, upserts that would be made:")
		for _, upsert := range upserts {
			rr := upsert.ResourceRecordSet
			fmt.Printf("+ %s %s %v\n", *rr.Name, *rr.Type, *rr.ResourceRecords[0].Value)
		}
	} else {
		resp := batchChanges(upserts, []*route53.Change{}, zone)
		fmt.Printf("%d records upserted\n", len(upserts))

		if args.wait && resp != nil {
			waitForChange(resp.ChangeInfo)
		}
	}
}
