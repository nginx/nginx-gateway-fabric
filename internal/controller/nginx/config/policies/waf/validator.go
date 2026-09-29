package waf

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

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
