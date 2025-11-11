package graph

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/sort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
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
	// BackendTLSPolicy is the BackendTLSPolicy of the Service which is referenced by the backendRef.
	BackendTLSPolicy *BackendTLSPolicy
	// EndpointPickerConfig holds the configuration for the EndpointPicker for this backend.
	EndpointPickerConfig EndpointPickerConfig
	// InvalidForGateways is a map of Gateways for which this BackendRef is invalid for, with the corresponding
	// condition. Certain NginxProxy configurations may result in a backend not being valid for some Gateways,
	// but not others.
	InvalidForGateways map[types.NamespacedName]conditions.Condition
	// SvcNsName is the NamespacedName of the Service referenced by the backendRef.
	SvcNsName types.NamespacedName
	// ServicePort is the ServicePort of the Service which is referenced by the backendRef.
	ServicePort v1.ServicePort
	// Weight is the weight of the backendRef.
	Weight int32
	// Valid indicates whether the backendRef is valid.
	// No configuration should be generated for an invalid BackendRef.
	Valid bool
	// IsMirrorBackend indicates whether the BackendGroup is for a mirrored backend.
	IsMirrorBackend bool
	// IsInferencePool indicates whether the BackendRef is for an InferencePool.
	IsInferencePool bool
}

// ServicePortReference returns a string representation for the service and port that is referenced by the BackendRef.
func (b BackendRef) ServicePortReference() string {
	if !b.Valid {
		return ""
	}
	return fmt.Sprintf("%s_%s_%d", b.SvcNsName.Namespace, b.SvcNsName.Name, b.ServicePort.Port)
}

func addBackendRefsToRouteRules(
	routes map[RouteKey]*L7Route,
	refGrantResolver *referenceGrantResolver,
	services map[types.NamespacedName]*v1.Service,
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
) {
	for _, r := range routes {
		addBackendRefsToRules(r, refGrantResolver, services, referencedInferencePools, backendTLSPolicies)
	}
}

// addHTTPBackendRefsToRules iterates over the rules of a Route and adds a list of BackendRef to each rule.
// If a reference in a rule is invalid, the function will add a condition to the rule.
//
//nolint:gocyclo
func addBackendRefsToRules(
	route *L7Route,
	refGrantResolver *referenceGrantResolver,
	services map[types.NamespacedName]*v1.Service,
	referencedInferencePools map[types.NamespacedName]*ReferencedInferencePool,
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
			basePath := field.NewPath("spec").Child("rules").Index(idx)
			refPath := basePath.Child("backendRefs").Index(refIdx)
			if ref.MirrorBackendIdx != nil {
				refPath = basePath.Child("filters").Index(*ref.MirrorBackendIdx).Child("backendRef")
			}
			routeNs := route.Source.GetNamespace()

			// if we have an InferencePool backend disguised as a Service, set the port value
			if ref.IsInferencePool {
				namespace := routeNs
				if ref.Namespace != nil {
					namespace = string(*ref.Namespace)
				}

				poolName := types.NamespacedName{
					Name:      controller.GetInferencePoolName(string(ref.Name)),
					Namespace: namespace,
				}

				if pool, exists := referencedInferencePools[poolName]; exists {
					// If the InferencePool is invalid, add a condition to the route
					if !pool.Valid {
						route.Conditions = append(route.Conditions, conditions.NewRouteBackendRefInvalidInferencePool(
							fmt.Sprintf("Referenced InferencePool %s/%s is invalid",
								poolName.Namespace,
								poolName.Name,
							),
						))
						continue
					}
					port := gatewayv1.PortNumber(pool.Source.Spec.TargetPorts[0].Number)
					ref.Port = helpers.GetPointer(port)
					ref.EndpointPickerConfig.EndpointPickerRef = &pool.Source.Spec.EndpointPickerRef
					ref.EndpointPickerConfig.NsName = poolName.Namespace
				}
			}

			ref, conds := createBackendRef(
				ref,
				route,
				refGrantResolver.refAllowedFrom(getRefGrantFromResourceForRoute(route.RouteType, routeNs)),
				services,
				refPath,
				backendTLSPolicies,
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
	}
}

func createBackendRef(
	ref RouteBackendRef,
	route *L7Route,
	refGrantResolver func(resource toResource) bool,
	services map[types.NamespacedName]*v1.Service,
	refPath *field.Path,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
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
			Weight:               weight,
			Valid:                false,
			IsMirrorBackend:      ref.MirrorBackendIdx != nil,
			IsInferencePool:      ref.IsInferencePool,
			InvalidForGateways:   make(map[types.NamespacedName]conditions.Condition),
			EndpointPickerConfig: ref.EndpointPickerConfig,
		}

		return backendRef, []conditions.Condition{cond}
	}

	ns := route.Source.GetNamespace()
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}
	svcNsName := types.NamespacedName{Name: string(ref.Name), Namespace: ns}
	svcIPFamily, svcPort, err := getIPFamilyAndPortFromRef(ref.BackendRef, svcNsName, services, refPath)
	if err != nil {
		backendRef := BackendRef{
			Weight:               weight,
			Valid:                false,
			SvcNsName:            svcNsName,
			ServicePort:          v1.ServicePort{},
			IsMirrorBackend:      ref.MirrorBackendIdx != nil,
			IsInferencePool:      ref.IsInferencePool,
			InvalidForGateways:   make(map[types.NamespacedName]conditions.Condition),
			EndpointPickerConfig: ref.EndpointPickerConfig,
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
				SvcNsName:            svcNsName,
				ServicePort:          svcPort,
				Weight:               weight,
				Valid:                false,
				IsMirrorBackend:      ref.MirrorBackendIdx != nil,
				IsInferencePool:      ref.IsInferencePool,
				InvalidForGateways:   invalidForGateways,
				EndpointPickerConfig: ref.EndpointPickerConfig,
			}

			return backendRef, append(conds, conditions.NewRouteBackendRefUnsupportedValue(
				"ExternalName service has empty or invalid externalName field",
			))
		}
	}

	for _, parentRef := range route.ParentRefs {
		if err := verifyIPFamily(parentRef.Gateway.EffectiveNginxProxy, svcIPFamily); err != nil {
			invalidForGateways[parentRef.Gateway.NamespacedName] = conditions.NewRouteInvalidIPFamily(err.Error())
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
			SvcNsName:            svcNsName,
			ServicePort:          svcPort,
			Weight:               weight,
			Valid:                false,
			IsMirrorBackend:      ref.MirrorBackendIdx != nil,
			IsInferencePool:      ref.IsInferencePool,
			InvalidForGateways:   invalidForGateways,
			EndpointPickerConfig: ref.EndpointPickerConfig,
		}

		return backendRef, append(conds, conditions.NewRouteBackendRefUnsupportedValue(err.Error()))
	}

	if svcPort.AppProtocol != nil {
		err = validateRouteBackendRefAppProtocol(route.RouteType, *svcPort.AppProtocol, backendTLSPolicy)
		if err != nil {
			backendRef := BackendRef{
				SvcNsName:            svcNsName,
				BackendTLSPolicy:     backendTLSPolicy,
				ServicePort:          svcPort,
				Weight:               weight,
				Valid:                false,
				IsMirrorBackend:      ref.MirrorBackendIdx != nil,
				IsInferencePool:      ref.IsInferencePool,
				InvalidForGateways:   invalidForGateways,
				EndpointPickerConfig: ref.EndpointPickerConfig,
			}

			return backendRef, append(conds, conditions.NewRouteBackendRefUnsupportedProtocol(err.Error()))
		}
	}

	backendRef := BackendRef{
		SvcNsName:            svcNsName,
		BackendTLSPolicy:     backendTLSPolicy,
		ServicePort:          svcPort,
		Valid:                true,
		Weight:               weight,
		IsMirrorBackend:      ref.MirrorBackendIdx != nil,
		IsInferencePool:      ref.IsInferencePool,
		InvalidForGateways:   invalidForGateways,
		EndpointPickerConfig: ref.EndpointPickerConfig,
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
	var referencePolicy *BackendTLSPolicy

	checkPoliciesEqual := func(p1, p2 *gatewayv1.BackendTLSPolicy) bool {
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
			continue
		}

		if referencePolicy == nil {
			// First reference, store the policy as reference
			referencePolicy = backendRef.BackendTLSPolicy
		} else if checkPoliciesEqual(backendRef.BackendTLSPolicy.Source, referencePolicy.Source) {
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

func findBackendTLSPolicyForService(
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	refNamespace *gatewayv1.Namespace,
	refName,
	routeNamespace string,
	servicePort v1.ServicePort,
) (*BackendTLSPolicy, error) {
	var beTLSPolicy *BackendTLSPolicy
	var conflictingPolicies []*BackendTLSPolicy
	var err error

	refNs := routeNamespace
	if refNamespace != nil {
		refNs = string(*refNamespace)
	}

	// First pass: find all policies targeting this service and port
	for _, btp := range backendTLSPolicies {
		btpNs := btp.Source.Namespace
		for _, targetRef := range btp.Source.Spec.TargetRefs {
			if string(targetRef.Name) == refName && btpNs == refNs {
				// Check if this policy applies to the specific port we're interested in
				if targetRef.SectionName != nil {
					// Policy targets a specific port by name
					if servicePort.Name != string(*targetRef.SectionName) {
						// This policy targets a different port, skip it
						continue
					}
				}
				// Policy applies to all ports (no sectionName) or matches our port

				if beTLSPolicy == nil {
					beTLSPolicy = btp
				} else {
					// Found a conflict - determine which policy wins
					if sort.LessClientObject(btp.Source, beTLSPolicy.Source) {
						// btp wins, beTLSPolicy loses
						conflictingPolicies = append(conflictingPolicies, beTLSPolicy)
						beTLSPolicy = btp
					} else {
						// beTLSPolicy wins, btp loses
						conflictingPolicies = append(conflictingPolicies, btp)
					}
				}
			}
		}
	}

	// Set conflicted conditions on losing policies
	for _, conflictedPolicy := range conflictingPolicies {
		conflictedPolicy.IsReferenced = true
		conflictedPolicy.Conditions = append(conflictedPolicy.Conditions,
			conditions.NewPolicyConflicted("Conflicts with another BackendTLSPolicy targeting the same Service"))
	}

	if beTLSPolicy != nil {
		beTLSPolicy.IsReferenced = true
		if !beTLSPolicy.Valid {
			//nolint:staticcheck // Capitalization required for alignment with other messages.
			err = fmt.Errorf("The BackendTLSPolicy is invalid: %s", beTLSPolicy.Conditions[0].Message)
		} else {
			beTLSPolicy.Conditions = append(beTLSPolicy.Conditions, conditions.NewPolicyAccepted())
		}
	}

	return beTLSPolicy, err
}

// getIPFamilyAndPortFromRef extracts the IPFamily of the Service and the port from a BackendRef.
// It can return an error and an empty v1.ServicePort in two cases:
// 1. The Service referenced from the BackendRef does not exist in the cluster/state.
// 2. The Port on the BackendRef does not match any of the ServicePorts on the Service.
func getIPFamilyAndPortFromRef(
	ref gatewayv1.BackendRef,
	svcNsName types.NamespacedName,
	services map[types.NamespacedName]*v1.Service,
	refPath *field.Path,
) ([]v1.IPFamily, v1.ServicePort, error) {
	svc, ok := services[svcNsName]
	if !ok {
		return []v1.IPFamily{}, v1.ServicePort{}, field.NotFound(refPath.Child("name"), ref.Name)
	}

	// safe to dereference port here because we already validated that the port is not nil in validateBackendRef.
	svcPort, err := getServicePort(svc, *ref.Port)
	if err != nil {
		return []v1.IPFamily{}, v1.ServicePort{}, err
	}

	return svc.Spec.IPFamilies, svcPort, nil
}

func verifyIPFamily(npCfg *EffectiveNginxProxy, svcIPFamily []v1.IPFamily) error {
	if npCfg == nil {
		return nil
	}

	containsIPv6 := slices.Contains(svcIPFamily, v1.IPv6Protocol)
	containsIPv4 := slices.Contains(svcIPFamily, v1.IPv4Protocol)

	//nolint: staticcheck // used in status condition which is normally capitalized
	errIPv6Mismatch := errors.New("The Service configured with IPv6 family but NginxProxy is configured with IPv4")
	//nolint: staticcheck // used in status condition which is normally capitalized
	errIPv4Mismatch := errors.New("The Service configured with IPv4 family but NginxProxy is configured with IPv6")

	npIPFamily := npCfg.IPFamily

	if npIPFamily == nil {
		// default is dual so we don't need to check the service IPFamily.
		return nil
	}

	if *npIPFamily == ngfAPIv1alpha2.IPv4 && containsIPv6 {
		return errIPv6Mismatch
	}

	if *npIPFamily == ngfAPIv1alpha2.IPv6 && containsIPv4 {
		return errIPv4Mismatch
	}

	return nil
}

func checkExternalNameValidForGateways(
	parentRefs []ParentRef,
	invalidForGateways map[types.NamespacedName]conditions.Condition,
) map[types.NamespacedName]conditions.Condition {
	for _, parentRef := range parentRefs {
		if parentRef.Gateway.EffectiveNginxProxy == nil || parentRef.Gateway.EffectiveNginxProxy.DNSResolver == nil {
			invalidForGateways[parentRef.Gateway.NamespacedName] = conditions.NewRouteBackendRefUnsupportedValue(
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
		var inferencePool bool
		var inferencePoolName types.NamespacedName

		switch {
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
				Name:      controller.GetInferencePoolName(string(ref.Name)),
			}
		default:
			refNsName := types.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}

			if !refGrantResolver(toService(refNsName)) {
				msg := fmt.Sprintf("Backend ref to Service %s not permitted by any ReferenceGrant", refNsName)
				valErr := field.Forbidden(path.Child("namespace"), msg)

				return false, conditions.NewRouteBackendRefRefNotPermitted(valErr.Error())
			}
		}

		if inferencePool {
			if !refGrantResolver(toInferencePool(inferencePoolName)) {
				msg := fmt.Sprintf(
					"Backend ref to InferencePool %s not permitted by any ReferenceGrant",
					inferencePoolName,
				)
				valErr := field.Forbidden(path.Child("namespace"), msg)
				return false, conditions.NewRouteBackendRefRefNotPermitted(valErr.Error())
			}
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

func validateBackendRefHTTPRouteGroupKind(
	ref gatewayv1.BackendRef,
	path *field.Path,
) (bool, conditions.Condition) {
	if ref.Group != nil {
		group := *ref.Group
		if group != "core" && group != "" && group != inferenceAPIGroup {
			valErr := field.NotSupported(path.Child("group"), group, []string{"core", "", inferenceAPIGroup})
			return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
		}
		if group == inferenceAPIGroup {
			if ref.Kind == nil || *ref.Kind != kinds.InferencePool {
				valErr := field.Invalid(
					path.Child("kind"),
					ref.Kind,
					fmt.Sprintf("kind must be InferencePool when group is %s", inferenceAPIGroup),
				)
				return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
			}
		}
	}

	if ref.Kind != nil {
		kind := *ref.Kind
		if kind != kinds.Service && kind != kinds.InferencePool {
			valErr := field.NotSupported(path.Child("kind"), kind, []string{kinds.Service, kinds.InferencePool})
			return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
		}
		if kind == kinds.InferencePool {
			if ref.Group == nil || *ref.Group != inferenceAPIGroup {
				valErr := field.Invalid(
					path.Child("group"),
					ref.Group,
					fmt.Sprintf("group must be %s when kind is InferencePool", inferenceAPIGroup),
				)
				return false, conditions.NewRouteBackendRefInvalidKind(valErr.Error())
			}
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
		if routeType == RouteTypeGRPC {
			return nil
		}

		if routeType == RouteTypeHTTP {
			return fmt.Errorf("%w; nginx does not support proxying to upstreams with http2 or h2c", err)
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
