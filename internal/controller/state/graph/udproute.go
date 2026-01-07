package graph

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func buildUDPRoute(
	udpRoute *v1alpha2.UDPRoute,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*apiv1.Service,
	refGrantResolver func(resource toResource) bool,
) *L4Route {
	// Convert UDPRoute rules to generic l4RouteRule format
	rules := make([]l4RouteRule, len(udpRoute.Spec.Rules))
	for i, rule := range udpRoute.Spec.Rules {
		rules[i] = l4RouteRule{
			backendRefs: rule.BackendRefs,
		}
	}

	// Use the generic L4 route builder
	config := l4RouteConfig{
		source:           udpRoute,
		namespace:        udpRoute.Namespace,
		parentRefs:       udpRoute.Spec.ParentRefs,
		rules:            rules,
		routeType:        RouteTypeUDP,
		refGrantResolver: refGrantResolver,
	}

	return buildGenericL4Route(config, gws, services)
}
