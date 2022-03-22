package cli53

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/urfave/cli/v2"
)

var r53 *route53.Route53
var version = "master"

// Main entry point for cli53 application
func Main(args []string) int {
	commonFlags := []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "enable debug logging",
		},
		&cli.StringFlag{
			Name:  "profile",
			Usage: "profile to use from credentials file",
		},
		&cli.StringFlag{
			Name:  "role-arn",
			Usage: "AWS role ARN to assume",
		},
		&cli.StringFlag{
			Name:  "endpoint-url",
			Usage: "override Route53 endpoint (hostname or fully qualified URI)",
		},
	}

	app := cli.NewApp()
	app.Name = "cli53"
	app.Usage = "manage route53 DNS"
	app.Version = version
	app.Commands = []*cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list domains",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:    "format",
					Aliases: []string{"f"},
					Value:   "table",
					Usage:   "output format: text, json, jl, table, csv",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 0 {
					cli.ShowCommandHelp(c, "list")
					return cli.NewExitError("No parameters expected", 1)
				}

				formatter := getFormatter(c)
				if formatter == nil {
					return cli.NewExitError("Unknown format", 1)
				}
				listZones(formatter)
				return nil
			},
		},
		{
			Name:      "create",
			Usage:     "create a domain",
			ArgsUsage: "domain.name",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:  "comment",
					Value: "",
					Usage: "comment on the domain",
				},
				&cli.StringFlag{
					Name:  "vpc-id",
					Value: "",
					Usage: "create a private zone in the VPC",
				},
				&cli.StringFlag{
					Name:  "vpc-region",
					Value: "",
					Usage: "VPC region (required if vpcId is specified)",
				},
				&cli.StringFlag{
					Name:  "delegation-set-id",
					Value: "",
					Usage: "use the given delegation set",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "create")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				createZone(c.Args().First(), c.String("comment"), c.String("vpc-id"), c.String("vpc-region"), c.String("delegation-set-id"))
				return nil
			},
		},
		{
			Name:      "delete",
			Usage:     "delete a domain",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				&cli.BoolFlag{
					Name:  "purge",
					Usage: "remove any existing records on the domain (otherwise deletion will fail)",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "delete")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				domain := c.Args().First()
				deleteZone(domain, c.Bool("purge"))
				return nil
			},
		},
		{
			Name:      "validate",
			Usage:     "validate a bind zone file syntax",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:  "file",
					Value: "",
					Usage: "bind zone filename, or - for stdin (required)",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 0 {
					cli.ShowCommandHelp(c, "validate")
					return cli.NewExitError("No parameters expected", 1)
				}
				args := importArgs{
					name: c.Args().First(),
					file: c.String("file"),
				}
				validateBindFile(args)
				return nil
			},
		},
		{
			Name:      "import",
			Usage:     "import a bind zone file",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:  "file",
					Value: "",
					Usage: "bind zone filename, or - for stdin (required)",
				},
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				&cli.BoolFlag{
					Name:  "editauth",
					Usage: "include SOA and NS records from zone file",
				},
				&cli.BoolFlag{
					Name:  "replace",
					Usage: "replace all existing records",
				},
				&cli.BoolFlag{
					Name:  "upsert",
					Usage: "update or replace records, do not delete existing",
				},
				&cli.BoolFlag{
					Name:    "dry-run",
					Aliases: []string{"n"},
					Usage:   "perform a trial run with no changes made",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "import")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				args := importArgs{
					name:     c.Args().First(),
					file:     c.String("file"),
					wait:     c.Bool("wait"),
					editauth: c.Bool("editauth"),
					replace:  c.Bool("replace"),
					upsert:   c.Bool("upsert"),
					dryrun:   c.Bool("dry-run"),
				}
				importBind(args)
				return nil
			},
		},
		{
			Name:      "instances",
			Usage:     "dynamically update your dns with EC2 instance names",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:  "off",
					Value: "",
					Usage: "if provided, then records for stopped instances will be updated. This option gives the dns name the CNAME should revert to",
				},
				&cli.StringSliceFlag{
					Name:    "region",
					EnvVars: []string{"AWS_REGION"},
					Usage:   "a list of regions to check",
				},
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				&cli.IntFlag{
					Name:    "ttl",
					Aliases: []string{"x"},
					Value:   60,
					Usage:   "resource record ttl",
				},
				&cli.StringFlag{
					Name:  "match",
					Value: "",
					Usage: "regular expression to select which Name tags to use",
				},
				&cli.BoolFlag{
					Name:    "internal",
					Aliases: []string{"i"},
					Usage:   "always use the internal hostname",
				},
				&cli.BoolFlag{
					Name:    "a-record",
					Aliases: []string{"a"},
					Usage:   "write an A record (IP) instead of CNAME",
				},
				&cli.BoolFlag{
					Name:    "dry-run",
					Aliases: []string{"n"},
					Usage:   "dry run - don't make any changes",
				},
			),
			Action: func(c *cli.Context) (err error) {
				config, err := getConfig(c)
				if err != nil {
					return err
				}
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "instances")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				args := instancesArgs{
					name:     c.Args().First(),
					off:      c.String("off"),
					regions:  c.StringSlice("region"),
					wait:     c.Bool("wait"),
					ttl:      c.Int("ttl"),
					match:    c.String("match"),
					internal: c.Bool("internal"),
					aRecord:  c.Bool("a-record"),
					dryRun:   c.Bool("dry-run"),
				}
				instances(args, config)
				return nil
			},
		},
		{
			Name:      "export",
			Usage:     "export a bind zone file (to stdout)",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				&cli.BoolFlag{
					Name:    "full",
					Aliases: []string{"f"},
					Usage:   "export prefixes as full names",
				},
				&cli.StringFlag{
					Name:  "output",
					Usage: "Write to an output file instead of STDOUT",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "export")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}

				outputFileName := c.String("output")
				writer := os.Stdout
				if len(outputFileName) > 0 {
					writer, err = os.Create(outputFileName)
					defer writer.Close()
					if err != nil {
						return err
					}
				}
				exportBind(c.Args().First(), c.Bool("full"), writer)
				return nil
			},
		},
		{
			Name:      "rrcreate",
			Aliases:   []string{"rc"},
			Usage:     "create one or more records",
			ArgsUsage: "zone record [record...]",
			Flags: append(commonFlags,
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				&cli.BoolFlag{
					Name:  "append",
					Usage: "append the record",
				},
				&cli.BoolFlag{
					Name:  "replace",
					Usage: "replace the record",
				},
				&cli.StringFlag{
					Name:    "identifier",
					Aliases: []string{"i"},
					Usage:   "record set identifier (for routed records)",
				},
				&cli.StringFlag{
					Name:  "failover",
					Usage: "PRIMARY or SECONDARY on a failover routing",
				},
				&cli.StringFlag{
					Name:  "health-check",
					Usage: "associated health check id for failover PRIMARY",
				},
				&cli.IntFlag{
					Name:  "weight",
					Usage: "weight on a weighted routing",
				},
				&cli.StringFlag{
					Name:  "region",
					Usage: "region for latency-based routing",
				},
				&cli.StringFlag{
					Name:  "country-code",
					Usage: "country code for geolocation routing",
				},
				&cli.StringFlag{
					Name:  "continent-code",
					Usage: "continent code for geolocation routing",
				},
				&cli.StringFlag{
					Name:  "subdivision-code",
					Usage: "subdivision code for geolocation routing",
				},
				&cli.BoolFlag{
					Name:  "multivalue",
					Usage: "use multivalue answer routing",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() < 2 {
					cli.ShowCommandHelp(c, "rrcreate")
					return cli.NewExitError("Expected at least 2 parameters", 1)
				}
				var weight *int
				if c.IsSet("weight") {
					weight = aws.Int(c.Int("weight"))
				}
				args := createArgs{
					name:            c.Args().Get(0),
					records:         c.Args().Slice()[1:],
					wait:            c.Bool("wait"),
					append:          c.Bool("append"),
					replace:         c.Bool("replace"),
					identifier:      c.String("identifier"),
					failover:        c.String("failover"),
					healthCheckId:   c.String("health-check"),
					weight:          weight,
					region:          c.String("region"),
					countryCode:     c.String("country-code"),
					continentCode:   c.String("continent-code"),
					subdivisionCode: c.String("subdivision-code"),
					multivalue:      c.Bool("multivalue"),
				}
				if args.validate() {
					createRecords(args)
				} else {
					return cli.NewExitError("Validation error", 1)
				}
				return nil
			},
		},
		{
			Name:      "rrdelete",
			Aliases:   []string{"rd"},
			Usage:     "delete a record",
			ArgsUsage: "zone prefix type",
			Flags: append(commonFlags,
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				&cli.StringFlag{
					Name:    "identifier",
					Aliases: []string{"i"},
					Usage:   "record set identifier to delete",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 3 {
					cli.ShowCommandHelp(c, "rrdelete")
					return cli.NewExitError("Expected exactly 3 parameters", 1)
				}
				deleteRecord(c.Args().Get(0), c.Args().Get(1), c.Args().Get(2), c.Bool("wait"), c.String("identifier"))
				return nil
			},
		},
		{
			Name:      "rrpurge",
			Usage:     "delete all the records (danger!)",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				&cli.BoolFlag{
					Name:  "confirm",
					Usage: "confirm you definitely want to do this!",
				},
				&cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "rrpurge")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				if !c.Bool("confirm") {
					return cli.NewExitError("You must --confirm this action", 1)
				}
				purgeRecords(c.Args().First(), c.Bool("wait"))
				return nil
			},
		},
		{
			Name:  "dslist",
			Usage: "list reusable delegation sets",
			Flags: commonFlags,
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				listReusableDelegationSets()
				return nil
			},
		},
		{
			Name:  "dscreate",
			Usage: "create a reusable delegation set",
			Flags: append(commonFlags,
				&cli.StringFlag{
					Name:  "zone-id",
					Value: "",
					Usage: "convert the given zone delegation set (optional)",
				},
			),
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				createReusableDelegationSet(c.String("zone-id"))
				return nil
			},
		},
		{
			Name:      "dsdelete",
			Usage:     "delete a reusable delegation set",
			ArgsUsage: "id",
			Flags:     commonFlags,
			Action: func(c *cli.Context) (err error) {
				r53, err = getService(c)
				if err != nil {
					return err
				}
				if c.Args().Len() != 1 {
					cli.ShowCommandHelp(c, "dsdelete")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				deleteReusableDelegationSet(c.Args().First())
				return nil
			},
		},
	}
	err := app.Run(args)
	if err != nil {
		return 1
	}
	return 0
}
