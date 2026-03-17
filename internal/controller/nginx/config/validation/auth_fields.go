package validation

import (
	"errors"
	"regexp"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// AuthFieldValidator validates fields related to authentication.
type AuthFieldValidator struct{}

var (
	oidcHTTPSURLRegexp           = regexp.MustCompile(oidcHTTPSURLFmt)
	oidcRedirectURIRegexp        = regexp.MustCompile(oidcRedirectURIFmt)
	oidcPostLogoutURIRegexp      = regexp.MustCompile(oidcPostLogoutURIFmt)
	oidcPathURIRegexp            = regexp.MustCompile(oidcPathURIFmt)
	oidcPathWithQueryParamRegexp = regexp.MustCompile(oidcPathWithQueryParamFmt)
)

//nolint:lll
const (
	// oidcHTTPSURLFmt validates HTTPS-only URLs. It is used for the issuer and configURL.
	// Semicolons and dollar signs are disallowed in the path.
	oidcHTTPSURLFmt    = `^https:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?$`
	oidcHTTPSURLErrMsg = "must be a valid HTTPS URL"
)

func validateHTTPSURL(url string) error {
	if !oidcHTTPSURLRegexp.MatchString(url) {
		examples := []string{
			"https://accounts.example.com",
			"https://auth.example.com:8080/oidc",
		}
		return errors.New(k8svalidation.RegexError(oidcHTTPSURLErrMsg, oidcHTTPSURLFmt, examples...))
	}
	return nil
}

const (
	// oidcPathURIFmt validates path-only URIs. It is used by the logoutURI and frontChannelLogoutURI fields.
	// Semicolons and dollar signs are disallowed in the path.
	oidcPathURIFmt    = `^/[A-Za-z0-9._~!&'()*+,=@/-]*$`
	oidcPathURIErrMsg = "must be a path-only URI starting with /"
)

func validatePathURI(uri string) error {
	if !oidcPathURIRegexp.MatchString(uri) {
		return errors.New(k8svalidation.RegexError(oidcPathURIErrMsg, oidcPathURIFmt, "/callback"))
	}
	return nil
}

// ValidateOIDCIssuer validates an OIDC issuer URL. Only HTTPS URLs are accepted.
func (AuthFieldValidator) ValidateOIDCIssuer(issuer string) error {
	return validateHTTPSURL(issuer)
}

// ValidateOIDCConfigURL validates an OIDC configuration URL. Only HTTPS URLs are accepted.
func (AuthFieldValidator) ValidateOIDCConfigURL(url string) error {
	return validateHTTPSURL(url)
}

const (
	// oidcPathWithQueryParamFmt matches path-only URIs (starting with /) that contain query parameters.
	oidcPathWithQueryParamFmt    = `^\/[^?]*\?`
	oidcPathWithQueryParamErrMsg = "query parameters are not allowed in path-only URIs"
)

//nolint:lll,gosec
const (
	// oidcRedirectURIFmt validates redirect URIs. It accepts HTTPS full URIs or path-only URIs starting with /.
	oidcRedirectURIFmt    = `^(https:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?|\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)$`
	oidcRedirectURIErrMsg = "must be a valid HTTPS URL or a path starting with /"
)

// ValidateOIDCRedirectURI validates an OIDC redirect URI.
// Only HTTPS full URIs or path-only URIs starting with / are accepted.
// Query parameters are not allowed in path-only URIs.
func (AuthFieldValidator) ValidateOIDCRedirectURI(uri string) error {
	if !oidcRedirectURIRegexp.MatchString(uri) {
		return errors.New(k8svalidation.RegexError(
			oidcRedirectURIErrMsg, oidcRedirectURIFmt, "/callback", "https://example.com/callback",
		))
	}
	if oidcPathWithQueryParamRegexp.MatchString(uri) {
		return errors.New(oidcPathWithQueryParamErrMsg)
	}
	return nil
}

//nolint:lll
const (
	// oidcPostLogoutURIFmt validates post-logout URIs. It accepts HTTP or HTTPS full URIs or path-only URIs starting with /.
	oidcPostLogoutURIFmt    = `^(https?:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?|\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)$`
	oidcPostLogoutURIErrMsg = "must be a valid HTTP or HTTPS URL or a path starting with /"
)

// ValidateOIDCPostLogoutURI validates an OIDC post-logout URI.
// HTTP and HTTPS full URIs and path-only URIs starting with / are accepted.
// Query parameters are not allowed in path-only URIs.
func (AuthFieldValidator) ValidateOIDCPostLogoutURI(uri string) error {
	if !oidcPostLogoutURIRegexp.MatchString(uri) {
		return errors.New(k8svalidation.RegexError(
			oidcPostLogoutURIErrMsg, oidcPostLogoutURIFmt, "/logged_out", "https://example.com/logged_out",
		))
	}
	if oidcPathWithQueryParamRegexp.MatchString(uri) {
		return errors.New(oidcPathWithQueryParamErrMsg)
	}
	return nil
}

// ValidateOIDCLogoutURI validates an OIDC logout URI. Only path-only URIs starting with / are accepted.
func (AuthFieldValidator) ValidateOIDCLogoutURI(uri string) error {
	return validatePathURI(uri)
}

// ValidateOIDCFrontChannelLogoutURI validates an OIDC front-channel logout URI.
// Only path-only URIs starting with / are accepted.
func (AuthFieldValidator) ValidateOIDCFrontChannelLogoutURI(uri string) error {
	return validatePathURI(uri)
}
