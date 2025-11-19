package proxysettings

import (
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// Validator validates a ProxySettingsPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator) *Validator {
	return &Validator{genericValidator: genericValidator}
}

// Validate validates the spec of a ProxySettingsPolicy.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	psp := helpers.MustCastObject[*ngfAPI.ProxySettingsPolicy](policy)

	if err := v.validateSettings(psp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// ValidateGlobalSettings validates a ProxySettingsPolicy with respect to the NginxProxy global settings.
func (v *Validator) ValidateGlobalSettings(
	_ policies.Policy,
	_ *policies.GlobalSettings,
) []conditions.Condition {
	return nil
}

// Conflicts returns true if the two ProxySettingsPolicies conflict.
func (v *Validator) Conflicts(polA, polB policies.Policy) bool {
	pspA := helpers.MustCastObject[*ngfAPI.ProxySettingsPolicy](polA)
	pspB := helpers.MustCastObject[*ngfAPI.ProxySettingsPolicy](polB)

	return conflicts(pspA.Spec, pspB.Spec)
}

func conflicts(a, b ngfAPI.ProxySettingsPolicySpec) bool {
	if a.Buffering != nil && b.Buffering != nil {
		if a.Buffering.Disable != nil && b.Buffering.Disable != nil {
			return true
		}

		if a.Buffering.BufferSize != nil && b.Buffering.BufferSize != nil {
			return true
		}

		if a.Buffering.Buffers != nil && b.Buffering.Buffers != nil {
			return true
		}

		if a.Buffering.BusyBuffersSize != nil && b.Buffering.BusyBuffersSize != nil {
			return true
		}
	}

	return false
}

// validateSettings performs validation on fields in the spec that are vulnerable to code injection.
// For all other fields, we rely on the CRD validation.
func (v *Validator) validateSettings(spec ngfAPI.ProxySettingsPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec")

	if spec.Buffering != nil {
		allErrs = append(allErrs, v.validateBufferSizes(*spec.Buffering, fieldPath.Child("buffering"))...)
		allErrs = append(allErrs, validateBusyBufferSizeRelationships(*spec.Buffering, fieldPath.Child("buffering"))...)
	}

	return allErrs.ToAggregate()
}

func (v *Validator) validateBufferSizes(buffering ngfAPI.ProxyBuffering, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if buffering.BufferSize != nil {
		if err := v.genericValidator.ValidateNginxSize(string(*buffering.BufferSize)); err != nil {
			path := fieldPath.Child("bufferSize")
			allErrs = append(allErrs, field.Invalid(path, buffering.BufferSize, err.Error()))
		}
	}

	if buffering.Buffers != nil {
		if err := v.genericValidator.ValidateNginxSize(string(buffering.Buffers.Size)); err != nil {
			path := fieldPath.Child("buffers").Child("size")
			allErrs = append(allErrs, field.Invalid(path, buffering.Buffers.Size, err.Error()))
		}
	}

	if buffering.BusyBuffersSize != nil {
		if err := v.genericValidator.ValidateNginxSize(string(*buffering.BusyBuffersSize)); err != nil {
			path := fieldPath.Child("busyBuffersSize")
			allErrs = append(allErrs, field.Invalid(path, buffering.BusyBuffersSize, err.Error()))
		}
	}

	return allErrs
}

// validateBusyBufferSizeRelationships validates NGINX constraints for busyBuffersSize.
// CEL cannot validate these constraints because it requires parsing NGINX size strings with units.
//
// NGINX constraints validated:
// 1. proxy_busy_buffers_size > proxy_buffer_size (when both are set)
// 2. proxy_busy_buffers_size < (proxy_buffers.number * proxy_buffers.size) - proxy_buffers.size (when buffers is set)
//
// Note: We only validate when fields are set in the same merged policy (same NGINX context level).
// We do not validate cross-level inheritance because NGINX handles that automatically.
func validateBusyBufferSizeRelationships(buffering ngfAPI.ProxyBuffering, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if buffering.BusyBuffersSize == nil {
		return nil
	}

	busyBuffersSize, err := ParseNginxSize(string(*buffering.BusyBuffersSize))
	if err != nil {
		return nil // Skip validation if size is invalid (will be caught by other validation)
	}

	// Validate: busyBuffersSize > bufferSize
	if buffering.BufferSize != nil {
		bufferSize, err := ParseNginxSize(string(*buffering.BufferSize))
		if err == nil && busyBuffersSize <= bufferSize {
			path := fieldPath.Child("busyBuffersSize")
			allErrs = append(allErrs, field.Invalid(
				path,
				buffering.BusyBuffersSize,
				"must be larger than bufferSize",
			))
		}
	}

	// Validate: busyBuffersSize < (buffers.number * buffers.size) - buffers.size
	if buffering.Buffers != nil {
		buffersSize, err := ParseNginxSize(string(buffering.Buffers.Size))
		if err == nil {
			totalBufferSpace := buffersSize * int64(buffering.Buffers.Number)
			maxBusyBuffersSize := totalBufferSpace - buffersSize
			if busyBuffersSize >= maxBusyBuffersSize {
				path := fieldPath.Child("busyBuffersSize")
				allErrs = append(allErrs, field.Invalid(
					path,
					buffering.BusyBuffersSize,
					"must be less than the size of all proxy_buffers minus one buffer",
				))
			}
		}
	}

	return allErrs
}

// ParseNginxSize parses an NGINX size string (e.g., "8k", "16m", "1024") and returns the size in bytes.
// Returns an error if the size string is invalid.
func ParseNginxSize(size string) (int64, error) {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" {
		return 0, strconv.ErrSyntax
	}

	var multiplier int64
	var numberPart string

	// Check for unit suffix
	switch {
	case strings.HasSuffix(size, "k"):
		multiplier = 1024
		numberPart = strings.TrimSuffix(size, "k")
	case strings.HasSuffix(size, "m"):
		multiplier = 1024 * 1024
		numberPart = strings.TrimSuffix(size, "m")
	case strings.HasSuffix(size, "g"):
		multiplier = 1024 * 1024 * 1024
		numberPart = strings.TrimSuffix(size, "g")
	default:
		multiplier = 1
		numberPart = size
	}

	num, err := strconv.ParseInt(numberPart, 10, 64)
	if err != nil {
		return 0, err
	}

	return num * multiplier, nil
}
