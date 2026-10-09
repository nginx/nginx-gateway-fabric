package graph

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func buildTLSRoute(
	gtr *gatewayv1.TLSRoute,
	gws map[types.NamespacedName]*Gateway,
	services map[types.NamespacedName]*apiv1.Service,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	refGrantResolver func(resource toResource) bool,
	listenerSets map[types.NamespacedName]*ListenerSet,
) *L4Route {
	r := &L4Route{
		Source:    gtr,
		RouteType: RouteTypeTLS,
	}

	sectionNameRefs, err := buildSectionNameRefs(gtr.Spec.ParentRefs, gtr.Namespace, gws, listenerSets)
	if err != nil {
		r.Valid = false

		return r
	}
	// route doesn't belong to any of the Gateways or ListenerSets
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

	if len(gtr.Spec.Rules) != 1 {
		r.Valid = false
		cond := conditions.NewRouteBackendRefUnsupportedValue(
			"Must have exactly one Rule",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	specRule := gtr.Spec.Rules[0]
	if len(specRule.BackendRefs) < 1 || len(specRule.BackendRefs) > 16 {
		r.Valid = false
		cond := conditions.NewRouteBackendRefUnsupportedValue(
			"Must have between 1 and 16 BackendRefs",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	tlsTerminateMode := hasTLSTerminateParent(sectionNameRefs, gws, listenerSets)
	br, conds := processTLSRouteRule(
		specRule,
		gtr.Namespace,
		services,
		backendTLSPolicies,
		tlsTerminateMode,
		refGrantResolver,
	)

	r.Spec.BackendRefs = br
	r.Valid = true
	r.Attachable = true

	if len(conds) > 0 {
		r.Conditions = append(r.Conditions, conds...)
	}

	return r
}

func processTLSRouteRule(
	specRule gatewayv1.TLSRouteRule,
	routeNamespace string,
	services map[types.NamespacedName]*apiv1.Service,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	tlsTerminateMode bool,
	refGrantResolver func(resource toResource) bool,
) ([]BackendRef, []conditions.Condition) {
	rulePath := field.NewPath("spec").Child("rules").Index(0)

	backendRefs, conds := getBackendRefsTLSRoute(
		specRule,
		routeNamespace,
		rulePath,
		services,
		backendTLSPolicies,
		tlsTerminateMode,
		refGrantResolver,
	)

	return backendRefs, conds
}

func getBackendRefsTLSRoute(
	specRule gatewayv1.TLSRouteRule,
	routeNamespace string,
	rulePath *field.Path,
	services map[types.NamespacedName]*apiv1.Service,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	tlsTerminateMode bool,
	refGrantResolver func(resource toResource) bool,
) ([]BackendRef, []conditions.Condition) {
	backendRefs := make([]BackendRef, 0, len(specRule.BackendRefs))
	var conds []conditions.Condition

	for i, b := range specRule.BackendRefs {
		refPath := rulePath.Child("backendRefs").Index(i)

		backendRef, cond := validateBackendRefTLSRoute(
			b,
			routeNamespace,
			refPath,
			services,
			backendTLSPolicies,
			tlsTerminateMode,
			refGrantResolver,
		)
		backendRefs = append(backendRefs, backendRef)
		conds = append(conds, cond...)
	}

	if len(backendRefs) > 1 {
		cond := validateBackendTLSPolicyMatchingAllBackends(backendRefs)
		if cond != nil {
			conds = append(conds, *cond)
			// mark all backendRefs as invalid
			for i := range backendRefs {
				backendRefs[i].Valid = false
			}
		}
	}

	return backendRefs, conds
}

func validateBackendRefTLSRoute(
	ref gatewayv1.BackendRef,
	routeNamespace string,
	refPath *field.Path,
	services map[types.NamespacedName]*apiv1.Service,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	tlsTerminateMode bool,
	refGrantResolver func(resource toResource) bool,
) (BackendRef, []conditions.Condition) {
	weight := int32(1)
	if ref.Weight != nil {
		weight = *ref.Weight
	}

	if valid, cond := validateBackendRef(
		ref,
		routeNamespace,
		refGrantResolver,
		refPath,
	); !valid {
		backendRef := BackendRef{
			Valid:              false,
			InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
		}

		return backendRef, []conditions.Condition{cond}
	}

	ns := routeNamespace
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	svcNsName := types.NamespacedName{
		Namespace: ns,
		Name:      string(ref.Name),
	}

	svcPort, err := getPortFromRef(
		ref,
		svcNsName,
		services,
		refPath,
	)

	backendRef := BackendRef{
		SvcNsName:          svcNsName,
		ServicePort:        svcPort,
		Weight:             weight,
		Valid:              true,
		InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
	}

	if err != nil {
		backendRef.Valid = false

		return backendRef, []conditions.Condition{conditions.NewRouteBackendRefRefBackendNotFound(err.Error())}
	}

	backendTLSPolicy, err := findBackendTLSPolicyForService(
		backendTLSPolicies,
		ref.Namespace,
		string(ref.Name),
		routeNamespace,
		svcPort,
	)
	backendRef.BackendTLSPolicy = backendTLSPolicy
	if err != nil {
		backendRef.Valid = false

		return backendRef, []conditions.Condition{conditions.NewRouteBackendRefUnsupportedValue(err.Error())}
	}

	if svcPort.AppProtocol != nil {
		err = validateRouteBackendRefAppProtocol(RouteTypeTLS, *svcPort.AppProtocol, backendTLSPolicy)
		if err == nil &&
			tlsTerminateMode &&
			*svcPort.AppProtocol == AppProtocolTypeWSS &&
			backendTLSPolicy == nil {
			//nolint: staticcheck // used in status condition which is normally capitalized
			err = fmt.Errorf(
				"The Route type %s does not support service port appProtocol %s; missing corresponding BackendTLSPolicy",
				RouteTypeTLS,
				*svcPort.AppProtocol,
			)
		}
		if err != nil {
			backendRef.Valid = false

			return backendRef, []conditions.Condition{conditions.NewRouteBackendRefUnsupportedProtocol(err.Error())}
		}
	}

	return backendRef, nil
}

func hasTLSTerminateParent(
	parentRefs []ParentRef,
	gws map[types.NamespacedName]*Gateway,
	listenerSets map[types.NamespacedName]*ListenerSet,
) bool {
	for _, parentRef := range parentRefs {
		switch parentRef.Kind {
		case kinds.Gateway:
			gw, exists := gws[parentRef.NamespacedName]
			if !exists || gw == nil {
				continue
			}
			if hasTLSTerminateListener(gw.Listeners, parentRef.SectionName) {
				return true
			}
		case kinds.ListenerSet:
			ls, exists := listenerSets[parentRef.NamespacedName]
			if !exists || ls == nil {
				continue
			}
			if hasTLSTerminateListener(ls.Listeners, parentRef.SectionName) {
				return true
			}
		}
	}

	return false
}

func hasTLSTerminateListener(listeners []*Listener, sectionName *gatewayv1.SectionName) bool {
	for _, listener := range listeners {
		if listener == nil || listener.Source.Protocol != gatewayv1.TLSProtocolType {
			continue
		}

		if sectionName != nil && listener.Name != string(*sectionName) {
			continue
		}

		if listener.Source.TLS != nil &&
			(listener.Source.TLS.Mode == nil || *listener.Source.TLS.Mode == gatewayv1.TLSModeTerminate) {
			return true
		}
	}

	return false
}
