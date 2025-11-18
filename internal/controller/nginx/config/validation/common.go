package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

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

type HTTPDurationValidator struct{}

func (d HTTPDurationValidator) ValidateDuration(duration string) (string, error) {
	return d.validateDurationCanBeConvertedToNginxFormat(duration)
}

// validateDurationCanBeConvertedToNginxFormat parses a Gateway API duration and returns a single-unit,
// NGINX-friendly duration that matches `^[0-9]{1,4}(ms|s|m|h)?$`
// The conversion rules are:
//   - duration must be > 0
//   - ceil to the next millisecond
//   - choose the smallest unit (ms→s→m→h) whose ceil value fits in 1–4 digits
//   - always include a unit suffix
func (d HTTPDurationValidator) validateDurationCanBeConvertedToNginxFormat(in string) (string, error) {
	td, err := time.ParseDuration(in)
	if err != nil {
		return "", fmt.Errorf("invalid duration: %w", err)
	}
	if td <= 0 {
		return "", errors.New("duration must be > 0")
	}

	ns := td.Nanoseconds()
	ceilDivision := func(a, b int64) int64 {
		return (a + b - 1) / b
	}

	totalMS := ceilDivision(ns, int64(time.Millisecond))

	type unit struct {
		suffix string
		step   int64
	}

	units := []unit{
		{"ms", 1},
		{"s", 1000},
		{"m", 60 * 1000},
		{"h", 60 * 60 * 1000},
	}

	const maxValue = 9999
	var out string
	for _, u := range units {
		v := ceilDivision(totalMS, u.step)
		if v >= 1 && v <= maxValue {
			out = fmt.Sprintf("%d%s", v, u.suffix)
			break
		}
	}
	if out == "" {
		return "", fmt.Errorf("duration is too large for NGINX format (exceeds %dh)", maxValue)
	}

	if !durationStringFmtRegexp.MatchString(out) {
		return "", fmt.Errorf("computed duration %q does not match NGINX format", out)
	}
	return out, nil
}
