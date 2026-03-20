package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=authfilter;authenticationfilter
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AuthenticationFilter configures request authentication and is
// referenced by HTTPRoute and GRPCRoute filters using ExtensionRef.
type AuthenticationFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired state of the AuthenticationFilter.
	Spec AuthenticationFilterSpec `json:"spec"`

	// Status defines the state of the AuthenticationFilter.
	Status AuthenticationFilterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
//
// AuthenticationFilterList contains a list of AuthenticationFilter resources.
type AuthenticationFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AuthenticationFilter `json:"items"`
}

// AuthenticationFilterSpec defines the desired configuration.
//
// +kubebuilder:validation:XValidation:message="type Basic requires spec.basic to be set",rule="self.type != 'Basic' || has(self.basic)"
// +kubebuilder:validation:XValidation:message="type Basic must not set spec.jwt", rule="self.type != 'Basic' || !has(self.jwt)"
// +kubebuilder:validation:XValidation:message="type Basic must not set spec.oidc", rule="self.type != 'Basic' || !has(self.oidc)"
// +kubebuilder:validation:XValidation:message="type OIDC requires spec.oidc to be set",rule="self.type != 'OIDC' || has(self.oidc)"
// +kubebuilder:validation:XValidation:message="type OIDC must not set spec.basic", rule="self.type != 'OIDC' || !has(self.basic)"
// +kubebuilder:validation:XValidation:message="type OIDC must not set spec.jwt", rule="self.type != 'OIDC' || !has(self.jwt)"
// +kubebuilder:validation:XValidation:message="type JWT requires spec.jwt to be set",rule="self.type != 'JWT' || has(self.jwt)"
// +kubebuilder:validation:XValidation:message="type JWT must not set spec.basic", rule="self.type != 'JWT' || !has(self.basic)"
// +kubebuilder:validation:XValidation:message="type JWT must not set spec.oidc", rule="self.type != 'JWT' || !has(self.oidc)"
//
//nolint:lll
type AuthenticationFilterSpec struct {
	// Basic configures HTTP Basic Authentication.
	//
	// +optional
	Basic *BasicAuth `json:"basic,omitempty"`

	// OIDC configures OpenID Connect Authentication (NGINX Plus).
	//
	// +optional
	OIDC *OIDCAuth `json:"oidc,omitempty"`

	// JWT configures JSON Web Token authentication (NGINX Plus).
	//
	// +optional
	JWT *JWTAuth `json:"jwt,omitempty"`

	// Type selects the authentication mechanism.
	Type AuthType `json:"type"`
}

// AuthType defines the authentication mechanism.
//
// +kubebuilder:validation:Enum=Basic;OIDC;JWT
type AuthType string

const (
	// AuthTypeBasic is the HTTP Basic Authentication mechanism.
	AuthTypeBasic AuthType = "Basic"
	// AuthTypeOIDC is the OpenID Connect Authentication mechanism.
	AuthTypeOIDC AuthType = "OIDC"
	// AuthTypeJWT is the JWT Authentication mechanism.
	AuthTypeJWT AuthType = "JWT"
)

// BasicAuth configures HTTP Basic Authentication.
type BasicAuth struct {
	// SecretRef references a Secret containing credentials in the same namespace.
	SecretRef LocalObjectReference `json:"secretRef"`

	// Realm used by NGINX `auth_basic` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html#auth_basic
	// Also configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	Realm string `json:"realm"`
}

// OIDCAuth configures OpenID Connect Authentication.
// Only available for NGINX Plus users.
//
//nolint:lll
type OIDCAuth struct {
	// CRLSecretRef references a Secret containing a certificate
	// revocation list in PEM format. The referenced Secret must contain an entry with the key "ca.crl".
	// This is used to verify that certificates presented by the OpenID Provider endpoints have not been revoked.
	//
	// +optional
	CRLSecretRef *LocalObjectReference `json:"crlSecretRef,omitempty"`

	// ConfigURL sets a custom URL to retrieve the OpenID Provider metadata.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#config_url
	// NGINX Default: <issuer>/.well-known/openid-configuration
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^https:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?$`
	ConfigURL *string `json:"configURL,omitempty"`

	// PKCE enables Proof Key for Code Exchange (PKCE) for the authentication flow.
	// If nil, NGINX automatically enables PKCE when the OpenID Provider requires it.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#pkce
	//
	// +optional
	PKCE *bool `json:"pkce,omitempty"`

	// ExtraAuthArgs sets additional query arguments for the authentication request URL.
	// Arguments are appended with "&". For example: "prompt=consent&audience=api".
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#extra_auth_args
	//
	// +optional
	ExtraAuthArgs map[string]string `json:"extraAuthArgs,omitempty"`

	// Session configures session management for OIDC authentication.
	//
	// +optional
	Session *OIDCSessionConfig `json:"session,omitempty"`

	// Logout defines the logout behavior for OIDC authentication.
	//
	// +optional
	Logout *OIDCLogoutConfig `json:"logout,omitempty"`

	// RedirectURI sets a custom redirect URI for the OIDC callback.
	// If a path-only URI is specified, a callback location block is created to handle the redirect from the OIDC provider.
	// If a full URI is specified, it points to an external callback handler; no location block is created.
	// If not specified, defaults to /oidc_callback_<filternamespace>_<filtername>.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#redirect_uri
	// NGINX Default: /oidc_callback
	// Example: /oidc_callback, https://cafe.example.com:8442/oidc_callback
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(https:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?|\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)$`
	RedirectURI *string `json:"redirectURI,omitempty"`

	// Issuer is the URL of the OpenID Provider.
	// Must exactly match the "issuer" value from the provider's
	// .well-known/openid-configuration endpoint.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#issuer
	// Examples:
	//   - Keycloak: "https://keycloak.example.com/realms/my-realm"
	//   - Okta: "https://dev-123456.okta.com/oauth2/default"
	//   - Auth0: "https://my-tenant.auth0.com/"
	//
	// +kubebuilder:validation:Pattern=`^https:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?$`
	Issuer string `json:"issuer"`

	// ClientID is the client identifier registered with the OpenID Provider.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#client_id
	//
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientID"`

	// ClientSecretRef references a Kubernetes secret which contains the OIDC client secret to be used in the
	// Authentication Request: https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest.
	// The referenced Secret must contain an entry with the key "client-secret".
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#client_secret
	ClientSecretRef LocalObjectReference `json:"clientSecretRef"`

	// CACertificateRefs references a list of secrets containing trusted CA certificates
	// in PEM format used to verify the certificates of the OpenID Provider endpoints.
	// The referenced secrets must contain an entry with the key "ca.crt".
	// Only one secret can be referenced currently.
	// If not specified, the system CA bundle is used.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#ssl_trusted_certificate
	// NGINX Default: system CA bundle
	//
	// +optional
	// +kubebuilder:validation:MaxItems=1
	CACertificateRefs []LocalObjectReference `json:"caCertificateRefs,omitempty"`
}

// OIDCSessionConfig configures session management for OIDC authentication.
type OIDCSessionConfig struct {
	// CookieName sets the name of the session cookie.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#cookie_name
	// NGINX Default: NGX_OIDC_SESSION
	//
	// +optional
	CookieName *string `json:"cookieName,omitempty"`

	// Timeout sets the session timeout duration.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#session_timeout
	// NGINX Default: 8h
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`
}

// OIDCLogoutConfig defines the logout behavior for OIDC authentication.
//
//nolint:lll
type OIDCLogoutConfig struct {
	// URI defines the path for initiating session logout. This path
	// should not conflict with the logout path of other AuthenticationFilters used within
	// the same hostname. If there is overlap, visiting the logout URI will log out the client
	// defined in the first AuthenticationFilter.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#logout_uri
	// Example: /logout
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9._~!&'()*+,=@/-]*$`
	URI *string `json:"uri,omitempty"`

	// PostLogoutURI defines the URI to redirect to after logout.
	// Must match the configuration on the provider's side.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#post_logout_uri
	// Example: /after_logout, https://example.com/after_logout
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(https?:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?|\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)$`
	PostLogoutURI *string `json:"postLogoutURI,omitempty"`

	// FrontChannelLogoutURI defines the path for front-channel logout.
	// The OpenID Provider should be configured to set "iss" and "sid" arguments. This path
	// should not conflict with the frontchannel logout path of other AuthenticationFilters used within
	// the same hostname. If there is overlap, visiting the frontchannel logout URI will log out the client
	// defined in the first AuthenticationFilter.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#frontchannel_logout_uri
	// Example: /frontchannel_logout
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9._~!&'()*+,=@/-]*$`
	FrontChannelLogoutURI *string `json:"frontChannelLogoutURI,omitempty"`

	// TokenHint adds the id_token_hint argument to the provider's Logout Endpoint.
	// Some OpenID Providers require this.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#logout_token_hint
	// NGINX Default: false
	//
	// +optional
	TokenHint *bool `json:"tokenHint,omitempty"`
}

// LocalObjectReference specifies a local Kubernetes object.
type LocalObjectReference struct {
	// Name is the name of the referenced object.
	Name string `json:"name"`
}

// JWTKeySource specifies the source of the keys used to verify JWT signatures.
// +kubebuilder:validation:Enum=File;Remote
type JWTKeySource string

const (
	// JWTKeySourceFile configures JWT to fetch JWKS from a local secret.
	JWTKeySourceFile JWTKeySource = "File"
	// JWTKeySourceRemote configures JWT to fetch JWKS from a remote source.
	JWTKeySourceRemote JWTKeySource = "Remote"
)

// JWTAuth configures JWT-based authentication (NGINX Plus).
// +kubebuilder:validation:XValidation:message="source File requires spec.file to be set.",rule="self.source != 'File' || has(self.file)"
// +kubebuilder:validation:XValidation:message="source File must not set spec.remote.", rule="self.source != 'File' || !has(self.remote)"
// +kubebuilder:validation:XValidation:message="source Remote requires spec.remote to be set.",rule="self.source != 'Remote' || has(self.remote)"
// +kubebuilder:validation:XValidation:message="source Remote must not set spec.file.", rule="self.source != 'Remote' || !has(self.file)"
//
//nolint:lll
type JWTAuth struct {
	// File specifies local JWKS configuration.
	// Required when Source == File.
	//
	// +optional
	File *JWTFileKeySource `json:"file,omitempty"`

	// KeyCache is the cache duration for keys.
	// Configures `auth_jwt_key_cache` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_cache
	// Example: "auth_jwt_key_cache 10m;".
	//
	// +optional
	KeyCache *Duration `json:"keyCache,omitempty"`

	// Remote specifies remote JWKS configuration.
	// Required when Source == Remote.
	//
	// +optional
	Remote *JWTRemoteKeySource `json:"remote,omitempty"`

	// Realm used by NGINX `auth_jwt` directive
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt
	// Configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	Realm string `json:"realm"`

	// Source selects how JWT keys are provided: local file or remote JWKS.
	Source JWTKeySource `json:"source"`
}

// JWTFileKeySource specifies local JWKS key configuration.
type JWTFileKeySource struct {
	// SecretRef references a Secret containing the JWKS.
	SecretRef LocalObjectReference `json:"secretRef"`
}

// JWTRemoteKeySource specifies remote JWKS configuration.
type JWTRemoteKeySource struct {
	URI               string                 `json:"uri"`
	CACertificateRefs []LocalObjectReference `json:"caCertificateRefs,omitempty"`
}

// AuthenticationFilterStatus defines the state of AuthenticationFilter.
type AuthenticationFilterStatus struct {
	// Controllers is a list of Gateway API controllers that processed the AuthenticationFilter
	// and the status of the AuthenticationFilter with respect to each controller.
	//
	// +kubebuilder:validation:MaxItems=16
	Controllers []ControllerStatus `json:"controllers,omitempty"`
}

// AuthenticationFilterConditionType is a type of condition associated with AuthenticationFilter.
type AuthenticationFilterConditionType string

// AuthenticationFilterConditionReason is a reason for an AuthenticationFilter condition type.
type AuthenticationFilterConditionReason string

const (
	// AuthenticationFilterConditionTypeAccepted indicates that the AuthenticationFilter is accepted.
	//
	// Possible reasons for this condition to be True:
	// * Accepted
	//
	// Possible reasons for this condition to be False:
	// * Invalid.
	AuthenticationFilterConditionTypeAccepted AuthenticationFilterConditionType = "Accepted"

	// AuthenticationFilterConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	AuthenticationFilterConditionReasonAccepted AuthenticationFilterConditionReason = "Accepted"

	// AuthenticationFilterConditionReasonInvalid is used with the Accepted condition type when
	// the filter is invalid.
	AuthenticationFilterConditionReasonInvalid AuthenticationFilterConditionReason = "Invalid"
)
