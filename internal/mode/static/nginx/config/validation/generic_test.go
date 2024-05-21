package validation

import "testing"

func TestGenericValidator_ValidateEscapedStringNoVarExpansion(t *testing.T) {
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateEscapedStringNoVarExpansion,
		`test`,
		`test test`,
		`\"`,
		`\\`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateEscapedStringNoVarExpansion,
		`\`,
		`test"test`,
		`$test`,
	)
}

func TestValidateServiceName(t *testing.T) {
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateServiceName,
		`test`,
		`Test-test`,
		`test_Test`,
		`test123`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateServiceName,
		`test#$%`,
		`test test`,
		`test.test`,
	)
}

func TestValidateNginxDuration(t *testing.T) {
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateNginxDuration,
		`5ms`,
		`10s`,
		`123ms`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateNginxDuration,
		`test`,
		`12345`,
		`5m`,
	)
}

func TestValidateNginxSize(t *testing.T) {
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateNginxSize,
		`1024`,
		`10k`,
		`123m`,
		`4096g`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateNginxSize,
		`test`,
		`12345`,
		`5b`,
	)
}

func TestValidateEndpoint(t *testing.T) {
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateEndpoint,
		`http://my-endpoint:5678`,
		`my.endpoint`,
		`myendpoint:123`,
		`my-endpoint123:456`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateEndpoint,
		`https://my-endpoint`,
		`my_endpoint`,
		`my$endpoint`,
	)
}
