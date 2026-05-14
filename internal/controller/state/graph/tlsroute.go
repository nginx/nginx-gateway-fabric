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

	if len(gtr.Spec.Rules) != 1 || len(gtr.Spec.Rules[0].BackendRefs) != 1 {
		r.Valid = false
		cond := conditions.NewRouteBackendRefUnsupportedValue(
			"Must have exactly one Rule and BackendRef",
		)
		r.Conditions = append(r.Conditions, cond)
		return r
	}

	tlsTerminateMode := hasTLSTerminateParent(sectionNameRefs, gws, listenerSets)
	br, conds := validateBackendRefTLSRoute(gtr, services, backendTLSPolicies, tlsTerminateMode, refGrantResolver)

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
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	tlsTerminateMode bool,
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

	svcPort, err := getPortFromRef(
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

	backendTLSPolicy, err := findBackendTLSPolicyForService(
		backendTLSPolicies,
		ref.Namespace,
		string(ref.Name),
		gtr.Namespace,
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
