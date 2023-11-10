package validation

import (
	"testing"
)

func TestValidatePathInMatch(t *testing.T) {
	validator := HTTPNJSMatchValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidatePathInMatch,
		"/",
		"/path",
		"/path/subpath-123",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidatePathInMatch,
		"/ ",
		"/path{",
		"/path}",
		"/path;",
		"path",
		"",
		"/path$",
	)
}

func TestValidateHeaderNameInMatch(t *testing.T) {
	validator := HTTPNJSMatchValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateHeaderNameInMatch,
		"header",
		"version",
		"version-2",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateHeaderNameInMatch,
		":",
		"",
		"version%!",
		"version_2",
		"hello$world",
		"   ",
	)
}

func TestValidateHeaderValueInMatch(t *testing.T) {
	validator := HTTPNJSMatchValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateHeaderValueInMatch,
		"value",
		"version%!",
		"version-2",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateHeaderValueInMatch,
		":",
		"",
		"hello$world",
		"   ",
	)
}

func TestValidateQueryParamNameInMatch(t *testing.T) {
	validator := HTTPNJSMatchValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateQueryParamNameInMatch,
		"param",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateQueryParamNameInMatch,
		"",
	)
}

func TestValidateQueryParamValueInMatch(t *testing.T) {
	validator := HTTPNJSMatchValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateQueryParamValueInMatch,
		"value",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateQueryParamValueInMatch,
		"",
	)
}

func TestValidateMethodInMatch(t *testing.T) {
	validator := HTTPNJSMatchValidator{}

	testValidValuesForSupportedValuesValidator(
		t,
		validator.ValidateMethodInMatch,
		"GET",
		"HEAD",
		"POST",
		"PUT",
		"DELETE",
		"OPTIONS",
		"PATCH",
	)
	testInvalidValuesForSupportedValuesValidator(
		t,
		validator.ValidateMethodInMatch,
		supportedMethods,
		"GOT",
		"TRACE",
	)
}

func TestValidateCommonMatchPart(t *testing.T) {
	testValidValuesForSimpleValidator(
		t,
		validateCommonNJSMatchPart,
		"test",
	)
	testInvalidValuesForSimpleValidator(
		t,
		validateCommonNJSMatchPart,
		"",
		" ",
		"$",
	)
}
