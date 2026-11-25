package proxysettings

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	httpConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const (
	cacheCodeFmt      = `^(?:[1-5][0-9]{2}|any)$`
	cacheCodeErrorMsg = `must be a 3-digit HTTP status code between 100 and 599 or "any"`

	cacheMethodFmt      = `^(GET|POST|HEAD)$`
	cacheMethodErrorMsg = "must be a valid HTTP method"
)

var httpMethodRegexp = regexp.MustCompile(cacheMethodFmt)

// Validator validates a ProxySettingsPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
	plusEnabled      bool
}

// NewValidator returns a new instance of Validator.
func NewValidator(genericValidator validation.GenericValidator, plusEnabled bool) *Validator {
	return &Validator{
		genericValidator: genericValidator,
		plusEnabled:      plusEnabled,
	}
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
// A policy that enables caching requires a cache zone (proxy_cache_path) to be configured via the
// NginxProxy resource; without it, the generated "proxy_cache" directive references an undefined zone
// and NGINX rejects the configuration.
func (v *Validator) ValidateGlobalSettings(
	policy policies.Policy,
	globalSettings *policies.GlobalSettings,
) []conditions.Condition {
	psp := helpers.MustCastObject[*ngfAPI.ProxySettingsPolicy](policy)

	if psp.Spec.Caching == nil || psp.Spec.Caching.Enable == nil || !*psp.Spec.Caching.Enable {
		return nil
	}

	if globalSettings == nil || !globalSettings.CacheConfigured {
		return []conditions.Condition{
			conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageCacheNotConfigured),
		}
	}

	return nil
}

// Conflicts returns true if the two ProxySettingsPolicies conflict.
func (v *Validator) Conflicts(polA, polB policies.Policy) bool {
	pspA := helpers.MustCastObject[*ngfAPI.ProxySettingsPolicy](polA)
	pspB := helpers.MustCastObject[*ngfAPI.ProxySettingsPolicy](polB)

	return conflicts(pspA.Spec, pspB.Spec)
}

func conflicts(a, b ngfAPI.ProxySettingsPolicySpec) bool {
	return bufferingConflicts(a.Buffering, b.Buffering) ||
		timeoutConflicts(a.Timeout, b.Timeout) ||
		cachingConflicts(a.Caching, b.Caching)
}

func bufferingConflicts(a, b *ngfAPI.ProxyBuffering) bool {
	if a == nil || b == nil {
		return false
	}

	return bothSet(a.Disable, b.Disable) ||
		bothSet(a.BufferSize, b.BufferSize) ||
		bothSet(a.Buffers, b.Buffers) ||
		bothSet(a.BusyBuffersSize, b.BusyBuffersSize)
}

func timeoutConflicts(a, b *ngfAPI.ProxyTimeout) bool {
	if a == nil || b == nil {
		return false
	}

	return bothSet(a.Connect, b.Connect) ||
		bothSet(a.Read, b.Read) ||
		bothSet(a.Send, b.Send)
}

func cachingConflicts(a, b *ngfAPI.ProxyCache) bool {
	if a == nil || b == nil {
		return false
	}

	return bothSet(a.Enable, b.Enable) ||
		validConflicts(a.Valid, b.Valid) ||
		bothSet(a.Key, b.Key) ||
		bothSliceSet(a.UseStale, b.UseStale) ||
		bothSet(a.Revalidate, b.Revalidate) ||
		bothSet(a.BackgroundUpdate, b.BackgroundUpdate) ||
		bothSet(a.MinUses, b.MinUses) ||
		bothSliceSet(a.Methods, b.Methods) ||
		cacheLockConflicts(a.Lock, b.Lock) ||
		cacheConditionsConflicts(a.Conditions, b.Conditions) ||
		bothSliceSet(a.IgnoreHeaders, b.IgnoreHeaders) ||
		bothSliceSet(a.CachePurgeAllow, b.CachePurgeAllow)
}

func validConflicts(a, b *ngfAPI.ProxyCacheValid) bool {
	if a == nil || b == nil {
		return false
	}

	return bothSliceSet(a.Codes, b.Codes) || bothSet(a.Time, b.Time)
}

func cacheLockConflicts(a, b *ngfAPI.ProxyCacheLock) bool {
	if a == nil || b == nil {
		return false
	}

	return bothSet(a.Enable, b.Enable) ||
		bothSet(a.Timeout, b.Timeout) ||
		bothSet(a.Age, b.Age)
}

func cacheConditionsConflicts(a, b *ngfAPI.ProxyCacheConditions) bool {
	if a == nil || b == nil {
		return false
	}

	return bothSliceSet(a.NoCache, b.NoCache) || bothSliceSet(a.Bypass, b.Bypass)
}

func bothSliceSet[T any](a, b []T) bool {
	return a != nil && b != nil
}

func bothSet[T any](a, b *T) bool {
	return a != nil && b != nil
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

	if spec.Timeout != nil {
		allErrs = append(allErrs, v.validateTimeouts(*spec.Timeout, fieldPath.Child("timeout"))...)
	}

	if spec.Caching != nil {
		allErrs = append(allErrs, v.validateCaching(*spec.Caching, fieldPath.Child("caching"))...)
	}

	return allErrs.ToAggregate()
}

func (v *Validator) validateCaching(caching ngfAPI.ProxyCache, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if caching.Valid != nil {
		allErrs = append(allErrs, v.validateCacheValid(caching.Valid, fieldPath)...)
	}

	if caching.Key != nil {
		if err := v.genericValidator.ValidateEscapedString(*caching.Key); err != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("key"), caching.Key, err.Error()))
		}
	}

	if caching.UseStale != nil {
		if err := validateUseStale(caching.UseStale); err != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("useStale"), caching.UseStale, err.Error()))
		}
	}

	if caching.Methods != nil {
		methodsPath := fieldPath.Child("methods")
		if err := validateCacheMethods(caching.Methods); err != nil {
			allErrs = append(allErrs, field.Invalid(methodsPath, caching.Methods, err.Error()))
		}
	}

	if caching.Lock != nil {
		allErrs = append(allErrs, v.validateCacheLock(caching.Lock, fieldPath)...)
	}

	if caching.Conditions != nil {
		allErrs = append(allErrs, v.validateCacheConditions(caching.Conditions, fieldPath)...)
	}

	if caching.IgnoreHeaders != nil {
		if err := validateIgnoreHeaders(caching.IgnoreHeaders); err != nil {
			allErrs = append(allErrs,
				field.Invalid(fieldPath.Child("ignoreHeaders"), caching.IgnoreHeaders, err.Error()),
			)
		}
	}

	if caching.CachePurgeAllow != nil {
		if !v.plusEnabled {
			path := fieldPath.Child("cachePurgeAllow")
			allErrs = append(allErrs, field.Forbidden(path, "cachePurgeAllow is an NGINX Plus only feature"))
		}

		allErrs = append(allErrs, v.validateEscapedStrings(caching.CachePurgeAllow, fieldPath.Child("cachePurgeAllow"))...)
	}

	return allErrs
}

func (v *Validator) validateCacheValid(valid *ngfAPI.ProxyCacheValid, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if valid.Codes != nil {
		codesPath := fieldPath.Child("valid").Child("codes")
		if err := validateCacheCodes(valid.Codes); err != nil {
			allErrs = append(allErrs, field.Invalid(codesPath, valid.Codes, err.Error()))
		}
	}

	if valid.Time != nil {
		duration := string(*valid.Time)
		timePath := fieldPath.Child("valid").Child("time")

		if strings.HasSuffix(duration, "ms") {
			allErrs = append(allErrs, field.Invalid(
				timePath,
				valid.Time,
				"millisecond durations are not supported for proxy_cache_valid",
			))
		} else if err := v.genericValidator.ValidateNginxDuration(string(*valid.Time)); err != nil {
			allErrs = append(allErrs, field.Invalid(timePath, valid.Time, err.Error()))
		}
	}

	return allErrs
}

func validateCacheCodes(codes []intstr.IntOrString) error {
	for _, code := range codes {
		valid := false

		switch code.Type {
		case intstr.Int:
			valid = code.IntVal >= 100 && code.IntVal <= 599
		case intstr.String:
			valid = code.StrVal == "any"
			if !valid && len(code.StrVal) == 3 {
				codeNum, err := strconv.Atoi(code.StrVal)
				valid = err == nil && codeNum >= 100 && codeNum <= 599
			}
		}

		if !valid {
			examples := []string{
				"200",
				"404",
				"503",
				"any",
			}

			return errors.New(k8svalidation.RegexError(cacheCodeErrorMsg, cacheCodeFmt, examples...))
		}
	}

	return nil
}

func validateUseStale(useStale []ngfAPI.UseStale) error {
	supportedResponses := httpConfig.SupportedUseStaleResponses
	for _, res := range useStale {
		if _, ok := supportedResponses[res]; !ok {
			responses := make([]string, 0, len(supportedResponses))
			for response := range supportedResponses {
				responses = append(responses, string(response))
			}
			sort.Strings(responses)

			return fmt.Errorf(
				"caching supports the following useStale responses: %s",
				strings.Join(responses, ", "),
			)
		}
	}

	if len(useStale) > 1 && slices.Contains(useStale, ngfAPI.UseStaleOff) {
		return errors.New(`"off" cannot be combined with other useStale responses`)
	}

	return nil
}

func validateCacheMethods(methods []string) error {
	for _, method := range methods {
		if !httpMethodRegexp.MatchString(method) {
			examples := []string{
				"GET",
				"HEAD",
				"POST",
			}

			return errors.New(k8svalidation.RegexError(cacheMethodErrorMsg, cacheMethodFmt, examples...))
		}
	}

	return nil
}

func (v *Validator) validateCacheLock(lock *ngfAPI.ProxyCacheLock, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if lock.Timeout != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*lock.Timeout)); err != nil {
			allErrs = append(allErrs,
				field.Invalid(fieldPath.Child("lock").Child("timeout"), lock.Timeout, err.Error()),
			)
		}
	}

	if lock.Age != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*lock.Age)); err != nil {
			allErrs = append(allErrs,
				field.Invalid(fieldPath.Child("lock").Child("age"), lock.Age, err.Error()),
			)
		}
	}

	return allErrs
}

func (v *Validator) validateCacheConditions(conds *ngfAPI.ProxyCacheConditions, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if conds.NoCache != nil {
		allErrs = append(allErrs, v.validateEscapedStrings(
			conds.NoCache, fieldPath.Child("conditions").Child("noCache"))...,
		)
	}

	if conds.Bypass != nil {
		allErrs = append(allErrs, v.validateEscapedStrings(
			conds.Bypass, fieldPath.Child("conditions").Child("bypass"))...,
		)
	}

	return allErrs
}

func validateIgnoreHeaders(ignoreHeaders []ngfAPI.ProxyCacheIgnoreHeader) error {
	supportedHeaders := httpConfig.SupportedHeaders

	for _, header := range ignoreHeaders {
		if _, ok := supportedHeaders[header]; !ok {
			headers := make([]string, 0, len(supportedHeaders))
			for header := range supportedHeaders {
				headers = append(headers, string(header))
			}
			sort.Strings(headers)

			return fmt.Errorf(
				"caching supports the following ignoreHeaders: %s",
				strings.Join(headers, ", "),
			)
		}
	}

	return nil
}

func (v *Validator) validateEscapedStrings(values []string, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, value := range values {
		if err := v.genericValidator.ValidateEscapedString(value); err != nil {
			allErrs = append(allErrs, field.Invalid(
				fieldPath.Index(i),
				value,
				err.Error(),
			))
		}
	}

	return allErrs
}

func (v *Validator) validateTimeouts(timeout ngfAPI.ProxyTimeout, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if timeout.Connect != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*timeout.Connect)); err != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("connect"), timeout.Connect, err.Error()))
		}
	}

	if timeout.Read != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*timeout.Read)); err != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("read"), timeout.Read, err.Error()))
		}
	}

	if timeout.Send != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*timeout.Send)); err != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("send"), timeout.Send, err.Error()))
		}
	}

	return allErrs
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
// 2. proxy_busy_buffers_size < (proxy_buffers.number * proxy_buffers.size) - proxy_buffers.size (when buffers are set)
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
