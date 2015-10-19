package cli53

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/codegangsta/cli"
)

var r53 *route53.Route53
var version string /* passed in by go build */

// Entry point for cli53 application
func Main(args []string) {
	app := cli.NewApp()
	app.Name = "cli53"
	app.Usage = "manage route53 DNS"
	app.Version = version
	app.Commands = []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list domains",
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				listZones()
			},
		},
		{
			Name:      "create",
			Usage:     "create a domain",
			ArgsUsage: "domain.name",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
				cli.StringFlag{
					Name:  "comment",
					Value: "",
					Usage: "comment on the domain",
				},
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "create")
					os.Exit(1)
				}
				createZone(c.Args().First(), c.String("comment"))
			},
		},
		{
			Name:      "delete",
			Usage:     "delete a domain",
			ArgsUsage: "zone",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
				cli.BoolFlag{
					Name:  "purge",
					Usage: "remove any existing records on the domain (otherwise deletion will fail)",
				},
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "delete")
					os.Exit(1)
				}
				domain := c.Args().First()
				deleteZone(domain, c.Bool("purge"))
			},
		},
		{
			Name:      "import",
			Usage:     "import a bind zone file",
			ArgsUsage: "zone",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
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
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "import")
					os.Exit(1)
				}
				importBind(c.Args().First(), c.String("file"), c.Bool("wait"), c.Bool("editauth"), c.Bool("replace"))
			},
		},
		{
			Name:      "export",
			Usage:     "export a bind zone file (to stdout)",
			ArgsUsage: "zone",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
				cli.BoolFlag{
					Name:  "full, f",
					Usage: "export prefixes as full names",
				},
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "export")
					os.Exit(1)
				}
				exportBind(c.Args().First(), c.Bool("full"))
			},
		},
		{
			Name:      "rrcreate",
			Aliases:   []string{"rc"},
			Usage:     "create a record",
			ArgsUsage: "zone record",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
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
					Usage: "associated health check id for failover PRIMART",
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
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 2 {
					cli.ShowCommandHelp(c, "rrcreate")
					os.Exit(1)
				}
				var weight *int
				if c.IsSet("weight") {
					weight = aws.Int(c.Int("weight"))
				}
				args := createArgs{
					name:          c.Args()[0],
					record:        c.Args()[1],
					wait:          c.Bool("wait"),
					replace:       c.Bool("replace"),
					identifier:    c.String("identifier"),
					failover:      c.String("failover"),
					healthCheckId: c.String("health-check"),
					weight:        weight,
					region:        c.String("region"),
					countryCode:   c.String("country-code"),
					continentCode: c.String("continent-code"),
				}
				createRecord(args)
			},
		},
		{
			Name:      "rrdelete",
			Aliases:   []string{"rd"},
			Usage:     "delete a record",
			ArgsUsage: "zone prefix type",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
				cli.StringFlag{
					Name:  "identifier, i",
					Usage: "record set identifier to delete",
				},
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 3 {
					cli.ShowCommandHelp(c, "rrdelete")
					os.Exit(1)
				}
				deleteRecord(c.Args()[0], c.Args()[1], c.Args()[2], c.Bool("wait"), c.String("identifier"))
			},
		},
		{
			Name:      "rrpurge",
			Usage:     "delete all the records (danger!)",
			ArgsUsage: "zone",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "debug, d",
					Usage: "enable debug logging",
				},
				cli.BoolFlag{
					Name:  "confirm",
					Usage: "confirm you definitely want to do this!",
				},
				cli.BoolFlag{
					Name:  "wait",
					Usage: "wait for changes to become live",
				},
			},
			Action: func(c *cli.Context) {
				r53 = getService(c.Bool("debug"))
				if len(c.Args()) != 1 {
					cli.ShowCommandHelp(c, "rrpurge")
					os.Exit(1)
				}
				if !c.Bool("confirm") {
					errorAndExit("You must --confirm this action")
				}
				purgeRecords(c.Args().First(), c.Bool("wait"))
			},
		},
	}
	app.Run(args)
}
