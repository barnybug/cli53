package cli53

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
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

func instances(ctx context.Context, args instancesArgs, config aws.Config) {
	zone := lookupZone(ctx, args.name)
	fmt.Println("Getting DNS records")

	describeInstancesInput := ec2.DescribeInstancesInput{}
	if args.off == "" {
		filter := ec2types.Filter{
			Name:   aws.String("instance-state-name"),
			Values: []string{"running"},
		}
		describeInstancesInput.Filters = []ec2types.Filter{filter}
	}

	var reMatch *regexp.Regexp
	if args.match != "" {
		var err error
		reMatch, err = regexp.Compile(args.match)
		if err != nil {
			fatalIfErr(err)
		}
	}

	insts := map[string]*ec2types.Instance{}
	for _, region := range args.regions {
		ec2conn := ec2.NewFromConfig(config, func(o *ec2.Options) {
			o.Region = region
		})
		paginator := ec2.NewDescribeInstancesPaginator(ec2conn, &describeInstancesInput)
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			fatalIfErr(err)
			for _, r := range output.Reservations {
				for _, i := range r.Instances {
					for _, tag := range i.Tags {
						// limit to instances with a Name tag
						if *tag.Key == "Name" {
							if reMatch != nil && !reMatch.MatchString(*tag.Value) {
								continue
							}
							instance := i
							insts[*tag.Value] = &instance
							continue
						}
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

	upserts := []route53types.Change{}
	for name, instance := range insts {
		var value *string
		if instance.State == nil || instance.State.Name != ec2types.InstanceStateNameRunning {
			value = &args.off
		} else if args.aRecord {
			if args.internal {
				value = instance.PrivateIpAddress
			} else {
				value = instance.PublicIpAddress
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
		rr := route53types.ResourceRecord{
			Value: value,
		}
		rrset := route53types.ResourceRecordSet{
			Name:            &dnsname,
			TTL:             aws.Int64(int64(args.ttl)),
			Type:            route53types.RRType(rtype),
			ResourceRecords: []route53types.ResourceRecord{rr},
		}
		change := route53types.Change{
			Action:            route53types.ChangeActionUpsert,
			ResourceRecordSet: &rrset,
		}
		upserts = append(upserts, change)
	}

	if args.dryRun {
		fmt.Println("Dry-run, upserts that would be made:")
		for _, upsert := range upserts {
			rr := upsert.ResourceRecordSet
			fmt.Printf("+ %s %s %v\n", *rr.Name, rr.Type, *rr.ResourceRecords[0].Value)
		}
	} else {
		resp := batchChanges(ctx, upserts, []route53types.Change{}, zone)
		fmt.Printf("%d records upserted\n", len(upserts))

		if args.wait && resp != nil {
			waitForChange(ctx, resp.ChangeInfo)
		}
	}
}
