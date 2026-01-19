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

// Validator validates a WAFGatewayBindingPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator) *Validator {
	return &Validator{genericValidator: genericValidator}
}

// Validate validates the spec of a WAFGatewayBindingPolicy.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	wp := helpers.MustCastObject[*ngfAPI.WAFGatewayBindingPolicy](policy)

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

	if err := v.validateSettings(wp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// ValidateGlobalSettings validates a WAFGatewayBindingPolicy with respect to the NginxProxy global settings.
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

// Conflicts returns false as we don't allow merging for WAFGatewayBindingPolicies.
func (v Validator) Conflicts(_, _ policies.Policy) bool {
	return false
}

func (v *Validator) validateSettings(spec ngfAPI.WAFGatewayBindingPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec")

	// Validate apPolicySource is set with a name.
	// Resource existence and cross-namespace ReferenceGrant checks are handled at graph processing time.
	if spec.ApPolicySource == nil {
		allErrs = append(allErrs, field.Required(fieldPath.Child("apPolicySource"), "apPolicySource is required"))
	} else if spec.ApPolicySource.Name == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("apPolicySource").Child("name"), "name is required"))
	}

	// Validate security logs if present
	for i, sl := range spec.SecurityLogs {
		logPath := fieldPath.Child("securityLogs").Index(i)
		if sl.ApLogConfSource.Name == "" {
			allErrs = append(allErrs, field.Required(logPath.Child("apLogConfSource").Child("name"), "name is required"))
		}
	}

	return allErrs.ToAggregate()
}
