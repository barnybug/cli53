package cli53

import (
	"errors"
	"fmt"

	"github.com/miekg/dns"
)

const ClassAWS = 253
const TypeALIAS = 0x0F99

type ALIASRdata struct {
	Type                 string
	Target               string
	ZoneId               string
	EvaluateTargetHealth bool
}

func (rd *ALIASRdata) Copy(dest dns.PrivateRdata) error {
	d := dest.(*ALIASRdata)
	d.Type = rd.Type
	d.Target = rd.Target
	d.ZoneId = rd.ZoneId
	d.EvaluateTargetHealth = rd.EvaluateTargetHealth
	return nil
}

func (rd *ALIASRdata) Len() int {
	return 0
}

func (rd *ALIASRdata) Parse(txt []string) error {
	if len(txt) != 4 {
		return errors.New("4 parts required for ALIAS: type target zoneid evaluateTargetHealth")
	}
	rd.Type = txt[0]
	rd.Target = txt[1]
	rd.ZoneId = txt[2]
	rd.EvaluateTargetHealth = (txt[3] == "true")
	return nil
}

func (rd *ALIASRdata) Pack(buf []byte) (int, error) {
	return 0, nil
}

func (rd *ALIASRdata) Unpack(buf []byte) (int, error) {
	return 0, nil
}

func (rr *ALIASRdata) String() string {
	return fmt.Sprintf("%s %s %s %v",
		rr.Type,
		rr.Target,
		rr.ZoneId,
		rr.EvaluateTargetHealth,
	)
}

func NewALIASRdata() dns.PrivateRdata { return new(ALIASRdata) }

func init() {
	dns.StringToClass["AWS"] = ClassAWS
	dns.ClassToString[ClassAWS] = "AWS"
	dns.PrivateHandle("ALIAS", TypeALIAS, NewALIASRdata)
}

type AWSRoute interface {
	String() string
	Parse(KeyValues)
}

type AWSRR struct {
	dns.RR
	Route         AWSRoute
	HealthCheckId *string
	Identifier    string
}

func (rr *AWSRR) String() string {
	var kvs KeyValues
	if rr.HealthCheckId != nil {
		kvs = append(kvs, "healthCheckId", *rr.HealthCheckId)
	}
	kvs = append(kvs, "identifier", rr.Identifier)
	return fmt.Sprintf("%s ; AWS %s %s",
		rr.RR,
		rr.Route,
		kvs,
	)
}

type FailoverRoute struct {
	Failover string
}

func (f *FailoverRoute) String() string {
	return KeyValues{"routing", "FAILOVER", "failover", f.Failover}.String()
}

func (f *FailoverRoute) Parse(kvs KeyValues) {
	f.Failover = kvs.GetString("failover")
}

type GeoLocationRoute struct {
	CountryCode     *string
	ContinentCode   *string
	SubdivisionCode *string
}

func (f *GeoLocationRoute) String() string {
	args := KeyValues{"routing", "GEOLOCATION"}
	if f.CountryCode != nil {
		args = append(args, "countryCode", *f.CountryCode)
	}
	if f.ContinentCode != nil {
		args = append(args, "continentCode", *f.ContinentCode)
	}
	if f.SubdivisionCode != nil {
		args = append(args, "subdivisionCode", *f.SubdivisionCode)
	}
	return args.String()
}

func (f *GeoLocationRoute) Parse(kvs KeyValues) {
	f.CountryCode = kvs.GetOptString("countryCode")
	f.ContinentCode = kvs.GetOptString("continentCode")
	f.SubdivisionCode = kvs.GetOptString("subdivisonCode")
}

type LatencyRoute struct {
	Region string
}

func (f *LatencyRoute) String() string {
	return KeyValues{"routing", "LATENCY", "region", f.Region}.String()
}

func (f *LatencyRoute) Parse(kvs KeyValues) {
	f.Region = kvs.GetString("region")
}

type WeightedRoute struct {
	Weight int64
}

func (f *WeightedRoute) String() string {
	return KeyValues{"routing", "WEIGHTED", "weight", f.Weight}.String()
}

func (f *WeightedRoute) Parse(kvs KeyValues) {
	f.Weight = int64(kvs.GetInt("weight"))
}

type MultiValueAnswerRoute struct {
}

func (f *MultiValueAnswerRoute) String() string {
	return KeyValues{"routing", "MULTIVALUE"}.String()
}

func (f *MultiValueAnswerRoute) Parse(kvs KeyValues) {
}

var RoutingTypes = map[string]func() AWSRoute{
	"FAILOVER":    func() AWSRoute { return &FailoverRoute{} },
	"GEOLOCATION": func() AWSRoute { return &GeoLocationRoute{} },
	"LATENCY":     func() AWSRoute { return &LatencyRoute{} },
	"WEIGHTED":    func() AWSRoute { return &WeightedRoute{} },
	"MULTIVALUE":  func() AWSRoute { return &MultiValueAnswerRoute{} },
}
