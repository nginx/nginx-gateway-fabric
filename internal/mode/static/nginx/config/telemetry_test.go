package config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

func TestExecuteTelemetry(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		Telemetry: dataplane.Telemetry{
			Endpoint:    "1.2.3.4:123",
			ServiceName: "ngf:gw-ns:gw-name:my-name",
			Interval:    "5s",
			BatchSize:   512,
			BatchCount:  4,
			Ratios: []dataplane.Ratio{
				{
					Name:  "ratio1",
					Value: 10,
				},
				{
					Name:  "ratio2",
					Value: 20,
				},
			},
		},
	}

	g := NewWithT(t)
	expSubStrings := map[string]int{
		"endpoint 1.2.3.4:123;":                        1,
		"otel_service_name ngf:gw-ns:gw-name:my-name;": 1,
		"interval 5s;":                                 1,
		"batch_size 512;":                              1,
		"batch_count 4;":                               1,
		"split_clients $otel_trace_id":                 2,
		"10% on":                                       1,
		"20% on":                                       1,
	}

	for expSubStr, expCount := range expSubStrings {
		res := executeTelemetry(conf)
		g.Expect(res).To(HaveLen(1))
		g.Expect(expCount).To(Equal(strings.Count(string(res[0].data), expSubStr)))
	}
}

func TestExecuteTelemetryNil(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		Telemetry: dataplane.Telemetry{},
	}

	g := NewWithT(t)

	res := executeTelemetry(conf)
	g.Expect(res).To(BeEmpty())
}
