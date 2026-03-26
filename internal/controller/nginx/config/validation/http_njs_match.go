package validation

import (
	"errors"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// HTTPNJSMatchValidator validates values used for matching a request.
// The matching is implemented in NJS (except for path matching),
// so changes to the implementation change the validation rules here.
type HTTPNJSMatchValidator struct{}

func (HTTPNJSMatchValidator) ValidateHeaderNameInMatch(name string) error {
	if err := k8svalidation.IsHTTPHeaderName(name); err != nil {
		return errors.New(err[0])
	}

	return validateNJSHeaderPart(name)
}

func (HTTPNJSMatchValidator) ValidateHeaderValueInMatch(value string) error {
	return validateNJSHeaderPart(value)
}

func validateNJSHeaderPart(value string) error {
	return validateCommonNJSMatchPart(value)
}

func (HTTPNJSMatchValidator) ValidateQueryParamNameInMatch(name string) error {
	return validateCommonNJSMatchPart(name)
}

func (HTTPNJSMatchValidator) ValidateQueryParamValueInMatch(value string) error {
	return validateCommonNJSMatchPart(value)
}

// validateCommonNJSMatchPart validates a string value used in NJS-based matching.
func validateCommonNJSMatchPart(value string) error {
	// empty values do not make sense, so we don't allow them.

	if value == "" {
		return errors.New("cannot be empty")
	}

	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return errors.New("cannot be empty after trimming whitespace")
	}

	return nil
}

// NGINX does not support CONNECT, TRACE methods (it will return 405 Not Allowed to clients).
var supportedMethods = map[string]struct{}{
	"GET":     {},
	"HEAD":    {},
	"POST":    {},
	"PUT":     {},
	"DELETE":  {},
	"OPTIONS": {},
	"PATCH":   {},
}

func (HTTPNJSMatchValidator) ValidateMethodInMatch(method string) (valid bool, supportedValues []string) {
	return validateInSupportedValues(method, supportedMethods)
}
