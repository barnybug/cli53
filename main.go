package cli53

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/codegangsta/cli"
)

var r53 *route53.Route53
var version = "undefined" /* passed in by Makefile */

// Main entry point for cli53 application
func Main(args []string) int {
	exitCode := 0

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
			Flags:   commonFlags,
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				listZones()
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
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "create")
					exitCode = 1
					return
				}
				createZone(c.Args().First(), c.String("comment"), c.String("vpc-id"), c.String("vpc-region"), c.String("delegation-set-id"))
			},
		},
		{
			Name:      "delete",
			Usage:     "delete a domain",
			ArgsUsage: "zone",
			Flags: append(commonFlags,
				cli.BoolFlag{
					Name:  "purge",
					Usage: "remove any existing records on the domain (otherwise deletion will fail)",
				},
			),
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "delete")
					exitCode = 1
					return
				}
				domain := c.Args().First()
				deleteZone(domain, c.Bool("purge"))
			},
		},
		{
			Name:      "import",
			Usage:     "import a bind zone file",
			ArgsUsage: "zone",
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
			),
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "import")
					exitCode = 1
					return
				}
				importBind(c.Args().First(), c.String("file"), c.Bool("wait"), c.Bool("editauth"), c.Bool("replace"))
			},
		},
		{
			Name:      "export",
			Usage:     "export a bind zone file (to stdout)",
			ArgsUsage: "zone",
			Flags: append(commonFlags,
				cli.BoolFlag{
					Name:  "full, f",
					Usage: "export prefixes as full names",
				},
			),
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "export")
					exitCode = 1
					return
				}
				exportBind(c.Args().First(), c.Bool("full"))
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
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) < 2 {
					cli.ShowCommandHelp(c, "rrcreate")
					exitCode = 1
					return
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
					exitCode = 1
				}
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
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 3 {
					cli.ShowCommandHelp(c, "rrdelete")
					exitCode = 1
					return
				}
				deleteRecord(c.Args()[0], c.Args()[1], c.Args()[2], c.Bool("wait"), c.String("identifier"))
			},
		},
		{
			Name:      "rrpurge",
			Usage:     "delete all the records (danger!)",
			ArgsUsage: "zone",
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
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "rrpurge")
					exitCode = 1
					return
				}
				if !c.Bool("confirm") {
					errorAndExit("You must --confirm this action")
				}
				purgeRecords(c.Args().First(), c.Bool("wait"))
			},
		},
		{
			Name:  "dslist",
			Usage: "list reusable delegation sets",
			Flags: commonFlags,
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				listReusableDelegationSets()
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
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				createReusableDelegationSet(c.String("zone-id"))
			},
		},
		{
			Name:      "dsdelete",
			Usage:     "delete a reusable delegation set",
			ArgsUsage: "id",
			Flags:     commonFlags,
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"), c.String("profile"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "dsdelete")
					exitCode = 1
					return
				}
				deleteReusableDelegationSet(c.Args().First())
			},
		},
	}
	err := app.Run(args)
	if err != nil {
		fmt.Println(err)
	}
	return exitCode
}
