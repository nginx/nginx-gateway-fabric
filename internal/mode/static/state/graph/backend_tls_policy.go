package graph

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

type BackendTLSPolicy struct {
	// Source is the source resource.
	Source *v1alpha3.BackendTLSPolicy
	// CaCertRef is the name of the ConfigMap that contains the CA certificate.
	CaCertRef types.NamespacedName
	// Gateway is the name of the Gateway that is being checked for this BackendTLSPolicy.
	Gateway types.NamespacedName
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
	gateway *Gateway,
) map[types.NamespacedName]*BackendTLSPolicy {
	if len(backendTLSPolicies) == 0 || gateway == nil {
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
			Gateway: types.NamespacedName{
				Namespace: gateway.Source.Namespace,
				Name:      gateway.Source.Name,
			},
			CaCertRef: caCertRef,
			Ignored:   ignored,
		}
	}
	return processedBackendTLSPolicies
}

func validateBackendTLSPolicy(
	backendTLSPolicy *v1alpha3.BackendTLSPolicy,
	configMapResolver *configMapResolver,
	secretResolver *secretResolver,
	ctlrName string,
) (valid, ignored bool, conds []conditions.Condition) {
	valid = true
	ignored = false

	// FIXME (kate-osborn): https://github.com/nginx/nginx-gateway-fabric/issues/1987
	if backendTLSPolicyAncestorsFull(backendTLSPolicy.Status.Ancestors, ctlrName) {
		valid = false
		ignored = true
	}

	if err := validateBackendTLSHostname(backendTLSPolicy); err != nil {
		valid = false
		conds = append(conds, staticConds.NewPolicyInvalid(fmt.Sprintf("invalid hostname: %s", err.Error())))
	}

	caCertRefs := backendTLSPolicy.Spec.Validation.CACertificateRefs
	wellKnownCerts := backendTLSPolicy.Spec.Validation.WellKnownCACertificates
	switch {
	case len(caCertRefs) > 0 && wellKnownCerts != nil:
		valid = false
		msg := "CACertificateRefs and WellKnownCACertificates are mutually exclusive"
		conds = append(conds, staticConds.NewPolicyInvalid(msg))

	case len(caCertRefs) > 0:
		if err := validateBackendTLSCACertRef(backendTLSPolicy, configMapResolver, secretResolver); err != nil {
			valid = false
			conds = append(conds, staticConds.NewPolicyInvalid(
				fmt.Sprintf("invalid CACertificateRef: %s", err.Error())))
		}

	case wellKnownCerts != nil:
		if err := validateBackendTLSWellKnownCACerts(backendTLSPolicy); err != nil {
			valid = false
			conds = append(conds, staticConds.NewPolicyInvalid(
				fmt.Sprintf("invalid WellKnownCACertificates: %s", err.Error())))
		}

	default:
		valid = false
		conds = append(conds, staticConds.NewPolicyInvalid("CACertRefs and WellKnownCACerts are both nil"))
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

func validateBackendTLSCACertRef(btp *v1alpha3.BackendTLSPolicy, configMapResolver *configMapResolver, secretResolver *secretResolver) error {
	if len(btp.Spec.Validation.CACertificateRefs) != 1 {
		path := field.NewPath("tls.cacertrefs")
		valErr := field.TooMany(path, len(btp.Spec.Validation.CACertificateRefs), 1)
		return valErr
	}

	selectedCertRef := btp.Spec.Validation.CACertificateRefs[0]
	allowedCaCertKinds := []v1.Kind{"ConfigMap", "Secret"}

	if !slices.Contains(allowedCaCertKinds, selectedCertRef.Kind) {
		path := field.NewPath("tls.cacertrefs[0].kind")
		valErr := field.NotSupported(path, btp.Spec.Validation.CACertificateRefs[0].Kind, allowedCaCertKinds)
		return valErr
	}
	if selectedCertRef.Group != "" &&
		selectedCertRef.Group != "core" {
		path := field.NewPath("tls.cacertrefs[0].group")
		valErr := field.NotSupported(path, selectedCertRef.Group, []string{"", "core"})
		return valErr
	}
	nsName := types.NamespacedName{
		Namespace: btp.Namespace,
		Name:      string(selectedCertRef.Name),
	}

	if selectedCertRef.Kind == "ConfigMap" {
		if err := configMapResolver.resolve(nsName); err != nil {
			path := field.NewPath("tls.cacertrefs[0]")
			return field.Invalid(path, selectedCertRef, err.Error())
		}
	} else if selectedCertRef.Kind == "Secret" {
		if err := secretResolver.resolve(nsName); err != nil {
			path := field.NewPath("tls.cacertrefs[0]")
			return field.Invalid(path, selectedCertRef, err.Error())
		}
	} else {
		return fmt.Errorf("`%s` invalid certificate reference supported", selectedCertRef.Kind)
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
