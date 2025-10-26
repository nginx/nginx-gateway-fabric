package status

import (
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestSupportedFeatures(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	features := SupportedFeatures()

	// Verify we have the expected features
	expectedFeatures := []gatewayv1.FeatureName{
		"BackendTLSPolicy",
		"GatewayAddressEmpty",
		"GatewayHTTPListenerIsolation",
		"GatewayInfrastructurePropagation",
		"GatewayPort8080",
		"GatewayStaticAddresses",
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
	}

	g.Expect(features).To(HaveLen(len(expectedFeatures)))

	// Verify all expected features are present
	for _, expected := range expectedFeatures {
		g.Expect(slices.ContainsFunc(features, func(f gatewayv1.SupportedFeature) bool {
			return f.Name == expected
		})).To(BeTrue(), "expected feature %s not found", expected)
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
}
