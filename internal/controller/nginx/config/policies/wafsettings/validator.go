package wafsettings

import (
	"errors"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
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

	targetRefPath := field.NewPath("spec").Child("targetRef")
	supportedKinds := []gatewayv1.Kind{kinds.Gateway, kinds.HTTPRoute, kinds.GRPCRoute}
	supportedGroups := []gatewayv1.Group{gatewayv1.GroupName}

	if err := policies.ValidateTargetRef(wp.Spec.TargetRef, targetRefPath, supportedGroups, supportedKinds); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	if err := v.validateSettings(wp.Spec); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
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

	// FIXME(ciarams87): Update to condition reason from conditions package when available.
	if !globalSettings.WAFEnabled {
		return []conditions.Condition{
			conditions.NewPolicyNotAcceptedNginxProxyNotSet("WAF is not enabled in NginxProxy"),
		}
	}
	return nil
}

// Conflicts returns false as we don't allow merging for WAFPolicies.
func (v Validator) Conflicts(polA, polB policies.Policy) bool {
	_ = helpers.MustCastObject[*ngfAPI.WAFPolicy](polA)
	_ = helpers.MustCastObject[*ngfAPI.WAFPolicy](polB)
	return false
}

func (v *Validator) validateSettings(spec ngfAPI.WAFPolicySpec) error {
	var allErrs field.ErrorList
	fieldPath := field.NewPath("spec")

	if spec.PolicySource != nil {
		allErrs = append(allErrs, v.validatePolicySource(*spec.PolicySource, fieldPath.Child("policySource"))...)
	}

	for i, sl := range spec.SecurityLogs {
		logPath := fieldPath.Child("securityLogs").Index(i)
		if sl.LogProfileBundle != nil {
			allErrs = append(allErrs, v.validatePolicySource(*sl.LogProfileBundle, logPath.Child("logProfileBundle"))...)
		}
	}

	return allErrs.ToAggregate()
}

func (v *Validator) validatePolicySource(source ngfAPI.WAFPolicySource, fieldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if err := v.validateFileLocation(source.FileLocation); err != nil {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("fileLocation"), source.FileLocation, err.Error()))
	}

	if source.Polling != nil {
		if source.Polling.ChecksumLocation != nil {
			if err := v.validateFileLocation(*source.Polling.ChecksumLocation); err != nil {
				path := fieldPath.Child("polling").Child("checksumLocation")
				allErrs = append(allErrs, field.Invalid(path, *source.Polling.ChecksumLocation, err.Error()))
			}
		}
	}

	return allErrs
}

// validateFileLocation validates that the file location is a valid URL.
// Supports HTTP, HTTPS, and S3 URLs:
// - HTTP/HTTPS: Standard web URLs (e.g., https://example.com/file.tgz)
// - S3: Amazon S3 URLs (e.g., s3://bucket-name/path/to/file.tgz).
func (v *Validator) validateFileLocation(location string) error {
	if location == "" {
		return errors.New("fileLocation cannot be empty")
	}

	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return v.validateHTTPURL(location)
	}

	if strings.HasPrefix(location, "s3://") {
		return v.validateS3URL(location)
	}

	return errors.New("unsupported URL scheme: must be http, https, or s3")
}

func (v *Validator) validateHTTPURL(url string) error {
	// Basic HTTP URL validation - just ensure it has a host
	httpRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?(\:[0-9]+)?(/.*)?$`)
	if !httpRegex.MatchString(url) {
		return errors.New("invalid HTTP/HTTPS URL format")
	}
	return nil
}

func (v *Validator) validateS3URL(url string) error {
	s3Regex := regexp.MustCompile(`^s3://[a-zA-Z0-9][a-zA-Z0-9\-\.]*(/.*)?$`)
	if !s3Regex.MatchString(url) {
		return errors.New("invalid S3 URL format: must be s3://bucket-name[/path]")
	}
	return nil
}
