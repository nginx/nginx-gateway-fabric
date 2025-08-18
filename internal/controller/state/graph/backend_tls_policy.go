package graph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type BackendTLSPolicy struct {
	// Source is the source resource.
	Source *v1alpha3.BackendTLSPolicy
	// CaCertRef is the name of the ConfigMap that contains the CA certificate.
	CaCertRef types.NamespacedName
	// Gateways are the names of the Gateways for which this BackendTLSPolicy is effectively applied.
	// Only contains gateways where the policy can be applied (not limited by ancestor status).
	Gateways []types.NamespacedName
	// Conditions include Conditions for the BackendTLSPolicy.
	Conditions []conditions.Condition
	// Valid shows whether the BackendTLSPolicy is valid.
	Valid bool
	// IsReferenced shows whether the BackendTLSPolicy is referenced by a BackendRef.
	IsReferenced bool
	// Ignored shows whether the BackendTLSPolicy is ignored.
	Ignored bool
}

func processBackendTLSPolicies(
	backendTLSPolicies map[types.NamespacedName]*v1alpha3.BackendTLSPolicy,
	configMapResolver *configMapResolver,
	secretResolver *secretResolver,
	gateways map[types.NamespacedName]*Gateway,
) map[types.NamespacedName]*BackendTLSPolicy {
	if len(backendTLSPolicies) == 0 || len(gateways) == 0 {
		return nil
	}

	processedBackendTLSPolicies := make(map[types.NamespacedName]*BackendTLSPolicy, len(backendTLSPolicies))
	for nsname, backendTLSPolicy := range backendTLSPolicies {
		var caCertRef types.NamespacedName

		valid, ignored, conds := validateBackendTLSPolicy(backendTLSPolicy, configMapResolver, secretResolver)

		if valid && !ignored && backendTLSPolicy.Spec.Validation.CACertificateRefs != nil {
			caCertRef = types.NamespacedName{
				Namespace: backendTLSPolicy.Namespace, Name: string(backendTLSPolicy.Spec.Validation.CACertificateRefs[0].Name),
			}
		}

		processedBackendTLSPolicies[nsname] = &BackendTLSPolicy{
			Source:     backendTLSPolicy,
			Valid:      valid,
			Conditions: conds,
			CaCertRef:  caCertRef,
			Ignored:    ignored,
		}
	}
	return processedBackendTLSPolicies
}

func validateBackendTLSPolicy(
	backendTLSPolicy *v1alpha3.BackendTLSPolicy,
	configMapResolver *configMapResolver,
	secretResolver *secretResolver,
) (valid, ignored bool, conds []conditions.Condition) {
	valid = true
	ignored = false

	if err := validateBackendTLSHostname(backendTLSPolicy); err != nil {
		valid = false
		conds = append(conds, conditions.NewPolicyInvalid(fmt.Sprintf("invalid hostname: %s", err.Error())))
	}

	caCertRefs := backendTLSPolicy.Spec.Validation.CACertificateRefs
	wellKnownCerts := backendTLSPolicy.Spec.Validation.WellKnownCACertificates
	switch {
	case len(caCertRefs) > 0 && wellKnownCerts != nil:
		valid = false
		msg := "CACertificateRefs and WellKnownCACertificates are mutually exclusive"
		conds = append(conds, conditions.NewPolicyInvalid(msg))

	case len(caCertRefs) > 0:
		if err := validateBackendTLSCACertRef(backendTLSPolicy, configMapResolver, secretResolver); err != nil {
			valid = false
			conds = append(conds, conditions.NewPolicyInvalid(
				fmt.Sprintf("invalid CACertificateRef: %s", err.Error())))
		}

	case wellKnownCerts != nil:
		if err := validateBackendTLSWellKnownCACerts(backendTLSPolicy); err != nil {
			valid = false
			conds = append(conds, conditions.NewPolicyInvalid(
				fmt.Sprintf("invalid WellKnownCACertificates: %s", err.Error())))
		}

	default:
		valid = false
		conds = append(conds, conditions.NewPolicyInvalid("CACertRefs and WellKnownCACerts are both nil"))
	}
	return valid, ignored, conds
}

func validateBackendTLSHostname(btp *v1alpha3.BackendTLSPolicy) error {
	h := string(btp.Spec.Validation.Hostname)

	if err := validateHostname(h); err != nil {
		path := field.NewPath("tls.hostname")
		valErr := field.Invalid(path, btp.Spec.Validation.Hostname, err.Error())
		return valErr
	}
	return nil
}

func validateBackendTLSCACertRef(
	btp *v1alpha3.BackendTLSPolicy,
	configMapResolver *configMapResolver,
	secretResolver *secretResolver,
) error {
	if len(btp.Spec.Validation.CACertificateRefs) != 1 {
		path := field.NewPath("validation.caCertificateRefs")
		valErr := field.TooMany(path, len(btp.Spec.Validation.CACertificateRefs), 1)
		return valErr
	}

	selectedCertRef := btp.Spec.Validation.CACertificateRefs[0]
	allowedCaCertKinds := []v1.Kind{"ConfigMap", "Secret"}

	if !slices.Contains(allowedCaCertKinds, selectedCertRef.Kind) {
		path := field.NewPath("validation.caCertificateRefs[0].kind")
		valErr := field.NotSupported(path, btp.Spec.Validation.CACertificateRefs[0].Kind, allowedCaCertKinds)
		return valErr
	}
	if selectedCertRef.Group != "" &&
		selectedCertRef.Group != "core" {
		path := field.NewPath("validation.caCertificateRefs[0].group")
		valErr := field.NotSupported(path, selectedCertRef.Group, []string{"", "core"})
		return valErr
	}
	nsName := types.NamespacedName{
		Namespace: btp.Namespace,
		Name:      string(selectedCertRef.Name),
	}

	switch selectedCertRef.Kind {
	case "ConfigMap":
		if err := configMapResolver.resolve(nsName); err != nil {
			path := field.NewPath("validation.caCertificateRefs[0]")
			return field.Invalid(path, selectedCertRef, err.Error())
		}
	case "Secret":
		if err := secretResolver.resolve(nsName); err != nil {
			path := field.NewPath("validation.caCertificateRefs[0]")
			return field.Invalid(path, selectedCertRef, err.Error())
		}
	default:
		return fmt.Errorf("invalid certificate reference kind %q", selectedCertRef.Kind)
	}
	return nil
}

func validateBackendTLSWellKnownCACerts(btp *v1alpha3.BackendTLSPolicy) error {
	if *btp.Spec.Validation.WellKnownCACertificates != v1alpha3.WellKnownCACertificatesSystem {
		path := field.NewPath("tls.wellknowncacertificates")
		return field.NotSupported(
			path,
			btp.Spec.Validation.WellKnownCACertificates,
			[]string{string(v1alpha3.WellKnownCACertificatesSystem)},
		)
	}
	return nil
}

// countNonNGFAncestors counts the number of non-NGF ancestors in policy status.
func countNonNGFAncestors(policy *v1alpha3.BackendTLSPolicy, ctlrName string) int {
	nonNGFCount := 0
	for _, ancestor := range policy.Status.Ancestors {
		if string(ancestor.ControllerName) != ctlrName {
			nonNGFCount++
		}
	}
	return nonNGFCount
}

// addPolicyAncestorLimitCondition adds or updates a PolicyAncestorLimitReached condition.
func addPolicyAncestorLimitCondition(
	conds []conditions.Condition,
	policyName string,
	policyType string,
) []conditions.Condition {
	for i, condition := range conds {
		if condition.Reason == string(conditions.PolicyReasonAncestorLimitReached) {
			if !strings.Contains(condition.Message, policyName) {
				conds[i].Message = fmt.Sprintf("%s, %s %s", condition.Message, policyType, policyName)
			}
			return conds
		}
	}

	newCondition := conditions.NewPolicyAncestorLimitReached(policyType, policyName)
	return append(conds, newCondition)
}

// collectOrderedGateways collects gateways in spec order (services) then creation time order (gateways within service).
func collectOrderedGateways(
	policy *v1alpha3.BackendTLSPolicy,
	services map[types.NamespacedName]*ReferencedService,
	gateways map[types.NamespacedName]*Gateway,
	existingNGFGatewayAncestors map[types.NamespacedName]struct{},
) []types.NamespacedName {
	seenGateways := make(map[types.NamespacedName]struct{})
	existingGateways := make([]types.NamespacedName, 0)
	newGateways := make([]types.NamespacedName, 0)

	// Process services in spec order to maintain deterministic gateway ordering
	for _, refs := range policy.Spec.TargetRefs {
		if refs.Kind != kinds.Service {
			continue
		}

		svcNsName := types.NamespacedName{
			Namespace: policy.Namespace,
			Name:      string(refs.Name),
		}

		referencedService, exists := services[svcNsName]
		if !exists {
			continue
		}

		// Add to ordered lists, categorizing existing vs new, skipping duplicates
		for gateway := range referencedService.GatewayNsNames {
			if _, seen := seenGateways[gateway]; seen {
				continue
			}
			seenGateways[gateway] = struct{}{}
			if _, exists := existingNGFGatewayAncestors[gateway]; exists {
				existingGateways = append(existingGateways, gateway)
			} else {
				newGateways = append(newGateways, gateway)
			}
		}
	}

	sortGatewaysByCreationTime(existingGateways, gateways)
	sortGatewaysByCreationTime(newGateways, gateways)

	return append(existingGateways, newGateways...)
}

func extractExistingNGFGatewayAncestors(
	backendTLSPolicy *v1alpha3.BackendTLSPolicy,
	ctlrName string,
) map[types.NamespacedName]struct{} {
	existingNGFGatewayAncestors := make(map[types.NamespacedName]struct{})

	for _, ancestor := range backendTLSPolicy.Status.Ancestors {
		if string(ancestor.ControllerName) != ctlrName {
			continue
		}

		if ancestor.AncestorRef.Kind != nil && *ancestor.AncestorRef.Kind == v1.Kind(kinds.Gateway) &&
			ancestor.AncestorRef.Namespace != nil {
			gatewayNsName := types.NamespacedName{
				Namespace: string(*ancestor.AncestorRef.Namespace),
				Name:      string(ancestor.AncestorRef.Name),
			}
			existingNGFGatewayAncestors[gatewayNsName] = struct{}{}
		}
	}

	return existingNGFGatewayAncestors
}

func addGatewaysForBackendTLSPolicies(
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	services map[types.NamespacedName]*ReferencedService,
	ctlrName string,
	gateways map[types.NamespacedName]*Gateway,
	logger logr.Logger,
) {
	for _, backendTLSPolicy := range backendTLSPolicies {
		existingNGFGatewayAncestors := extractExistingNGFGatewayAncestors(backendTLSPolicy.Source, ctlrName)
		orderedGateways := collectOrderedGateways(
			backendTLSPolicy.Source,
			services,
			gateways,
			existingNGFGatewayAncestors,
		)

		ancestorCount := countNonNGFAncestors(backendTLSPolicy.Source, ctlrName)

		// Process each gateway, respecting ancestor limits
		for _, gatewayNsName := range orderedGateways {
			// Check if adding this gateway would exceed the ancestor limit
			if ancestorCount >= maxAncestors {
				policyName := backendTLSPolicy.Source.Namespace + "/" + backendTLSPolicy.Source.Name
				proposedAncestor := createParentReference(v1.GroupName, kinds.Gateway, gatewayNsName)
				gatewayName := getAncestorName(proposedAncestor)

				if gateway, ok := gateways[gatewayNsName]; ok {
					gateway.Conditions = addPolicyAncestorLimitCondition(gateway.Conditions, policyName, kinds.BackendTLSPolicy)
				} else {
					// This should never happen, but we'll log it if it does
					logger.Error(fmt.Errorf("gateway not found in the graph"),
						"Gateway not found in the graph", "policy", policyName, "ancestor", gatewayName)
				}

				logAncestorLimitReached(logger, policyName, "BackendTLSPolicy", gatewayName)
				continue
			}

			ancestorCount++

			backendTLSPolicy.Gateways = append(backendTLSPolicy.Gateways, gatewayNsName)
		}
	}
}
