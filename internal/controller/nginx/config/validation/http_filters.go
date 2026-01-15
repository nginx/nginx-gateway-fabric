package validation

// HTTPRedirectValidator validates values for a redirect, which in NGINX is done with the return directive.
// For example, return 302 "https://example.com:8080";
type HTTPRedirectValidator struct{}

// HTTPURLRewriteValidator validates values for a URL rewrite.
type HTTPURLRewriteValidator struct{}

// HTTPHeaderValidator validates values for request headers,
// which in NGINX is done with the proxy_set_header directive.
type HTTPHeaderValidator struct{}

// HTTPPathValidator validates values for path used in filters.
type HTTPPathValidator struct{}

var supportedRedirectSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
}

// ValidateRedirectScheme validates a scheme to be used in the return directive for a redirect.
// NGINX rules are not restrictive, but it is easier to validate just for two allowed values http and https,
// dictated by the Gateway API spec.
func (HTTPRedirectValidator) ValidateRedirectScheme(scheme string) (valid bool, supportedValues []string) {
	return validateInSupportedValues(scheme, supportedRedirectSchemes)
}

func (HTTPRedirectValidator) ValidateRedirectPort(_ int32) error {
	// any value is allowed
	return nil
}

var hostnameExamples = []string{"host", "example.com"}

func (HTTPRedirectValidator) ValidateHostname(hostname string) error {
	return validateEscapedStringNoVarExpansion(hostname, hostnameExamples)
}

// ValidatePath validates a path used in filters.
func (HTTPPathValidator) ValidatePath(path string) error {
	return validatePath(path)
}

// ValidatePathInMatch a path used in the location directive.
func (HTTPPathValidator) ValidatePathInMatch(path string) error {
	return validatePathInMatch(path)
}

// ValidatePathInRegexMatch a path used in a regex location directive.
func (HTTPPathValidator) ValidatePathInRegexMatch(path string) error {
	return validatePathInRegexMatch(path)
}

func (HTTPHeaderValidator) ValidateFilterHeaderName(name string) error {
	return validateHeaderName(name)
}

var requestHeaderValueExamples = []string{"my-header-value", "example/12345=="}

func (HTTPHeaderValidator) ValidateFilterHeaderValue(value string) error {
	// Variables in header values are supported by NGINX but not required by the Gateway API.
	return validateEscapedStringNoVarExpansion(value, requestHeaderValueExamples)
}
