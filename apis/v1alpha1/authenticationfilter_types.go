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

// AuthenticationFilterList contains a list of AuthenticationFilter resources.
type AuthenticationFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AuthenticationFilter `json:"items"`
}

// AuthenticationFilterSpec defines the desired configuration.
// +kubebuilder:validation:XValidation:message="type Basic requires spec.basic to be set.",rule="self.type != 'Basic' || has(self.basic)"
// +kubebuilder:validation:XValidation:message="type JWT requires spec.jwt to be set.",rule="self.type != 'JWT' || has(self.jwt)"
//
//nolint:lll
type AuthenticationFilterSpec struct {
	// Basic configures HTTP Basic Authentication.
	//
	// +optional
	Basic *BasicAuth `json:"basic,omitempty"`

	// JWT configures JSON Web Token authentication (NGINX Plus).
	//
	// +optional
	JWT *JWTAuth `json:"jwt,omitempty"`

	// Type selects the authentication mechanism.
	Type AuthType `json:"type"`
}

// AuthType defines the authentication mechanism.
//
// +kubebuilder:validation:Enum=Basic;JWT
type AuthType string

const (
	// AuthTypeBasic is the HTTP Basic Authentication mechanism.
	AuthTypeBasic AuthType = "Basic"
	// AuthTypeJWT is the JWT Authentication mechanism.
	AuthTypeJWT AuthType = "JWT"
)

// BasicAuth configures HTTP Basic Authentication.
type BasicAuth struct {
	// SecretRef allows referencing a Secret in the same namespace.
	SecretRef LocalObjectReference `json:"secretRef"`

	// Realm used by NGINX `auth_basic` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html#auth_basic
	// Also configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	Realm string `json:"realm"`
}

// LocalObjectReference specifies a local Kubernetes object.
type LocalObjectReference struct {
	// Name is the referenced object.
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
// +kubebuilder:validation:XValidation:message="source Remote requires spec.remote to be set.",rule="self.source != 'Remote' || has(self.remote)"
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
	// TLS defines HTTPS client parameters for retrieving JWKS.
	//
	// +optional
	TLS *JWTRemoteTLSConfig `json:"tls,omitempty"`

	// URI is the JWKS endpoint.
	//
	//nolint:lll
	// +kubebuilder:validation:Pattern=`^(?:http?:\/\/)?[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(?::\d{1,5})?$`
	URI string `json:"uri"`
}

// JWTRemoteTLSConfig defines TLS settings for remote JWKS retrieval.
type JWTRemoteTLSConfig struct {
	// SecretRef references a Secret containing client TLS cert and key.
	// Expects secret type kubernetes.io/tls.
	//
	// +optional
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`

	// Verify controls server certificate verification.
	//
	// +optional
	// +kubebuilder:default=true
	Verify *bool `json:"verify,omitempty"`

	// SNI controls server name indication.
	// Configures `proxy_ssl_server_name` directive.
	// https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_server_name
	//
	// +optional
	// +kubebuilder:default=true
	SNI *bool `json:"sni,omitempty"`

	// SNIName sets a custom SNI.
	// By default, NGINX uses the host from proxy_pass.
	// Configures `proxy_ssl_name` directive.
	// https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_name
	//
	// +optional
	SNIName *string `json:"sniName,omitempty"`
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
