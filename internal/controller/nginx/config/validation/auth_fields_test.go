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

func TestValidateOIDCRedirectURI(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCRedirectURI,
		`/callback`,
		`/auth/callback`,
		`/redirect/path`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCRedirectURI,
		`callback`,
		`https://example.com/callback`,
		`/path with spaces`,
	)
}
