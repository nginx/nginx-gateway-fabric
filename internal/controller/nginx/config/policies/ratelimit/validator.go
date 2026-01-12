package ratelimit

import (
	"errors"
	"regexp"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
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
			path := fieldPath.Child("rateLimit").Child("local").Child("rules")

			if rule.ZoneSize != nil {
				if err := v.genericValidator.ValidateNginxSize(string(*rule.ZoneSize)); err != nil {
					allErrs = append(allErrs,
						field.Invalid(
							path.Child("zoneSize"),
							*rule.ZoneSize,
							err.Error(),
						),
					)
				}
			}

			if rule.Rate != "" {
				if err := validateNginxRate(string(rule.Rate)); err != nil {
					allErrs = append(allErrs,
						field.Invalid(
							path.Child("rate"),
							rule.Rate,
							err.Error(),
						),
					)
				}
			}

			if rule.Key != "" {
				if err := validateLimitReqKey(rule.Key); err != nil {
					allErrs = append(allErrs,
						field.Invalid(
							path.Child("key"),
							rule.Key,
							err.Error(),
						),
					)
				}
			}
		}
	}

	return allErrs.ToAggregate()
}

// validateNginxRate validates a rate string that nginx can understand.
func validateNginxRate(rate string) error {
	const (
		rateStringFmt    = `^\d+r/[sm]$`
		rateStringErrMsg = `must contain a number followed by 'r/s' or 'r/m'`
	)

	rateStringRegexp := regexp.MustCompile("^" + rateStringFmt + "$")

	if !rateStringRegexp.MatchString(rate) {
		examples := []string{
			"10r/s",
			"500r/m",
		}

		return errors.New(k8svalidation.RegexError(rateStringFmt, rateStringErrMsg, examples...))
	}

	return nil
}

// validateLimitReqKey validates a limit_req key string that nginx can understand.
func validateLimitReqKey(key string) error {
	const (
		// ?: is a non-capturing group
		// [^ \t\r\n;{}#$]+ matches any run of characters except the separators that
		//   would make nginx stop parsing the argument.
		// $\w+ matches an nginx variable.
		limitReqKeyFmt = `^(?:[^ \t\r\n;{}#$]+|\$\w+)+$`
		limitReqErrMsg = "must be a valid limit_req key consisting of nginx variables " +
			"and/or strings without spaces or special characters"
	)

	limitReqKeyRegexp := regexp.MustCompile("^" + limitReqKeyFmt + "$")

	if !limitReqKeyRegexp.MatchString(key) {
		examples := []string{
			"$binary_remote_addr",
			"$binary_remote_addr:$request_uri",
			"my_fixed_key",
		}

		return errors.New(k8svalidation.RegexError(limitReqKeyFmt, limitReqErrMsg, examples...))
	}

	return nil
}
