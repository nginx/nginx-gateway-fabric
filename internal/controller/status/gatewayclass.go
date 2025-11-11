package status

import (
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/pkg/features"
)

// supportedFeatures returns the list of features supported by NGINX Gateway Fabric.
// The list must be sorted in ascending alphabetical order.
// If experimental is true, experimental features like TLSRoute will be included.
func supportedFeatures(experimental bool) []gatewayv1.SupportedFeature {
	featSet := sets.New(
		// Core features
		features.GatewayFeature,
		features.GRPCRouteFeature,
		features.HTTPRouteFeature,
		features.ReferenceGrantFeature,

		// BackendTLSPolicy
		features.BackendTLSPolicyFeature,

		// Gateway extended
		features.GatewayEmptyAddressFeature,
		features.GatewayHTTPListenerIsolationFeature,
		features.GatewayInfrastructurePropagationFeature,
		features.GatewayPort8080Feature,
		features.GatewayStaticAddressesFeature,

		// HTTPRoute extended
		features.HTTPRouteBackendProtocolWebSocketFeature,
		features.HTTPRouteDestinationPortMatchingFeature,
		features.HTTPRouteHostRewriteFeature,
		features.HTTPRouteMethodMatchingFeature,
		features.HTTPRouteParentRefPortFeature,
		features.HTTPRoutePathRedirectFeature,
		features.HTTPRoutePathRewriteFeature,
		features.HTTPRoutePortRedirectFeature,
		features.HTTPRouteQueryParamMatchingFeature,
		features.HTTPRouteRequestMirrorFeature,
		features.HTTPRouteRequestMultipleMirrorsFeature,
		features.HTTPRouteRequestPercentageMirrorFeature,
		features.HTTPRouteResponseHeaderModificationFeature,
		features.HTTPRouteSchemeRedirectFeature,
	)

	// Add experimental features if enabled
	if experimental {
		featSet.Insert(features.TLSRouteFeature)
	}

	// Convert features to SupportedFeature slice
	result := make([]gatewayv1.SupportedFeature, 0, featSet.Len())
	for _, feat := range featSet.UnsortedList() {
		result = append(result, gatewayv1.SupportedFeature{Name: gatewayv1.FeatureName(feat.Name)})
	}

	// Sort alphabetically by feature name
	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Name) < string(result[j].Name)
	})

	return result
}
