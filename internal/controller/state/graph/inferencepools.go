package graph

import (
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
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
	// BackendTLSPolicy is the backend TLS policy for the EndpointPicker.
	BackendTLSPolicy *BackendTLSPolicy
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
	listenerSets map[types.NamespacedName]*ListenerSet,
) map[types.NamespacedName]*ReferencedInferencePool {
	referencedInferencePools := make(map[types.NamespacedName]*ReferencedInferencePool, len(inferencePools))

	for _, gw := range gws {
		if gw == nil {
			continue
		}

		processInferencePoolsForGateway(routes, gw, referencedInferencePools, inferencePools, listenerSets)
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
	listenerSets map[types.NamespacedName]*ListenerSet,
) {
	gwKey := client.ObjectKeyFromObject(gw.Source)

	for _, route := range routes {
		if !routeBelongsToGateway(route.ParentRefs, gwKey, listenerSets) {
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

				name := ref.InferencePoolName
				if name == "" {
					name = string(ref.Name)
				}

				poolName := types.NamespacedName{
					Name:      name,
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

	if ip.Spec.EndpointPickerRef == nil {
		failingCond = conditions.NewInferencePoolEndpointPickerRefMissing(
			"spec.endpointPickerRef must be set; NGINX Gateway Fabric requires " +
				"an EndpointPicker extension to route to this InferencePool",
		)
		return &failingCond
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

func addInferencePoolEPPServicesToReferencedServices(
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*v1.Service,
) map[types.NamespacedName]*ReferencedService {
	for poolNsName, pool := range referencedInferencePools {
		if !pool.Valid || pool.Source == nil || pool.Source.Spec.EndpointPickerRef == nil {
			continue
		}
		eppRef := pool.Source.Spec.EndpointPickerRef
		kind := string(eppRef.Kind)
		if kind == "" {
			kind = kinds.Service
		}
		if kind != kinds.Service {
			continue
		}

		ns := pool.Source.Namespace
		if ns == "" {
			ns = poolNsName.Namespace
		}

		eppSvcNsName := types.NamespacedName{
			Namespace: ns,
			Name:      string(eppRef.Name),
		}

		if referencedServices == nil {
			referencedServices = make(map[types.NamespacedName]*ReferencedService)
		}
		ensureReferencedService(eppSvcNsName, referencedServices, services)
		for _, gw := range pool.Gateways {
			gwNsName := client.ObjectKeyFromObject(gw)
			referencedServices[eppSvcNsName].GatewayNsNames[gwNsName] = struct{}{}
		}
	}
	return referencedServices
}

func getEPPServicePort(
	inferencePool *inference.InferencePool,
	namespace string,
	services map[types.NamespacedName]*v1.Service,
) (v1.ServicePort, error) {
	if inferencePool == nil || inferencePool.Spec.EndpointPickerRef == nil {
		return v1.ServicePort{}, errors.New("spec.endpointPickerRef is nil")
	}

	if namespace == "" {
		namespace = inferencePool.Namespace
	}

	eppNsName := types.NamespacedName{
		Name:      string(inferencePool.Spec.EndpointPickerRef.Name),
		Namespace: namespace,
	}

	eppSvc, ok := services[eppNsName]
	if !ok {
		return v1.ServicePort{}, fmt.Errorf(
			"EndpointPicker Service %s/%s referenced by InferencePool %s/%s does not exist",
			eppNsName.Namespace,
			eppNsName.Name,
			namespace,
			inferencePool.Name,
		)
	}
	switch {
	case inferencePool.Spec.EndpointPickerRef.Port != nil:
		return getServicePort(eppSvc, int32(inferencePool.Spec.EndpointPickerRef.Port.Number))
	case len(eppSvc.Spec.Ports) == 1:
		return eppSvc.Spec.Ports[0], nil
	default:
		return v1.ServicePort{}, fmt.Errorf(
			"EndpointPicker Service %s/%s must have one port when EndpointPickerRef.port is unset",
			eppNsName.Namespace,
			eppNsName.Name,
		)
	}
}
