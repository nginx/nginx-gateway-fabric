package graph

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
)

func buildUDPRoute(
	udpRoute *v1alpha2.UDPRoute,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*apiv1.Service,
	refGrantResolver func(resource toResource) bool,
) *L4Route {
	r := &L4Route{
		Source: udpRoute,
	}

	sectionNameRefs, err := buildSectionNameRefs(udpRoute.Spec.ParentRefs, udpRoute.Namespace, gws)
	if err != nil {
		r.Valid = false
		return r
	}

	// route doesn't belong to any of the Gateways
	if len(sectionNameRefs) == 0 {
		return nil
	}
	r.ParentRefs = sectionNameRefs

	// UDPRoute doesn't have hostnames like TLSRoute, so we skip hostname validation

	if len(udpRoute.Spec.Rules) != 1 || len(udpRoute.Spec.Rules[0].BackendRefs) != 1 {
		r.Valid = false
		cond := conditions.NewRouteBackendRefUnsupportedValue(
			"Must have exactly one Rule and BackendRef",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	br, conds := validateBackendRefUDPRoute(udpRoute, services, r.ParentRefs, refGrantResolver)

	r.Spec.BackendRef = br
	r.Valid = true
	r.Attachable = true

	if len(conds) > 0 {
		r.Conditions = append(r.Conditions, conds...)
	}

	return r
}

func validateBackendRefUDPRoute(
	udpRoute *v1alpha2.UDPRoute,
	services map[types.NamespacedName]*apiv1.Service,
	parentRefs []ParentRef,
	refGrantResolver func(resource toResource) bool,
) (BackendRef, []conditions.Condition) {
	// Length of BackendRefs and Rules is guaranteed to be one due to earlier check in buildUDPRoute
	refPath := field.NewPath("spec").Child("rules").Index(0).Child("backendRefs").Index(0)

	ref := udpRoute.Spec.Rules[0].BackendRefs[0]

	if valid, cond := validateBackendRef(
		ref,
		udpRoute.Namespace,
		refGrantResolver,
		refPath,
	); !valid {
		backendRef := BackendRef{
			Valid:              false,
			InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
		}

		return backendRef, []conditions.Condition{cond}
	}

	ns := udpRoute.Namespace
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	svcNsName := types.NamespacedName{
		Namespace: ns,
		Name:      string(udpRoute.Spec.Rules[0].BackendRefs[0].Name),
	}

	svcIPFamily, svcPort, err := getIPFamilyAndPortFromRef(
		ref,
		svcNsName,
		services,
		refPath,
	)

	backendRef := BackendRef{
		SvcNsName:          svcNsName,
		ServicePort:        svcPort,
		Valid:              true,
		InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
	}

	if err != nil {
		backendRef.Valid = false

		return backendRef, []conditions.Condition{conditions.NewRouteBackendRefRefBackendNotFound(err.Error())}
	}

	// For UDPRoute, we don't need to validate app protocol compatibility
	// as UDP is protocol-agnostic at the application layer

	var conds []conditions.Condition
	for _, parentRef := range parentRefs {
		if err := verifyIPFamily(parentRef.Gateway.EffectiveNginxProxy, svcIPFamily); err != nil {
			backendRef.Valid = backendRef.Valid || false
			backendRef.InvalidForGateways[parentRef.Gateway.NamespacedName] = conditions.NewRouteInvalidIPFamily(err.Error())
		}
	}

	return backendRef, conds
}
