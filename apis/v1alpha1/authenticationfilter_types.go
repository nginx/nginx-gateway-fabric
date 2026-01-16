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
// +kubebuilder:validation:XValidation:message="Basic requires spec.basic. spec.jwt must be unset; JWT requires spec.jwt. spec.basic must be unset",rule="(self.type == 'Basic' && has(self.basic) && !has(self.jwt)) || (self.type == 'JWT' && has(self.jwt) && !has(self.basic))"
//
//nolint:lll
type AuthenticationFilterSpec struct {
	// Basic configures HTTP Basic Authentication.
	//
	// +optional
	Basic *BasicAuth `json:"basic,omitempty"`

	// JWT configures JSON Web Token authentication (NGINX Plus).
	// Required when Type == JWT.
	//
	// +optional
	JWT *JWTAuth `json:"jwt,omitempty"`

	// Type selects the authentication mechanism.
	Type AuthType `json:"type"`
}

// AuthType defines the authentication mechanism.
//
// +kubebuilder:validation:Enum=Basic;JWT;
type AuthType string

const (
	// AuthTypeBasic is the HTTP Basic Authentication mechanism.
	AuthTypeBasic AuthType = "Basic"
	AuthTypeJWT   AuthType = "JWT"
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

// JWTKeyMode selects where JWT keys come from.
// +kubebuilder:validation:Enum=File;Remote
type JWTKeyMode string

const (
	JWTKeyModeFile   JWTKeyMode = "File"
	JWTKeyModeRemote JWTKeyMode = "Remote"
)

// JWTAuth configures JWT-based authentication (NGINX Plus).
// +kubebuilder:validation:XValidation:message="File requires spec.file. spec.remote must be unset; Remote requires spec.remote. spec.file must be unset",rule="(self.mode == 'File' && has(self.file) && !has(self.remote)) || (self.mode == 'Remote' && has(self.remote) && !has(self.file))"
type JWTAuth struct {
	// Realm used by NGINX `auth_jwt` directive
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt
	// Configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	Realm string `json:"realm"`

	// Mode selects how JWT keys are provided: local file or remote JWKS.
	Mode JWTKeyMode `json:"mode"`

	// File specifies local JWKS configuration.
	// Required when Mode == File.
	//
	// +optional
	File *JWTFileKeySource `json:"file,omitempty"`

	// Remote specifies remote JWKS configuration.
	// Required when Mode == Remote.
	//
	// +optional
	Remote *RemoteKeySource `json:"remote,omitempty"`

	// Leeway is the acceptable clock skew for exp/nbf checks.
	// Configures `auth_jwt_leeway` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_leeway
	// Example: "auth_jwt_leeway 60s".
	//
	// +optional
	Leeway *Duration `json:"leeway,omitempty"`

	// Type sets token type: signed | encrypted | nested.
	// Default: signed.
	// Configures `auth_jwt_type` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_type
	// Example: "auth_jwt_type signed;".
	//
	// +optional
	// +kubebuilder:default=signed
	Type *JWTType `json:"type,omitempty"`

	// KeyCache is the cache duration for keys.
	// Configures auth_jwt_key_cache directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_cache
	// Example: "auth_jwt_key_cache 10m".
	//
	// +optional
	KeyCache *Duration `json:"keyCache,omitempty"`
}

// JWTFileKeySource specifies local JWKS key configuration.
type JWTFileKeySource struct {
	// SecretRef references a Secret containing the JWKS.
	SecretRef LocalObjectReference `json:"secretRef"`

	// KeyCache is the cache duration for keys.
	// Configures `auth_jwt_key_cache` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_cache
	// Example: "auth_jwt_key_cache 10m;".
	//
	// +optional
	KeyCache *Duration `json:"keyCache,omitempty"`
}

// RemoteKeySource specifies remote JWKS configuration.
type RemoteKeySource struct {
	// URL is the JWKS endpoint, e.g. "https://issuer.example.com/.well-known/jwks.json".
	URL string `json:"url"`

	// Cache configures NGINX proxy_cache for JWKS fetches made via auth_jwt_key_request.
	// When set, NGF will render proxy_cache_path in http{} and attach proxy_cache to the internal JWKS location.
	//
	// +optional
	Cache *JWKSCache `json:"cache,omitempty"`
}

// JWKSCache controls NGINX `proxy_cache_path` and `proxy_cache` settings used for JWKS responses.
type JWKSCache struct {
	// Levels specifies the directory hierarchy for cached files.
	// Used in `proxy_cache_path` directive.
	// https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_path
	// Example: "levels=1:2".
	//
	// +optional
	Levels *string `json:"levels,omitempty"`

	// KeysZoneName is the name of the cache keys zone.
	// If omitted, the controller SHOULD derive a unique, stable name per filter instance.
	//
	// +optional
	KeysZoneName *string `json:"keysZoneName,omitempty"`

	// KeysZoneSize is the size of the cache keys zone (e.g. "10m").
	// This is required to avoid unbounded allocations.
	KeysZoneSize string `json:"keysZoneSize"`

	// MaxSize limits the total size of the cache (e.g. "50m").
	//
	// +optional
	MaxSize *string `json:"maxSize,omitempty"`

	// Inactive defines the inactivity timeout before cached items are evicted (e.g. "10m").
	//
	// +optional
	Inactive *string `json:"inactive,omitempty"`

	// UseTempPath controls whether a temporary file is used for cache writes.
	// Maps to use_temp_path=(on|off). Default: false (off).
	//
	// +optional
	UseTempPath *bool `json:"useTempPath,omitempty"`
}

// JWTType represents NGINX auth_jwt_type.
// +kubebuilder:validation:Enum=signed;encrypted;nested
type JWTType string

const (
	JWTTypeSigned    JWTType = "signed"
	JWTTypeEncrypted JWTType = "encrypted"
	JWTTypeNested    JWTType = "nested"
)

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
