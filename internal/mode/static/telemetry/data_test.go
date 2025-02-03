package telemetry

import (
	"testing"

	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/attribute"
)

func TestDataAttributes(t *testing.T) {
	t.Parallel()
	data := Data{
		ImageSource: "local",
		Data: tel.Data{
			ProjectName:         "NGF",
			ProjectVersion:      "edge",
			ProjectArchitecture: "arm64",
			ClusterID:           "1",
			ClusterVersion:      "1.23",
			ClusterPlatform:     "test",
			InstallationID:      "123",
			ClusterNodeCount:    3,
		},
		FlagNames:  []string{"test-flag"},
		FlagValues: []string{"test-value"},
		NGFResourceCounts: NGFResourceCounts{
			GatewayCount:                             1,
			GatewayClassCount:                        2,
			HTTPRouteCount:                           3,
			SecretCount:                              4,
			ServiceCount:                             5,
			EndpointCount:                            6,
			GRPCRouteCount:                           7,
			TLSRouteCount:                            5,
			BackendTLSPolicyCount:                    8,
			GatewayAttachedClientSettingsPolicyCount: 9,
			RouteAttachedClientSettingsPolicyCount:   10,
			ObservabilityPolicyCount:                 11,
			NginxProxyCount:                          12,
			SnippetsFilterCount:                      13,
			UpstreamSettingsPolicyCount:              14,
		},
		NGFReplicaCount:                3,
		SnippetsFiltersDirectives:      []string{"main-three-count", "http-two-count", "server-one-count"},
		SnippetsFiltersDirectivesCount: []int64{3, 2, 1},
	}

	expected := []attribute.KeyValue{
		attribute.String("dataType", "ngf-product-telemetry"),
		attribute.String("ImageSource", "local"),
		attribute.String("ProjectName", "NGF"),
		attribute.String("ProjectVersion", "edge"),
		attribute.String("ProjectArchitecture", "arm64"),
		attribute.String("ClusterID", "1"),
		attribute.String("ClusterVersion", "1.23"),
		attribute.String("ClusterPlatform", "test"),
		attribute.String("InstallationID", "123"),
		attribute.Int64("ClusterNodeCount", 3),
		attribute.StringSlice("FlagNames", []string{"test-flag"}),
		attribute.StringSlice("FlagValues", []string{"test-value"}),
		attribute.StringSlice(
			"SnippetsFiltersDirectives",
			[]string{"main-three-count", "http-two-count", "server-one-count"},
		),
		attribute.IntSlice("SnippetsFiltersDirectivesCount", []int{3, 2, 1}),
		attribute.Int64("GatewayCount", 1),
		attribute.Int64("GatewayClassCount", 2),
		attribute.Int64("HTTPRouteCount", 3),
		attribute.Int64("TLSRouteCount", 5),
		attribute.Int64("SecretCount", 4),
		attribute.Int64("ServiceCount", 5),
		attribute.Int64("EndpointCount", 6),
		attribute.Int64("GRPCRouteCount", 7),
		attribute.Int64("BackendTLSPolicyCount", 8),
		attribute.Int64("GatewayAttachedClientSettingsPolicyCount", 9),
		attribute.Int64("RouteAttachedClientSettingsPolicyCount", 10),
		attribute.Int64("ObservabilityPolicyCount", 11),
		attribute.Int64("NginxProxyCount", 12),
		attribute.Int64("SnippetsFilterCount", 13),
		attribute.Int64("UpstreamSettingsPolicyCount", 14),
		attribute.Int64("NGFReplicaCount", 3),
	}

	result := data.Attributes()

	g := NewWithT(t)
	g.Expect(result).To(Equal(expected))
}

func TestDataAttributesWithEmptyData(t *testing.T) {
	t.Parallel()
	data := Data{}

	expected := []attribute.KeyValue{
		attribute.String("dataType", "ngf-product-telemetry"),
		attribute.String("ImageSource", ""),
		attribute.String("ProjectName", ""),
		attribute.String("ProjectVersion", ""),
		attribute.String("ProjectArchitecture", ""),
		attribute.String("ClusterID", ""),
		attribute.String("ClusterVersion", ""),
		attribute.String("ClusterPlatform", ""),
		attribute.String("InstallationID", ""),
		attribute.Int64("ClusterNodeCount", 0),
		attribute.StringSlice("FlagNames", nil),
		attribute.StringSlice("FlagValues", nil),
		attribute.StringSlice("SnippetsFiltersDirectives", nil),
		attribute.IntSlice("SnippetsFiltersDirectivesCount", nil),
		attribute.Int64("GatewayCount", 0),
		attribute.Int64("GatewayClassCount", 0),
		attribute.Int64("HTTPRouteCount", 0),
		attribute.Int64("TLSRouteCount", 0),
		attribute.Int64("SecretCount", 0),
		attribute.Int64("ServiceCount", 0),
		attribute.Int64("EndpointCount", 0),
		attribute.Int64("GRPCRouteCount", 0),
		attribute.Int64("BackendTLSPolicyCount", 0),
		attribute.Int64("GatewayAttachedClientSettingsPolicyCount", 0),
		attribute.Int64("RouteAttachedClientSettingsPolicyCount", 0),
		attribute.Int64("ObservabilityPolicyCount", 0),
		attribute.Int64("NginxProxyCount", 0),
		attribute.Int64("SnippetsFilterCount", 0),
		attribute.Int64("UpstreamSettingsPolicyCount", 0),
		attribute.Int64("NGFReplicaCount", 0),
	}

	result := data.Attributes()

	g := NewWithT(t)

	g.Expect(result).To(Equal(expected))
}
