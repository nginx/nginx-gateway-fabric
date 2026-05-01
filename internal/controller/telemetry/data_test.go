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
			GatewayAttachedNpCount:                   15,
			GatewayAttachedRateLimitPolicyCount:      16,
			RouteAttachedRateLimitPolicyCount:        17,
			AuthenticationFilterCount:                18,
			SnippetsPolicyCount:                      19,
			TCPRouteCount:                            20,
			UDPRouteCount:                            21,
			InferencePoolCount:                       22,
			GatewayAttachedProxySettingsPolicyCount:  23,
			RouteAttachedProxySettingsPolicyCount:    24,
			GatewayAttachedWAFPolicyCount:            25,
			RouteAttachedWAFPolicyCount:              26,
			WAFEnabledGatewayCount:                   27,
			ListenerSetCount:                         28,
		},
		SnippetsFiltersDirectives:       []string{"main-three-count", "http-two-count", "server-one-count"},
		SnippetsFiltersDirectivesCount:  []int64{3, 2, 1},
		NginxPodCount:                   3,
		ControlPlanePodCount:            3,
		NginxOneConnectionEnabled:       true,
		SnippetsPoliciesDirectives:      []string{"main-three-count", "http-two-count"},
		SnippetsPoliciesDirectivesCount: []int64{3, 2},
	}

	// Define the expected attributes
	// Ordered by attributes defined in Data struct
	expected := []attribute.KeyValue{
		// Top level attributes
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

		// Nested NGFResourceCounts attributes
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
		attribute.Int64("GatewayAttachedNpCount", 15),
		attribute.Int64("GatewayAttachedRateLimitPolicyCount", 16),
		attribute.Int64("RouteAttachedRateLimitPolicyCount", 17),
		attribute.Int64("AuthenticationFilterCount", 18),
		attribute.Int64("SnippetsPolicyCount", 19),
		attribute.Int64("TCPRouteCount", 20),
		attribute.Int64("UDPRouteCount", 21),
		attribute.Int64("InferencePoolCount", 22),
		attribute.Int64("GatewayAttachedProxySettingsPolicyCount", 23),
		attribute.Int64("RouteAttachedProxySettingsPolicyCount", 24),
		attribute.Int64("GatewayAttachedWAFPolicyCount", 25),
		attribute.Int64("RouteAttachedWAFPolicyCount", 26),
		attribute.Int64("WAFEnabledGatewayCount", 27),
		attribute.Int64("ListenerSetCount", 28),

		// Top level attributes
		attribute.Int64("NginxPodCount", 3),
		attribute.Int64("ControlPlanePodCount", 3),
		attribute.Bool("NginxOneConnectionEnabled", true),
		attribute.String("BuildOS", ""),
		attribute.StringSlice(
			"SnippetsPoliciesDirectives",
			[]string{"main-three-count", "http-two-count"},
		),
		attribute.IntSlice("SnippetsPoliciesDirectivesCount", []int{3, 2}),
	}

	result := data.Attributes()

	g := NewWithT(t)
	g.Expect(result).To(Equal(expected))
}

func TestDataAttributesWithEmptyData(t *testing.T) {
	t.Parallel()
	data := Data{}

	// Define the expected attributes
	// Ordered by attributes defined in Data struct
	expected := []attribute.KeyValue{
		// Top level attributes
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

		// Nested NGFResourceCounts attributes
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
		attribute.Int64("GatewayAttachedNpCount", 0),
		attribute.Int64("GatewayAttachedRateLimitPolicyCount", 0),
		attribute.Int64("RouteAttachedRateLimitPolicyCount", 0),
		attribute.Int64("AuthenticationFilterCount", 0),
		attribute.Int64("SnippetsPolicyCount", 0),
		attribute.Int64("TCPRouteCount", 0),
		attribute.Int64("UDPRouteCount", 0),
		attribute.Int64("InferencePoolCount", 0),
		attribute.Int64("GatewayAttachedProxySettingsPolicyCount", 0),
		attribute.Int64("RouteAttachedProxySettingsPolicyCount", 0),
		attribute.Int64("GatewayAttachedWAFPolicyCount", 0),
		attribute.Int64("RouteAttachedWAFPolicyCount", 0),
		attribute.Int64("WAFEnabledGatewayCount", 0),
		attribute.Int64("ListenerSetCount", 0),

		// Top level attributes
		attribute.Int64("NginxPodCount", 0),
		attribute.Int64("ControlPlanePodCount", 0),
		attribute.Bool("NginxOneConnectionEnabled", false),
		attribute.String("BuildOS", ""),
		attribute.StringSlice("SnippetsPoliciesDirectives", nil),
		attribute.IntSlice("SnippetsPoliciesDirectivesCount", nil),
	}

	result := data.Attributes()

	g := NewWithT(t)

	g.Expect(result).To(Equal(expected))
}
