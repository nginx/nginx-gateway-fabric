package validation

//go:generate go tool counterfeiter -generate

import (
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
)

// Validators include validators for API resources from the perspective of a data-plane.
// It is used for fields that propagate into the data plane configuration. For example, the path in a routing rule.
// However, not all such fields are validated: NGF will not validate a field using Validators if it is confident that
// the field is valid.
type Validators struct {
	HTTPFieldsValidator HTTPFieldsValidator
	GenericValidator    GenericValidator
	PolicyValidator     PolicyValidator
}

// HTTPFieldsValidator validates the HTTP-related fields of Gateway API resources from the perspective of
// a data-plane. Data-plane implementations must implement this interface.
//
//counterfeiter:generate . HTTPFieldsValidator
type HTTPFieldsValidator interface {
	SkipValidation() bool
	ValidatePathInMatch(path string) error
	ValidateHeaderNameInMatch(name string) error
	ValidateHeaderValueInMatch(value string) error
	ValidateQueryParamNameInMatch(name string) error
	ValidateQueryParamValueInMatch(name string) error
	ValidateMethodInMatch(method string) (valid bool, supportedValues []string)
	ValidateRedirectScheme(scheme string) (valid bool, supportedValues []string)
	ValidateRedirectPort(port int32) error
	ValidateRedirectStatusCode(statusCode int) (valid bool, supportedValues []string)
	ValidateHostname(hostname string) error
	ValidateFilterHeaderName(name string) error
	ValidateFilterHeaderValue(value string) error
	ValidatePath(path string) error
}

// GenericValidator validates any generic values from NGF API resources from the perspective of a data-plane.
// These could be values that we want to re-validate in case of any CRD schema manipulation.
//
//counterfeiter:generate . GenericValidator
type GenericValidator interface {
	ValidateEscapedStringNoVarExpansion(value string) error
	ValidateServiceName(name string) error
	ValidateNginxDuration(duration string) error
	ValidateNginxSize(size string) error
	ValidateEndpoint(endpoint string) error
}

// PolicyValidator validates an NGF Policy.
//
//counterfeiter:generate . PolicyValidator
type PolicyValidator interface {
	// Validate validates an NGF Policy.
	Validate(policy policies.Policy) []conditions.Condition
	// ValidateGlobalSettings validates an NGF Policy with the NginxProxy settings.
	ValidateGlobalSettings(policy policies.Policy, globalSettings *policies.GlobalSettings) []conditions.Condition
	// Conflicts returns true if the two Policies conflict.
	Conflicts(a, b policies.Policy) bool
}

// SkipValidator is used to skip validation on internally-created routes for request mirroring.
type SkipValidator struct{}

func (SkipValidator) SkipValidation() bool { return true }

func (SkipValidator) ValidatePathInMatch(string) error                { return nil }
func (SkipValidator) ValidateHeaderNameInMatch(string) error          { return nil }
func (SkipValidator) ValidateHeaderValueInMatch(string) error         { return nil }
func (SkipValidator) ValidateQueryParamNameInMatch(string) error      { return nil }
func (SkipValidator) ValidateQueryParamValueInMatch(string) error     { return nil }
func (SkipValidator) ValidateMethodInMatch(string) (bool, []string)   { return true, nil }
func (SkipValidator) ValidateRedirectScheme(string) (bool, []string)  { return true, nil }
func (SkipValidator) ValidateRedirectPort(int32) error                { return nil }
func (SkipValidator) ValidateRedirectStatusCode(int) (bool, []string) { return true, nil }
func (SkipValidator) ValidateHostname(string) error                   { return nil }
func (SkipValidator) ValidateFilterHeaderName(string) error           { return nil }
func (SkipValidator) ValidateFilterHeaderValue(string) error          { return nil }
func (SkipValidator) ValidatePath(string) error                       { return nil }
