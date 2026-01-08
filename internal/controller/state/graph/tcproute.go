package graph

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func buildTCPRoute(
	tcpRoute *v1alpha2.TCPRoute,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*apiv1.Service,
	refGrantResolver func(resource toResource) bool,
) *L4Route {
	// Convert TCPRoute rules to generic l4RouteRule format
	rules := make([]l4RouteRule, len(tcpRoute.Spec.Rules))
	for i, rule := range tcpRoute.Spec.Rules {
		rules[i] = l4RouteRule{
			backendRefs: rule.BackendRefs,
		}
	}

	// Use the generic L4 route builder
	config := l4RouteConfig{
		source:           tcpRoute,
		namespace:        tcpRoute.Namespace,
		parentRefs:       tcpRoute.Spec.ParentRefs,
		rules:            rules,
		routeType:        RouteTypeTCP,
		refGrantResolver: refGrantResolver,
	}

	return buildGenericL4Route(config, gws, services)
}
