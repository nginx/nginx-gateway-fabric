package graph

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

func buildTLSRoute(
	gtr *v1alpha2.TLSRoute,
	gatewayNsNames []types.NamespacedName,
	services map[types.NamespacedName]*apiv1.Service,
	npCfg *NginxProxy,
	refGrantResolver func(resource toResource) bool,
) *L4Route {
	r := &L4Route{
		Source: gtr,
	}

	sectionNameRefs, err := buildSectionNameRefs(gtr.Spec.ParentRefs, gtr.Namespace, gatewayNsNames)
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
		r.Conditions = append(r.Conditions, staticConds.NewRouteUnsupportedValue(err.Error()))
		return r
	}

	r.Spec.Hostnames = gtr.Spec.Hostnames

	if len(gtr.Spec.Rules) != 1 || len(gtr.Spec.Rules[0].BackendRefs) != 1 {
		r.Valid = false
		cond := staticConds.NewRouteBackendRefUnsupportedValue(
			"Must have exactly one Rule and BackendRef",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	br, cond := validateBackendRefTLSRoute(gtr, services, npCfg, refGrantResolver)

	r.Spec.BackendRef = br
	r.Valid = true
	r.Attachable = true

	if cond != nil {
		r.Conditions = append(r.Conditions, *cond)
	}

	return r
}

func validateBackendRefTLSRoute(gtr *v1alpha2.TLSRoute,
	services map[types.NamespacedName]*apiv1.Service,
	npCfg *NginxProxy,
	refGrantResolver func(resource toResource) bool,
) (BackendRef, *conditions.Condition) {
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
			Valid: false,
		}

		return backendRef, &cond
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
		SvcNsName:   svcNsName,
		ServicePort: svcPort,
		Valid:       true,
	}

	if err != nil {
		backendRef.Valid = false

		return backendRef, helpers.GetPointer(staticConds.NewRouteBackendRefRefBackendNotFound(err.Error()))
	}

	if err := verifyIPFamily(npCfg, svcIPFamily); err != nil {
		backendRef.Valid = false

		return backendRef, helpers.GetPointer(staticConds.NewRouteInvalidIPFamily(err.Error()))
	}

	return backendRef, nil
}
