package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A ReferencedService represents a Kubernetes Service that is referenced by a Route and the Gateways it belongs to.
// It does not contain the v1.Service object, because Services are resolved when building
// the dataplane.Configuration.
type ReferencedService struct {
	// GatewayNsNames are all the Gateways that this Service indirectly attaches to through a Route.
	GatewayNsNames map[types.NamespacedName]struct{}
	// Policies is a list of NGF Policies that target this Service.
	Policies []*Policy
}

func buildReferencedServices(
	l7routes map[RouteKey]*L7Route,
	l4Routes map[L4RouteKey]*L4Route,
	gws map[types.NamespacedName]*Gateway,
) map[types.NamespacedName]*ReferencedService {
	referencedServices := make(map[types.NamespacedName]*ReferencedService)
	for gwNsName, gw := range gws {
		if gw == nil {
			continue
		}

		belongsToGw := func(refs []ParentRef) bool {
			for _, ref := range refs {
				if ref.Gateway.NamespacedName == client.ObjectKeyFromObject(gw.Source) {
					return true
				}
			}
			return false
		}

		// routes all have populated ParentRefs from when they were created.
		//
		// Get all the service names referenced from all the l7 and l4 routes.
		for _, route := range l7routes {
			if !route.Valid || !belongsToGw(route.ParentRefs) {
				continue
			}

			// Processes both valid and invalid BackendRefs as invalid ones still have referenced services
			// we may want to track.
			addServicesAndGatewayForL7Routes(route.Spec.Rules, gwNsName, referencedServices)
		}

		for _, route := range l4Routes {
			if !route.Valid || !belongsToGw(route.ParentRefs) {
				continue
			}

			addServicesAndGatewayForL4Routes(route, gwNsName, referencedServices)
		}
	}

	if len(referencedServices) == 0 {
		return nil
	}

	return referencedServices
}

func addServicesAndGatewayForL4Routes(
	route *L4Route,
	gwNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
) {
	nsname := route.Spec.BackendRef.SvcNsName
	if nsname != (types.NamespacedName{}) {
		if _, ok := referencedServices[nsname]; !ok {
			referencedServices[nsname] = &ReferencedService{
				Policies:       nil,
				GatewayNsNames: make(map[types.NamespacedName]struct{}),
			}
		}
		referencedServices[nsname].GatewayNsNames[gwNsName] = struct{}{}
	}
}

func addServicesAndGatewayForL7Routes(
	routeRules []RouteRule,
	gwNsName types.NamespacedName,
	referencedServices map[types.NamespacedName]*ReferencedService,
) {
	for _, rule := range routeRules {
		for _, ref := range rule.BackendRefs {
			if ref.SvcNsName != (types.NamespacedName{}) {
				if _, ok := referencedServices[ref.SvcNsName]; !ok {
					referencedServices[ref.SvcNsName] = &ReferencedService{
						Policies:       nil,
						GatewayNsNames: make(map[types.NamespacedName]struct{}),
					}
				}

				referencedServices[ref.SvcNsName].GatewayNsNames[gwNsName] = struct{}{}
			}
		}
	}
}
