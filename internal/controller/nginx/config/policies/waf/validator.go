package waf

import (
	"errors"
	"regexp"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const (
	securityLogFilePathFmt    = `^/[^\s;{}#$"\\]*$`
	securityLogFilePathErrMsg = "must start with / and must not contain whitespace, or the characters " +
		`';', '{', '}', '#', '$', '"', or '\'`
)

var securityLogFilePathRegexp = regexp.MustCompile(securityLogFilePathFmt)

// Validator validates a WAFPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator) *Validator {
	return &Validator{genericValidator: genericValidator}
}

// Validate validates the spec of a WAFPolicy.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	wp := helpers.MustCastObject[*ngfAPI.WAFPolicy](policy)

	targetRefsPath := field.NewPath("spec").Child("targetRefs")
	supportedKinds := []gatewayv1.Kind{kinds.Gateway, kinds.HTTPRoute, kinds.GRPCRoute}
	supportedGroups := []gatewayv1.Group{gatewayv1.GroupName}

	for i, targetRef := range wp.Spec.TargetRefs {
		if err := policies.ValidateTargetRef(
			targetRef,
			targetRefsPath.Index(i),
			supportedGroups,
			supportedKinds,
		); err != nil {
			return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
		}
	}

	if err := v.validatePLMNames(wp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	if err := validateSecurityLogs(wp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// validateSecurityLogs performs validation on fields in the spec that are vulnerable to code injection.
// For all other fields, we rely on the CRD validation.
func validateSecurityLogs(spec ngfAPI.WAFPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec").Child("securityLogs")

	for i, secLog := range spec.SecurityLogs {
		if secLog.Destination.Type != ngfAPI.SecurityLogDestinationTypeFile || secLog.Destination.File == nil {
			continue
		}

		path := secLog.Destination.File.Path
		if err := validateSecurityLogFilePath(path); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					fieldPath.Index(i).Child("destination").Child("file").Child("path"),
					path,
					err.Error(),
				),
			)
		}
	}

	return allErrs.ToAggregate()
}

func validateSecurityLogFilePath(path string) error {
	if !securityLogFilePathRegexp.MatchString(path) {
		examples := []string{
			"/var/log/app_protect/security.log",
			"/var/log/nginx/security.log",
		}

		return errors.New(k8svalidation.RegexError(securityLogFilePathErrMsg, securityLogFilePathFmt, examples...))
	}

	return nil
}

// validatePLMNames validates the Name fields of PLM-managed CRD references.
func (v *Validator) validatePLMNames(spec ngfAPI.WAFPolicySpec) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	if spec.PolicyRef != nil && spec.PolicyRef.APPolicyRef != nil {
		name := spec.PolicyRef.APPolicyRef.Name
		path := specPath.Child("policyRef", "apPolicyRef", "name")
		if err := v.genericValidator.ValidateDNSSubdomainName(name); err != nil {
			allErrs = append(allErrs, field.Invalid(path, name, err.Error()))
		}
	}

	for i, log := range spec.SecurityLogs {
		if log.LogRef == nil || log.LogRef.APLogConfRef == nil {
			continue
		}
		name := log.LogRef.APLogConfRef.Name
		path := specPath.Child("securityLogs").Index(i).Child("logRef", "apLogConfRef", "name")
		if err := v.genericValidator.ValidateDNSSubdomainName(name); err != nil {
			allErrs = append(allErrs, field.Invalid(path, name, err.Error()))
		}
	}

	return allErrs.ToAggregate()
}

// ValidateGlobalSettings validates a WAFPolicy with respect to the NginxProxy global settings.
func (v *Validator) ValidateGlobalSettings(
	_ policies.Policy,
	globalSettings *policies.GlobalSettings,
) []conditions.Condition {
	if globalSettings == nil {
		return []conditions.Condition{
			conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageNginxProxyInvalid),
		}
	}

	if !globalSettings.WAFEnabled {
		return []conditions.Condition{
			conditions.NewPolicyNotAcceptedNginxProxyNotSet("WAF is not enabled in NginxProxy"),
		}
	}
	return nil
}

// Conflicts returns false as we don't allow merging for WAFPolicies.
func (v Validator) Conflicts(_, _ policies.Policy) bool {
	return false
}
