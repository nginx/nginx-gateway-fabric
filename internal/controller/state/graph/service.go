package graph

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A ReferencedService represents a Kubernetes Service that is referenced by a Route and the Gateways it belongs to.
// It does not contain the v1.Service object, because Services are resolved when building
// the dataplane.Configuration.
type ReferencedService struct {
	// GatewayNsNames are all the Gateways that this Service indirectly attaches to through a Route.
	GatewayNsNames map[types.NamespacedName]struct{}
	// ExternalName holds the external service name for ExternalName type services.
	ExternalName string
	// Policies is a list of NGF Policies that target this Service.
	Policies []*Policy
	// IsExternalName indicates whether this Service is of type ExternalName.
	IsExternalName bool
}

func buildReferencedServices(
	l7routes map[RouteKey]*L7Route,
	l4Routes map[L4RouteKey]*L4Route,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*v1.Service,
) map[types.NamespacedName]*ReferencedService {
	referencedServices := make(map[types.NamespacedName]*ReferencedService)

	for gwNsName, gw := range gws {
		if gw == nil {
			continue
		}

		processL7RoutesForGateway(l7routes, gw, gwNsName, referencedServices, services)
		processL4RoutesForGateway(l4Routes, gw, gwNsName, referencedServices, services)
	}

	if len(referencedServices) == 0 {
		return nil
	}

	return referencedServices
}

// processL7RoutesForGateway processes all L7 routes that belong to the given gateway.
func processL7RoutesForGateway(
	l7routes map[RouteKey]*L7Route,
	gw *Gateway,
	gwNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*v1.Service,
) {
	gwKey := client.ObjectKeyFromObject(gw.Source)

	for _, route := range l7routes {
		if !route.Valid || !routeBelongsToGateway(route.ParentRefs, gwKey) {
			continue
		}

		// Process both valid and invalid BackendRefs as invalid ones still have referenced services
		// we may want to track.
		addServicesFromL7RouteRules(route.Spec.Rules, gwNsName, referencedServices, services)
	}
}

// processL4RoutesForGateway processes all L4 routes that belong to the given gateway.
func processL4RoutesForGateway(
	l4Routes map[L4RouteKey]*L4Route,
	gw *Gateway,
	gwNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*v1.Service,
) {
	gwKey := client.ObjectKeyFromObject(gw.Source)

	for _, route := range l4Routes {
		if !route.Valid || !routeBelongsToGateway(route.ParentRefs, gwKey) {
			continue
		}

		addServiceFromL4Route(route, gwNsName, referencedServices, services)
	}
}

// routeBelongsToGateway checks if a route belongs to the specified gateway.
func routeBelongsToGateway(refs []ParentRef, gwKey types.NamespacedName) bool {
	for _, ref := range refs {
		if ref.Gateway.NamespacedName == gwKey {
			return true
		}
	}
	return false
}

// addServiceFromL4Route adds services from an L4 route to the referenced services map.
// Supports multiple BackendRefs for TCPRoute/UDPRoute.
func addServiceFromL4Route(
	route *L4Route,
	gwNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*v1.Service,
) {
	// Use helper method to get all backend references
	backendRefs := route.Spec.GetBackendRefs()

	for _, br := range backendRefs {
		svcNsName := br.SvcNsName
		if svcNsName == (types.NamespacedName{}) {
			continue
		}

		ensureReferencedService(svcNsName, referencedServices, services)
		referencedServices[svcNsName].GatewayNsNames[gwNsName] = struct{}{}
	}
}

// addServicesFromL7RouteRules adds services from L7 route rules to the referenced services map.
func addServicesFromL7RouteRules(
	routeRules []RouteRule,
	gwNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*v1.Service,
) {
	for _, rule := range routeRules {
		for _, ref := range rule.BackendRefs {
			if ref.SvcNsName == (types.NamespacedName{}) {
				continue
			}

			ensureReferencedService(ref.SvcNsName, referencedServices, services)
			referencedServices[ref.SvcNsName].GatewayNsNames[gwNsName] = struct{}{}
		}
	}
}

// ensureReferencedService ensures a ReferencedService exists in the map for the given service.
func ensureReferencedService(
	svcNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
	services map[types.NamespacedName]*v1.Service,
) {
	if _, exists := referencedServices[svcNsName]; exists {
		return
	}

	isExternal, externalName := getServiceExternalNameInfo(svcNsName, services)
	referencedServices[svcNsName] = &ReferencedService{
		Policies:       nil,
		GatewayNsNames: make(map[types.NamespacedName]struct{}),
		IsExternalName: isExternal,
		ExternalName:   externalName,
	}
}

// getServiceExternalNameInfo returns whether a service is an ExternalName service and its external name.
func getServiceExternalNameInfo(
	svcNsName types.NamespacedName,
	services map[types.NamespacedName]*v1.Service,
) (isExternal bool, externalName string) {
	svc, exists := services[svcNsName]
	if !exists || svc.Spec.Type != v1.ServiceTypeExternalName {
		return false, ""
	}

	return true, svc.Spec.ExternalName
}
