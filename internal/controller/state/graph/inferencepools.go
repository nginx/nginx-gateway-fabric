package graph

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// A ReferencedInferencePool represents an InferencePool that is referenced by a Route and the
// Gateways it belongs to.
type ReferencedInferencePool struct {
	// Source is the original InferencePool that this ReferencedInferencePool is based on.
	Source *inference.InferencePool
	// Gateways are the Gateways that this ReferencedInferencePool is attached to.
	Gateways []*apiv1.Gateway
	// HTTPRoutes are the HTTPRoutes that reference this InferencePool.
	HTTPRoutes []*L7Route
	// Conditions contains the conditions that should be applied to the InferencePool.
	Conditions []conditions.Condition
	// Valid indicates whether the InferencePool is valid or not.
	Valid bool
}

// EndpointPickerConfig specifies the namespace and reference to the EndpointPicker extension.
type EndpointPickerConfig struct {
	// EndpointPickerRef is the reference to the EndpointPicker.
	EndpointPickerRef *inference.EndpointPickerRef
	// NsName is the namespace of the EndpointPicker.
	NsName string
}

// buildReferencedInferencePools builds a map of InferencePools that are referenced by HTTPRoutes
// per Gateway that we process.
func buildReferencedInferencePools(
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
	services map[types.NamespacedName]*v1.Service,
) map[types.NamespacedName]*ReferencedInferencePool {
	referencedInferencePools := make(map[types.NamespacedName]*ReferencedInferencePool, len(inferencePools))

	for _, gw := range gws {
		if gw == nil {
			continue
		}

		processInferencePoolsForGateway(routes, gw, referencedInferencePools, inferencePools)
	}

	if len(referencedInferencePools) == 0 {
		return nil
	}

	// validate each referenced InferencePool and add conditions.
	for _, refPool := range referencedInferencePools {
		if routeCond := validateInferencePoolRoutesAcceptance(refPool.Source, refPool.HTTPRoutes); routeCond != nil {
			refPool.Conditions = append(refPool.Conditions, *routeCond)
		}

		if extensionRefCond := validateInferencePoolExtensionRef(refPool.Source, services); extensionRefCond != nil {
			refPool.Conditions = append(refPool.Conditions, *extensionRefCond)
		}

		refPool.Valid = len(refPool.Conditions) == 0
	}

	return referencedInferencePools
}

// processInferencePoolsForGateway processes all InferencePools that belong to the given gateway.
func processInferencePoolsForGateway(
	routes map[RouteKey]*L7Route,
	gw *Gateway,
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
) {
	gwKey := client.ObjectKeyFromObject(gw.Source)

	for _, route := range routes {
		if !routeBelongsToGateway(route.ParentRefs, gwKey) {
			continue
		}

		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.RouteBackendRefs {
				if !ref.IsInferencePool && (ref.Kind == nil || *ref.Kind != kinds.InferencePool) {
					continue
				}

				namespace := route.Source.GetNamespace()
				if ref.Namespace != nil {
					namespace = string(*ref.Namespace)
				}

				poolName := types.NamespacedName{
					Name:      controller.GetInferencePoolName(string(ref.Name)),
					Namespace: namespace,
				}

				if _, referenced := referencedInferencePools[poolName]; !referenced {
					referencedInferencePools[poolName] = &ReferencedInferencePool{
						Conditions: make([]conditions.Condition, 0, 2),
						Gateways:   make([]*apiv1.Gateway, 0),
						HTTPRoutes: make([]*L7Route, 0),
					}
				}

				if pool, exists := inferencePools[poolName]; exists {
					referencedInferencePools[poolName].Source = pool
					referencedInferencePools[poolName].Gateways = append(
						referencedInferencePools[poolName].Gateways,
						gw.Source,
					)
					referencedInferencePools[poolName].HTTPRoutes = append(
						referencedInferencePools[poolName].HTTPRoutes,
						route,
					)
				}
			}
		}
	}
}

// validateInferencePoolExtensionRef validates the ExtensionRef of the InferencePool.
func validateInferencePoolExtensionRef(
	ip *inference.InferencePool,
	svc map[types.NamespacedName]*v1.Service,
) *conditions.Condition {
	var failingCond conditions.Condition
	if ip == nil {
		return nil
	}

	// if kind is empty, it defaults to Service
	kind := string(ip.Spec.EndpointPickerRef.Kind)
	if kind == "" {
		kind = kinds.Service
	}

	if kind != kinds.Service {
		failingCond = conditions.NewInferencePoolInvalidExtensionref("Invalid ExtensionRef kind: " + kind)
		return &failingCond
	}

	eppNsName := types.NamespacedName{
		Name:      string(ip.Spec.EndpointPickerRef.Name),
		Namespace: ip.GetNamespace(),
	}

	if _, ok := svc[eppNsName]; !ok {
		failingCond = conditions.NewInferencePoolInvalidExtensionref(
			"The ExtensionRef Service not found: " + eppNsName.String(),
		)
		return &failingCond
	}

	return nil
}

// validateInferencePoolRoutesAcceptance checks if the routes that reference the InferencePool
// are accepted by the Gateway.
func validateInferencePoolRoutesAcceptance(ip *inference.InferencePool, routes []*L7Route) *conditions.Condition {
	if ip == nil || len(routes) == 0 {
		return nil
	}

	// we do not need to validate that the route belongs to the gateway or not
	// we only process routes that belong to the gateway in the first place
	for _, route := range routes {
		if !route.Valid {
			cond := conditions.NewInferencePoolInvalidHTTPRouteNotAccepted(
				fmt.Sprintf("Referenced HTTPRoute %s/%s is not accepted by the Gateway",
					route.Source.GetNamespace(),
					route.Source.GetName(),
				),
			)
			return &cond
		}
	}

	return nil
}
