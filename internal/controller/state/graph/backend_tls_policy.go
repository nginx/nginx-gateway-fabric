package graph

import (
	"fmt"
	"slices"

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
	ctlrName string,
	gateways map[types.NamespacedName]*Gateway,
) map[types.NamespacedName]*BackendTLSPolicy {
	if len(backendTLSPolicies) == 0 || len(gateways) == 0 {
		return nil
	}

	processedBackendTLSPolicies := make(map[types.NamespacedName]*BackendTLSPolicy, len(backendTLSPolicies))
	for nsname, backendTLSPolicy := range backendTLSPolicies {
		var caCertRef types.NamespacedName

		valid, ignored, conds := validateBackendTLSPolicy(backendTLSPolicy, configMapResolver, secretResolver, ctlrName)

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
	_ string,
) (valid, ignored bool, conds []conditions.Condition) {
	valid = true
	ignored = false

	// Note: Ancestor limit checking moved to addGatewaysForBackendTLSPolicies for per-gateway effectiveness tracking
	// The policy may be partially effective (work for some gateways but not others due to ancestor limits)

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

func addGatewaysForBackendTLSPolicies(
	backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy,
	services map[types.NamespacedName]*ReferencedService,
	ctlrName string,
) {
	for _, backendTLSPolicy := range backendTLSPolicies {
		potentialGateways := make(map[types.NamespacedName]struct{})

		// First, collect all potential gateways for this policy
		for _, refs := range backendTLSPolicy.Source.Spec.TargetRefs {
			if refs.Kind != kinds.Service {
				continue
			}

			for svcNsName, referencedServices := range services {
				if svcNsName.Name != string(refs.Name) {
					continue
				}

				for gateway := range referencedServices.GatewayNsNames {
					potentialGateways[gateway] = struct{}{}
				}
			}
		}

		// Now check each potential gateway against ancestor limits
		for gatewayNsName := range potentialGateways {
			// Create a proposed ancestor reference for this gateway
			proposedAncestor := createParentReference(v1.GroupName, kinds.Gateway, gatewayNsName)

			// Check ancestor limit for BackendTLS policy
			isFull := backendTLSPolicyAncestorsFull(
				backendTLSPolicy.Source.Status.Ancestors,
				ctlrName,
			)

			if isFull {
				policyName := backendTLSPolicy.Source.Namespace + "/" + backendTLSPolicy.Source.Name
				gatewayName := getAncestorName(proposedAncestor)
				LogAncestorLimitReached(policyName, "BackendTLSPolicy", gatewayName)

				continue
			}

			// Gateway can be effectively used by this policy
			backendTLSPolicy.Gateways = append(backendTLSPolicy.Gateways, gatewayNsName)
		}
	}
}
