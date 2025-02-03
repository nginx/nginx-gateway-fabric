package graph

import (
	"errors"
	"fmt"
	"slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/sort"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

// BackendRef is an internal representation of a backendRef in an HTTP/GRPC/TLSRoute.
type BackendRef struct {
	// BackendTLSPolicy is the BackendTLSPolicy of the Service which is referenced by the backendRef.
	BackendTLSPolicy *BackendTLSPolicy
	// SvcNsName is the NamespacedName of the Service referenced by the backendRef.
	SvcNsName types.NamespacedName
	// ServicePort is the ServicePort of the Service which is referenced by the backendRef.
	ServicePort v1.ServicePort
	// Weight is the weight of the backendRef.
	Weight int32
	// Valid indicates whether the backendRef is valid.
	// No configuration should be generated for an invalid BackendRef.
	Valid bool
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
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	npCfg *NginxProxy,
) {
	for _, r := range routes {
		addBackendRefsToRules(r, refGrantResolver, services, backendTLSPolicies, npCfg)
	}
}

// addHTTPBackendRefsToRules iterates over the rules of a Route and adds a list of BackendRef to each rule.
// If a reference in a rule is invalid, the function will add a condition to the rule.
func addBackendRefsToRules(
	route *L7Route,
	refGrantResolver *referenceGrantResolver,
	services map[types.NamespacedName]*v1.Service,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	npCfg *NginxProxy,
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
			refPath := field.NewPath("spec").Child("rules").Index(idx).Child("backendRefs").Index(refIdx)
			routeNs := route.Source.GetNamespace()

			ref, cond := createBackendRef(
				ref,
				routeNs,
				refGrantResolver.refAllowedFrom(getRefGrantFromResourceForRoute(route.RouteType, routeNs)),
				services,
				refPath,
				backendTLSPolicies,
				npCfg,
			)

			backendRefs = append(backendRefs, ref)
			if cond != nil {
				route.Conditions = append(route.Conditions, *cond)
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
	sourceNamespace string,
	refGrantResolver func(resource toResource) bool,
	services map[types.NamespacedName]*v1.Service,
	refPath *field.Path,
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	npCfg *NginxProxy,
) (BackendRef, *conditions.Condition) {
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

	var backendRef BackendRef

	valid, cond := validateRouteBackendRef(ref, sourceNamespace, refGrantResolver, refPath)
	if !valid {
		backendRef = BackendRef{
			Weight: weight,
			Valid:  false,
		}

		return backendRef, &cond
	}

	ns := sourceNamespace
	if ref.BackendRef.Namespace != nil {
		ns = string(*ref.Namespace)
	}
	svcNsName := types.NamespacedName{Name: string(ref.BackendRef.Name), Namespace: ns}
	svcIPFamily, svcPort, err := getIPFamilyAndPortFromRef(ref.BackendRef, svcNsName, services, refPath)
	if err != nil {
		backendRef = BackendRef{
			Weight:      weight,
			Valid:       false,
			SvcNsName:   svcNsName,
			ServicePort: v1.ServicePort{},
		}

		cond := staticConds.NewRouteBackendRefRefBackendNotFound(err.Error())
		return backendRef, &cond
	}

	if err := verifyIPFamily(npCfg, svcIPFamily); err != nil {
		backendRef = BackendRef{
			SvcNsName:   svcNsName,
			ServicePort: svcPort,
			Weight:      weight,
			Valid:       false,
		}

		cond := staticConds.NewRouteInvalidIPFamily(err.Error())
		return backendRef, &cond
	}

	backendTLSPolicy, err := findBackendTLSPolicyForService(
		backendTLSPolicies,
		ref.Namespace,
		string(ref.Name),
		sourceNamespace,
	)
	if err != nil {
		backendRef = BackendRef{
			SvcNsName:   svcNsName,
			ServicePort: svcPort,
			Weight:      weight,
			Valid:       false,
		}

		cond := staticConds.NewRouteBackendRefUnsupportedValue(err.Error())
		return backendRef, &cond
	}

	backendRef = BackendRef{
		SvcNsName:        svcNsName,
		BackendTLSPolicy: backendTLSPolicy,
		ServicePort:      svcPort,
		Valid:            true,
		Weight:           weight,
	}

	return backendRef, nil
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

	checkPoliciesEqual := func(p1, p2 *v1alpha3.BackendTLSPolicy) bool {
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
		return helpers.GetPointer(staticConds.NewRouteBackendRefUnsupportedValue(msg))
	}
	return nil
}

func findBackendTLSPolicyForService(
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	refNamespace *gatewayv1.Namespace,
	refName,
	routeNamespace string,
) (*BackendTLSPolicy, error) {
	var beTLSPolicy *BackendTLSPolicy
	var err error

	refNs := routeNamespace
	if refNamespace != nil {
		refNs = string(*refNamespace)
	}

	for _, btp := range backendTLSPolicies {
		btpNs := btp.Source.Namespace
		for _, targetRef := range btp.Source.Spec.TargetRefs {
			if string(targetRef.Name) == refName && btpNs == refNs {
				if beTLSPolicy != nil {
					if sort.LessClientObject(btp.Source, beTLSPolicy.Source) {
						beTLSPolicy = btp
					}
				} else {
					beTLSPolicy = btp
				}
			}
		}
	}

	if beTLSPolicy != nil {
		beTLSPolicy.IsReferenced = true
		if !beTLSPolicy.Valid {
			err = fmt.Errorf("the backend TLS policy is invalid: %s", beTLSPolicy.Conditions[0].Message)
		} else {
			beTLSPolicy.Conditions = append(beTLSPolicy.Conditions, staticConds.NewPolicyAccepted())
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
	svcPort, err := getServicePort(svc, int32(*ref.Port))
	if err != nil {
		return []v1.IPFamily{}, v1.ServicePort{}, err
	}

	return svc.Spec.IPFamilies, svcPort, nil
}

func verifyIPFamily(npCfg *NginxProxy, svcIPFamily []v1.IPFamily) error {
	if npCfg == nil || npCfg.Source == nil || !npCfg.Valid {
		return nil
	}

	// we can access this field since we have already validated that ipFamily is not nil in validateNginxProxy.
	npIPFamily := npCfg.Source.Spec.IPFamily
	if *npIPFamily == ngfAPI.IPv4 {
		if slices.Contains(svcIPFamily, v1.IPv6Protocol) {
			// capitalizing error message to match the rest of the error messages associated with a condition
			//nolint: stylecheck
			return errors.New(
				"Service configured with IPv6 family but NginxProxy is configured with IPv4",
			)
		}
	}
	if *npIPFamily == ngfAPI.IPv6 {
		if slices.Contains(svcIPFamily, v1.IPv4Protocol) {
			// capitalizing error message to match the rest of the error messages associated with a condition
			//nolint: stylecheck
			return errors.New(
				"Service configured with IPv4 family but NginxProxy is configured with IPv6",
			)
		}
	}

	return nil
}

func validateRouteBackendRef(
	ref RouteBackendRef,
	routeNs string,
	refGrantResolver func(resource toResource) bool,
	path *field.Path,
) (valid bool, cond conditions.Condition) {
	// Because all errors cause the same condition but different reasons, we return as soon as we find an error
	if len(ref.Filters) > 0 {
		valErr := field.TooMany(path.Child("filters"), len(ref.Filters), 0)
		return false, staticConds.NewRouteBackendRefUnsupportedValue(valErr.Error())
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

	if ref.Group != nil && !(*ref.Group == "core" || *ref.Group == "") {
		valErr := field.NotSupported(path.Child("group"), *ref.Group, []string{"core", ""})
		return false, staticConds.NewRouteBackendRefInvalidKind(valErr.Error())
	}

	if ref.Kind != nil && *ref.Kind != "Service" {
		valErr := field.NotSupported(path.Child("kind"), *ref.Kind, []string{"Service"})
		return false, staticConds.NewRouteBackendRefInvalidKind(valErr.Error())
	}

	// no need to validate ref.Name

	if ref.Namespace != nil && string(*ref.Namespace) != routeNs {
		refNsName := types.NamespacedName{Namespace: string(*ref.Namespace), Name: string(ref.Name)}

		if !refGrantResolver(toService(refNsName)) {
			msg := fmt.Sprintf("Backend ref to Service %s not permitted by any ReferenceGrant", refNsName)

			return false, staticConds.NewRouteBackendRefRefNotPermitted(msg)
		}
	}

	if ref.Port == nil {
		valErr := field.Required(path.Child("port"), "port cannot be nil")
		return false, staticConds.NewRouteBackendRefUnsupportedValue(valErr.Error())
	}

	// any value of port is OK

	if ref.Weight != nil {
		if err := validateWeight(*ref.Weight); err != nil {
			valErr := field.Invalid(path.Child("weight"), *ref.Weight, err.Error())
			return false, staticConds.NewRouteBackendRefUnsupportedValue(valErr.Error())
		}
	}

	return true, conditions.Condition{}
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

	return v1.ServicePort{}, fmt.Errorf("no matching port for Service %s and port %d", svc.Name, port)
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
