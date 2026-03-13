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

//nolint:lll
const (
	oidcURIFmt    = `^(https?://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?)?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
	oidcURIErrMsg = "must be a valid full URI (e.g. https://example.com/path) or a path-only URI (e.g. /path)"
)

var oidcURIRegexp = regexp.MustCompile(oidcURIFmt)

func validateOIDCURI(uri string) error {
	if !oidcURIRegexp.MatchString(uri) {
		examples := []string{
			"https://example.com/path",
			"/path",
		}
		return errors.New(k8svalidation.RegexError(oidcURIErrMsg, oidcURIFmt, examples...))
	}
	return nil
}

// ValidateOIDCRedirectURI validates an OIDC redirect URI (full URI or path-only).
func (AuthFieldValidator) ValidateOIDCRedirectURI(uri string) error {
	return validateOIDCURI(uri)
}

// ValidateOIDCLogoutURI validates an OIDC logout URI (full URI or path-only).
func (AuthFieldValidator) ValidateOIDCLogoutURI(uri string) error {
	return validateOIDCURI(uri)
}
