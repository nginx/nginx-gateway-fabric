package upstreamsettings

import (
	"fmt"
	"strings"

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

	if !checkConflictForLoadBalancingFields(a, b) {
		return true
	}

	return false
}

func checkConflictForLoadBalancingFields(a, b ngfAPI.UpstreamSettingsPolicySpec) bool {
	if a.LoadBalancingMethod != nil && b.LoadBalancingMethod != nil {
		return *a.LoadBalancingMethod == *b.LoadBalancingMethod
	}

	if a.HashMethodKey != nil && b.HashMethodKey != nil {
		return *a.HashMethodKey == *b.HashMethodKey
	}

	return true
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
	return strings.Join(methods, ", ")
}
