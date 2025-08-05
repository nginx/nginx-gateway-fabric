package graph

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
)

func buildTCPRoute(
	tcpRoute *v1alpha2.TCPRoute,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*apiv1.Service,
	refGrantResolver func(resource toResource) bool,
) *L4Route {
	r := &L4Route{
		Source: tcpRoute,
	}

	sectionNameRefs, err := buildSectionNameRefs(tcpRoute.Spec.ParentRefs, tcpRoute.Namespace, gws)
	if err != nil {
		r.Valid = false
		return r
	}

	// route doesn't belong to any of the Gateways
	if len(sectionNameRefs) == 0 {
		return nil
	}
	r.ParentRefs = sectionNameRefs

	// TCPRoute doesn't have hostnames like TLSRoute, so we skip hostname validation

	if len(tcpRoute.Spec.Rules) != 1 || len(tcpRoute.Spec.Rules[0].BackendRefs) != 1 {
		r.Valid = false
		cond := conditions.NewRouteBackendRefUnsupportedValue(
			"Must have exactly one Rule and BackendRef",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	br, conds := validateBackendRefTCPRoute(tcpRoute, services, r.ParentRefs, refGrantResolver)

	r.Spec.BackendRef = br
	r.Valid = true
	r.Attachable = true

	if len(conds) > 0 {
		r.Conditions = append(r.Conditions, conds...)
	}

	return r
}

func validateBackendRefTCPRoute(
	tcpRoute *v1alpha2.TCPRoute,
	services map[types.NamespacedName]*apiv1.Service,
	parentRefs []ParentRef,
	refGrantResolver func(resource toResource) bool,
) (BackendRef, []conditions.Condition) {
	// Length of BackendRefs and Rules is guaranteed to be one due to earlier check in buildTCPRoute
	refPath := field.NewPath("spec").Child("rules").Index(0).Child("backendRefs").Index(0)

	ref := tcpRoute.Spec.Rules[0].BackendRefs[0]

	if valid, cond := validateBackendRef(
		ref,
		tcpRoute.Namespace,
		refGrantResolver,
		refPath,
	); !valid {
		backendRef := BackendRef{
			Valid:              false,
			InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
		}

		return backendRef, []conditions.Condition{cond}
	}

	ns := tcpRoute.Namespace
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	svcNsName := types.NamespacedName{
		Namespace: ns,
		Name:      string(tcpRoute.Spec.Rules[0].BackendRefs[0].Name),
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

	// For TCPRoute, we don't need to validate app protocol compatibility
	// as TCP is protocol-agnostic at the application layer

	var conds []conditions.Condition
	for _, parentRef := range parentRefs {
		if err := verifyIPFamily(parentRef.Gateway.EffectiveNginxProxy, svcIPFamily); err != nil {
			backendRef.Valid = backendRef.Valid || false
			backendRef.InvalidForGateways[parentRef.Gateway.NamespacedName] = conditions.NewRouteInvalidIPFamily(err.Error())
		}
	}

	return backendRef, conds
}
