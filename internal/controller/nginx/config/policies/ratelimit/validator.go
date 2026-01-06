package ratelimit

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// Validator validates a RateLimitPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator) *Validator {
	return &Validator{genericValidator: genericValidator}
}

// Validate validates the spec of a RateLimitPolicy.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	rlp := helpers.MustCastObject[*ngfAPI.RateLimitPolicy](policy)

	if err := v.validateSettings(rlp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// ValidateGlobalSettings validates a RateLimitPolicy with respect to the NginxProxy global settings.
func (v *Validator) ValidateGlobalSettings(
	_ policies.Policy,
	_ *policies.GlobalSettings,
) []conditions.Condition {
	return nil
}

// Conflicts returns true if the two ProxySettingsPolicies conflict.
func (v *Validator) Conflicts(polA, polB policies.Policy) bool {
	rlpA := helpers.MustCastObject[*ngfAPI.RateLimitPolicy](polA)
	rlpB := helpers.MustCastObject[*ngfAPI.RateLimitPolicy](polB)

	return conflicts(rlpA.Spec, rlpB.Spec)
}

func conflicts(a, b ngfAPI.RateLimitPolicySpec) bool {
	if a.RateLimit != nil && b.RateLimit != nil {
		if a.RateLimit.DryRun != nil && b.RateLimit.DryRun != nil {
			return true
		}

		if a.RateLimit.LogLevel != nil && b.RateLimit.LogLevel != nil {
			return true
		}

		if a.RateLimit.RejectCode != nil && b.RateLimit.RejectCode != nil {
			return true
		}
	}

	return false
}

// validateSettings performs validation on fields in the spec that are vulnerable to code injection.
// For all other fields, we rely on the CRD validation.
func (v *Validator) validateSettings(spec ngfAPI.RateLimitPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec")

	if spec.RateLimit != nil && spec.RateLimit.Local != nil {
		for _, rule := range spec.RateLimit.Local.Rules {
			if err := v.genericValidator.ValidateNginxSize(string(*rule.ZoneSize)); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						fieldPath.
							Child("rateLimit").
							Child("local").
							Child("rules").
							Child("zoneSize"),
						*rule.ZoneSize,
						err.Error(),
					),
				)
			}
		}
	}

	return allErrs.ToAggregate()
}
