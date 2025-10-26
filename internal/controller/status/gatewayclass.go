package status

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// SupportedFeatures returns the list of features supported by NGINX Gateway Fabric.
// The list must be sorted in ascending alphabetical order.
func SupportedFeatures() []gatewayv1.SupportedFeature {
	return []gatewayv1.SupportedFeature{
		{Name: "BackendTLSPolicy"},
		{Name: "GatewayAddressEmpty"},
		{Name: "GatewayHTTPListenerIsolation"},
		{Name: "GatewayInfrastructurePropagation"},
		{Name: "GatewayPort8080"},
		{Name: "GatewayStaticAddresses"},
		{Name: "HTTPRouteBackendProtocolWebSocket"},
		{Name: "HTTPRouteDestinationPortMatching"},
		{Name: "HTTPRouteHostRewrite"},
		{Name: "HTTPRouteMethodMatching"},
		{Name: "HTTPRouteParentRefPort"},
		{Name: "HTTPRoutePathRedirect"},
		{Name: "HTTPRoutePathRewrite"},
		{Name: "HTTPRoutePortRedirect"},
		{Name: "HTTPRouteQueryParamMatching"},
		{Name: "HTTPRouteRequestMirror"},
		{Name: "HTTPRouteRequestMultipleMirrors"},
		{Name: "HTTPRouteRequestPercentageMirror"},
		{Name: "HTTPRouteResponseHeaderModification"},
		{Name: "HTTPRouteSchemeRedirect"},
	}
}
