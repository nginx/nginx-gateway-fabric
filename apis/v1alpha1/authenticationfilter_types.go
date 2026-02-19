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
// +kubebuilder:validation:XValidation:message="for type=Basic, spec.basic must be set",rule="!(!has(self.basic) && self.type == 'Basic')"
// +kubebuilder:validation:XValidation:message="for type=OIDC, spec.oidc must be set",rule="!(!has(self.oidc) && self.type == 'OIDC')"
// +kubebuilder:validation:XValidation:message="type Basic must not be set when spec.oidc is set", rule="self.type != 'Basic' || !has(self.oidc)"
// +kubebuilder:validation:XValidation:message="type OIDC must not be set when spec.basic is set", rule="self.type != 'OIDC' || !has(self.basic)"
//
//nolint:lll
type AuthenticationFilterSpec struct {
	// Basic configures HTTP Basic Authentication.
	//
	// +optional
	Basic *BasicAuth `json:"basic,omitempty"`

	// OIDC configures OpenID Connect Authentication.
	//
	// +optional
	OIDC *OIDCAuth `json:"oidc,omitempty"`

	// Type selects the authentication mechanism.
	Type AuthType `json:"type"`
}

// AuthType defines the authentication mechanism.
//
// +kubebuilder:validation:Enum=Basic;OIDC
type AuthType string

const (
	// AuthTypeBasic is the HTTP Basic Authentication mechanism.
	AuthTypeBasic AuthType = "Basic"
	// AuthTypeOIDC is the OpenID Connect Authentication mechanism.
	AuthTypeOIDC AuthType = "OIDC"
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
	// revocation list in PEM format. This Secret must be of type `nginx.org/oidc`
	// with the value stored under the key "ca.crl". This is used to verify that
	// certificates presented by the OpenID Provider endpoints have not been revoked.
	//
	// +optional
	CRLSecretRef *LocalObjectReference `json:"crlSecretRef,omitempty"`

	// ConfigURL sets a custom URL to retrieve the OpenID Provider metadata.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#config_url
	// NGINX Default: <issuer>/.well-known/openid-configuration
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
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

	// Issuer is the URL of the OpenID Provider.
	// Must exactly match the "issuer" value from the provider's
	// .well-known/openid-configuration endpoint.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#issuer
	// Examples:
	//   - Keycloak: "https://keycloak.example.com/realms/my-realm"
	//   - Okta: "https://dev-123456.okta.com/oauth2/default"
	//   - Auth0: "https://my-tenant.auth0.com/"
	//
	// +kubebuilder:validation:Pattern=`^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
	Issuer string `json:"issuer"`

	// ClientID is the client identifier registered with the OpenID Provider.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#client_id
	//
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientID"`

	// ClientSecretRef references a Kubernetes secret which contains the OIDC client secret to be used in the
	// [Authentication Request](https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest).
	// This Secret must be of type `nginx.org/oidc` with the value stored under the key "client-secret".
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#client_secret
	ClientSecretRef LocalObjectReference `json:"clientSecretRef"`

	// CACertificateRefs references a list of secrets containing trusted CA certificates
	// in PEM format used to verify the certificates of the OpenID Provider endpoints.
	// The Secrets must be of type `nginx.org/oidc` and must be stored in a key named `ca.crt`.
	// If not specified, the system CA bundle is used.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#ssl_trusted_certificate
	// NGINX Default: system CA bundle
	//
	// +optional
	// +kubebuilder:validation:MaxItems=8
	CACertificateRefs []LocalObjectReference `json:"caCertificateRefs,omitempty"`

	// The OIDC scopes to be used in the Authentication Request.
	// By default, the "openid" scope is always added to the list of scopes
	// if not already specified.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#scope
	// NGINX Default: openid
	//
	// +optional
	Scopes []string `json:"scopes,omitempty"`
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
	// URI defines the path for initiating session logout.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#logout_uri
	// Example: /logout
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(https?://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?)?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
	URI *string `json:"uri,omitempty"`

	// PostLogoutURI defines the path to redirect to after logout.
	// Must match the configuration on the provider's side.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#post_logout_uri
	// Example: /logged_out.html
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(https?://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?)?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
	PostLogoutURI *string `json:"postLogoutURI,omitempty"`

	// FrontChannelLogoutURI defines the path for front-channel logout.
	// The OpenID Provider should be configured to set "iss" and "sid" arguments.
	// Directive: https://nginx.org/en/docs/http/ngx_http_oidc_module.html#frontchannel_logout_uri
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(https?://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?)?(/[a-zA-Z0-9._~:/?@!$&'()*+,;=-]*)?$`
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
