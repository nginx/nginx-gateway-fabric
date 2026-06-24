package validation

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateOIDCIssuer(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCIssuer,
		`https://accounts.example.com`,
		`https://auth.example.com:8080`,
		`https://auth.example.com:8080/oidc`,
		`https://my-idp.example.com/realms/master`,
		`https://example.com/path/with-dashes_and.dots`,
		`https://example.com/oidc?param=value`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCIssuer,
		`http://example.com`,
		`example.com`,
		`https://UPPERCASE.com`,
		`https://example.com/path;with;semis`,
		`https://example.com/path$with$dollars`,
		``,
		`ftp://example.com`,
		`/just/a/path`,
	)
}

func TestValidateOIDCConfigURL(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCConfigURL,
		`https://accounts.example.com/.well-known/openid-configuration`,
		`https://auth.example.com:8080/oidc/.well-known/openid-configuration`,
		`https://keycloak.example.com/realms/master/.well-known/openid-configuration`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCConfigURL,
		`http://example.com/.well-known/openid-configuration`,
		`example.com/.well-known/openid-configuration`,
		`https://MYIDP.com/.well-known/openid-configuration`,
		`https://example.com/.well-known/openid-configuration;extra`,
		`https://example.com/openid$config`,
		``,
		`/.well-known/openid-configuration`,
	)
}

func TestValidateOIDCRedirectURI(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCRedirectURI,
		`/callback`,
		`/oidc/callback`,
		`/oidc_callback_default_my-filter`,
		`https://example.com/callback`,
		`https://cafe.example.com:8442/oidc_callback`,
		`https://auth.example.com/realms/master/callback`,
		`https://example.com/callback?state=abc`,
		`https://cafe.example.com:8442/oidc_callback?foo=bar&baz=qux`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCRedirectURI,
		`/callback?state=abc`,
		`http://example.com/callback`,
		`ftp://example.com/callback`,
		`callback`,
		`example.com/callback`,
		`https://MYHOST.com/callback`,
		`https://example.com/callback;bad`,
		`/callback;bad`,
		`https://example.com/callback$bad`,
		`/callback$bad`,
		``,
		`/path with spaces`,
		`https://example.com/path with spaces`,
	)
}

func TestValidateOIDCPostLogoutURI(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCPostLogoutURI,
		`/logged_out`,
		`/after_logout/`,
		`/logout/done`,
		`https://example.com/logged_out`,
		`https://example.com:8443/after_logout`,
		`http://example.com/logged_out`,
		`http://auth.example.com:8080/logged_out`,
		`https://example.com/logged_out?hint=token`,
		`http://example.com/logged_out?hint=token`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCPostLogoutURI,
		`/logged_out?hint=token`,
		`ftp://example.com/logged_out`,
		`logged_out`,
		`example.com/logged_out`,
		`https://MYHOST.com/logged_out`,
		`https://example.com/logged_out;bad`,
		`/logged_out;bad`,
		`https://example.com/logged_out$bad`,
		`/logged_out$bad`,
		``,
		`/path with spaces`,
		`https://example.com/path with spaces`,
	)
}

func TestValidateOIDCLogoutURI(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCLogoutURI,
		`/logout`,
		`/logout/path`,
		`/oidc-logout`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCLogoutURI,
		`/logout?session=abc`,
		`logout`,
		`https://example.com/logout`,
		`http://example.com/logout`,
		``,
		`/path with spaces`,
		`/logout;session`,
		`/logout$end`,
	)
}

func TestValidateOIDCFrontChannelLogoutURI(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCFrontChannelLogoutURI,
		`/frontchannel_logout`,
		`/front/channel/logout`,
		`/fc-logout`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateOIDCFrontChannelLogoutURI,
		`/frontchannel_logout?sid=abc`,
		`frontchannel_logout`,
		`https://example.com/frontchannel_logout`,
		`http://example.com/frontchannel_logout`,
		``,
		`/path with spaces`,
		`/frontchannel_logout;bad`,
		`/frontchannel_logout$bad`,
	)
}

func TestValidateAuthZClaimName(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateAuthZClaimName,
		`role`,
		`app-1/role`,
		`app_1-role`,
		`sub`,
		`groups`,
		`my-claim`,
		`my_claim`,
		`claim/nested/path`,
		`ABC123`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateAuthZClaimName,
		``,
		`claim;name`,
		`claim$name`,
		`claim name`,
		`claim#name`,
		`claim{name}`,
		`claim|name`,
		`claim&name`,
		`claim>name`,
		`claim<name`,
		`claim'name`,
		`claim"name`,
	)
}

func TestValidateAuthZClaimValue(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateAuthZClaimValue,
		`admin`,
		`user`,
		`app-1`,
		`role_name`,
		`value/with/slashes`,
		`value-with-dashes`,
		`value_with_underscores`,
		`MixedCase123`,
		`value with spaces`,
		`https://issuer.example.com/`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateAuthZClaimValue,
		``,
		"value\nwith\nnewlines",
		"value\rwith\rcarriage",
		`value;semicolon`,
		`value#hash`,
		`value$dollar`,
		`value{brace}`,
		`value|pipe`,
		`value&ampersand`,
		`value>greater`,
		`value<less`,
		`value'quote`,
		`value"doublequote`,
	)
}

func TestValidateAuthZProxySetHeader(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	testValidValuesForSimpleValidator(
		t,
		validator.ValidateAuthZProxySetHeader,
		`X-User-Role`,
		`X-App-Name`,
		`X-JWT-Claim-Sub`,
		`Authorization`,
		`X-Groups`,
	)

	testInvalidValuesForSimpleValidator(
		t,
		validator.ValidateAuthZProxySetHeader,
		``,
		`Header;Name`,
		`Header$Name`,
		`Header Name`,
		`Header#Name`,
		`Header{Name}`,
		`Header|Name`,
		`Header&Name`,
		`Header>Name`,
		`Header<Name`,
		`Header'Name`,
		`Header"Name`,
		`X-Custom-Header_1`,
		`My-Header/Path`,
	)
}

func TestValidateOIDCExtraAuthArg(t *testing.T) {
	t.Parallel()
	validator := AuthFieldValidator{}

	t.Run("valid key-value pairs", func(t *testing.T) {
		t.Parallel()
		validPairs := []struct {
			key   string
			value string
		}{
			{key: "prompt", value: "consent"},
			{key: "audience", value: "api"},
			{key: "scope", value: "openid profile"},
			{key: "acr_values", value: "urn:mace:incommon:iap:silver"},
			{key: "login_hint", value: "user@example.com"},
			{key: "ui.locales", value: "en"},
			{key: "max-age", value: "3600"},
			{key: "key", value: ""},
			{key: "key", value: "value;with;semicolons"},
			{key: "key", value: "value{with}braces"},
		}

		for _, pair := range validPairs {
			g := NewWithT(t)
			err := validator.ValidateOIDCExtraAuthArg(pair.key, pair.value)
			g.Expect(err).ToNot(HaveOccurred(), "key=%q value=%q", pair.key, pair.value)
		}
	})

	t.Run("invalid keys", func(t *testing.T) {
		t.Parallel()
		invalidKeys := []string{
			"",
			"key with spaces",
			"key;semi",
			"key$dollar",
			`key"quote`,
			"key{brace",
			"key=equals",
			"key&amp",
		}

		for _, key := range invalidKeys {
			g := NewWithT(t)
			err := validator.ValidateOIDCExtraAuthArg(key, "valid-value")
			g.Expect(err).To(HaveOccurred(), "key=%q", key)
		}
	})

	t.Run("invalid values", func(t *testing.T) {
		t.Parallel()
		invalidValues := []string{
			`value"with"quotes`,
			`value$with$dollars`,
			`value\`,
			"value\nwith\nnewlines",
		}

		for _, value := range invalidValues {
			g := NewWithT(t)
			err := validator.ValidateOIDCExtraAuthArg("valid-key", value)
			g.Expect(err).To(HaveOccurred(), "value=%q", value)
		}
	})
}
