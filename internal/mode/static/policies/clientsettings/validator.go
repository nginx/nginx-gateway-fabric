package clientsettings

import (
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/policies"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// Validator validates a ClientSettingsPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator) *Validator {
	return &Validator{genericValidator: genericValidator}
}

// Validate validates the spec of a ClientSettingsPolicy.
func (v *Validator) Validate(policy policies.Policy) error {
	csp := helpers.MustCastObject[*ngfAPI.ClientSettingsPolicy](policy)

	if err := validateTargetRef(csp.Spec.TargetRef); err != nil {
		return err
	}

	return v.validateSettings(csp.Spec)
}

// Conflicts returns true if the two ClientSettingsPolicies conflict.
func (v *Validator) Conflicts(polA, polB policies.Policy) bool {
	cspA := helpers.MustCastObject[*ngfAPI.ClientSettingsPolicy](polA)
	cspB := helpers.MustCastObject[*ngfAPI.ClientSettingsPolicy](polB)

	return conflicts(cspA.Spec, cspB.Spec)
}

func conflicts(a, b ngfAPI.ClientSettingsPolicySpec) bool {
	if a.Body != nil && b.Body != nil {
		if a.Body.Timeout != nil && b.Body.Timeout != nil {
			return true
		}

		if a.Body.MaxSize != nil && b.Body.MaxSize != nil {
			return true
		}
	}

	if a.KeepAlive != nil && b.KeepAlive != nil {
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

func validateTargetRef(ref v1alpha2.LocalPolicyTargetReference) error {
	basePath := field.NewPath("spec").Child("targetRef")

	if ref.Group != gatewayv1.GroupName {
		path := basePath.Child("group")

		return field.NotSupported(
			path,
			ref.Group,
			[]string{gatewayv1.GroupName},
		)
	}

	supportedKinds := []gatewayv1.Kind{kinds.Gateway, kinds.HTTPRoute, kinds.GRPCRoute}

	if !slices.Contains(supportedKinds, ref.Kind) {
		path := basePath.Child("kind")

		return field.NotSupported(
			path,
			ref.Kind,
			supportedKinds,
		)
	}

	return nil
}

// validateSettings performs validation on fields in the spec that are vulnerable to code injection.
// For all other fields, we rely on the CRD validation.
func (v *Validator) validateSettings(spec ngfAPI.ClientSettingsPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec")

	if spec.Body != nil {
		allErrs = append(allErrs, v.validateClientBody(*spec.Body, fieldPath.Child("body"))...)
	}

	if spec.KeepAlive != nil {
		allErrs = append(allErrs, v.validateClientKeepAlive(*spec.KeepAlive, fieldPath.Child("keepAlive"))...)
	}

	return allErrs.ToAggregate()
}

func (v *Validator) validateClientBody(body ngfAPI.ClientBody, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if body.Timeout != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*body.Timeout)); err != nil {
			path := fieldPath.Child("timeout")

			allErrs = append(allErrs, field.Invalid(path, body.Timeout, err.Error()))
		}
	}

	if body.MaxSize != nil {
		if err := v.genericValidator.ValidateNginxSize(string(*body.MaxSize)); err != nil {
			path := fieldPath.Child("maxSize")

			allErrs = append(allErrs, field.Invalid(path, body.MaxSize, err.Error()))
		}
	}

	return allErrs
}

func (v *Validator) validateClientKeepAlive(keepAlive ngfAPI.ClientKeepAlive, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if keepAlive.Time != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*keepAlive.Time)); err != nil {
			path := fieldPath.Child("time")

			allErrs = append(allErrs, field.Invalid(path, *keepAlive.Time, err.Error()))
		}
	}

	if keepAlive.Timeout != nil {
		timeout := keepAlive.Timeout

		if timeout.Server != nil {
			if err := v.genericValidator.ValidateNginxDuration(string(*timeout.Server)); err != nil {
				path := fieldPath.Child("timeout").Child("server")

				allErrs = append(
					allErrs,
					field.Invalid(path, *keepAlive.Timeout.Server, err.Error()),
				)
			}
		}

		if timeout.Header != nil {
			if err := v.genericValidator.ValidateNginxDuration(string(*timeout.Header)); err != nil {
				path := fieldPath.Child("timeout").Child("header")

				allErrs = append(
					allErrs,
					field.Invalid(path, *keepAlive.Timeout.Header, err.Error()),
				)
			}
		}

		// This is a special case. The keepalive_timeout directive takes two parameters:
		// keepalive_timeout server [header], where header is optional. If header is provided and server is not,
		// we can't properly configure the directive.
		if keepAlive.Timeout.Header != nil && keepAlive.Timeout.Server == nil {
			path := fieldPath.Child("timeout")

			allErrs = append(
				allErrs,
				field.Invalid(
					path,
					nil,
					"server timeout must be set if header timeout is set",
				),
			)
		}
	}

	return allErrs
}
