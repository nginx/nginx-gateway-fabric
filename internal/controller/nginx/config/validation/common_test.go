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
		`/api/v[0-9]+`,                 // basic char class + quantifier
		`/users/(?P<id>[0-9]+)`,        // re2-style named group
		`/users/(?<id>[0-9]+)`,         // pcre-style named group
		`/foo_service/\w+`,             // \w class
		`/foo/bar`,                     // plain literal path
		`/foo/\\$bar`,                  // escaped backslash + dollar
		`/foo/(\w+)\1$`,                // numeric backreference
		`/foo(?=bar)/baz`,              // lookahead
		`/(service\/(?!private/).*)`,   // negative lookahead
		`/rest/.*/V1/order/get/.*`,     // wildcard match
		`/users/(?P<id>[0-9]+)/\k<id>`, // named backreference
		`/foo(?<=/foo)\w+`,             // fixed-width lookbehind
		`/foo(?<=\w+)bar`,              // variable-width lookbehind
		`/foo(?=bar)`,                  // lookahead
		`/users/(?=admin|staff)\w+`,    // alternation in lookahead
		`/api/v1(?=/)`,                 // lookahead for slash
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator,
		``,                          // empty: must be non-empty
		`(foo`,                      // unbalanced parenthesis
		`/path with space`,          // whitespace forbidden by pathFmt
		`/foo;bar`,                  // ';' forbidden by pathFmt
		`/foo{2}`,                   // '{' '}' forbidden by pathFmt
		`(\w+)\2$`,                  // invalid backref: group 2 doesn't exist
		`/foo/(?P<bad-name>[0-9]+)`, // invalid group name: hyphen not allowed
		`/foo/(?P<bad name>[0-9]+)`, // invalid group name: space not allowed
		`(\w+)\2$`,                  // invalid backref: group 2 doesn't exist
		`/users/\k<nonexistent>`,    // invalid named backreference
		`^(([a-z])+)+$`,             // nested quantifiers not allowed
	)
}
