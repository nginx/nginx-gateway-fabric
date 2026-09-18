package upstreamsettings

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	httpConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const (
	healthCheckPathFmt      = `^[^\s{};$\\]*$`
	healthCheckPathErrorMsg = "must be a valid URI path starting with a '/' containing no spaces or control characters"

	healthCheckMatchStatusFmt      = `^(!\s+)?\d{3}(-\d{3})?(\s+\d{3}(-\d{3})?)*$`
	healthCheckMatchStatusErrorMsg = "must be a valid 3-digit HTTP response code or range of codes, " +
		"optionally containing a '!'"

	grpcServiceFmt      = `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`
	grpcServiceErrorMsg = "must be a valid gRPC service name, consisting of one of more dot-separated segments, " +
		"each containing only letters, digits and underscores"

	httpHeaderNameFmt      = `^[A-Za-z0-9!#$%&'*+\-.^_\x60|~]+$`
	httpHeaderNameErrorMsg = "must be a valid HTTP header name"

	httpHeaderValueFmt      = `^[!-~]+([\t ]?[!-~]+)*$`
	httpHeaderValueErrorMsg = "Must consist of printable US-ASCII characters, optionally separated " +
		"by single tabs or spaces"
)

var (
	healthCheckPathRegexp        = regexp.MustCompile(healthCheckPathFmt)
	healthCheckMatchStatusRegexp = regexp.MustCompile(healthCheckMatchStatusFmt)
	grpcServiceRegexp            = regexp.MustCompile(grpcServiceFmt)
	httpHeaderNameRegexp         = regexp.MustCompile(httpHeaderNameFmt)
	httpHeaderValueRegexp        = regexp.MustCompile(httpHeaderValueFmt)
)

// Validator validates an UpstreamSettingsPolicy.
// Implements policies.Validator interface.
type Validator struct {
	genericValidator validation.GenericValidator
	plusEnabled      bool
}

// NewValidator returns a new Validator.
func NewValidator(genericValidator validation.GenericValidator, plusEnabled bool) Validator {
	return Validator{
		genericValidator: genericValidator,
		plusEnabled:      plusEnabled,
	}
}

// Validate validates the spec of an UpstreamsSettingsPolicy.
func (v Validator) Validate(policy policies.Policy) []conditions.Condition {
	usp := helpers.MustCastObject[*ngfAPI.UpstreamSettingsPolicy](policy)

	targetRefsPath := field.NewPath("spec").Child("targetRefs")
	supportedKinds := []gatewayv1.Kind{kinds.Service}
	supportedGroups := []gatewayv1.Group{"", "core"}

	for i, ref := range usp.Spec.TargetRefs {
		indexedPath := targetRefsPath.Index(i)
		if err := policies.ValidateTargetRef(ref, indexedPath, supportedGroups, supportedKinds); err != nil {
			return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
		}
	}

	if err := v.validateSettings(usp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// ValidateGlobalSettings validates an UpstreamSettingsPolicy with respect to the NginxProxy global settings.
func (v Validator) ValidateGlobalSettings(
	_ policies.Policy,
	_ *policies.GlobalSettings,
) []conditions.Condition {
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

	if healthCheckConflicts(a.HealthCheck, b.HealthCheck) {
		return true
	}

	if resolveConflictsInUpstreamSettings(a, b) {
		return true
	}

	return false
}

func healthCheckConflicts(a, b *ngfAPI.HealthCheck) bool {
	if a == nil || b == nil {
		return false
	}

	if a.Passive != nil && b.Passive != nil {
		if a.Passive.MaxFails != nil && b.Passive.MaxFails != nil {
			return true
		}
		if a.Passive.FailTimeout != nil && b.Passive.FailTimeout != nil {
			return true
		}
	}

	return activeHealthCheckConflicts(a.Active, b.Active)
}

//nolint:gocyclo // additional abstraction would make readability worse
func activeHealthCheckConflicts(a, b *ngfAPI.ActiveHealthCheck) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Interval != nil && b.Interval != nil {
		return true
	}
	if a.Jitter != nil && b.Jitter != nil {
		return true
	}
	if a.Fails != nil && b.Fails != nil {
		return true
	}
	if a.Passes != nil && b.Passes != nil {
		return true
	}
	if a.Path != nil && b.Path != nil {
		return true
	}
	if a.Port != nil && b.Port != nil {
		return true
	}
	if a.Mandatory != nil && b.Mandatory != nil {
		return true
	}
	if a.Persistent != nil && b.Persistent != nil {
		return true
	}
	if a.KeepAliveTime != nil && b.KeepAliveTime != nil {
		return true
	}
	if len(a.Headers) > 0 && len(b.Headers) > 0 {
		return true
	}
	if a.Match != nil && b.Match != nil {
		if a.Match.Status != nil && b.Match.Status != nil {
			return true
		}
	}
	if a.GRPC != nil && b.GRPC != nil {
		if a.GRPC.Service != nil && b.GRPC.Service != nil {
			return true
		}
		if a.GRPC.Status != nil && b.GRPC.Status != nil {
			return true
		}
	}
	if a.Timeout != nil && b.Timeout != nil {
		if a.Timeout.Connect != nil && b.Timeout.Connect != nil {
			return true
		}
		if a.Timeout.Read != nil && b.Timeout.Read != nil {
			return true
		}
		if a.Timeout.Send != nil && b.Timeout.Send != nil {
			return true
		}
	}

	return false
}

func resolveConflictsInUpstreamSettings(a, b ngfAPI.UpstreamSettingsPolicySpec) bool {
	if a.LoadBalancingMethod != nil && b.LoadBalancingMethod != nil {
		return true
	}

	if a.HashMethodKey != nil && b.HashMethodKey != nil {
		return true
	}

	if a.UseClusterIP != nil && b.UseClusterIP != nil {
		return true
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

	if spec.HealthCheck != nil {
		allErrs = append(allErrs, v.validateHealthCheck(*spec.HealthCheck, fieldPath.Child("healthCheck"))...)
	}

	allErrs = append(allErrs, v.validateLoadBalancingMethod(spec)...)

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

func (v Validator) validateHealthCheck(
	healthCheck ngfAPI.HealthCheck,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if healthCheck.Passive != nil {
		allErrs = append(allErrs, v.validatePassiveHealthCheck(healthCheck.Passive, fieldPath)...)
	}

	if healthCheck.Active != nil {
		if !v.plusEnabled {
			path := fieldPath.Child("active")

			allErrs = append(allErrs, field.Forbidden(
				path,
				"active health checks are only supported with NGINX Plus",
			))
		}

		allErrs = append(allErrs, v.validateActiveHealthCheck(healthCheck.Active, fieldPath)...)
	}

	return allErrs
}

func (v Validator) validatePassiveHealthCheck(
	passive *ngfAPI.PassiveHealthCheck,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if passive.FailTimeout != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*passive.FailTimeout)); err != nil {
			path := fieldPath.Child("passive").Child("failTimeout")

			allErrs = append(allErrs, field.Invalid(path, *passive.FailTimeout, err.Error()))
		}
	}

	return allErrs
}

func (v Validator) validateActiveHealthCheck(
	active *ngfAPI.ActiveHealthCheck,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if active.Interval != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*active.Interval)); err != nil {
			path := fieldPath.Child("active").Child("interval")

			allErrs = append(allErrs, field.Invalid(path, *active.Interval, err.Error()))
		}
	}

	if active.Jitter != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*active.Jitter)); err != nil {
			path := fieldPath.Child("active").Child("jitter")

			allErrs = append(allErrs, field.Invalid(path, *active.Jitter, err.Error()))
		}
	}

	if active.Path != nil && *active.Path != "" {
		if err := validateHealthCheckPath(active.Path); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					fieldPath.Child("active").Child("path"),
					active.Path,
					err.Error(),
				),
			)
		}
	}

	if isMatchStatusSet(active) {
		if err := validateHealthCheckMatchStatus(active.Match.Status); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					fieldPath.Child("active").Child("match").Child("status"),
					active.Match.Status,
					err.Error(),
				),
			)
		}
	}

	if active.GRPC != nil {
		allErrs = append(allErrs, validateGRPCHealthCheck(active.GRPC, fieldPath)...)
	}

	if active.KeepAliveTime != nil {
		if err := v.genericValidator.ValidateNginxDuration(string(*active.KeepAliveTime)); err != nil {
			path := fieldPath.Child("active").Child("keepAliveTime")

			allErrs = append(allErrs, field.Invalid(path, *active.KeepAliveTime, err.Error()))
		}
	}

	if active.Timeout != nil {
		allErrs = append(
			allErrs,
			v.validateTimeouts(
				*active.Timeout,
				fieldPath.Child("active").Child("timeout"),
			)...,
		)
	}

	if active.Headers != nil {
		allErrs = append(
			allErrs,
			validateHTTPHeaders(
				active.Headers,
				fieldPath.Child("active").Child("header"),
			)...,
		)
	}

	return allErrs
}

func validateHTTPHeaders(
	headers []gatewayv1.HTTPHeader,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	for index, header := range headers {
		if header.Name != "" {
			if err := validateHTTPHeaderName(header.Name); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						fieldPath.Index(index).Child("name"),
						header.Name,
						err.Error(),
					),
				)
			}
		}

		if header.Value != "" {
			if err := validateHTTPHeaderValue(&header.Value); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						fieldPath.Index(index).Child("value"),
						header.Value,
						err.Error(),
					),
				)
			}
		}
	}

	return allErrs
}

func validateHTTPHeaderName(name gatewayv1.HTTPHeaderName) error {
	if !httpHeaderNameRegexp.MatchString(string(name)) {
		examples := []string{
			"Content-Type",
			"X-Request-ID",
		}

		return errors.New(k8svalidation.RegexError(
			httpHeaderNameErrorMsg,
			httpHeaderNameFmt,
			examples...,
		))
	}

	return nil
}

func validateHTTPHeaderValue(value *string) error {
	if !httpHeaderValueRegexp.MatchString(*value) {
		examples := []string{
			"application/json",
			"$remote_addr",
			"Bearer token",
		}

		return errors.New(k8svalidation.RegexError(
			httpHeaderValueErrorMsg,
			httpHeaderValueFmt,
			examples...,
		))
	}

	return nil
}

func validateGRPCHealthCheck(
	grpc *ngfAPI.GRPCHealthCheck,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if grpc.Service != nil {
		if err := validateGRPCService(grpc.Service); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					fieldPath.Child("active").Child("grpc").Child("service"),
					grpc.Service,
					err.Error(),
				),
			)
		}
	}

	if grpc.Status != nil {
		allErrs = append(
			allErrs,
			validateGRPCStatus(
				*grpc.Status,
				fieldPath.Child("active").Child("grpc").Child("status"),
			)...,
		)
	}

	return allErrs
}

func validateHealthCheckPath(path *string) error {
	if !healthCheckPathRegexp.MatchString(*path) {
		examples := []string{
			"/",
			"/healthz",
		}

		return errors.New(k8svalidation.RegexError(healthCheckPathErrorMsg, healthCheckPathFmt, examples...))
	}

	return nil
}

func validateHealthCheckMatchStatus(status *string) error {
	if !healthCheckMatchStatusRegexp.MatchString(*status) {
		examples := []string{
			"200",
			"! 500",
			"200 204",
			"200-399",
		}

		return errors.New(k8svalidation.RegexError(healthCheckMatchStatusErrorMsg, healthCheckMatchStatusFmt, examples...))
	}

	return nil
}

func isMatchStatusSet(active *ngfAPI.ActiveHealthCheck) bool {
	return active.Match != nil && active.Match.Status != nil && *active.Match.Status != ""
}

func validateGRPCService(service *string) error {
	if !grpcServiceRegexp.MatchString(*service) {
		examples := []string{
			"my.grpc.Service",
		}

		return errors.New(k8svalidation.RegexError(grpcServiceErrorMsg, grpcServiceFmt, examples...))
	}

	return nil
}

func validateGRPCStatus(status ngfAPI.GRPCStatus, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allowedStatuses := httpConfig.AllowedGRPCStatuses
	allowedStatusCodes := httpConfig.AllowedGRPCStatusCodes

	_, statusOk := allowedStatuses[status]
	_, codeOk := allowedStatusCodes[status]

	if !statusOk || !codeOk {
		statuses, codes := getGRPCStatusList(allowedStatuses, allowedStatusCodes)
		allErrs = append(allErrs, field.Invalid(
			fieldPath,
			status,
			fmt.Sprintf(
				"Health Checks support the following status codes: %s %s",
				statuses,
				codes,
			),
		))
	}

	return allErrs
}

func getGRPCStatusList(allowedStatuses, allowedStatusCodes map[ngfAPI.GRPCStatus]struct{}) (string, string) {
	statuses := make([]string, 0, len(allowedStatuses))
	for status := range allowedStatuses {
		statuses = append(statuses, string(status))
	}

	codes := make([]string, 0, len(allowedStatusCodes))
	for code := range allowedStatusCodes {
		codes = append(codes, string(code))
	}

	sort.Strings(statuses)
	sort.Strings(codes)
	return strings.Join(statuses, ", "), strings.Join(codes, ", ")
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

// ValidateLoadBalancingMethod validates the load balancing method for upstream servers.
func (v Validator) validateLoadBalancingMethod(spec ngfAPI.UpstreamSettingsPolicySpec) field.ErrorList {
	if spec.LoadBalancingMethod == nil {
		return nil
	}

	var allErrs field.ErrorList
	path := field.NewPath("spec")
	lbPath := path.Child("loadBalancingMethod")

	allowedMethods := httpConfig.OSSAllowedLBMethods
	nginxType := "NGINX OSS"
	if v.plusEnabled {
		allowedMethods = httpConfig.PlusAllowedLBMethods
		nginxType = "NGINX Plus"
	}

	if _, ok := allowedMethods[*spec.LoadBalancingMethod]; !ok {
		allErrs = append(allErrs, field.Invalid(
			lbPath,
			*spec.LoadBalancingMethod,
			fmt.Sprintf(
				"%s supports the following load balancing methods: %s",
				nginxType,
				getLoadBalancingMethodList(allowedMethods),
			),
		))
	}

	if spec.HashMethodKey != nil {
		hashMethodKey := *spec.HashMethodKey
		if err := v.genericValidator.ValidateNginxVariableName(string(hashMethodKey)); err != nil {
			path := path.Child("hashMethodKey")
			allErrs = append(allErrs, field.Invalid(path, hashMethodKey, err.Error()))
		}
	}

	return allErrs
}

func getLoadBalancingMethodList(lbMethods map[ngfAPI.LoadBalancingType]struct{}) string {
	methods := make([]string, 0, len(lbMethods))
	for method := range lbMethods {
		methods = append(methods, string(method))
	}
	// Sort so that the error message is deterministic regardless of map iteration order.
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}
