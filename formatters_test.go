package cli53

import (
	"bytes"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/stretchr/testify/assert"
)

func testZones() chan *route53types.HostedZone {
	ret := make(chan *route53types.HostedZone)
	go func() {
		zone := &route53types.HostedZone{
			Id:   aws.String("/hostedzone/Z1RWMUCMCPKCJX"),
			Name: aws.String("example.com."),
			Config: &route53types.HostedZoneConfig{
				Comment: aws.String("comment"),
			},
			ResourceRecordSetCount: aws.Int64(2),
		}
		ret <- zone
		close(ret)
	}()
	return ret
}

func formatTest(f Formatter) string {
	w := &bytes.Buffer{}
	f.formatZoneList(testZones(), w)
	return w.String()
}

func TestTextFormatter(t *testing.T) {
	f := &TextFormatter{}
	assert.Equal(t, "{\n  \"CallerReference\": null,\n  \"Id\": \"/hostedzone/Z1RWMUCMCPKCJX\",\n  \"Name\": \"example.com.\",\n  \"Config\": {\n    \"Comment\": \"comment\",\n    \"PrivateZone\": false\n  },\n  \"Features\": null,\n  \"LinkedService\": null,\n  \"ResourceRecordSetCount\": 2\n}\n", formatTest(f))
}

func TestJsonFormatter(t *testing.T) {
	f := &JsonFormatter{}
	assert.Equal(t, "[{\"CallerReference\":null,\"Id\":\"/hostedzone/Z1RWMUCMCPKCJX\",\"Name\":\"example.com.\",\"Config\":{\"Comment\":\"comment\",\"PrivateZone\":false},\"Features\":null,\"LinkedService\":null,\"ResourceRecordSetCount\":2}]\n", formatTest(f))
}

func TestJlFormatter(t *testing.T) {
	f := &JlFormatter{}
	assert.Equal(t, "{\"CallerReference\":null,\"Id\":\"/hostedzone/Z1RWMUCMCPKCJX\",\"Name\":\"example.com.\",\"Config\":{\"Comment\":\"comment\",\"PrivateZone\":false},\"Features\":null,\"LinkedService\":null,\"ResourceRecordSetCount\":2}\n", formatTest(f))
}

func TestTableFormatter(t *testing.T) {
	f := &TableFormatter{}
	assert.Equal(t, "ID             Name         Record count Comment\nZ1RWMUCMCPKCJX example.com. 2            comment\n", formatTest(f))
}

func TestCSVFormatter(t *testing.T) {
	f := &CSVFormatter{}
	assert.Equal(t, "id,name,record count,comment\nZ1RWMUCMCPKCJX,example.com.,2,comment\n", formatTest(f))
}
