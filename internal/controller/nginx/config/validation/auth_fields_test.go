package validation

import "testing"

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
