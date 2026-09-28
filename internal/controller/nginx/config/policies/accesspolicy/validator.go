package accesspolicy

import (
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	nginxvalidation "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	statevalidation "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// Validator validates an AccessPolicy.
type Validator struct {
	genericValidator statevalidation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator statevalidation.GenericValidator) *Validator {
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
func (v *Validator) Conflicts(_, _ policies.Policy) bool {
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
		addrPath := rulePath.Child("source", "ipAddress", "address")

		if err := k8svalidation.IsValidCIDR(addrPath, addr); err != nil {
			if errs := k8svalidation.IsValidIP(addrPath, addr); len(errs) > 0 {
				allErrs = append(allErrs, field.Invalid(addrPath, addr, nginxvalidation.ErrInvalidIPAddress))
			}
		}
	}

	return allErrs.ToAggregate()
}
