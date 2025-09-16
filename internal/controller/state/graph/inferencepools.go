package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// A ReferencedInferencePool represents an InferencePool that is referenced by a Route and the
// Gateways it belongs to.
type ReferencedInferencePool struct {
	// Source is the original InferencePool that this ReferencedInferencePool is based on.
	Source *inference.InferencePool
}

// buildReferencedInferencePools builds a map of InferencePools that are referenced by HTTPRoutes
// per Gateway that we process.
func buildReferencedInferencePools(
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
) map[types.NamespacedName]*ReferencedInferencePool {
	referencedInferencePools := make(map[types.NamespacedName]*ReferencedInferencePool)

	for _, gw := range gws {
		if gw == nil {
			continue
		}

		processInferencePoolsForGateway(routes, gw, referencedInferencePools, inferencePools)
	}

	if len(referencedInferencePools) == 0 {
		return nil
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
		if !route.Valid || !routeBelongsToGateway(route.ParentRefs, gwKey) {
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
					referencedInferencePools[poolName] = &ReferencedInferencePool{}
				}

				if pool, exists := inferencePools[poolName]; exists {
					referencedInferencePools[poolName].Source = pool
				}
			}
		}
	}
}
