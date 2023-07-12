package graph

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/validation"
)

// ClusterState includes cluster resources necessary to build the Graph.
type ClusterState struct {
	GatewayClasses  map[types.NamespacedName]*v1beta1.GatewayClass
	Gateways        map[types.NamespacedName]*v1beta1.Gateway
	HTTPRoutes      map[types.NamespacedName]*v1beta1.HTTPRoute
	Services        map[types.NamespacedName]*v1.Service
	Namespaces      map[types.NamespacedName]*v1.Namespace
	ReferenceGrants map[types.NamespacedName]*v1beta1.ReferenceGrant
	Secrets         map[types.NamespacedName]*v1.Secret
}

// Graph is a Graph-like representation of Gateway API resources.
type Graph struct {
	// GatewayClass holds the GatewayClass resource.
	GatewayClass *GatewayClass
	// Gateway holds the winning Gateway resource.
	Gateway *Gateway
	// IgnoredGatewayClasses holds the ignored GatewayClass resources, which reference NGINX Gateway in the
	// controllerName, but are not configured via the NGINX Gateway CLI argument. It doesn't hold the GatewayClass
	// resources that do not belong to the NGINX Gateway.
	IgnoredGatewayClasses map[types.NamespacedName]*v1beta1.GatewayClass
	// IgnoredGateways holds the ignored Gateway resources, which belong to the NGINX Gateway (based on the
	// GatewayClassName field of the resource) but ignored. It doesn't hold the Gateway resources that do not belong to
	// the NGINX Gateway.
	IgnoredGateways map[types.NamespacedName]*v1beta1.Gateway
	// Routes holds Route resources.
	Routes map[types.NamespacedName]*Route
	// ReferencedSecrets includes Secrets referenced by Gateway Listeners, including invalid ones.
	// It is different from the other maps, because it includes entries for Secrets that do not exist
	// in the cluster. We need such entries so that we can query the Graph to determine if a Secret is referenced
	// by the Gateway, including the case when the Secret is newly created.
	ReferencedSecrets map[types.NamespacedName]*Secret
}

// IsReferenced returns true if the Graph references the resource.
func (g *Graph) IsReferenced(resourceType client.Object, nsname types.NamespacedName) bool {
	// FIMXE(pleshakov): For now, only works with Secrets.
	// Support EndpointSlices and Namespaces so that we can remove relationship.Capturer and use the Graph
	// as source to determine the relationships.
	// See https://github.com/nginxinc/nginx-kubernetes-gateway/issues/824

	switch resourceType.(type) {
	case *v1.Secret:
		_, exists := g.ReferencedSecrets[nsname]
		return exists
	default:
		return false
	}
}

// BuildGraph builds a Graph from a state.
func BuildGraph(
	state ClusterState,
	controllerName string,
	gcName string,
	validators validation.Validators,
) *Graph {
	processedGwClasses, gcExists := processGatewayClasses(state.GatewayClasses, gcName, controllerName)
	if gcExists && processedGwClasses.Winner == nil {
		// configured GatewayClass does not reference this controller
		return &Graph{}
	}
	gc := buildGatewayClass(processedGwClasses.Winner)

	secretResolver := newSecretResolver(state.Secrets)

	processedGws := processGateways(state.Gateways, gcName)

	refGrantResolver := newReferenceGrantResolver(state.ReferenceGrants)
	gw := buildGateway(processedGws.Winner, secretResolver, gc, refGrantResolver)

	routes := buildRoutesForGateways(validators.HTTPFieldsValidator, state.HTTPRoutes, processedGws.GetAllNsNames())
	bindRoutesToListeners(routes, gw, state.Namespaces)
	addBackendRefsToRouteRules(routes, refGrantResolver, state.Services)

	g := &Graph{
		GatewayClass:          gc,
		Gateway:               gw,
		Routes:                routes,
		IgnoredGatewayClasses: processedGwClasses.Ignored,
		IgnoredGateways:       processedGws.Ignored,
		ReferencedSecrets:     secretResolver.getResolvedSecrets(),
	}

	return g
}
