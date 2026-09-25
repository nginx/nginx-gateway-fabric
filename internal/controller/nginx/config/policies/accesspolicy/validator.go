package accesspolicy

import (
	"fmt"
	"net/netip"

	"k8s.io/apimachinery/pkg/util/validation/field"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// Validator validates an AccessPolicy.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator) *Validator {
	return &Validator{genericValidator: genericValidator}
}

// Validate validates the spec of an AccessPolicy.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	ap := helpers.MustCastObject[*ngfAPI.AccessPolicy](policy)

	if err := v.validateRules(ap.Spec.Rules); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// ValidateGlobalSettings is a no-op for AccessPolicy.
func (v *Validator) ValidateGlobalSettings(
	_ policies.Policy,
	_ *policies.GlobalSettings,
) []conditions.Condition {
	return nil
}

// Conflicts returns false for all AccessPolicies. Multiple policies on the same target are merged.
func (v *Validator) Conflicts(polA, polB policies.Policy) bool {
	helpers.MustCastObject[*ngfAPI.AccessPolicy](polA)
	helpers.MustCastObject[*ngfAPI.AccessPolicy](polB)
	return false
}

// validateRules validates each rule's name and, when present, its IP address or CIDR.
func (v *Validator) validateRules(rules []ngfAPI.AccessRule) error {
	var allErrs field.ErrorList
	rulesPath := field.NewPath("spec").Child("rules")

	for i, rule := range rules {
		rulePath := rulesPath.Index(i)

		if err := v.genericValidator.ValidateDNSSubdomainName(rule.Name); err != nil {
			allErrs = append(allErrs, field.Invalid(rulePath.Child("name"), rule.Name, err.Error()))
		}

		if rule.Source == nil || rule.Source.IPAddress == nil {
			continue
		}

		addr := rule.Source.IPAddress.Address
		if err := validateIPAddress(addr); err != nil {
			allErrs = append(allErrs, field.Invalid(rulePath.Child("source", "ipAddress", "address"), addr, err.Error()))
		}
	}

	return allErrs.ToAggregate()
}

// validateIPAddress returns an error if the value is not a valid IPv4/IPv6 address or CIDR range.
func validateIPAddress(s string) error {
	if _, err := netip.ParsePrefix(s); err == nil {
		return nil
	}

	if _, err := netip.ParseAddr(s); err == nil {
		return nil
	}

	return fmt.Errorf("must be a valid IPv4/IPv6 address or CIDR range (e.g. 192.168.1.1, 10.0.0.0/8, 2001:db8::/32)")
}
