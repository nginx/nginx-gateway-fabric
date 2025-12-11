package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	pathFmt    = `/[^\s{};]*`
	pathErrMsg = "must start with / and must not include any whitespace character, `{`, `}` or `;`"
)

var (
	pathRegexp   = regexp2.MustCompile("^"+pathFmt+"$", 0)
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

	if valid, err := pathRegexp.MatchString(path); err != nil {
		return fmt.Errorf("failed to validate path %q: %w", path, err)
	} else if !valid {
		msg := k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...)
		return errors.New(msg)
	}

	if strings.Contains(path, "$") {
		return errors.New("cannot contain $")
	}

	return nil
}

// validatePathInMatch a path used in the location directive.
func validatePathInMatch(path string) error {
	if path == "" {
		return errors.New("cannot be empty")
	}
	if valid, err := pathRegexp.MatchString(path); err != nil {
		return fmt.Errorf("failed to validate path in match %q: %w", path, err)
	} else if !valid {
		msg := k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...)
		return errors.New(msg)
	}

	return nil
}

// validatePathInRegexMatch validates a path used in a regex location directive.
//
// It uses Perl5 compatible regexp2 package along with RE2 compatibility.
//
// Checks:
//  1. Non-empty.
//  2. Satisfies NGINX location path shape.
//  3. Compiles as a regexp2 regular expression with RE2 option to support named capturing group.
//     No extra bans on backrefs, lookarounds, '$'.
func validatePathInRegexMatch(path string) error {
	if path == "" {
		return errors.New("cannot be empty")
	}

	if valid, err := pathRegexp.MatchString(path); err != nil {
		return fmt.Errorf("failed to validate path %q: %w", path, err)
	} else if !valid {
		msg := k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...)
		return errors.New(msg)
	}

	if _, err := regexp2.Compile(path, regexp2.RE2); err != nil {
		return fmt.Errorf("invalid regex for path %q: %w", path, err)
	}

	return nil
}
