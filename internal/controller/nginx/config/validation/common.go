package validation

import (
	"errors"
	"regexp"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	pathFmt    = `/[^\s{};]*`
	pathErrMsg = "must start with / and must not include any whitespace character, `{`, `}` or `;`"
)

var (
	pathRegexp   = regexp.MustCompile("^" + pathFmt + "$")
	pathExamples = []string{"/", "/path", "/path/subpath-123"}
)

const (
	escapedStringsFmt    = `([^"\\]|\\.)*`
	escapedStringsErrMsg = `must have all '"' (double quotes) escaped and must not end with an unescaped '\' ` +
		`(backslash)`
)

var escapedStringsFmtRegexp = regexp.MustCompile("^" + escapedStringsFmt + "$")

// validateEscapedString is used to validate a string that is surrounded by " in the NGINX config for a directive
// that doesn't support any regex rules or variables (it doesn't try to expand the variable name behind $).
// For example, server_name "hello $not_a_var world"
// If the value is invalid, the function returns an error that includes the specified examples of valid values.
func validateEscapedString(value string, examples []string) error {
	if !escapedStringsFmtRegexp.MatchString(value) {
		msg := k8svalidation.RegexError(escapedStringsErrMsg, escapedStringsFmt, examples...)
		return errors.New(msg)
	}
	return nil
}

const (
	escapedStringsNoVarExpansionFmt           = `([^"$\\]|\\[^$])*`
	escapedStringsNoVarExpansionErrMsg string = `a valid value must have all '"' escaped and must not contain any ` +
		`'$' or end with an unescaped '\'`
)

var escapedStringsNoVarExpansionFmtRegexp = regexp.MustCompile("^" + escapedStringsNoVarExpansionFmt + "$")

// validateEscapedStringNoVarExpansion is the same as validateEscapedString except it doesn't allow $ to
// prevent variable expansion.
// If the value is invalid, the function returns an error that includes the specified examples of valid values.
func validateEscapedStringNoVarExpansion(value string, examples []string) error {
	if !escapedStringsNoVarExpansionFmtRegexp.MatchString(value) {
		msg := k8svalidation.RegexError(
			escapedStringsNoVarExpansionErrMsg,
			escapedStringsNoVarExpansionFmt,
			examples...,
		)
		return errors.New(msg)
	}
	return nil
}

const (
	invalidHeadersErrMsg string = "unsupported header name configured, unsupported names are: "
	maxHeaderLength      int    = 256
)

var invalidHeaders = map[string]struct{}{
	"host":       {},
	"connection": {},
	"upgrade":    {},
}

func validateHeaderName(name string) error {
	if len(name) > maxHeaderLength {
		return errors.New(k8svalidation.MaxLenError(maxHeaderLength))
	}
	if msg := k8svalidation.IsHTTPHeaderName(name); msg != nil {
		return errors.New(msg[0])
	}
	if valid, invalidHeadersAsStrings := validateNoUnsupportedValues(strings.ToLower(name), invalidHeaders); !valid {
		return errors.New(invalidHeadersErrMsg + strings.Join(invalidHeadersAsStrings, ", "))
	}
	return nil
}

func validatePath(path string) error {
	if path == "" {
		return nil
	}

	if !pathRegexp.MatchString(path) {
		msg := k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...)
		return errors.New(msg)
	}

	if strings.Contains(path, "$") {
		return errors.New("cannot contain $")
	}

	return nil
}
