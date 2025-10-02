package validation

import (
	"strings"
	"testing"
)

func TestValidateEscapedString(t *testing.T) {
	t.Parallel()
	validator := func(value string) error { return validateEscapedString(value, []string{"example"}) }

	testValidValuesForSimpleValidator(
		t,
		validator,
		`test`,
		`test test`,
		`\"`,
		`\\`,
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator,
		`\`,
		`test"test`,
	)
}

func TestValidateEscapedStringNoVarExpansion(t *testing.T) {
	t.Parallel()
	validator := func(value string) error { return validateEscapedStringNoVarExpansion(value, []string{"example"}) }

	testValidValuesForSimpleValidator(
		t,
		validator,
		`test`,
		`test test`,
		`\"`,
		`\\`,
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator,
		`\`,
		`test"test`,
		`$test`,
	)
}

func TestValidateValidHeaderName(t *testing.T) {
	t.Parallel()
	validator := validateHeaderName

	testValidValuesForSimpleValidator(
		t,
		validator,
		`Content-Encoding`,
		`X-Forwarded-For`,
		// max supported length is 256, generate string with 16*16 chars (256)
		strings.Repeat("very-long-header", 16),
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator,
		`\`,
		`test test`,
		`test"test`,
		`$test`,
		"Host",
		"host",
		"connection",
		"upgrade",
		"my-header[]",
		"my-header&",
		strings.Repeat("very-long-header", 16)+"1",
	)
}

func TestValidatePathForFilters(t *testing.T) {
	t.Parallel()
	validator := validatePath

	testValidValuesForSimpleValidator(
		t,
		validator,
		`/path`,
		`/longer/path`,
		`/trailing/`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator,
		`path`,
		`$path`,
		"/path$",
	)
}

func TestValidatePathInMatch(t *testing.T) {
	t.Parallel()
	validator := validatePathInMatch

	testValidValuesForSimpleValidator(
		t,
		validator,
		"/",
		"/path",
		"/path/subpath-123",
		"/_ngf-internal-route0-rule0",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator,
		"/ ",
		"/path{",
		"/path}",
		"/path;",
		"path",
		"",
	)
}

func TestValidatePathInRegexMatch(t *testing.T) {
	t.Parallel()
	validator := validatePathInRegexMatch

	testValidValuesForSimpleValidator(
		t,
		validator,
		`/api/v[0-9]+`,
		`/users/(?P<id>[0-9]+)`,
		`/foo_service/\w+`,
		`/foo/bar`,
		`/foo/\\$bar`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator,
		``,
		`(foo`,
		`/path with space`,
		`/foo;bar`,
		`/foo{2}`,
		`/foo$bar`,
		`/foo(?=bar)`,
		`/foo(?!bar)`,
		`/foo(?<=bar)`,
		`/foo(?<!bar)`,
		`(\w+)\1$`,
		`(\w+)\2$`,
		`/foo/(?P<bad-name>[0-9]+)`,
		`/foo/(?P<bad name>[0-9]+)`,
	)
}
