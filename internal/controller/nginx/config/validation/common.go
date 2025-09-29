package validation

import (
	"errors"
	"fmt"
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

// validatePathInMatch a path used in the location directive.
func validatePathInMatch(path string) error {
	if path == "" {
		return errors.New("cannot be empty")
	}

	if !pathRegexp.MatchString(path) {
		msg := k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...)
		return errors.New(msg)
	}

	return nil
}

// validatePathInRegexMatch a path used in a regex location directive.
// 1. Must be non-empty and start with '/'
// 2. Forbidden characters in NGINX location context: {}, ;, whitespace
// 3. Must compile under Go's regexp (RE2)
// 4. Disallow unescaped '$' (NGINX variables / PCRE backrefs)
// 5. Disallow lookahead/lookbehind (unsupported in RE2)
// 6. Disallow backreferences like \1, \2 (RE2 unsupported).
func validatePathInRegexMatch(path string) error {
	if path == "" {
		return errors.New("cannot be empty")
	}

	if !pathRegexp.MatchString(path) {
		return errors.New(k8svalidation.RegexError(pathErrMsg, pathFmt, pathExamples...))
	}

	if _, err := regexp.Compile(path); err != nil {
		return fmt.Errorf("invalid RE2 regex for path '%s': %w", path, err)
	}

	for i := range len(path) {
		if path[i] == '$' && (i == 0 || path[i-1] != '\\') {
			return fmt.Errorf("invalid unescaped `$` at position %d in path '%s'", i, path)
		}
	}

	lookarounds := []string{"(?=", "(?!", "(?<=", "(?<!"}
	for _, la := range lookarounds {
		if strings.Contains(path, la) {
			return fmt.Errorf("lookahead/lookbehind '%s' found in path '%s' which is not supported in RE2", la, path)
		}
	}

	backref := regexp.MustCompile(`\\[0-9]+`)
	matches := backref.FindAllStringIndex(path, -1)
	if len(matches) > 0 {
		var positions []string
		for _, m := range matches {
			positions = append(positions, fmt.Sprintf("[%d-%d]", m[0], m[1]))
		}
		return fmt.Errorf("backreference(s) %v found in path '%s' which are not supported in RE2", positions, path)
	}

	return nil
}
