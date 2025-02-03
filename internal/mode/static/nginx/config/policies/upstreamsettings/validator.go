package upstreamsettings

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// Validator validates an UpstreamSettingsPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new Validator.
func NewValidator(genericValidator validation.GenericValidator) Validator {
	return Validator{genericValidator: genericValidator}
}

// Validate validates the spec of an UpstreamsSettingsPolicy.
func (v Validator) Validate(policy policies.Policy, _ *policies.GlobalSettings) []conditions.Condition {
	usp := helpers.MustCastObject[*ngfAPI.UpstreamSettingsPolicy](policy)

	targetRefsPath := field.NewPath("spec").Child("targetRefs")
	supportedKinds := []gatewayv1.Kind{kinds.Service}
	supportedGroups := []gatewayv1.Group{"", "core"}

	for i, ref := range usp.Spec.TargetRefs {
		indexedPath := targetRefsPath.Index(i)
		if err := policies.ValidateTargetRef(ref, indexedPath, supportedGroups, supportedKinds); err != nil {
			return []conditions.Condition{staticConds.NewPolicyInvalid(err.Error())}
		}
	}

	if err := v.validateSettings(usp.Spec); err != nil {
		return []conditions.Condition{staticConds.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// Conflicts returns true if the two UpstreamsSettingsPolicies conflict.
func (v Validator) Conflicts(polA, polB policies.Policy) bool {
	cspA := helpers.MustCastObject[*ngfAPI.UpstreamSettingsPolicy](polA)
	cspB := helpers.MustCastObject[*ngfAPI.UpstreamSettingsPolicy](polB)

	return conflicts(cspA.Spec, cspB.Spec)
}

func conflicts(a, b ngfAPI.UpstreamSettingsPolicySpec) bool {
	if a.ZoneSize != nil && b.ZoneSize != nil {
		return true
	}

	if a.KeepAlive != nil && b.KeepAlive != nil {
		if a.KeepAlive.Connections != nil && b.KeepAlive.Connections != nil {
			return true
		}
		if a.KeepAlive.Requests != nil && b.KeepAlive.Requests != nil {
			return true
		}

		if a.KeepAlive.Time != nil && b.KeepAlive.Time != nil {
			return true
		}

		if a.KeepAlive.Timeout != nil && b.KeepAlive.Timeout != nil {
			return true
		}
	}

	return false
}

// validateSettings performs validation on fields in the spec that are vulnerable to code injection.
// For all other fields, we rely on the CRD validation.
func (v Validator) validateSettings(spec ngfAPI.UpstreamSettingsPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec")

	if spec.ZoneSize != nil {
		if err := v.genericValidator.ValidateNginxSize(string(*spec.ZoneSize)); err != nil {
			path := fieldPath.Child("zoneSize")
			allErrs = append(allErrs, field.Invalid(path, spec.ZoneSize, err.Error()))
		}
	}

	if spec.KeepAlive != nil {
		allErrs = append(allErrs, v.validateUpstreamKeepAlive(*spec.KeepAlive, fieldPath.Child("keepAlive"))...)
	}

	return allErrs.ToAggregate()
}

func (v Validator) validateUpstreamKeepAlive(
	keepAlive ngfAPI.UpstreamKeepAlive,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if keepAlive.Time != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*keepAlive.Time)); err != nil {
			path := fieldPath.Child("time")

			allErrs = append(allErrs, field.Invalid(path, *keepAlive.Time, err.Error()))
		}
	}

	if keepAlive.Timeout != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*keepAlive.Timeout)); err != nil {
			path := fieldPath.Child("timeout")

			allErrs = append(allErrs, field.Invalid(path, *keepAlive.Timeout, err.Error()))
		}
	}

	return allErrs
}
