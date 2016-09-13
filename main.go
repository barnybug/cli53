package cli53

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/urfave/cli"
)

var r53 *route53.Route53
var version = "0.8.4"

// Main entry point for cli53 application
func Main(args []string) int {
	cli.OsExiter = func(c int) {
		// noop - don't exit
	}

	commonFlags := []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "enable debug logging",
		},
		cli.StringFlag{
			Name:  "profile",
			Usage: "profile to use from credentials file",
		},
	}

	app := cli.NewApp()
	app.Name = "cli53"
	app.Usage = "manage route53 DNS"
	app.Version = version
	app.Commands = []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list domains",
			Flags: append(commonFlags,
				cli.StringFlag{
					Name:  "format, f",
					Value: "table",
					Usage: "output format: text, json, jl, table, csv",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 0 {
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
				cli.StringFlag{
					Name:  "comment",
					Value: "",
					Usage: "comment on the domain",
				},
				cli.StringFlag{
					Name:  "vpc-id",
					Value: "",
					Usage: "create a private zone in the VPC",
				},
				cli.StringFlag{
					Name:  "vpc-region",
					Value: "",
					Usage: "VPC region (required if vpcId is specified)",
				},
				cli.StringFlag{
					Name:  "delegation-set-id",
					Value: "",
					Usage: "use the given delegation set",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 1 {
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
				cli.BoolFlag{
					Name:  "purge",
					Usage: "remove any existing records on the domain (otherwise deletion will fail)",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "delete")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				domain := c.Args().First()
				deleteZone(domain, c.Bool("purge"))
				return nil
			},
		},
		{
			Name:      "import",
			Usage:     "import a bind zone file",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				cli.StringFlag{
					Name:  "file",
					Value: "",
					Usage: "bind zone file (required)",
				},
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				cli.BoolFlag{
					Name:  "editauth",
					Usage: "include SOA and NS records from zone file",
				},
				cli.BoolFlag{
					Name:  "replace",
					Usage: "replace all existing records",
				},
				cli.BoolFlag{
					Name:  "dry-run, n",
					Usage: "perform a trial run with no changes made",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "import")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				args := importArgs{
					name:     c.Args().First(),
					file:     c.String("file"),
					wait:     c.Bool("wait"),
					editauth: c.Bool("editauth"),
					replace:  c.Bool("replace"),
					dryrun:   c.Bool("dry-run"),
				}
				importBind(args)
				return nil
			},
		},
		{
			Name:      "export",
			Usage:     "export a bind zone file (to stdout)",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				cli.BoolFlag{
					Name:  "full, f",
					Usage: "export prefixes as full names",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "export")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				exportBind(c.Args().First(), c.Bool("full"))
				return nil
			},
		},
		{
			Name:      "rrcreate",
			Aliases:   []string{"rc"},
			Usage:     "create one or more records",
			ArgsUsage: "zone record [record...]",
			Flags: append(commonFlags,
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				cli.BoolFlag{
					Name:  "append",
					Usage: "append the record",
				},
				cli.BoolFlag{
					Name:  "replace",
					Usage: "replace the record",
				},
				cli.StringFlag{
					Name:  "identifier, i",
					Usage: "record set identifier (for routed records)",
				},
				cli.StringFlag{
					Name:  "failover",
					Usage: "PRIMARY or SECONDARY on a failover routing",
				},
				cli.StringFlag{
					Name:  "health-check",
					Usage: "associated health check id for failover PRIMARY",
				},
				cli.IntFlag{
					Name:  "weight",
					Usage: "weight on a weighted routing",
				},
				cli.StringFlag{
					Name:  "region",
					Usage: "region for latency-based routing",
				},
				cli.StringFlag{
					Name:  "country-code",
					Usage: "country code for geolocation routing",
				},
				cli.StringFlag{
					Name:  "continent-code",
					Usage: "continent code for geolocation routing",
				},
				cli.StringFlag{
					Name:  "subdivision-code",
					Usage: "subdivision code for geolocation routing",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) < 2 {
					cli.ShowCommandHelp(c, "rrcreate")
					return cli.NewExitError("Expected at least 2 parameters", 1)
				}
				var weight *int
				if c.IsSet("weight") {
					weight = aws.Int(c.Int("weight"))
				}
				args := createArgs{
					name:            c.Args()[0],
					records:         c.Args()[1:],
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
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				cli.StringFlag{
					Name:  "identifier, i",
					Usage: "record set identifier to delete",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 3 {
					cli.ShowCommandHelp(c, "rrdelete")
					return cli.NewExitError("Expected exactly 3 parameters", 1)
				}
				deleteRecord(c.Args()[0], c.Args()[1], c.Args()[2], c.Bool("wait"), c.String("identifier"))
				return nil
			},
		},
		{
			Name:      "rrpurge",
			Usage:     "delete all the records (danger!)",
			ArgsUsage: "name|ID",
			Flags: append(commonFlags,
				cli.BoolFlag{
					Name:  "confirm",
					Usage: "confirm you definitely want to do this!",
				},
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 1 {
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
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				listReusableDelegationSets()
				return nil
			},
		},
		{
			Name:  "dscreate",
			Usage: "create a reusable delegation set",
			Flags: append(commonFlags,
				cli.StringFlag{
					Name:  "zone-id",
					Value: "",
					Usage: "convert the given zone delegation set (optional)",
				},
			),
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				createReusableDelegationSet(c.String("zone-id"))
				return nil
			},
		},
		{
			Name:      "dsdelete",
			Usage:     "delete a reusable delegation set",
			ArgsUsage: "id",
			Flags:     commonFlags,
			Action: func(c *cli.Context) error {
				r53 = getService(c)
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "dsdelete")
					return cli.NewExitError("Expected exactly 1 parameter", 1)
				}
				deleteReusableDelegationSet(c.Args().First())
				return nil
			},
		},
	}
	err := app.Run(args)
	exitCode := 0
	if err != nil {
		if _, ok := err.(*cli.ExitError); !ok {
			// Exit errors are already printed
			fmt.Println(err)
		}
		exitCode = 1
	}
	return exitCode
}
