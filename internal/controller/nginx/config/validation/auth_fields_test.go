package validation

import "testing"

func TestValidateOIDCIssuer(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCIssuer,
		`https://accounts.example.com`,
		`https://auth.example.com:8080/oidc`,
		`https://my-idp.example.com/realms/master`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCIssuer,
		`http://example.com`,
		`example.com`,
		`https://UPPERCASE.com`,
	)
}

func TestValidateOIDCURI(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	validValues := []string{
		`/callback`,
		`/auth/callback`,
		`/logout`,
		`https://example.com/callback`,
		`https://example.com/logout`,
		`http://auth.example.com:8080/logout`,
	}

	invalidValues := []string{
		`callback`,
		`ftp://example.com/logout`,
		`https://UPPERCASE.com`,
		`/path with spaces`,
	}

	for _, fn := range []func(string) error{
		validator.ValidateOIDCRedirectURI,
		validator.ValidateOIDCLogoutURI,
	} {
		testValidValuesForSimpleValidator(t, fn, validValues...)
		testInvalidValuesForSimpleValidator(t, fn, invalidValues...)
	}
}
