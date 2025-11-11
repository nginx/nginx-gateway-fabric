package status

import (
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/pkg/features"
)

func TestSupportedFeatures(t *testing.T) {
	t.Parallel()

	standardFeatures := []gatewayv1.FeatureName{
		gatewayv1.FeatureName(features.SupportBackendTLSPolicy),
		gatewayv1.FeatureName(features.SupportGRPCRoute),
		gatewayv1.FeatureName(features.SupportGateway),
		gatewayv1.FeatureName(features.SupportGatewayAddressEmpty),
		gatewayv1.FeatureName(features.SupportGatewayHTTPListenerIsolation),
		gatewayv1.FeatureName(features.SupportGatewayInfrastructurePropagation),
		gatewayv1.FeatureName(features.SupportGatewayPort8080),
		gatewayv1.FeatureName(features.SupportGatewayStaticAddresses),
		gatewayv1.FeatureName(features.SupportHTTPRoute),
		gatewayv1.FeatureName(features.SupportHTTPRouteBackendProtocolWebSocket),
		gatewayv1.FeatureName(features.SupportHTTPRouteDestinationPortMatching),
		gatewayv1.FeatureName(features.SupportHTTPRouteHostRewrite),
		gatewayv1.FeatureName(features.SupportHTTPRouteMethodMatching),
		gatewayv1.FeatureName(features.SupportHTTPRouteParentRefPort),
		gatewayv1.FeatureName(features.SupportHTTPRoutePathRedirect),
		gatewayv1.FeatureName(features.SupportHTTPRoutePathRewrite),
		gatewayv1.FeatureName(features.SupportHTTPRoutePortRedirect),
		gatewayv1.FeatureName(features.SupportHTTPRouteQueryParamMatching),
		gatewayv1.FeatureName(features.SupportHTTPRouteRequestMirror),
		gatewayv1.FeatureName(features.SupportHTTPRouteRequestMultipleMirrors),
		gatewayv1.FeatureName(features.SupportHTTPRouteRequestPercentageMirror),
		gatewayv1.FeatureName(features.SupportHTTPRouteResponseHeaderModification),
		gatewayv1.FeatureName(features.SupportHTTPRouteSchemeRedirect),
		gatewayv1.FeatureName(features.SupportReferenceGrant),
	}

	experimentalFeatures := []gatewayv1.FeatureName{
		gatewayv1.FeatureName(features.SupportTLSRoute),
	}

	allFeatures := append(slices.Clone(standardFeatures), experimentalFeatures...)

	tests := []struct {
		name               string
		expectedFeatures   []gatewayv1.FeatureName
		unexpectedFeatures []gatewayv1.FeatureName
		experimental       bool
	}{
		{
			name:               "standard features only",
			experimental:       false,
			expectedFeatures:   standardFeatures,
			unexpectedFeatures: experimentalFeatures,
		},
		{
			name:               "standard and experimental features",
			experimental:       true,
			expectedFeatures:   allFeatures,
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
