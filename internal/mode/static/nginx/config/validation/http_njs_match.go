package validation

import (
	"errors"
	"fmt"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config"
)

// HTTPNJSMatchValidator validates values used for matching a request.
// The matching is implemented in NJS (except for path matching),
// so changes to the implementation change the validation rules here.
type HTTPNJSMatchValidator struct{}

// ValidatePathInMatch a path used in the location directive.
func (HTTPNJSMatchValidator) ValidatePathInMatch(path string) error {
	if path == "" {
		return errors.New("cannot be empty")
	}

	if !pathRegexp.MatchString(path) {
		msg := k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...)
		return errors.New(msg)
	}

	return nil
}

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
	// if it contains the separator, it will break NJS code.
	if strings.Contains(value, config.HeaderMatchSeparator) {
		return fmt.Errorf("cannot contain %q", config.HeaderMatchSeparator)
	}

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

	// the JSON marshaled match (see config.httpMatch) is used as a value of the set directive in a location.
	// The directive supports NGINX variables.
	// We don't want to allow them, as any undefined variable will cause NGINX to fail to reload.
	if strings.Contains(value, "$") {
		return errors.New("cannot contain $")
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
