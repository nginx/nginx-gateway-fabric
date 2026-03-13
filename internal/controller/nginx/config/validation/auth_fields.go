package validation

import (
	"errors"
	"regexp"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// AuthFieldValidator validates fields related to authentication.
type AuthFieldValidator struct{}

const (
	//nolint:lll
	oidcIssuerFmt    = `^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
	oidcIssuerErrMsg = "must be a valid HTTPS URL"
)

var oidcIssuerRegexp = regexp.MustCompile(oidcIssuerFmt)

// ValidateOIDCIssuer validates an OIDC issuer URL.
func (AuthFieldValidator) ValidateOIDCIssuer(issuer string) error {
	if !oidcIssuerRegexp.MatchString(issuer) {
		examples := []string{
			"https://accounts.example.com",
			"https://auth.example.com:8080/oidc",
		}
		return errors.New(k8svalidation.RegexError(oidcIssuerErrMsg, oidcIssuerFmt, examples...))
	}
	return nil
}

//nolint:gosec
const (
	oidcRedirectURIFmt    = `^/[A-Za-z0-9._~!&'()*+,=@/-]*$`
	oidcRedirectURIErrMsg = "must be a path-only URI starting with '/' and must not include scheme, host, or port"
)

var oidcRedirectURIRegexp = regexp.MustCompile(oidcRedirectURIFmt)

// ValidateOIDCRedirectURI validates an OIDC redirect URI path.
func (AuthFieldValidator) ValidateOIDCRedirectURI(uri string) error {
	if !oidcRedirectURIRegexp.MatchString(uri) {
		examples := []string{
			"/callback",
			"/auth/callback",
		}
		return errors.New(k8svalidation.RegexError(oidcRedirectURIErrMsg, oidcRedirectURIFmt, examples...))
	}
	return nil
}
