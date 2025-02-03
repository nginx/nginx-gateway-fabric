package validation

import (
	"math"
	"testing"
)

func TestValidateRedirectScheme(t *testing.T) {
	t.Parallel()
	validator := HTTPRedirectValidator{}

	testValidValuesForSupportedValuesValidator(
		t,
		validator.ValidateRedirectScheme,
		"http",
		"https",
	)

	testInvalidValuesForSupportedValuesValidator(
		t,
		validator.ValidateRedirectScheme,
		supportedRedirectSchemes,
		"test",
	)
}

func TestValidateRedirectPort(t *testing.T) {
	t.Parallel()
	validator := HTTPRedirectValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateRedirectPort,
		math.MinInt32,
		math.MaxInt32,
	)
}

func TestValidateRedirectStatusCode(t *testing.T) {
	t.Parallel()
	validator := HTTPRedirectValidator{}

	testValidValuesForSupportedValuesValidator(
		t,
		validator.ValidateRedirectStatusCode,
		301,
		302)

	testInvalidValuesForSupportedValuesValidator(
		t,
		validator.ValidateRedirectStatusCode,
		supportedRedirectStatusCodes,
		404,
	)
}

func TestValidateHostname(t *testing.T) {
	t.Parallel()
	validator := HTTPRedirectValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateHostname,
		"example.com",
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateHostname,
		"example.com$",
	)
}

func TestValidatePath(t *testing.T) {
	t.Parallel()
	validator := HTTPPathValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidatePath,
		"",
		"/path",
		"/longer/path",
		"/trailing/",
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidatePath,
		"path",
		"$path",
		"/path$",
	)
}

func TestValidateFilterHeaderName(t *testing.T) {
	t.Parallel()
	validator := HTTPHeaderValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateFilterHeaderName,
		"Content-Encoding",
		"MyBespokeHeader",
	)

	testInvalidValuesForSimpleValidator(t, validator.ValidateFilterHeaderName, "$Content-Encoding")
}

func TestValidateFilterHeaderValue(t *testing.T) {
	t.Parallel()
	validator := HTTPHeaderValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateFilterHeaderValue,
		"my-cookie-name",
		"ssl_(server_name}",
		"example/1234==",
		"1234:3456",
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateFilterHeaderValue,
		"$Content-Encoding",
		`"example"`,
	)
}
