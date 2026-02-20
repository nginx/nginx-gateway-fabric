package status

import (
	"sort"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/pkg/features"
)

// supportedFeatures returns the list of features supported by NGINX Gateway Fabric.
// The list must be sorted in ascending alphabetical order.
// If experimental is true, experimental features like TLSRoute will be included.
func supportedFeatures(experimental bool) []gatewayv1.SupportedFeature {
	featureNames := []features.FeatureName{
		// Core features
		features.SupportGateway,
		features.SupportGRPCRoute,
		features.SupportHTTPRoute,
		features.SupportReferenceGrant,

		// BackendTLSPolicy
		features.SupportBackendTLSPolicy,

		// Gateway extended
		features.SupportGatewayAddressEmpty,
		features.SupportGatewayHTTPListenerIsolation,
		features.SupportGatewayInfrastructurePropagation,
		features.SupportGatewayPort8080,
		features.SupportGatewayStaticAddresses,

		// HTTPRoute extended
		features.SupportHTTPRouteBackendProtocolWebSocket,
		features.SupportHTTPRouteDestinationPortMatching,
		features.SupportHTTPRouteHostRewrite,
		features.SupportHTTPRouteMethodMatching,
		features.SupportHTTPRouteParentRefPort,
		features.SupportHTTPRoutePathRedirect,
		features.SupportHTTPRoutePathRewrite,
		features.SupportHTTPRoutePortRedirect,
		features.SupportHTTPRouteQueryParamMatching,
		features.SupportHTTPRouteRequestMirror,
		features.SupportHTTPRouteRequestMultipleMirrors,
		features.SupportHTTPRouteRequestPercentageMirror,
		features.SupportHTTPRouteResponseHeaderModification,
		features.SupportHTTPRouteSchemeRedirect,
	}

	// Add experimental features if enabled
	if experimental {
		featureNames = append(
			featureNames,
			features.SupportTLSRoute,
			features.SupportUDPRoute,
			features.SupportHTTPRouteCORS,
		)
	}

	// Sort alphabetically by feature name
	sort.Slice(featureNames, func(i, j int) bool {
		return string(featureNames[i]) < string(featureNames[j])
	})

	// Convert to SupportedFeature slice
	result := make([]gatewayv1.SupportedFeature, 0, len(featureNames))
	for _, name := range featureNames {
		result = append(result, gatewayv1.SupportedFeature{Name: gatewayv1.FeatureName(name)})
	}

	return result
}
