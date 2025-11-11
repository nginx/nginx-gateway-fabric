package status

import (
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestSupportedFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		expectedFeatures   []gatewayv1.FeatureName
		unexpectedFeatures []gatewayv1.FeatureName
		experimental       bool
	}{
		{
			name:         "standard features only",
			experimental: false,
			expectedFeatures: []gatewayv1.FeatureName{
				"BackendTLSPolicy",
				"GRPCRoute",
				"Gateway",
				"GatewayAddressEmpty",
				"GatewayHTTPListenerIsolation",
				"GatewayInfrastructurePropagation",
				"GatewayPort8080",
				"GatewayStaticAddresses",
				"HTTPRoute",
				"HTTPRouteBackendProtocolWebSocket",
				"HTTPRouteDestinationPortMatching",
				"HTTPRouteHostRewrite",
				"HTTPRouteMethodMatching",
				"HTTPRouteParentRefPort",
				"HTTPRoutePathRedirect",
				"HTTPRoutePathRewrite",
				"HTTPRoutePortRedirect",
				"HTTPRouteQueryParamMatching",
				"HTTPRouteRequestMirror",
				"HTTPRouteRequestMultipleMirrors",
				"HTTPRouteRequestPercentageMirror",
				"HTTPRouteResponseHeaderModification",
				"HTTPRouteSchemeRedirect",
				"ReferenceGrant",
			},
			unexpectedFeatures: []gatewayv1.FeatureName{
				"TLSRoute",
			},
		},
		{
			name:         "standard and experimental features",
			experimental: true,
			expectedFeatures: []gatewayv1.FeatureName{
				"BackendTLSPolicy",
				"GRPCRoute",
				"Gateway",
				"GatewayAddressEmpty",
				"GatewayHTTPListenerIsolation",
				"GatewayInfrastructurePropagation",
				"GatewayPort8080",
				"GatewayStaticAddresses",
				"HTTPRoute",
				"HTTPRouteBackendProtocolWebSocket",
				"HTTPRouteDestinationPortMatching",
				"HTTPRouteHostRewrite",
				"HTTPRouteMethodMatching",
				"HTTPRouteParentRefPort",
				"HTTPRoutePathRedirect",
				"HTTPRoutePathRewrite",
				"HTTPRoutePortRedirect",
				"HTTPRouteQueryParamMatching",
				"HTTPRouteRequestMirror",
				"HTTPRouteRequestMultipleMirrors",
				"HTTPRouteRequestPercentageMirror",
				"HTTPRouteResponseHeaderModification",
				"HTTPRouteSchemeRedirect",
				"ReferenceGrant",
				"TLSRoute",
			},
			unexpectedFeatures: []gatewayv1.FeatureName{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			features := supportedFeatures(tc.experimental)

			g.Expect(features).To(HaveLen(len(tc.expectedFeatures)))

			// Verify all expected features are present
			for _, expected := range tc.expectedFeatures {
				g.Expect(slices.ContainsFunc(features, func(f gatewayv1.SupportedFeature) bool {
					return f.Name == expected
				})).To(BeTrue(), "expected feature %s not found", expected)
			}

			// Verify unexpected features are not present
			for _, unexpected := range tc.unexpectedFeatures {
				g.Expect(slices.ContainsFunc(features, func(f gatewayv1.SupportedFeature) bool {
					return f.Name == unexpected
				})).To(BeFalse(), "unexpected feature %s found", unexpected)
			}

			// Verify the list is sorted alphabetically
			g.Expect(slices.IsSortedFunc(features, func(a, b gatewayv1.SupportedFeature) int {
				if a.Name < b.Name {
					return -1
				}
				if a.Name > b.Name {
					return 1
				}
				return 0
			})).To(BeTrue(), "features should be sorted alphabetically")
		})
	}
}
