package graph

import (
	"fmt"
	"slices"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	apisx "sigs.k8s.io/gateway-api/apisx/v1alpha1"

	sort "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const (
	AppProtocolTypeH2C string = "kubernetes.io/h2c"
	AppProtocolTypeWS  string = "kubernetes.io/ws"
	AppProtocolTypeWSS string = "kubernetes.io/wss"
)

// BackendRef is an internal representation of a backendRef in an HTTP/GRPC/TLSRoute.
type BackendRef struct {
	// PodSelector holds the labels that select the pods of the referenced XBackend, if any. The data
	// plane must route to those pods directly; there is no Service in front of them.
	PodSelector map[string]string
	// BackendTLSPolicy is the BackendTLSPolicy of the Service which is referenced by the backendRef.
	BackendTLSPolicy *BackendTLSPolicy
	// InvalidForGateways is a map of Gateways for which this BackendRef is invalid for, with the corresponding
	// condition. Certain NginxProxy configurations may result in a backend not being valid for some Gateways,
	// but not others.
	InvalidForGateways map[types.NamespacedName]conditions.Condition
	// SessionPersistence is the SessionPersistenceConfig of the backendRef.
	SessionPersistence *SessionPersistenceConfig
	// SvcNsName is the NamespacedName of the Service referenced by the backendRef. For an XBackend
	// backendRef it holds the NamespacedName of the XBackend, which is also kept in XBackendNsName.
	SvcNsName types.NamespacedName
	// XBackendNsName is the NamespacedName of the XBackend referenced by the backendRef, if any.
	XBackendNsName types.NamespacedName
	// EndpointPickerConfig holds the configuration for the EndpointPicker for this backend.
	EndpointPickerConfig EndpointPickerConfig
	// ServicePort is the ServicePort of the Service which is referenced by the backendRef. It is empty
	// for an XBackend backendRef, which has no Service ports; see BackendPort instead.
	ServicePort v1.ServicePort
	// BackendPort is the port of the referenced XBackend to connect to, if any. It comes from
	// XBackend spec.port, which is where the Gateway API Backend API keeps the target port.
	BackendPort int32
	// Weight is the weight of the backendRef.
	Weight int32
	// Valid indicates whether the backendRef is valid.
	// No configuration should be generated for an invalid BackendRef.
	Valid bool
	// IsMirrorBackend indicates whether the BackendGroup is for a mirrored backend.
	IsMirrorBackend bool
	// IsExternalAuthBackend indicates whether this BackendRef is for an ExternalAuth filter backend.
	IsExternalAuthBackend bool
	// IsInferencePool indicates whether the BackendRef is for an InferencePool.
	IsInferencePool bool
	// IsXBackend indicates whether the BackendRef is for an XBackend, which has endpoints but no
	// Service in front of them.
	IsXBackend bool
}

// BaseServicePortKey returns a base unique string key for the Service port of the BackendRef.
func (b BackendRef) BaseServicePortKey() string {
	if b.IsXBackend {
		// An XBackend has no Service, so it must not produce the same key as a Service that happens to
		// share its namespace and name.
		return fmt.Sprintf("%s_%s_%d_xbackend", b.XBackendNsName.Namespace, b.XBackendNsName.Name, b.BackendPort)
	}

	return fmt.Sprintf("%s_%s_%d", b.SvcNsName.Namespace, b.SvcNsName.Name, b.ServicePort.Port)
}

// ServicePortReference returns a unique string reference for the Service port of the BackendRef including
// session persistence index if applicable.
func (b BackendRef) ServicePortReference() string {
	if !b.Valid {
		return ""
	}

	base := b.BaseServicePortKey()
	if b.SessionPersistence == nil || b.SessionPersistence.Name == "" {
		return base
	}

	return fmt.Sprintf("%s_%s", base, b.SessionPersistence.Idx)
}

func addBackendRefsToRouteRules(
	routes map[RouteKey]*L7Route,
	refGrantResolver *referenceGrantResolver,
	services map[types.NamespacedName]*v1.Service,
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
) {
	for _, r := range routes {
		addBackendRefsToRules(r, refGrantResolver, services, referencedInferencePools, referencedXBackends,
			backendTLSPolicies)
	}
}

// addHTTPBackendRefsToRules iterates over the rules of a Route and adds a list of BackendRef to each rule.
// If a reference in a rule is invalid, the function will add a condition to the rule.
func addBackendRefsToRules(
	route *L7Route,
	refGrantResolver *referenceGrantResolver,
	services map[types.NamespacedName]*v1.Service,
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
) {
	if !route.Valid {
		return
	}

	for idx, rule := range route.Spec.Rules {
		if !rule.ValidMatches {
			continue
		}
		if !rule.Filters.Valid {
			continue
		}

		// zero backendRefs is OK. For example, a rule can include a redirect filter.
		if len(rule.RouteBackendRefs) == 0 {
			continue
		}

		backendRefs := make([]BackendRef, 0, len(rule.RouteBackendRefs))

		for refIdx, ref := range rule.RouteBackendRefs {
			refPath := backendRefPath(idx, refIdx, ref)
			routeNs := route.Source.GetNamespace()

			// if we have an InferencePool backend disguised as a Service, set any necessary values
			if ref.IsInferencePool {
				updated, ok := resolveInferencePoolRef(
					route, ref, routeNs, referencedInferencePools, services, backendTLSPolicies)
				if !ok {
					continue
				}
				ref = updated
			}

			// if we have an XBackend backend, check that NGF can route to it and record why not if it
			// cannot. The resolved XBackend is passed on so that the BackendRef can carry the endpoint
			// selector and port instead of a Service.
			var resolvedXBackend *ReferencedXBackend
			if refIsXBackend(ref.BackendRef) {
				resolved, conds := resolveXBackendRef(ref, routeNs, referencedXBackends, refPath)
				route.Conditions = append(route.Conditions, conds...)
				resolvedXBackend = resolved
			}

			ref, conds := createBackendRef(
				ref,
				route,
				refGrantResolver.refAllowedFrom(getRefGrantFromResourceForRoute(route.RouteType, routeNs)),
				services,
				refPath,
				backendTLSPolicies,
				resolvedXBackend,
			)

			backendRefs = append(backendRefs, ref)
			if len(conds) > 0 {
				route.Conditions = append(route.Conditions, conds...)
			}
		}

		if len(backendRefs) > 1 {
			cond := validateBackendTLSPolicyMatchingAllBackends(backendRefs)
			if cond != nil {
				route.Conditions = append(route.Conditions, *cond)
				// mark all backendRefs as invalid
				for i := range backendRefs {
					backendRefs[i].Valid = false
				}
			}
		}
		route.Spec.Rules[idx].BackendRefs = backendRefs
		if cond := invalidateRuleIfExternalAuthBackendUnresolved(route, idx); cond != nil {
			route.Conditions = append(route.Conditions, *cond)
		}
	}
}

// backendRefPath returns the field path used for conditions on a single backendRef. For refs that
// originate from a mirror or external-auth filter, the path points at the filter's backendRef rather
// than the rule's backendRefs list.
func backendRefPath(ruleIdx, refIdx int, ref RouteBackendRef) *field.Path {
	basePath := field.NewPath("spec").Child("rules").Index(ruleIdx)
	if ref.MirrorBackendIdx != nil {
		return basePath.Child("filters").Index(*ref.MirrorBackendIdx).Child("backendRef")
	}
	if ref.ExternalAuthBackendIdx != nil {
		return basePath.Child("filters").Index(*ref.ExternalAuthBackendIdx).Child("externalAuth").Child("backendRef")
	}
	return basePath.Child("backendRefs").Index(refIdx)
}

// resolveInferencePoolRef enriches a ref that targets an InferencePool (disguised as a Service) with
// the pool's port and EndpointPicker configuration. It returns false, after recording a condition on
// the route, when the referenced InferencePool exists but is invalid, signaling that the ref should be
// skipped. When the pool is not found, the ref is returned unchanged.
func resolveInferencePoolRef(
	route *L7Route,
	ref RouteBackendRef,
	routeNs string,
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
	services map[types.NamespacedName]*v1.Service,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
) (RouteBackendRef, bool) {
	namespace := routeNs
	if ref.Namespace != nil {
		namespace = string(*ref.Namespace)
	}

	poolName := types.NamespacedName{
		Name:      ref.InferencePoolName,
		Namespace: namespace,
	}

	pool, exists := referencedInferencePools[poolName]
	if !exists {
		return ref, true
	}

	// If the InferencePool is invalid, add a condition to the route
	if !pool.Valid {
		route.Conditions = append(route.Conditions, conditions.NewRouteBackendRefInvalidInferencePool(
			fmt.Sprintf("Referenced InferencePool %s/%s is invalid",
				poolName.Namespace,
				poolName.Name,
			),
		))
		return ref, false
	}

	port := gatewayv1.PortNumber(pool.Source.Spec.TargetPorts[0].Number)
	ref.Port = helpers.GetPointer(port)
	ref.EndpointPickerConfig.EndpointPickerRef = pool.Source.Spec.EndpointPickerRef
	ref.EndpointPickerConfig.NsName = poolName.Namespace

	if pool.Source.Spec.EndpointPickerRef == nil {
		return ref, true
	}

	eppNsName := types.NamespacedName{
		Name:      string(pool.Source.Spec.EndpointPickerRef.Name),
		Namespace: poolName.Namespace,
	}

	eppSvcPort, err := getEPPServicePort(pool.Source, poolName.Namespace, services)
	if err != nil {
		route.Conditions = append(
			route.Conditions,
			conditions.NewRouteBackendRefUnsupportedValue(err.Error()),
		)
		return ref, false
	}
	if pool.Source.Spec.EndpointPickerRef.Port == nil {
		eppRef := *pool.Source.Spec.EndpointPickerRef
		eppRef.Port = &inference.Port{Number: inference.PortNumber(eppSvcPort.Port)}
		ref.EndpointPickerConfig.EndpointPickerRef = &eppRef
	}

	btp, losers, err := selectBackendTLSPolicyForService(
		backendTLSPolicies,
		nil,
		eppNsName.Name,
		eppNsName.Namespace,
		eppSvcPort,
	)
	for _, conflicted := range losers {
		conflicted.IsReferenced = true
		conflicted.Conditions = append(
			conflicted.Conditions,
			conditions.NewPolicyConflicted(
				"Conflicts with another BackendTLSPolicy targeting the same Service",
			),
		)
	}

	markBackendTLSPolicyAccepted(btp)

	if err != nil {
		route.Conditions = append(
			route.Conditions,
			conditions.NewRouteBackendRefUnsupportedValue(err.Error()),
		)
		return ref, false
	}

	ref.EndpointPickerConfig.BackendTLSPolicy = btp
	if btp == nil && route.RouteType == RouteTypeHTTP {
		mergeOrAppendRouteCondition(route, conditions.NewRouteEndpointPickerTLSVerificationSkipped())
	}

	return ref, true
}

// resolveXBackendRef resolves a backendRef that targets an XBackend. It returns the XBackend to use,
// together with the conditions that explain why it cannot be used, which is signaled by a nil
// XBackend: the referenced XBackend does not exist, or NGF cannot route to its type. A referenced
// XBackend that does not exist is still in the map, but with a nil Source, so the absence of the
// XBackend is told from an unusable one by the Source, not by the map. A backendRef that sets its own
// port does not get to choose the destination port - the Backend API keeps that on the XBackend - so
// such a reference keeps working but is warned about.
func resolveXBackendRef(
	ref RouteBackendRef,
	routeNs string,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
	refPath *field.Path,
) (*ReferencedXBackend, []conditions.Condition) {
	xbackendNsName := xbackendNsNameFromRef(ref.BackendRef, routeNs)

	refXBackend, exists := referencedXBackends[xbackendNsName]
	if !exists || refXBackend == nil || refXBackend.Source == nil {
		msg := fmt.Sprintf("Backend ref to XBackend %s not found", xbackendNsName)
		return nil, []conditions.Condition{
			conditions.NewRouteBackendRefRefBackendNotFound(
				field.NotFound(refPath.Child("name"), msg).Error(),
			),
		}
	}

	if !refXBackend.Valid {
		msg := fmt.Sprintf(
			"XBackend %s is not routable: NGF supports the %s backend type with a non-empty endpoint selector only",
			xbackendNsName,
			apisx.BackendTypeEndpointSelector,
		)

		return nil, []conditions.Condition{conditions.NewRouteBackendRefUnsupportedValue(msg)}
	}

	if ref.Port != nil && *ref.Port != int32(refXBackend.Source.Spec.Port.Number) {
		valErr := field.Forbidden(
			refPath.Child("port"),
			fmt.Sprintf(
				"Backend ref to XBackend %s port %d ignored, the XBackend selects port %d",
				xbackendNsName,
				*ref.Port,
				int32(refXBackend.Source.Spec.Port.Number),
			),
		)

		return refXBackend, []conditions.Condition{conditions.NewRouteBackendRefUnsupportedValue(valErr.Error())}
	}

	return refXBackend, nil
}

// invalidateRuleIfExternalAuthBackendUnresolved marks a rule's filters invalid when it carries an
// ExternalAuth filter but no resolved/valid ExternalAuth backend. Without this, the rule would still
// proxy to the primary backend with no auth check (fail-open). Returns the condition to append, or
// nil if no invalidation was needed.
func invalidateRuleIfExternalAuthBackendUnresolved(route *L7Route, ruleIdx int) *conditions.Condition {
	rule := &route.Spec.Rules[ruleIdx]
	if !rule.Filters.Valid {
		return nil
	}

	hasExternalAuth := false
	for _, f := range rule.Filters.Filters {
		if f.FilterType == FilterExternalAuth && f.ExternalAuth != nil {
			hasExternalAuth = true
			break
		}
	}
	if !hasExternalAuth {
		return nil
	}

	for _, br := range rule.BackendRefs {
		if br.IsExternalAuthBackend && br.Valid {
			return nil
		}
	}

	rule.Filters.Valid = false
	cond := conditions.NewRouteResolvedRefsInvalidFilter(
		"ExternalAuth filter references a backend that could not be resolved; " +
			"the rule is rejected to avoid proxying without authentication",
	)
	return &cond
}

func createBackendRef(
	ref RouteBackendRef,
	route *L7Route,
	refGrantResolver func(resource toResource) bool,
	services map[types.NamespacedName]*v1.Service,
	refPath *field.Path,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	resolvedXBackend *ReferencedXBackend,
) (BackendRef, []conditions.Condition) {
	// Data plane will handle invalid ref by responding with 500.
	// Because of that, we always need to add a BackendRef to group.Backends, even if the ref is invalid.
	// Additionally, we always calculate the weight, even if it is invalid.
	weight := int32(1)
	if ref.Weight != nil {
		if validateWeight(*ref.Weight) != nil {
			// We don't need to add a condition because validateHTTPBackendRef will do that.
			weight = 0 // 0 will get no traffic
		} else {
			weight = *ref.Weight
		}
	}

	valid, cond := validateRouteBackendRef(
		route.RouteType,
		ref,
		route.Source.GetNamespace(),
		refGrantResolver,
		refPath,
	)

	if !valid {
		backendRef := BackendRef{
			Weight:                weight,
			Valid:                 false,
			IsMirrorBackend:       ref.MirrorBackendIdx != nil,
			IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
			IsInferencePool:       ref.IsInferencePool,
			IsXBackend:            refIsXBackend(ref.BackendRef),
			InvalidForGateways:    make(map[types.NamespacedName]conditions.Condition),
			EndpointPickerConfig:  ref.EndpointPickerConfig,
		}

		return backendRef, []conditions.Condition{cond}
	}

	// An XBackend is not a Service: there is no Service port to look up, no ExternalName or DNS-form
	// name to accept, no BackendTLSPolicy to attach, and no AppProtocol to check. Everything the data
	// plane needs about the backend - which Pods are its endpoints and which port to send traffic to -
	// comes from the XBackend itself.
	if resolvedXBackend != nil {
		xbackendNsName := types.NamespacedName{
			Name:      resolvedXBackend.Source.Name,
			Namespace: resolvedXBackend.Source.Namespace,
		}

		return BackendRef{
			SvcNsName:             xbackendNsName,
			XBackendNsName:        xbackendNsName,
			PodSelector:           resolvedXBackend.PodSelector,
			BackendPort:           int32(resolvedXBackend.Source.Spec.Port.Number),
			Valid:                 true,
			Weight:                weight,
			IsMirrorBackend:       ref.MirrorBackendIdx != nil,
			IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
			IsXBackend:            true,
			EndpointPickerConfig:  ref.EndpointPickerConfig,
			SessionPersistence:    ref.SessionPersistence,
		}, nil
	}

	ns := route.Source.GetNamespace()
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	// The reference is an XBackend that NGF cannot use - it does not exist, or its type is not
	// supported. resolveXBackendRef already reported why, and looking the name up as a Service would
	// only add a second, misleading condition.
	if refIsXBackend(ref.BackendRef) {
		return BackendRef{
			SvcNsName:             xbackendNsNameFromRef(ref.BackendRef, route.Source.GetNamespace()),
			Weight:                weight,
			Valid:                 false,
			IsMirrorBackend:       ref.MirrorBackendIdx != nil,
			IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
			IsXBackend:            true,
			InvalidForGateways:    make(map[types.NamespacedName]conditions.Condition),
			EndpointPickerConfig:  ref.EndpointPickerConfig,
			SessionPersistence:    ref.SessionPersistence,
		}, nil
	}

	svcNsName := types.NamespacedName{Name: string(ref.Name), Namespace: ns}
	svcPort, err := getPortFromRef(ref.BackendRef, svcNsName, services, refPath)
	if err != nil {
		backendRef := BackendRef{
			Weight:                weight,
			Valid:                 false,
			SvcNsName:             svcNsName,
			ServicePort:           v1.ServicePort{},
			IsMirrorBackend:       ref.MirrorBackendIdx != nil,
			IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
			IsInferencePool:       ref.IsInferencePool,
			IsXBackend:            refIsXBackend(ref.BackendRef),
			InvalidForGateways:    make(map[types.NamespacedName]conditions.Condition),
			EndpointPickerConfig:  ref.EndpointPickerConfig,
		}

		return backendRef, []conditions.Condition{conditions.NewRouteBackendRefRefBackendNotFound(err.Error())}
	}

	var conds []conditions.Condition
	invalidForGateways := make(map[types.NamespacedName]conditions.Condition)

	// Check if this is an ExternalName service and validate DNS resolver configuration
	svc, svcExists := services[svcNsName]
	if svcExists && svc.Spec.Type == v1.ServiceTypeExternalName {
		invalidForGateways = checkExternalNameValidForGateways(route.ParentRefs, invalidForGateways)

		// Check if externalName field is empty or whitespace-only
		if strings.TrimSpace(svc.Spec.ExternalName) == "" {
			backendRef := BackendRef{
				SvcNsName:             svcNsName,
				ServicePort:           svcPort,
				Weight:                weight,
				Valid:                 false,
				IsMirrorBackend:       ref.MirrorBackendIdx != nil,
				IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
				IsInferencePool:       ref.IsInferencePool,
				IsXBackend:            refIsXBackend(ref.BackendRef),
				InvalidForGateways:    invalidForGateways,
				EndpointPickerConfig:  ref.EndpointPickerConfig,
			}

			return backendRef, append(conds, conditions.NewRouteBackendRefUnsupportedValue(
				"ExternalName service has empty or invalid externalName field",
			))
		}
	}

	backendTLSPolicy, err := findBackendTLSPolicyForService(
		backendTLSPolicies,
		ref.Namespace,
		string(ref.Name),
		route.Source.GetNamespace(),
		svcPort,
	)
	if err != nil {
		backendRef := BackendRef{
			SvcNsName:             svcNsName,
			ServicePort:           svcPort,
			Weight:                weight,
			Valid:                 false,
			IsMirrorBackend:       ref.MirrorBackendIdx != nil,
			IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
			IsInferencePool:       ref.IsInferencePool,
			IsXBackend:            refIsXBackend(ref.BackendRef),
			InvalidForGateways:    invalidForGateways,
			EndpointPickerConfig:  ref.EndpointPickerConfig,
		}

		return backendRef, append(conds, conditions.NewRouteBackendRefUnsupportedValue(err.Error()))
	}

	if svcPort.AppProtocol != nil {
		err = validateRouteBackendRefAppProtocol(route.RouteType, *svcPort.AppProtocol, backendTLSPolicy)
		if err != nil {
			backendRef := BackendRef{
				SvcNsName:             svcNsName,
				BackendTLSPolicy:      backendTLSPolicy,
				ServicePort:           svcPort,
				Weight:                weight,
				Valid:                 false,
				IsMirrorBackend:       ref.MirrorBackendIdx != nil,
				IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
				IsInferencePool:       ref.IsInferencePool,
				IsXBackend:            refIsXBackend(ref.BackendRef),
				InvalidForGateways:    invalidForGateways,
				EndpointPickerConfig:  ref.EndpointPickerConfig,
			}

			return backendRef, append(conds, conditions.NewRouteBackendRefUnsupportedProtocol(err.Error()))
		}
	}

	backendRef := BackendRef{
		SvcNsName:             svcNsName,
		BackendTLSPolicy:      backendTLSPolicy,
		ServicePort:           svcPort,
		Valid:                 true,
		Weight:                weight,
		IsMirrorBackend:       ref.MirrorBackendIdx != nil,
		IsExternalAuthBackend: ref.ExternalAuthBackendIdx != nil,
		IsInferencePool:       ref.IsInferencePool,
		IsXBackend:            refIsXBackend(ref.BackendRef),
		InvalidForGateways:    invalidForGateways,
		EndpointPickerConfig:  ref.EndpointPickerConfig,
		SessionPersistence:    ref.SessionPersistence,
	}

	return backendRef, conds
}

// validateBackendTLSPolicyMatchingAllBackends validates that all backends in a rule reference the same
// BackendTLSPolicy. We require that all backends in a group have the same backend TLS policy configuration.
// The backend TLS policy configuration is considered matching if: 1. CACertRefs reference the same ConfigMap, or
// 2. WellKnownCACerts are the same, and 3. Hostname is the same.
// FIXME (ciarams87): This is a temporary solution until we can support multiple backend TLS policies per group.
// https://github.com/nginx/nginx-gateway-fabric/issues/1546
func validateBackendTLSPolicyMatchingAllBackends(backendRefs []BackendRef) *conditions.Condition {
	var mismatch bool
	var nilPolicySeen bool
	var referencePolicy *BackendTLSPolicy

	checkPoliciesNotEqual := func(p1, p2 *gatewayv1.BackendTLSPolicy) bool {
		return !slices.Equal(p1.Spec.Validation.CACertificateRefs, p2.Spec.Validation.CACertificateRefs) ||
			p1.Spec.Validation.WellKnownCACertificates != p2.Spec.Validation.WellKnownCACertificates ||
			p1.Spec.Validation.Hostname != p2.Spec.Validation.Hostname
	}

	for _, backendRef := range backendRefs {
		if backendRef.BackendTLSPolicy == nil {
			if referencePolicy != nil {
				// There was a reference before, so they do not all match
				mismatch = true
				break
			}
			nilPolicySeen = true
			continue
		}

		if nilPolicySeen {
			mismatch = true
			break
		}

		if referencePolicy == nil {
			// First reference, store the policy as reference
			referencePolicy = backendRef.BackendTLSPolicy
		} else if checkPoliciesNotEqual(backendRef.BackendTLSPolicy.Source, referencePolicy.Source) {
			// Check if the policies match
			mismatch = true
			break
		}
	}
	if mismatch {
		msg := "Backend TLS policies do not match for all backends"
		return helpers.GetPointer(conditions.NewRouteBackendRefUnsupportedValue(msg))
	}
	return nil
}

// btpTargetsService reports whether the given targetRef targets a Service with the specified name and namespace.
func btpTargetsService(
	targetRef gatewayv1.LocalPolicyTargetReferenceWithSectionName,
	btpNs, refName, refNs string,
) bool {
	return string(targetRef.Name) == refName &&
		btpNs == refNs &&
		targetRef.Kind == kinds.Service &&
		(targetRef.Group == "" || targetRef.Group == "core")
}

// selectBackendTLSPolicyForService performs the pure lookup: it finds the BackendTLSPolicy targeting
// the given Service and port, resolving conflicts deterministically. It returns the winning policy,
// the losing (conflicting) policies, and an error if the winner is invalid. It does NOT mutate any
// policy status (IsReferenced/Conditions); callers decide how to record status so a single BackendTLSPolicy
// referenced by multiple backends (e.g. a Route backend AND a guardrails backend) is not marked twice.
func selectBackendTLSPolicyForService(
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	refNamespace *gatewayv1.Namespace,
	refName,
	routeNamespace string,
	servicePort v1.ServicePort,
) (winner *BackendTLSPolicy, losers []*BackendTLSPolicy, err error) {
	refNs := routeNamespace
	if refNamespace != nil {
		refNs = string(*refNamespace)
	}

	for _, btp := range backendTLSPolicies {
		btpNs := btp.Source.Namespace
		for _, targetRef := range btp.Source.Spec.TargetRefs {
			if !btpTargetsService(targetRef, btpNs, refName, refNs) {
				continue
			}
			// Check if this policy applies to the specific port we're interested in
			if targetRef.SectionName != nil {
				// Policy targets a specific port by name
				if servicePort.Name != string(*targetRef.SectionName) {
					// This policy targets a different port, skip it
					continue
				}
			}
			// Policy applies to all ports (no sectionName) or matches our port

			if winner == nil {
				winner = btp
			} else {
				// Found a conflict - determine which policy wins
				if sort.LessClientObject(btp.Source, winner.Source) {
					// btp wins, current winner loses
					losers = append(losers, winner)
					winner = btp
				} else {
					// current winner wins, btp loses
					losers = append(losers, btp)
				}
			}
		}
	}

	if winner != nil && !winner.Valid {
		//nolint:staticcheck // Capitalization required for alignment with other messages.
		err = fmt.Errorf("The BackendTLSPolicy is invalid: %s", winner.Conditions[0].Message)
	}

	return winner, losers, err
}

// findBackendTLSPolicyForService selects the BackendTLSPolicy for a Service/port (see
// selectBackendTLSPolicyForService) and records status on the affected policies: losers get a
// Conflicted condition; a valid winner gets an Accepted condition. Used by the Route/TLSRoute backend
// paths, which own BackendTLSPolicy status.
func findBackendTLSPolicyForService(
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	refNamespace *gatewayv1.Namespace,
	refName,
	routeNamespace string,
	servicePort v1.ServicePort,
) (*BackendTLSPolicy, error) {
	beTLSPolicy, conflictingPolicies, err := selectBackendTLSPolicyForService(
		backendTLSPolicies,
		refNamespace,
		refName,
		routeNamespace,
		servicePort,
	)

	// Set conflicted conditions on losing policies
	for _, conflictedPolicy := range conflictingPolicies {
		conflictedPolicy.IsReferenced = true
		conflictedPolicy.Conditions = append(conflictedPolicy.Conditions,
			conditions.NewPolicyConflicted("Conflicts with another BackendTLSPolicy targeting the same Service"))
	}

	if beTLSPolicy != nil {
		beTLSPolicy.IsReferenced = true
		if beTLSPolicy.Valid {
			beTLSPolicy.Conditions = append(beTLSPolicy.Conditions, conditions.NewPolicyAccepted())
		}
	}

	return beTLSPolicy, err
}

// markBackendTLSPolicyAccepted records that a valid BackendTLSPolicy is referenced and accepted,
// idempotently: it will not append a duplicate Accepted condition. This lets a non-Route referrer
// (the guardrails/PayloadProcessor backend path) ensure a BackendTLSPolicy that is otherwise only
// referenced by it still gets Accepted/IsReferenced status, without double-appending when the Route
// backend path already recorded the same policy.
func markBackendTLSPolicyAccepted(btp *BackendTLSPolicy) {
	if btp == nil {
		return
	}
	btp.IsReferenced = true
	if !btp.Valid {
		return
	}
	for _, c := range btp.Conditions {
		if c.Reason == conditions.NewPolicyAccepted().Reason {
			return
		}
	}
	btp.Conditions = append(btp.Conditions, conditions.NewPolicyAccepted())
}

// getPortFromRef extracts the port from a BackendRef.
// It can return an error and an empty v1.ServicePort in two cases:
// 1. The Service referenced from the BackendRef does not exist in the cluster/state.
// 2. The Port on the BackendRef does not match any of the ServicePorts on the Service.
func getPortFromRef(
	ref gatewayv1.BackendRef,
	svcNsName types.NamespacedName,
	services map[types.NamespacedName]*v1.Service,
	refPath *field.Path,
) (v1.ServicePort, error) {
	svc, ok := services[svcNsName]
	if !ok {
		return v1.ServicePort{}, field.NotFound(refPath.Child("name"), ref.Name)
	}

	// safe to dereference port here because we already validated that the port is not nil in validateBackendRef.
	svcPort, err := getServicePort(svc, *ref.Port)
	if err != nil {
		return v1.ServicePort{}, err
	}

	return svcPort, nil
}

func checkExternalNameValidForGateways(
	parentRefs []ParentRef,
	invalidForGateways map[types.NamespacedName]conditions.Condition,
) map[types.NamespacedName]conditions.Condition {
	for _, parentRef := range parentRefs {
		if parentRef.EffectiveNginxProxy == nil ||
			parentRef.EffectiveNginxProxy.DNSResolver == nil {
			invalidForGateways[parentRef.GatewayNsName] = conditions.NewRouteBackendRefUnsupportedValue(
				"ExternalName service requires DNS resolver configuration in Gateway's NginxProxy",
			)
		}
	}
	return invalidForGateways
}

func validateRouteBackendRef(
	routeType RouteType,
	ref RouteBackendRef,
	routeNs string,
	refGrantResolver func(resource toResource) bool,
	path *field.Path,
) (valid bool, cond conditions.Condition) {
	// Because all errors cause the same condition but different reasons, we return as soon as we find an error
	if len(ref.Filters) > 0 {
		valErr := field.TooMany(path.Child("filters"), len(ref.Filters), 0)
		return false, conditions.NewRouteBackendRefUnsupportedValue(valErr.Error())
	}

	if routeType == RouteTypeHTTP {
		return validateBackendRefHTTPRoute(ref, routeNs, refGrantResolver, path)
	}

	return validateBackendRef(ref.BackendRef, routeNs, refGrantResolver, path)
}

func validateBackendRef(
	ref gatewayv1.BackendRef,
	routeNs string,
	refGrantResolver func(toResource toResource) bool,
	path *field.Path,
) (valid bool, cond conditions.Condition) {
	// Because all errors cause same condition but different reasons, we return as soon as we find an error

	// An XBackend is neither in the core group nor a Service, so it is handled before the Service
	// checks - and it brings its own port, so the port requirement below does not apply to it.
	if refIsXBackend(ref) {
		return validateXBackendBackendRef(ref, routeNs, path)
	}

	if ref.Group != nil && (*ref.Group != "core" && *ref.Group != "") {
		valErr := field.NotSupported(path.Child("group"), *ref.Group, []string{"core", ""})
		return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
	}

	if ref.Kind != nil && *ref.Kind != "Service" {
		valErr := field.NotSupported(path.Child("kind"), *ref.Kind, []string{"Service"})
		return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
	}

	// no need to validate ref.Name

	if ref.Namespace != nil && string(*ref.Namespace) != routeNs {
		refNsName := types.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}

		if !refGrantResolver(toService(refNsName)) {
			msg := fmt.Sprintf("Backend ref to Service %s not permitted by any ReferenceGrant", refNsName)
			valErr := field.Forbidden(path.Child("namespace"), msg)

			return false, conditions.NewRouteBackendRefRefNotPermitted(valErr.Error())
		}
	}

	if ref.Port == nil {
		valErr := field.Required(path.Child("port"), "port cannot be nil")
		return false, conditions.NewRouteBackendRefUnsupportedValue(valErr.Error())
	}

	// any value of port is OK

	if ref.Weight != nil {
		if err := validateWeight(*ref.Weight); err != nil {
			valErr := field.Invalid(path.Child("weight"), *ref.Weight, err.Error())
			return false, conditions.NewRouteBackendRefUnsupportedValue(valErr.Error())
		}
	}

	return true, conditions.Condition{}
}

func validateBackendRefHTTPRoute(
	ref RouteBackendRef,
	routeNs string,
	refGrantResolver func(toResource toResource) bool,
	path *field.Path,
) (valid bool, cond conditions.Condition) {
	// Because all errors cause same condition but different reasons, we return as soon as we find an error

	if valid, cond := validateBackendRefHTTPRouteGroupKind(ref.BackendRef, path); !valid {
		return false, cond
	}

	// no need to validate ref.Name

	if ref.Namespace != nil && string(*ref.Namespace) != routeNs {
		if valid, cond := validateBackendRefHTTPRouteNamespace(ref, routeNs, refGrantResolver, path); !valid {
			return false, cond
		}
	}

	if ref.Port == nil && (ref.Kind == nil || *ref.Kind == kinds.Service) {
		valErr := field.Required(path.Child("port"), "port cannot be nil")
		return false, conditions.NewRouteBackendRefUnsupportedValue(valErr.Error())
	}

	// any value of port is OK

	if ref.Weight != nil {
		if err := validateWeight(*ref.Weight); err != nil {
			valErr := field.Invalid(path.Child("weight"), *ref.Weight, err.Error())
			return false, conditions.NewRouteBackendRefUnsupportedValue(valErr.Error())
		}
	}

	return true, conditions.Condition{}
}

// validateBackendRefHTTPRouteNamespace validates a backendRef that references a backend in another
// namespace, which is allowed only if a ReferenceGrant permits it.
func validateBackendRefHTTPRouteNamespace(
	ref RouteBackendRef,
	routeNs string,
	refGrantResolver func(toResource toResource) bool,
	path *field.Path,
) (bool, conditions.Condition) {
	var inferencePool bool
	var inferencePoolName types.NamespacedName

	switch {
	case refIsXBackend(ref.BackendRef):
		// A ReferenceGrant cannot name an XBackend as a to: kind, so no grant can permit this.
		return validateXBackendBackendRef(ref.BackendRef, routeNs, path)
	case ref.Kind != nil && *ref.Kind == kinds.InferencePool:
		inferencePool = true
		inferencePoolName = types.NamespacedName{
			Namespace: string(*ref.Namespace),
			Name:      string(ref.Name),
		}
	case ref.IsInferencePool:
		// Case where RouteBackendRef has been updated with headless Service backend for the InferencePool
		inferencePool = true
		inferencePoolName = types.NamespacedName{
			Namespace: string(*ref.Namespace),
			Name:      ref.InferencePoolName,
		}
	default:
		refNsName := types.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}

		if !refGrantResolver(toService(refNsName)) {
			msg := fmt.Sprintf("Backend ref to Service %s not permitted by any ReferenceGrant", refNsName)
			valErr := field.Forbidden(path.Child("namespace"), msg)

			return false, conditions.NewRouteBackendRefRefNotPermitted(valErr.Error())
		}
	}

	if inferencePool && !refGrantResolver(toInferencePool(inferencePoolName)) {
		msg := fmt.Sprintf("Backend ref to InferencePool %s not permitted by any ReferenceGrant", inferencePoolName)
		valErr := field.Forbidden(path.Child("namespace"), msg)

		return false, conditions.NewRouteBackendRefRefNotPermitted(valErr.Error())
	}

	return true, conditions.Condition{}
}

// validateXBackendBackendRef validates the parts of a cross-namespace XBackend backendRef that NGF can
// check before the XBackend itself is resolved. A ReferenceGrant cannot authorize a backendRef to an
// XBackend because the API only allows a grant to name a Service as its to: kind, so any reference that
// leaves the Route's namespace is rejected.
func validateXBackendBackendRef(
	ref gatewayv1.BackendRef,
	routeNs string,
	path *field.Path,
) (bool, conditions.Condition) {
	if ref.Namespace != nil && string(*ref.Namespace) != routeNs {
		refNsName := types.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}

		msg := fmt.Sprintf("Cross-namespace backend ref to XBackend %s is not supported", refNsName)
		valErr := field.Forbidden(path.Child("namespace"), msg)

		return false, conditions.NewRouteBackendRefRefNotPermitted(valErr.Error())
	}

	return true, conditions.Condition{}
}

// supportedBackendRefGroups and supportedBackendRefKinds list the group and kind values that a backendRef
// of an L7 Route may use. They are used both for validation and for the messages shown to the user.
var (
	supportedBackendRefGroups = []string{"core", "", inferenceAPIGroup, apisx.GroupName}
	supportedBackendRefKinds  = []string{kinds.Service, kinds.InferencePool, kinds.XBackend}
)

// backendRefGroupKindPairs holds the group and kind of every backend that a Route can reference using a
// group other than the core group. A backendRef that sets one of them must set both consistently. Service
// needs no entry because both its group and kind default to the core group.
var backendRefGroupKindPairs = []struct {
	group gatewayv1.Group
	kind  gatewayv1.Kind
}{
	{inferenceAPIGroup, kinds.InferencePool},
	{apisx.GroupName, kinds.XBackend},
}

func validateBackendRefHTTPRouteGroupKind(
	ref gatewayv1.BackendRef,
	path *field.Path,
) (bool, conditions.Condition) {
	if valid, cond := validateBackendRefHTTPRouteGroup(ref, path); !valid {
		return false, cond
	}

	return validateBackendRefHTTPRouteKind(ref, path)
}

// validateBackendRefHTTPRouteGroup validates the group of a backendRef of an L7 Route and that it matches
// the kind of the backendRef.
func validateBackendRefHTTPRouteGroup(
	ref gatewayv1.BackendRef,
	path *field.Path,
) (bool, conditions.Condition) {
	if ref.Group == nil {
		return true, conditions.Condition{}
	}

	group := *ref.Group
	if !slices.Contains(supportedBackendRefGroups, string(group)) {
		valErr := field.NotSupported(path.Child("group"), group, supportedBackendRefGroups)
		return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
	}

	for _, pair := range backendRefGroupKindPairs {
		if group != pair.group {
			continue
		}

		if ref.Kind == nil || *ref.Kind != pair.kind {
			msg := fmt.Sprintf("kind must be %s when group is %s", pair.kind, pair.group)
			valErr := field.Invalid(path.Child("kind"), ref.Kind, msg)

			return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
		}
	}

	return true, conditions.Condition{}
}

// validateBackendRefHTTPRouteKind validates the kind of a backendRef of an L7 Route and that it matches
// the group of the backendRef.
func validateBackendRefHTTPRouteKind(
	ref gatewayv1.BackendRef,
	path *field.Path,
) (bool, conditions.Condition) {
	if ref.Kind == nil {
		return true, conditions.Condition{}
	}

	kind := *ref.Kind
	if !slices.Contains(supportedBackendRefKinds, string(kind)) {
		valErr := field.NotSupported(path.Child("kind"), kind, supportedBackendRefKinds)
		return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
	}

	for _, pair := range backendRefGroupKindPairs {
		if kind != pair.kind {
			continue
		}

		if ref.Group == nil || *ref.Group != pair.group {
			msg := fmt.Sprintf("group must be %s when kind is %s", pair.group, pair.kind)
			valErr := field.Invalid(path.Child("group"), ref.Group, msg)

			return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
		}
	}

	return true, conditions.Condition{}
}

// validateRouteBackendRefAppProtocol checks if a given RouteType supports sending traffic to a service AppProtocol.
// Returns nil if true or AppProtocol is not a Kubernetes Standard Application Protocol.
func validateRouteBackendRefAppProtocol(
	routeType RouteType,
	appProtocol string,
	backendTLSPolicy *BackendTLSPolicy,
) error {
	//nolint: staticcheck // used in status condition which is normally capitalized
	err := fmt.Errorf(
		"The Route type %s does not support service port appProtocol %s",
		routeType,
		appProtocol,
	)

	// Currently we only support recognition of the Kubernetes Standard Application Protocols defined in KEP-3726.
	switch appProtocol {
	case AppProtocolTypeH2C:
		if routeType == RouteTypeGRPC || routeType == RouteTypeHTTP {
			return nil
		}

		return err
	case AppProtocolTypeWS:
		if routeType == RouteTypeHTTP {
			return nil
		}

		return err
	case AppProtocolTypeWSS:
		if routeType == RouteTypeHTTP {
			if backendTLSPolicy != nil {
				return nil
			}

			return fmt.Errorf("%w; missing corresponding BackendTLSPolicy", err)
		}

		if routeType == RouteTypeTLS {
			return nil
		}

		return err
	}

	return nil
}

func validateWeight(weight int32) error {
	const (
		minWeight = 0
		maxWeight = 1_000_000
	)

	if weight < minWeight || weight > maxWeight {
		return fmt.Errorf("must be in the range [%d, %d]", minWeight, maxWeight)
	}

	return nil
}

func getServicePort(svc *v1.Service, port int32) (v1.ServicePort, error) {
	for _, p := range svc.Spec.Ports {
		if p.Port == port {
			return p, nil
		}
	}
	//nolint: staticcheck // used in status condition which is normally capitalized
	return v1.ServicePort{}, fmt.Errorf("No matching port for Service %s and port %d", svc.Name, port)
}

func getRefGrantFromResourceForRoute(routeType RouteType, routeNs string) fromResource {
	switch routeType {
	case RouteTypeHTTP:
		return fromHTTPRoute(routeNs)
	case RouteTypeGRPC:
		return fromGRPCRoute(routeNs)
	default:
		panic(fmt.Errorf("unknown route type %s", routeType))
	}
}
