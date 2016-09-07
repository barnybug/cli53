package cli53

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/urfave/cli"
)

type Formatter interface {
	formatZoneList(zones <-chan *route53.HostedZone, w io.Writer)
}

type TextFormatter struct {
}

func (self *TextFormatter) formatZoneList(zones <-chan *route53.HostedZone, w io.Writer) {
	for zone := range zones {
		fmt.Fprintf(w, "%+v\n", zone)
	}
}

type JsonFormatter struct {
}

func (self *JsonFormatter) formatZoneList(zones <-chan *route53.HostedZone, w io.Writer) {
	var all []*route53.HostedZone
	for zone := range zones {
		all = append(all, zone)
	}
	if err := json.NewEncoder(w).Encode(all); err != nil {
		fatalIfErr(err)
	}
}

type JlFormatter struct {
}

func (self *JlFormatter) formatZoneList(zones <-chan *route53.HostedZone, w io.Writer) {
	for zone := range zones {
		if err := json.NewEncoder(w).Encode(zone); err != nil {
			fatalIfErr(err)
		}
	}
}

type TableFormatter struct {
}

func (self *TableFormatter) formatZoneList(zones <-chan *route53.HostedZone, w io.Writer) {
	wr := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	fmt.Fprintln(wr, "ID\tName\tRecord count\tComment")
	for zone := range zones {
		fmt.Fprintf(wr, "%s\t%s\t%d\t%s\n", (*zone.Id)[12:], *zone.Name, *zone.ResourceRecordSetCount, *zone.Config.Comment)
	}
	wr.Flush()
}

type CSVFormatter struct {
}

func (self *CSVFormatter) formatZoneList(zones <-chan *route53.HostedZone, w io.Writer) {
	wr := csv.NewWriter(w)
	wr.Write([]string{"id", "name", "record count", "comment"})
	for zone := range zones {
		wr.Write([]string{(*zone.Id)[12:], *zone.Name, fmt.Sprint(*zone.ResourceRecordSetCount), *zone.Config.Comment})
	}
	wr.Flush()
}

func getFormatter(c *cli.Context) Formatter {
	switch c.String("format") {
	case "text":
		return &TextFormatter{}
	case "json":
		return &JsonFormatter{}
	case "jl":
		return &JlFormatter{}
	case "table":
		return &TableFormatter{}
	case "csv":
		return &CSVFormatter{}
	}
	return nil
}
