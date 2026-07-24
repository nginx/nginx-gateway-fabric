package validation

import (
	"strings"
	"testing"
)

// TestValidateGenericErrorMessageOrder ensures that validation errors lead with the
// human-readable message and reference the raw regex only at the end, i.e. that the
// msg and fmt arguments passed to k8svalidation.RegexError are not reversed.
func TestValidateGenericErrorMessageOrder(t *testing.T) {
	t.Parallel()
	validator := GenericValidator{}

	tests := []struct {
		validate   func(string) error
		name       string
		value      string
		wantMsg    string
		wantRegexp string
	}{
		{
			name:       "duration",
			validate:   validator.ValidateNginxDuration,
			value:      "invalid",
			wantMsg:    durationStringErrMsg,
			wantRegexp: durationStringFmt,
		},
		{
			name:       "size",
			validate:   validator.ValidateNginxSize,
			value:      "invalid",
			wantMsg:    sizeStringErrMsg,
			wantRegexp: sizeStringFmt,
		},
		{
			name:       "endpoint",
			validate:   validator.ValidateEndpoint,
			value:      "my$endpoint",
			wantMsg:    endpointStringErrMsg,
			wantRegexp: endpointStringFmt,
		},
		{
			name:       "variable name",
			validate:   validator.ValidateNginxVariableName,
			value:      "var-name",
			wantMsg:    variableNameErrMsg,
			wantRegexp: variableNameFmt,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.validate(test.value)
			if err == nil {
				t.Fatalf("%s: expected an error for value %q, got nil", test.name, test.value)
			}

			msg := err.Error()
			if !strings.HasPrefix(msg, test.wantMsg) {
				t.Errorf("%s: expected error to start with human-readable message %q, got %q",
					test.name, test.wantMsg, msg)
			}

			wantRegexpFragment := "regex used for validation is '" + test.wantRegexp + "'"
			if !strings.Contains(msg, wantRegexpFragment) {
				t.Errorf("%s: expected error to reference regex %q at the end, got %q",
					test.name, wantRegexpFragment, msg)
			}
		})
	}
}

func TestGenericValidator_ValidateEscapedStringNoVarExpansion(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateNginxDuration,
		`5ms`,
		`10s`,
		`123ms`,
		`5m`,
		`2h`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateNginxDuration,
		`test`,
		`12345`,
		`5k`,
	)
}

func TestValidateNginxSize(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestValidateNginxVariableName(t *testing.T) {
	t.Parallel()
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateNginxVariableName,
		`$upstream_bytes_sent`,
		`$upstream_last_server_name`,
		`$remote_addr`,
		`$http2`,
		`${remote_addr}`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateNginxVariableName,
		`1varname`,
		`var-name`,
		`var name`,
		`var$name`,
		`$Uppercase`,
		`${}`,
		`$`,
		`$_internal`,
		`${_internal_var}`,
		`$request_id;`,
	)
}

func TestValidateServerTokensValue(t *testing.T) {
	t.Parallel()
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateServerTokensValue,
		`my-server`,
		`nginx`,
		`custom server token`,
		``,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateServerTokensValue,
		`bad"value`,
		`value\`,
		"bad\nvalue",
	)
}

func TestValidateAccessLogFormatString(t *testing.T) {
	t.Parallel()
	validator := GenericValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateAccessLogFormatString,
		`$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent`,
		`{"remote_addr": "$remote_addr", "status": "$status"}`,
		`$request\t$status`,
		``,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateAccessLogFormatString,
		`format with 'quotes'`,
		`'; bad stuff; #`,
		`$remote_addr'`,
		"$remote_addr\n$status",
	)
}
