package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/nginx/config"
)

// HTTPNJSMatchValidator validates values used for matching a request.
// The matching is implemented in NJS (except for path matching),
// so changes to the implementation change the validation rules here.
type HTTPNJSMatchValidator struct{}

const (
	prefixPathFmt    = `/[^\s{};]*`
	prefixPathErrMsg = "must start with / and must not include any whitespace character, `{`, `}` or `;`"
)

var (
	prefixPathRegexp   = regexp.MustCompile("^" + prefixPathFmt + "$")
	prefixPathExamples = []string{"/", "/path", "/path/subpath-123"}
)

// ValidatePathInPrefixMatch a prefix path used in the location directive.
func (HTTPNJSMatchValidator) ValidatePathInPrefixMatch(path string) error {
	if path == "" {
		return errors.New("cannot be empty")
	}

	if !prefixPathRegexp.MatchString(path) {
		msg := k8svalidation.RegexError(prefixPathErrMsg, prefixPathFmt, prefixPathExamples...)
		return errors.New(msg)
	}

	// FIXME(pleshakov): This is temporary until https://github.com/nginxinc/nginx-kubernetes-gateway/issues/428
	// is fixed.
	// That's because the location path gets into the set directive in the location block.
	// Example: set $http_matches "[{\"redirectPath\":\"/coffee_route0\" ...
	// Where /coffee is tha path.
	return validateCommonNJSMatchPart(path)
}

func (HTTPNJSMatchValidator) ValidateHeaderNameInMatch(name string) error {
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
