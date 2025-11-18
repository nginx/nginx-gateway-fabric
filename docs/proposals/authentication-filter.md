# Enhancement Proposal-4052: Authentiation Filter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Implementable

## Summary

Design and implement a means for users of NGINX Gateway Fabric to enable authentication on requests to their backend applications.
This new filter should eventually expose all forms of authentication available through NGINX, both Open Source and Plus.

## Goals

- Design a means of configuring authentication for NGF
- Design Authentication CRD with Basic Auth and JWT Auth in mind
- Determine initial resource specification
- Evaluate filter early in request processing, occurring before URLRewrite, header modifiers and backend selection
- Authentication failures returns 401 Unauthorized by default
- Ensure response codes are configurable

## Non-Goals

- Design for OIDC Auth
- An Auth filter for TCP and UDP routes
- Design for integration with [ExternalAuth in the Gateway API](https://gateway-api.sigs.k8s.io/geps/gep-1494/)

## Introduction

This document focuses explicitly on Authentication (AuthN) and not Authorization (AuthZ). Authentication (AuthN) defines the verification of identity. It asks the question, "Who are you?". This is different from Authorization (AuthZ), which preceeds Authentication. It asks the question, "What are you allowed to do".

This document also focus on HTTP Basic Authentication and JWT Authentication. Other authentication methods such as OpenID Connect (OIDC) are mentioned, but are not part of the CRD design. These will be covered in future design and implementation tasks.


## Use Cases

- As an Application Developer, I want to secure access to my APIs and Backend Applications.
- As an Application Developer, I want to enforce authentication on specific routes and matches.

### Understanding NGINX authentication methods

| **Authentication Method**     | **OSS**      | **Plus** | **NGINX Module**                | **Details**                                                        |
|-------------------------------|--------------|----------------|----------------------------------|--------------------------------------------------------------------|
| **HTTP Basic Authentication** | ✅           | ✅             | [ngx_http_auth_basic](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html) | Requires a username and password sent in an HTTP header.           |
| **JWT (JSON Web Token)**     | ❌           | ✅             | [ngx_http_auth_jwt_module](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html) | Tokens are used for stateless authentication between client and server. |
| **OpenID Connect**            | ❌           | ✅             | [ngx_http_oidc_module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)| Allows authentication through third-party providers like Google.   |

## API, Customer Driven Interfaces, and User Experience

This portion of the proposal will cover API design and interaction experience for use of Basic Auth and JWT.
This portion also contains:

1. The Golang API
2. Example spec for Basic Auth
    - Example HTTPRoutes and NGINX configuration
3. Example spec for JWT Auth
    - Example HTTPRoutes
    - Examples for Local & Remote JWKS configration
    - Example NGINX configuration for both Local & Remote JWKS
    - Example of additioanl optional fields

### Golang API

Below is the Golang API for the `AuthenticationFilter` API:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=authfilter;authenticationfilter
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AuthenticationFilter configures request authentication (Basic or JWT) and is
// referenced by HTTPRoute filters via ExtensionRef.
type AuthenticationFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the AuthenticationFilter.
	Spec AuthenticationFilterSpec `json:"spec"`

	// Status defines the state of the AuthenticationFilter, following the same
	// pattern as SnippetsFilter: per-controller conditions with an Accepted condition.
	//
	// +optional
	Status AuthenticationFilterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthenticationFilterList contains a list of AuthenticationFilter.
type AuthenticationFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthenticationFilter `json:"items"`
}

// AuthenticationFilterSpec defines the desired configuration.
// Exactly one of Basic or JWT must be set according to Type.
// +kubebuilder:validation:XValidation:message="for type=Basic, spec.basic must be set and spec.jwt must be empty; for type=JWT, spec.jwt must be set and spec.basic must be empty",rule="self.type == 'Basic' ? self.basic != null && self.jwt == null : self.type == 'JWT' ? self.jwt != null && self.basic == null : false"

// +kubebuilder:validation:XValidation:message="type 'Basic' requires spec.basic to be set. All other spec types must be unset",rule="self.type == 'Basic' ? self.type != null && self.jwt == null : true"
// +kubebuilder:validation:XValidation:message="type 'JWT' requires spec.jwt to be set. All other spec types must be unset",rule="self.type == 'JWT' ? self.type != null && self.basic == null : true"
// +kubebuilder:validation:XValidation:message="when spec.basic is set, type must be 'Basic'",rule="self.basic != null ? self.type == 'Basic' : true"
// +kubebuilder:validation:XValidation:message="when spec.jwt is set, type must be 'JWT'",rule="self.jwt != null ? self.type == 'JWT' : true"
type AuthenticationFilterSpec struct {
	// Type selects the authentication mechanism.
	Type AuthType `json:"type"`

	// Basic configures HTTP Basic Authentication.
	//
	// +optional
	Basic *BasicAuth `json:"basic,omitempty"`

	// JWT configures JSON Web Token authentication (NGINX Plus).
	//
	// +optional
	JWT *JWTAuth `json:"jwt,omitempty"`
}

// AuthType defines the authentication mechanism.
// +kubebuilder:validation:Enum=Basic;JWT
type AuthType string

const (
	AuthTypeBasic AuthType = "Basic"
	AuthTypeJWT   AuthType = "JWT"
)

// BasicAuth configures HTTP Basic Authentication.
type BasicAuth struct {
	// SecretRef allows referencing a Secret in the same or different namespace.
	// When namespace is set and differs from the filter's namespace, a ReferenceGrant in the target namespace is required.
	//
	// +optional
	SecretRef *NamespacedSecretKeyReference `json:"secretRef,omitempty"`

	// Realm used by NGINX `auth_basic`.
  // Configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	//
	// +optional
	Realm *string `json:"realm,omitempty"`

	// OnFailure customizes the 401 response for failed authentication.
	//
	// +optional
	OnFailure *AuthFailureResponse `json:"onFailure,omitempty"`
}

// JWTAuth configures JWT-based authentication (NGINX Plus).
// +kubebuilder:validation:XValidation:message="mode 'File' requires file set and remote unset",rule="self.mode == 'File' ? self.file != null && self.remote == null : true"
// +kubebuilder:validation:XValidation:message="mode 'Remote' requires remote set and file unset",rule="self.mode == 'Remote' ? self.remote != null && self.file == null : true"
// +kubebuilder:validation:XValidation:message="when file is set, mode must be 'File'",rule="self.file != null ? self.mode == 'File' : true"
// +kubebuilder:validation:XValidation:message="when remote is set, mode must be 'Remote'",rule="self.remote != null ? self.mode == 'Remote' : true"
type JWTAuth struct {
	// Realm used by NGINX `auth_jwt` directive.
  // Configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	//
	// +optional
  // +kubebuilder:default="Restricted"
	Realm *string `json:"realm,omitempty"`

	// Mode selects how JWT keys are provided: local file or remote JWKS.
	// Default: File.
	//
  // +kubebuilder:default=File"
	Mode JWTKeyMode `json:"mode,omitempty"`

	// File specifies local JWKS configuration (Secret or ConfigMap, mount path, file name).
	// Required when Mode == File. Exactly one of ConfigMapRef or SecretRef must be set.
	//
	// +optional
	File *JWTFileKeySource `json:"file,omitempty"`

	// Remote specifies remote JWKS configuration.
	// Required when Mode == Remote.
	//
	// +optional
	Remote *JWTRemoteKeySource `json:"remote,omitempty"`

	// Leeway is the acceptable clock skew for exp/nbf checks.
  // Configures `auth_jwt_leeway` directive.
	// Example: "auth_jwt_leeway 60s".
	//
	// +optional
  // +kubebuilder:default=60s
	Leeway *v1alpha1.Duration `json:"leeway,omitempty"`

	// Type sets token type: signed | encrypted | nested.
	// Default: signed.
  // Configures `auth_jwt_type` directive.
  // Example: "auth_jwt_type signed;".
	//
	// +optional
  // +kubebuilder:default=signed
	Type *JWTTokenType `json:"type,omitempty"`

	// KeyCache is the cache duration for keys.
  // Configures auth_jwt_key_cache directive.
	// Example: "auth_jwt_key_cache 10m".
	//
	// +optional
	KeyCache *v1alpha1.Duration `json:"keyCache,omitempty"`

	// OnFailure customizes the 401 response for failed authentication.
	//
	// +optional
	OnFailure *AuthFailureResponse `json:"onFailure,omitempty"`

	// Require defines claims that must match exactly (e.g. iss, aud).
	// These translate into NGINX maps and auth_jwt_require directives.
  // Example directives and maps:
  //
  //  auth_jwt_require $valid_jwt_iss;
  //  auth_jwt_require $valid_jwt_aud;
  //
  //  map $jwt_claim_iss $valid_jwt_iss {
  //      "https://issuer.example.com" 1;
  //      "https://issuer.example1.com" 1;
  //      default 0;
  //  }
  //  map $jwt_claim_aud $valid_jwt_aud {
  //      "api" 1;
  //      "cli" 1;
  //      default 0;
  //  }
	//
	// +optional
	Require *JWTRequiredClaims `json:"require,omitempty"`

	// TokenSource defines where the client presents the token.
	// Defaults to reading from Authorization header.
	//
	// +optional
	TokenSource *JWTTokenSource `json:"tokenSource,omitempty"`

	// Propagation controls identity header propagation to upstream and header stripping.
	//
	// +optional
	Propagation *JWTPropagation `json:"propagation,omitempty"`
}

// JWTKeyMode selects where JWT keys come from.
// +kubebuilder:validation:Enum=File;Remote
type JWTKeyMode string

const (
	JWTKeyModeFile   JWTKeyMode = "File"
	JWTKeyModeRemote JWTKeyMode = "Remote"
)

// JWTFileKeySource specifies local JWKS key configuration.
// +kubebuilder:validation:XValidation:message="exactly one of configMapRef or secretRef must be set",rule="(self.configMapRef == null) != (self.secretRef == null)"
type JWTFileKeySource struct {
	// SecretRef references a Secret containing the JWKS (with optional key).
	// Exactly one of ConfigMapRef or SecretRef must be set.
	//
	// +optional
	SecretRef *NamespacedSecretKeyReference `json:"secretRef,omitempty"`

	// KeyCache is the cache duration for keys.
  // Configures `auth_jwt_key_cache` directive
	// Example: "auth_jwt_key_cache 10m;".
	//
	// +optional
	KeyCache *v1alpha1.Duration `json:"keyCache,omitempty"`
}

 // JWTRemoteKeySource specifies remote JWKS configuration.
type JWTRemoteKeySource struct {
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
	// Path is the filesystem path for cached JWKS objects.
	// Example: "/var/cache/nginx/jwks".
	Path string `json:"path"`

	// Levels specifies the directory hierarchy for cached files.
  // Used in `proxy_cache_path` directive.
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

// JWTTokenType represents NGINX auth_jwt_type.
// +kubebuilder:validation:Enum=signed;encrypted;nested
type JWTTokenType string

const (
	JWTTokenTypeSigned    JWTTokenType = "signed"
	JWTTokenTypeEncrypted JWTTokenType = "encrypted"
	JWTTokenTypeNested    JWTTokenType = "nested"
)

// JWTRequiredClaims specifies exact-match requirements for claims.
type JWTRequiredClaims struct {
	// Issuer (iss) required exact value.
	//
	// +optional
	Iss *string `json:"iss,omitempty"`

	// Audience (aud) required exact value.
	//
	// +optional
	Aud *string `json:"aud,omitempty"`
}

 // JWTTokenSourceMode selects where the JWT token is read from.
type JWTTokenSourceMode string

const (
	// Read from Authorization header (Bearer). Default.
	JWTTokenSourceModeHeader JWTTokenSourceMode = "Header"
	// Read from a cookie named tokenName.
	JWTTokenSourceModeCookie JWTTokenSourceMode = "Cookie"
	// Read from a query arg named tokenName.
	JWTTokenSourceModeQueryArg JWTTokenSourceMode = "QueryArg"
)

// JWTTokenSource specifies where tokens may be read from and the name when required.
type JWTTokenSource struct {
	// Mode selects the token source.
	// +kubebuilder:validation:Enum=Header;Cookie;QueryArg
	// +kubebuilder:default=Header
	Type JWTTokenSourceMode `json:"mode"`

	// TokenName is the cookie or query parameter name when Mode=Cookie or Mode=QueryArg.
	// Ignored when Mode=Header.
	//
	// +optional
	// +kubebuilder:default=access_token
	TokenName string `json:"tokenName,omitempty"`
}


// JWTPropagation controls identity header propagation and header stripping.
type JWTPropagation struct {
	// AddIdentityHeaders defines headers to add on success with values
	// typically derived from jwt_claim_* variables.
	//
	// +optional
	AddIdentityHeaders []HeaderValue `json:"addIdentityHeaders,omitempty"`

	// StripAuthorization removes the incoming Authorization header before proxying.
	//
	// +optional
	StripAuthorization *bool `json:"stripAuthorization,omitempty"`
}

// HeaderValue defines a header name and a value (may reference NGINX variables).
type HeaderValue struct {
	Name      string `json:"name"`
	ValueFrom string `json:"valueFrom"`
}

// AuthScheme enumerates supported WWW-Authenticate schemes.
// +kubebuilder:validation:Enum=Basic;Bearer
type AuthScheme string

const (
    AuthSchemeBasic  AuthScheme = "Basic"
    AuthSchemeBearer AuthScheme = "Bearer"
)

// AuthFailureBodyPolicy controls the failure response body behavior.
// +kubebuilder:validation:Enum=Unauthorized;Forbidden;Empty
type AuthFailureBodyPolicy string

const (
    AuthFailureBodyPolicyUnauthorized AuthFailureBodyPolicy = "Unauthorized"
    AuthFailureBodyPolicyForbidden AuthFailureBodyPolicy = "Forbidden"
    AuthFailureBodyPolicyEmpty   AuthFailureBodyPolicy = "Empty"
)

// AuthFailureResponse customizes 401/403 failures.
type AuthFailureResponse struct {
    // Allowed: 401, 403.
    // Default: 401.
    //
    // +optional
    // +kubebuilder:default=401
    // +kubebuilder:validation:XValidation:message="statusCode must be 401 or 403",rule="self == null || self in [401, 403]"
    StatusCode *int32 `json:"statusCode,omitempty"`

    // Challenge scheme. If omitted, inferred from filter Type (Basic|Bearer).
    // Configures WWW-Authenticate header in error page location.
    //
    // +optional
    // +kubebuilder:default=Basic
    Scheme *AuthScheme `json:"scheme,omitempty"`

    // Controls whether a default canned body is sent or an empty body.
    // Default: Unauthorized.
    //
    // +optional
    // +kubebuilder:default=Unauthorized
    BodyPolicy *AuthFailureBodyPolicy `json:"bodyPolicy,omitempty"`
}

// NamespacedSecretKeyReference references a Secret and optional key, with an optional namespace.
// If namespace differs from the filter's, a ReferenceGrant in the target namespace is required.
type NamespacedSecretKeyReference struct {
	// +optional
	Namespace *string `json:"namespace,omitempty"`
	Name      string  `json:"name"`
	// +optional
	Key       *string `json:"key,omitempty"`
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
	// * Invalid
	AuthenticationFilterConditionTypeAccepted AuthenticationFilterConditionType = "Accepted"

	// AuthenticationFilterConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	AuthenticationFilterConditionReasonAccepted AuthenticationFilterConditionReason = "Accepted"

	// AuthenticationFilterConditionReasonInvalid is used with the Accepted condition type when
	// the filter is invalid.
	AuthenticationFilterConditionReasonInvalid AuthenticationFilterConditionReason = "Invalid"
)
```

### Example spec for Basic Auth

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: basic-auth
spec:
  type: Basic
  basic:
    secretRef:
      name: basic-auth-users   # Secret containing htpasswd data
      key: htpasswd            # key within the Secret
    realm: "Restricted"        # Optional. Helps with logging
    onFailure:                 # Optional. These setting may be defaults.
      statusCode: 401
      scheme: Basic
```

In the case of Basic Auth, the deployed Secret and HTTPRoute may look like this:

#### Secret referenced by filter

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-users
type: Opaque
stringData:
  htpasswd: |
    admin:$apr1$ZxY12345$abcdefghijklmnopqrstuvwx/
    user:$apr1$AbC98765$mnopqrstuvwxyzabcdefghiJKL/
```

#### HTTPRoute that will reference this filter

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-basic
spec:
  parentRefs:
  - name: gateway
  hostnames:
  - api.example.com
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v2
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: basic-auth
    backendRefs:
    - name: backend
      port: 80
```

#### Generated NGINX config

```nginx
http {
    upstream backend_default {
        server 10.0.0.10:80;
        server 10.0.0.11:80;
    }

    server {
        listen 80;
        server_name api.example.com;

        location /v2 {
            # Injected by BasicAuthFilter "basic-auth"
            auth_basic "Restricted";
            auth_basic_user_file /etc/nginx/secrets/basic-auth-users/htpasswd;

            # Optional: customize failure per filter onFailure
            # Ensures a consistent body and explicit WWW-Authenticate header
            error_page 401 = @basic_auth_failure;

            # Optional: do not forward client Authorization header to upstream
            proxy_set_header Authorization "";

            # NGF standard proxy headers
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # Pass traffic to upstream
            proxy_pass http://backend_default;
        }

        # Internal location for custom 401 response
        location @basic_auth_failure {
            add_header WWW-Authenticate 'Basic realm="Restricted"' always;
            add_header Content-Type "text/plain; charset=utf-8" always;
            add_header X-Content-Type-Options "nosniff" always;
            add_header Cache-Control "no-store" always;
            return 401 'Unauthorized';
        }
    }
}
```

### Example spec for JWT Auth

For JWT Auth, there is two options.

1. Local JWKS file stored as as a Secret or as a ConfigMap
2. Remote JWKS from an IdP provider like Keycloak

#### Example JWT AuthenticationFilter with Local JWKS

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    # Key verification mode: Local file or Remote JWKs
    mode: File # Defaults to File.
    file:
      secretRef:
        name: jwt-keys-secure
        key: jwks.json
      keyCache: 10m  # Optional cache time for keys (auth_jwt_key_cache)
    # Acceptable clock skew for exp/nbf
    leeway: 60s # Configures auth_jwt_leeway
    # Sets auth_jwt_type
    type: signed # signed | encrypted | nested
    onFailure:
      statusCode: 403 # Set to 403 for example purposes. Defaults to 401.
      scheme: Bearer
```

#### Example JWT AuthenticationFilter with Remote JWKs

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    # Key verification mode: Local file or Remote JWKs
    mode: Remote # Defaults to File.
    remote:
      url: https://issuer.example.com/.well-known/jwks.json
    # Acceptable clock skew for exp/nbf
    leeway: 60s # Configures auth_jwt_leeway
    # Sets auth_jwt_type
    type: signed # signed | encrypted | nested
    # Optional cache duration for keys (auth_jwt_key_cache)
    keyCache: 10m
    onFailure:
      statusCode: 403 # Set to 403 for example purposes. Defaults to 401.
      scheme: Bearer
```

#### ConfigMap referenced by filter (if using configMapRef)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jwt-keys
data:
  jwks.json: ewogICJrZXlzIjogWwogICAgewogICAgICAia3R5IjogIlJTQSIsCiAgICAgICJ1c2UiOiAic2lnIiwKICAgICAgImtpZCI6ICJleGFtcGxlLWtleS1pZCIsCiAgICAgICJhbGciOiAiUlMyNTYiLAogICAgICAibiI6ICJiYXNlNjR1cmwtbW9kdWx1cyIsCiAgICAgICJlIjogIkFRQUIiCiAgICB9CiAgXQp9Cg==
```

#### Secret referenced by filter (if using secretRef)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: jwt-keys-secure
type: Opaque
data:
  jwks.json: ewogICJrZXlzIjogWwogICAgewogICAgICAia3R5IjogIlJTQSIsCiAgICAgICJ1c2UiOiAic2lnIiwKICAgICAgImtpZCI6ICJleGFtcGxlLWtleS1pZCIsCiAgICAgICJhbGciOiAiUlMyNTYiLAogICAgICAibiI6ICJiYXNlNjR1cmwtbW9kdWx1cyIsCiAgICAgICJlIjogIkFRQUIiCiAgICB9CiAgXQp9Cg==
```

Note: Secret data values must be base64-encoded and are decoded by the kubelet on mount, producing a valid jwks.json file. ConfigMap data values are plain text and should contain the raw JSON (not base64).

#### HTTPRoute that will reference this filter

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-jwt
spec:
  parentRefs:
  - name: gateway
  hostnames:
  - api.example.com
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v2
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: jwt-auth
    backendRefs:
    - name: backend
      port: 80
```

#### Generated NGINX Config

Below are `two` potential NGINX configurations based on the mode used.

1. NGINX Config when using `Mode: Key` (i.e. locally referenced JWKS key)

```nginx
http {
    upstream backend_default {
        server 10.0.0.10:80;
        server 10.0.0.11:80;
    }

    # Exact claim matching via maps for iss/aud
    map $jwt_claim_iss $valid_jwt_iss {
        "https://issuer.example.com" 1;
        default 0;
    }
    map $jwt_claim_aud $valid_jwt_aud {
        "api" 1;
        default 0;
    }

    server {
        listen 80;
        server_name api.example.com;

        location /v2 {
            auth_jwt "Restricted";

            # File-based JWKS
            auth_jwt_key_file /etc/nginx/keys/jwks.json;

            # Optional: key cache duration
            auth_jwt_key_cache 10m;

            # Leeway for exp/nbf
            auth_jwt_leeway 60s;

            # Token type
            auth_jwt_type signed;

            # Required claims (enforced via maps above)
            auth_jwt_require $valid_jwt_iss;
            auth_jwt_require $valid_jwt_aud;

            # Identity headers to pass back on success
            add_header X-User-Id        $jwt_claim_sub always;
            add_header X-User-Email     $jwt_claim_email always;
            add_header X-Auth-Mechanism "jwt" always;

            # Optional: customize failure per filter onFailure
            error_page 401 = @jwt_auth_failure;

            # Optional: do not forward client Authorization header to upstream
            proxy_set_header Authorization "";

            # NGF standard proxy headers
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # Pass traffic to upstream
            proxy_pass http://backend_default;
        }

        # Internal location for custom 401 response
        location @jwt_auth_failure {
            add_header WWW-Authenticate 'Bearer realm="Restricted", error="insufficient_scope"' always;
            add_header Content-Type "text/plain; charset=utf-8" always;
            add_header X-Content-Type-Options "nosniff" always;
            add_header Cache-Control "no-store" always;
            return 403 'Forbidden';
        }
    }
}
```

1. NGINX Config when using `Mode: Remote`

These are some directives the `Remote` mode uses over the `File` mode:

- `auth_jwt_key_request`: When using the `Remote` mode, this is used in place of `auth_jwt_key_file`. This will call the `internal` NGINX location `/jwks_uri` to redirect the request to the external auth provider (e.g. KeyCloak)
- `proxy_cache_path`: This is used to configuring caching of the JWKS after an initial request allowing subseuqnt requests to not request re-authenticaiton for a time

```nginx
http {
    # Serve JWKS from cache after the first fetch
    proxy_cache_path /var/cache/nginx/jwks levels=1:2 keys_zone=jwks_jwt_auth:10m max_size=50m inactive=10m use_temp_path=off;

    upstream backend_default {
        server 10.0.0.10:80;
        server 10.0.0.11:80;
    }

    # Exact claim matching via maps for iss/aud
    map $jwt_claim_iss $valid_jwt_iss {
        "https://issuer.example.com" 1;
        "https://issuer.example1.com" 1;
        default 0;
    }
    map $jwt_claim_aud $valid_jwt_aud {
        "api" 1;
        "cli" 1;
        default 0;
    }

    server {
        listen 80;
        server_name api.example.com;

        location /v2 {
            auth_jwt "Restricted";
            # Remote JWKS
            auth_jwt_key_request /jwks_uri;

            # Optional: key cache duration
            auth_jwt_key_cache 10m;

            # Leeway for exp/nbf
            auth_jwt_leeway 60s;

            # Token type
            auth_jwt_type signed;

            # Required claims (enforced via maps above)
            auth_jwt_require $valid_jwt_iss;
            auth_jwt_require $valid_jwt_aud;

            # Identity headers to pass back on success
            add_header X-User-Id        $jwt_claim_sub always;
            add_header X-User-Email     $jwt_claim_email always;
            add_header X-Auth-Mechanism "jwt" always;

            # Optional: customize failure per filter onFailure
            error_page 401 = @jwt_auth_failure;

            # Optional: do not forward client Authorization header to upstream
            proxy_set_header Authorization "";

            # NGF standard proxy headers
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # Pass traffic to upstream
            proxy_pass http://backend_default;
        }

        # Internal endpoint to fetch JWKS from IdP
        location = /jwks_uri {
            internal;
            # Enable caching of JWKS
            proxy_cache jwks_jwt_auth;
            proxy_pass  https://issuer.example.com/.well-known/jwks.json;
        }

        # Internal location for custom 401 response
        location @jwt_auth_failure {
            add_header WWW-Authenticate 'Bearer realm="Restricted", error="invalid_token"' always;
            add_header Content-Type "text/plain; charset=utf-8" always;
            add_header X-Content-Type-Options "nosniff" always;
            add_header Cache-Control "no-store" always;
            return 401 'Unauthorized';
        }
    }
}
```

#### Additional Optional Fields

`require`, `tokenSource` and `propagation` are some additioanl fields we may choose to include.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    keys:
      mode: Remote
      remote:
        url: https://issuer.example.com/.well-known/jwks.json

    # Required claims (exact matching done via maps in NGINX; see config)
    require:
      iss:
        - "https://issuer.example.com"
        - "https://issuer2.example.com"
      aud:
        - "api"
        - "cli"

    # Where client presents the token
    # By defaults to reading from Authorization header (Bearer)
    tokenSource:
      type: Header
      # Alternative: read from a cookie named tokenName
      # type: Cookie
      # tokenName: access_token
      # Alternative: read from a query arg named tokenName
      # type: QueryArg
      # tokenName: access_token

    # Identity propagation to backend and header stripping
    propagation:
      addIdentityHeaders:
        - name: X-User-Id
          valueFrom: "$jwt_claim_sub"
        - name: X-User-Email
          valueFrom: "$jwt_claim_email"
      stripAuthorization: true # Optionally remove client Authorization header
```

### Caching configuration

Users may also choose to change the caching configuration set by `proxy_cache_path`.
This can be made available in the `cache` configuration under `jwt.remote.cache`

```yaml
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    mode: Remote
    remote:
      url: https://issuer.example.com/.well-known/jwks.json
      cache:
        path: /var/cache/nginx/jwks # required when cache is set
        levels: "1:2"               # optional; defaults to "1:2"
        keysZoneName: jwks_jwtauth  # optional; controller can default to a derived name
        keysZoneSize: 10m           # required; size for keys_zone
        maxSize: 50m                # optional; limit total cache size
        inactive: 10m               # optional; inactivity TTL before eviction
        useTempPath: false          # optional; sets use_temp_path
```

### Attachment

Filters must be attached to a HTTPRoute at the `rules.matches` level.
This means that a single `AuthenticationFilter` may be attached mutliple times to a single HTTPRoute.

#### Basic example

This example shows a single HTTPRoute, with a single `filter` defined in a `rule`

![reference-1](/docs/images/authentication-filter/reference-1.png)

### Status

#### Referencing multiple AuthenticationFilter resources in a single rule

Only a single `AuthenticationFilter` may be referened in a single rule.

The `Status` the HTTPRoute/GRPCRoute in this scenario should be set to `Invalid`, and the resource should be `Rejected`

This behavour falls in line with the expected behaviour of filters in the Gateway API, which generally allows only one type of a specific filter (authentication, rewriting, etc.) within a rule.

Below is an eample of an **invalid** HTTPRoute that references multiple `AuthenticationFilter` resources in a single rule

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: invalid-httproute
spec:
  parentRefs:
  - name: gateway
  hostnames:
  - api.example.com
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: basic-auth
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: jwt-auth
    backendRefs:
    - name: backend
      port: 80
```

## Testing

- Unit tests
- Functional tests to validate behavioural scenarios when referncing filters in different combinations. The details of these tests are out of scope for this document.

## Security Considerations

### Basic Auth and Local JWKS

Basic Auth sends credentials in an Authorization header that is base64-encoded.
JWT Auth requires users to provided a bearer token through the Authroization header.

Both of these methods can be easily intercepted over HTTP.

Users that attach an `AuthenticaitonFilter` to a HTTPRoute/GRPCRoute should be advised to enable HTTPS traffic at the Gateway level for the routes.

Any exmaple configurations and deployments for the `AuthenticationFilter` should enable HTTPS at the Gateway level by default.

### Namespace isolataion and cross-namespace references
Both Auth and Local JWKS should only have access to Secrets and ConfigMaps in the same namespace by default.

Cross-namespace references are allowed only when authorized via a Gateway API ReferenceGrant in the target namespace.

Controller behavior:
- Same-namespace references are permitted without a grant.
- For cross-namespace references, the controller MUST verify a ReferenceGrant exists in the target namespace:
  - from: group=gateway.nginx.org, kind=AuthenticationFilter, namespace=<filter-namespace>
  - to:   group="", kind=(Secret|ConfigMap), name=<target-name>
- If no valid grant is found, the filter status should update the status to `Accepted=False` with `reason=RefNotPermitted` and a clear message. We should avoid rendering any NGINX configuration in this scenario.

Example: Grant BasicAuth in app-ns to read a Secret in security-ns
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ReferenceGrant
metadata:
  name: allow-basic-auth-secret
  namespace: security-ns # target namespace where the Secret lives
spec:
  from:
  - group: gateway.nginx.org
    kind: AuthenticationFilter
    namespace: app-ns
  to:
  - group: ""   # core API group
    kind: Secret
    name: basic-auth-users
```

AuthenticationFilter referencing the cross-namespace Secret
```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: basic-auth
  namespace: app-ns
spec:
  type: Basic
  basic:
    secretRef:
      namespace: security-ns
      name: basic-auth-users
      key: htpasswd
    realm: "Restricted"
```

Example: Grant JWT file-based JWKS in keys-ns to filter in app-ns
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ReferenceGrant
metadata:
  name: allow-jwks-configmap
  namespace: keys-ns
spec:
  from:
  - group: gateway.nginx.org
    kind: AuthenticationFilter
    namespace: app-ns
  to:
  - group: ""    # core API group
    kind: ConfigMap
    name: jwt-keys
```

AuthenticationFilter referencing cross-namespace JWKS ConfigMap
```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
  namespace: app-ns
spec:
  type: JWT
  jwt:
    mode: File
    file:
      configMapRef:
        namespace: keys-ns
        name: jwt-keys
```

### Remote JWKS

Proxy cache TTL should be configurable and set to a resonable default, reducing periods of stale cached JWKs.

### Key rotation

Users should be advised to regularly rotate their JWKS keys in cases where they chose to reference a local JWKS via a `secrefRef` or `configMapRef`

### Auth failure behaviour

3xx response codes should not be allowed and AuthenticationFilter.onFailure must not support redirect targets. This is to prevent to prevent open-redirect abuse.

401 and 403 should be the only allowable auth failure codes.

### Auth failure default headers

Below are a list of default defensive headers for authentication failure reponses.
We may choose to include these headers by default for improved robustness in auth failure responses.

```nginx
add_header Content-Type "text/plain; charset=utf-8" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Cache-Control "no-store" always;
```

Detailed header breakdown:

- Content-Type: "text/plain; charset=utf-8"
  - This header explicitly set the body as plain text. This prevents browsers from treating the response as HTML or JavaScript, and is effective at mitigating Cross-side scrpting (XSS) through error pages

- X-Content-Type-Options: "nosniff"
  - This header prevents content type confusion. This occurrs when browsers guesses HTML & JavaScript, and executes it despite a benign type.

- Cache-Control: "no-store"
  - This header informs browsers and proxies not to cache the response. Avoids sensitive, auth-related content, from being being stored and served later to unintended recipients.


### Validation

When referencing an `AuthenticationFilter` in either a HTTPRoute or GRPCRoute, it is important that we ensure all configurable fields are validated, and that the resulting NGINX configuration is correct and secure.

All fields in the `AuthenticationFilter` will be validated with Open API Schema.
We should also include [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules) validation where required.

We should validated that only one `AuthenticationFilter` is referenced per-rule. Multiple references to an `AuthenticationFilter` in a single rule should result in an `Invalid` HTTPRoute/GRPCRoute, and the resource should be `Rejected`.

An `AuthenticationFilter` that sets a `onFailure.statusCode` to anything other than `401` or `403` should be rejected. This relates to the "Auth failure behaviour" section in the Security Considerations section.

## Alternatives

The Gateway API defines a means to standardise authentication through use of the [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter) available in the HTTPRoute specification.

This allows users to reference an external authentication services, such as Keycloak, to handle the authentication requests.
While this API is available in the experimental channel, it is subject to change.

Our decision to go forward with our own `AuthenticationFilter` was to ensure we could quickly provide authentication to our users while allowing us to closely monitor progress of the ExternalAuthFilter.

It is certainly possible for us to provide an External Authentication Services that leverages NGINX and is something we can further investigate as the API progresses.

## Additional considerations

### Documenting filter behavour

In regards to documentation of filter behaviour with the `AuthenticationFilter`, the Gateway API documentation on filters states the following:

```text
Wherever possible, implementations SHOULD implement filters in the order they are specified.

Implementations MAY choose to implement this ordering strictly, rejecting
any combination or order of filters that cannot be supported.
If implementations choose a strict interpretation of filter ordering, they MUST clearly
document that behavior.
```

## References

- [Gateway API ExternalAuthFilter GEP]((https://gateway-api.sigs.k8s.io/geps/gep-1494/))
- [HTTPExternalAuthFilter Specification](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter)
- [Kubernetes documentation on CEL validaton](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules)
- [NGINX HTTP Basic Auth Module](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html)
- [NGINX JWT Auth Module](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html)
- [NGINX OIDC Module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)
