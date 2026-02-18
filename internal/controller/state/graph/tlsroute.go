package graph

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func buildTLSRoute(
	gtr *gatewayv1.TLSRoute,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*apiv1.Service,
	refGrantResolver func(resource toResource) bool,
) *L4Route {
	r := &L4Route{
		Source:    gtr,
		RouteType: RouteTypeTLS,
	}

	sectionNameRefs, err := buildSectionNameRefs(gtr.Spec.ParentRefs, gtr.Namespace, gws)
	if err != nil {
		r.Valid = false

		return r
	}
	// route doesn't belong to any of the Gateways
	if len(sectionNameRefs) == 0 {
		return nil
	}
	r.ParentRefs = sectionNameRefs

	if err := validateHostnames(
		gtr.Spec.Hostnames,
		field.NewPath("spec").Child("hostnames"),
	); err != nil {
		r.Valid = false
		condMsg := helpers.CapitalizeString(err.Error())
		r.Conditions = append(r.Conditions, conditions.NewRouteUnsupportedValue(condMsg))
		return r
	}

	r.Spec.Hostnames = gtr.Spec.Hostnames

	if len(gtr.Spec.Rules) != 1 || len(gtr.Spec.Rules[0].BackendRefs) != 1 {
		r.Valid = false
		cond := conditions.NewRouteBackendRefUnsupportedValue(
			"Must have exactly one Rule and BackendRef",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	br, conds := validateBackendRefTLSRoute(gtr, services, r.ParentRefs, refGrantResolver)

	r.Spec.BackendRef = br
	r.Valid = true
	r.Attachable = true

	if len(conds) > 0 {
		r.Conditions = append(r.Conditions, conds...)
	}

	return r
}

func validateBackendRefTLSRoute(
	gtr *gatewayv1.TLSRoute,
	services map[types.NamespacedName]*apiv1.Service,
	parentRefs []ParentRef,
	refGrantResolver func(resource toResource) bool,
) (BackendRef, []conditions.Condition) {
	// Length of BackendRefs and Rules is guaranteed to be one due to earlier check in buildTLSRoute
	refPath := field.NewPath("spec").Child("rules").Index(0).Child("backendRefs").Index(0)

	ref := gtr.Spec.Rules[0].BackendRefs[0]

	if valid, cond := validateBackendRef(
		ref,
		gtr.Namespace,
		refGrantResolver,
		refPath,
	); !valid {
		backendRef := BackendRef{
			Valid:              false,
			InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
		}

		return backendRef, []conditions.Condition{cond}
	}

	ns := gtr.Namespace
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	svcNsName := types.NamespacedName{
		Namespace: ns,
		Name:      string(gtr.Spec.Rules[0].BackendRefs[0].Name),
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

	if svcPort.AppProtocol != nil {
		err = validateRouteBackendRefAppProtocol(RouteTypeTLS, *svcPort.AppProtocol, nil)
		if err != nil {
			backendRef.Valid = false

			return backendRef, []conditions.Condition{conditions.NewRouteBackendRefUnsupportedProtocol(err.Error())}
		}
	}

	var conds []conditions.Condition
	for _, parentRef := range parentRefs {
		if err := verifyIPFamily(parentRef.Gateway.EffectiveNginxProxy, svcIPFamily); err != nil {
			backendRef.Valid = backendRef.Valid || false
			backendRef.InvalidForGateways[parentRef.Gateway.NamespacedName] = conditions.NewRouteInvalidIPFamily(err.Error())
		}
	}

	return backendRef, conds
}
