# Enhancement Proposal-4052: Authentication Filter

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
- Authentication failures return 401 Unauthorized by default
- Ensure response codes are configurable

## Non-Goals

- Design for OIDC Auth
- An Auth filter for TCP and UDP routes
- Design for integration with [ExternalAuth in the Gateway API](https://gateway-api.sigs.k8s.io/geps/gep-1494/)

## Introduction

This document focuses explicitly on Authentication (AuthN) and not Authorization (AuthZ). Authentication (AuthN) defines the verification of identity. It asks the question, "Who are you?". This is different from Authorization (AuthZ), which follows Authentication. It asks the question, "What are you allowed to do?"

This document also focuses on HTTP Basic Authentication and JWT Authentication. Other authentication methods such as OpenID Connect (OIDC) are mentioned, but are not part of the CRD design. These will be covered in future design and implementation tasks.


## Use Cases

- As an Application Developer, I want to secure access to my APIs and Backend Applications.
- As an Application Developer, I want to enforce authentication on specific routes and matches.

### Understanding NGINX authentication methods

| **Authentication Method** | **OSS** | **Plus** | **NGINX Module** | **Details** |
| ------------------------------- | -------------- | ---------------- | ---------------------------------- | -------------------------------------------------------------------- |
| **HTTP Basic Authentication** | ✅ | ✅ | [ngx_http_auth_basic](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html) | Requires a username and password sent in an HTTP header. |
| **JWT (JSON Web Token)** | ❌ | ✅ | [ngx_http_auth_jwt_module](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html) | Tokens are used for stateless authentication between client and server. |
| **OpenID Connect** | ❌ | ✅ | [ngx_http_oidc_module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html) | Allows authentication through third-party providers like Google. |

### Understanding authentication terminology

#### Realms

[RFC 7617](https://www.rfc-editor.org/rfc/rfc7617) gives an overview of the Realm parameter, which is used by `auth_basic` and `auth_jwt` directives in NGINX.

```text
The realm value is a free-form string
that can only be compared for equality with other realms on that
server. The server will service the request only if it can validate
the user-id and password for the protection space applying to the
requested resource.
```


## API, Customer Driven Interfaces, and User Experience

This portion of the proposal will cover API design and interaction experience for use of Basic Auth and JWT.
This portion also contains:

1. The Golang API
2. Example spec for Basic Auth
    - Example HTTPRoutes and NGINX configuration
3. Example spec for JWT Auth
    - Example HTTPRoutes
    - Examples for Local & Remote JWKS configuration
    - Example NGINX configuration for both Local & Remote JWKS
    - Example of additional optional fields

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

// AuthenticationFilterList contains a list of AuthenticationFilter resources.
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
  // Required when Type == Basic.
  //
  // +optional
  Basic *BasicAuth `json:"basic,omitempty"`

  // JWT configures JSON Web Token authentication (NGINX Plus).
  // Required when Type == JWT.
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
// +kubebuilder:validation:XValidation:message="mode 'File' requires file set and remote unset",rule="self.mode == 'File' ? self.file != null && self.remote == null : true"
// +kubebuilder:validation:XValidation:message="mode 'Remote' requires remote set and file unset",rule="self.mode == 'Remote' ? self.remote != null && self.file == null : true"
// +kubebuilder:validation:XValidation:message="when file is set, mode must be 'File'",rule="self.file != null ? self.mode == 'File' : true"
// +kubebuilder:validation:XValidation:message="when remote is set, mode must be 'Remote'",rule="self.remote != null ? self.mode == 'Remote' : true"
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
  Leeway *v1alpha1.Duration `json:"leeway,omitempty"`

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
  KeyCache *v1alpha1.Duration `json:"keyCache,omitempty"`
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
  KeyCache *v1alpha1.Duration `json:"keyCache,omitempty"`
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

// AuthScheme enumerates supported WWW-Authenticate schemes.
// +kubebuilder:validation:Enum=Basic;Bearer
type AuthScheme string

const (
  AuthSchemeBasic  AuthScheme = "Basic"
  AuthSchemeBearer AuthScheme = "Bearer"
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
      name: basic-auth-users   # Secret containing auth data.
    realm: "Restricted"
```

In the case of Basic Auth, the deployed Secret and HTTPRoute may look like this:

#### Secret referenced by filter

For Basic Auth, we will process a custom secret type of `nginx.org/htpasswd`.
This will allow us to be more confident that the user is providing us with the appropriate kind of secret for this use case.

To create this kind of secret for Basic Auth first run this command:

```bash
htpasswd -c auth user
```

This will create a file called `auth` with the username and an MD5-hashed password:

```bash
cat auth
user:$apr1$prQ3Bh4t$A6bmTv7VgmemGe5eqR61j0
```

Use these options in the `htpasswd` command for stronger hashing algorithms:

```bash
 -2  Force SHA-256 hashing of the password (secure).
 -5  Force SHA-512 hashing of the password (secure).
 -B  Force bcrypt hashing of the password (very secure).
```

You can then run this command to generate the secret from the `auth` file:

```bash
kubectl create secret generic auth-basic-test --type='nginx.org/htpasswd' --from-file=auth
```

Note: `auth` will be the default key for secrets referenced by `AuthenticationFilters` of `Type: Basic`.

Example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-users
type: nginx.org/htpasswd
data:
  auth: YWRtaW46JGFwcjEkWnhZMTIzNDUkYWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4Lwp1c2VyOiRhcHIxJEFiQzk4NzY1JG1ub3BxcnN0dXZ3eHl6YWJjZGVmZ2hpSktMLwo=
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

Note: For Basic Auth, NGF will store the file used by `auth_basic_user_file` in `/etc/nginx/secrets/`
The full path will use the `name`, `namespace`, and `key` of the secret referenced by `AuthenticationFilter`
In this case, the full path will be `/etc/nginx/secrets/default/basic-auth-users/auth`

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

            # Path is generated by NGF using the name and key from the secret
            auth_basic_user_file /etc/nginx/secrets/default/basic-auth-users/auth;

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
    }
}
```

### Example spec for JWT Auth

For JWT Auth, there are two options.

1. Local JWKS file stored as a Secret
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
    # Key verification mode. Local file or Remote JWKS
    mode: File
    file:
      secretRef:
        name: jwt-keys-secure
      keyCache: 10m  # Optional cache time for keys (auth_jwt_key_cache)
    # Acceptable clock skew for exp/nbf
    leeway: 60s # Configures auth_jwt_leeway
    # Sets auth_jwt_type
    type: signed # signed | encrypted | nested
```

#### Example JWT AuthenticationFilter with Remote JWKS

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    # Key verification mode. Local file or Remote JWKS
    mode: Remote
    remote:
      url: https://issuer.example.com/.well-known/jwks.json
    # Acceptable clock skew for exp/nbf
    leeway: 60s # Configures auth_jwt_leeway
    # Sets auth_jwt_type
    type: signed # signed | encrypted | nested
    # Optional cache duration for keys (auth_jwt_key_cache)
    keyCache: 10m
```

#### Secret referenced by filter

For JWT Auth, we will process a custom secret type of `nginx.org/jwt`.
This will allow us to be more confident that the user is providing us with the appropriate kind of secret for this use case.

To create the example secret, run the following command:

```bash
kubectl create secret generic jwt-keys-secure --type='nginx.org/jwt' --from-file=jwks.json
```

Note: `jwks.json` will be the default key for secrets referenced by `AuthenticationFilters` of `Type: JWT`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: jwt-keys-secure
type: nginx.org/jwt
data:
  jwks.json: ewogICJrZXlzIjogWwogICAgewogICAgICAia3R5IjogIlJTQSIsCiAgICAgICJ1c2UiOiAic2lnIiwKICAgICAgImtpZCI6ICJleGFtcGxlLWtleS1pZCIsCiAgICAgICJhbGciOiAiUlMyNTYiLAogICAgICAibiI6ICJiYXNlNjR1cmwtbW9kdWx1cyIsCiAgICAgICJlIjogIkFRQUIiCiAgICB9CiAgXQp9Cg==
```

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

1. NGINX Config when using `Mode: File` (i.e. locally referenced JWKS key)

Note: For JWT Auth, NGF will store the file used by `auth_jwt_key_file` in `/etc/nginx/keys/`
The full path will use the `name`, `namespace`, and `key` of the secret referenced by `AuthenticationFilter`
In this case, the full path will be `/etc/nginx/keys/default/jwt-keys-secure/jwks.json`

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
            # Path is generated by NGF using the name and key from the secret
            auth_jwt_key_file /etc/nginx/keys/default/jwt-keys-secure/jwks.json;

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
    }
}
```

1. NGINX Config when using `Mode: Remote`

These are some directives the `Remote` mode uses over the `File` mode:

- `auth_jwt_key_request`: When using the `Remote` mode, this is used in place of `auth_jwt_key_file`. This will call the `internal` NGINX location `/_ngf-internal_jwks_uri` to redirect the request to the external auth provider (e.g. Keycloak)
- `proxy_cache_path`: This is used to configure caching of the JWKS after an initial request, allowing subsequent requests to avoid re-authentication for a time

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
            auth_jwt_key_request /_ngf-internal_jwks_uri;

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
        location = /_ngf-internal_jwks_uri {
            internal;
            # Enable caching of JWKS
            proxy_cache jwks_jwt_auth;
            proxy_pass  https://issuer.example.com/.well-known/jwks.json;
        }
    }
}
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
        levels: "1:2"               # optional; defaults to "1:2"
        keysZoneName: jwks_jwtauth  # optional; controller can default to a derived name
        keysZoneSize: 10m           # required; size for keys_zone
        maxSize: 50m                # optional; limit total cache size
        inactive: 10m               # optional; inactivity TTL before eviction
        useTempPath: false          # optional; sets use_temp_path
```

### Attachment

Filters must be attached to an HTTPRoute at the `rules.matches` level.
This means that a single `AuthenticationFilter` may be attached multiple times to a single HTTPRoute.

#### Basic example

This example shows a single HTTPRoute, with a single `filter` defined in a `rule`

![reference-1](/docs/images/authentication-filter/reference-1.png)

### Status

#### Referencing multiple AuthenticationFilter resources in a single rule

Only a single `AuthenticationFilter` may be referenced in a single rule.

The Status of the HTTPRoute/GRPCRoute in this scenario should be set to `Invalid`, and the resource should be `Rejected`.
In this scenario, the route rule that is referencing multiple `AuthenticationFilter` resources will be `Rejected`.
All other route rules will remain working.

The HTTPRoute/GRPCRoute resource will display an `UnresolvedRef` message to inform the user that the rule has been `Rejected`.

This behaviour falls in line with the expected behaviour of filters in the Gateway API, which generally allows only one type of a specific filter (authentication, rewriting, etc.) within a rule.

Below is an example of an **invalid** HTTPRoute that references multiple `AuthenticationFilter` resources in a single rule:

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

#### Referencing an AuthenticationFilter resource that is invalid

Note: With appropriate use of CEL validation, we are less likely to encounter a scenario where an AuthenticationFilter has been deployed to the cluster with an invalid configuration.
If this does happen, and a route rule references this AuthenticationFilter, the route rule will be set to `Invalid` and the HTTPRoute/GRPCRoute will display the `UnresolvedRef` status.

#### Attaching a JWT AuthenticationFilter to a route when using NGINX OSS

If a user attempts to attach a JWT type AuthenticationFilter while using NGINX OSS, the rule referencing the filter will be `Rejected`.

This can use the status `RouteConditionPartiallyInvalid` defined in the Gateway API here: https://github.com/nginx/nginx-gateway-fabric/blob/3934c5c8c60b5aea91be4337d63d4e1d8640baa8/internal/controller/state/conditions/conditions.go#L402

## Testing

- Unit tests
- Functional tests to validate behavioural scenarios when referencing filters in different combinations.

## Functional Test Cases

### Valid scenarios

This section covers deployment scenarios that are considered valid

- Single route rule with a single path in an HTTPRoute/GRPCRoute referencing a valid AuthenticationFilter
- Single route rule with two or more paths in an HTTPRoute/GRPCRoute referencing a valid AuthenticationFilter
- Two or more route rules each with a single path in an HTTPRoute/GRPCRoute referencing a valid AuthenticationFilter
- Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute referencing a valid AuthenticationFilter
- Two or more HTTPRoute/GRPCRoute resources each with a single route rule with a single path referencing a valid AuthenticationFilter.
- Two or more HTTPRoute/GRPCRoute resources each with a single route rule, each with two or more paths referencing a valid AuthenticationFilter.
- Two or more HTTPRoute/GRPCRoute resources each with two or more route rules, each with a single path referencing a valid AuthenticationFilter.

### Invalid scenarios

This section covers deployment scenarios that are considered invalid

- Single route rule with a single path in an HTTPRoute/GRPCRoute referencing an invalid AuthenticationFilter
- Single route rule with two or more paths in an HTTPRoute/GRPCRoute referencing an invalid AuthenticationFilter
- Two or more route rules each with a single path in an HTTPRoute/GRPCRoute referencing an invalid AuthenticationFilter
- Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute referencing an invalid AuthenticationFilter
- Two or more HTTPRoute/GRPCRoute resources each with a single route rule with a single path referencing an invalid AuthenticationFilter.
- Two or more HTTPRoute/GRPCRoute resources each with a single route rule, each with two or more paths referencing an invalid AuthenticationFilter.
- Two or more HTTPRoute/GRPCRoute resources each with two or more route rules, each with a single path referencing an invalid AuthenticationFilter.
- Two or more route rules each with a single path in an HTTPRoute/GRPCRoute, where one rule references a valid AuthenticationFilter, and the other references an invalid AuthenticationFilter.
- Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute where one rule references a valid AuthenticationFilter, and the other references an invalid AuthenticationFilter.
- Two or more valid or invalid AuthenticationFilters referenced in a route rule.

### Invalid AuthenticationFilter scenarios

This section covers configuration scenarios for an AuthenticationFilter resource that would be considered invalid

- An AuthenticationFilter deployed with an empty `Realm` value
- An AuthenticationFilter deployed with an empty `secretRef.Name` value
- An AuthenticationFilter referencing a secret that does not exist
- An AuthenticationFilter referencing a secret in a different namespace
- An AuthenticationFilter referencing a secret with an incorrect type (e.g., Opaque)
- An AuthenticationFilter referencing a secret with an incorrect key

## Security Considerations

### Basic Auth and Local JWKS

Basic Auth sends credentials in an Authorization header that is base64-encoded.
JWT Auth requires users to provide a bearer token through the Authorization header.

Both of these methods can be easily intercepted over HTTP.

Users that attach an `AuthenticationFilter` to an HTTPRoute/GRPCRoute should be advised to enable HTTPS traffic at the Gateway level for the routes.

Any example configurations and deployments for the `AuthenticationFilter` should enable HTTPS at the Gateway level by default.

### Remote JWKS

Proxy cache TTL should be configurable and set to a reasonable default, reducing periods of stale cached JWKs.

### Key rotation

Users should be advised to regularly rotate their JWKS keys in cases where they choose to reference a local JWKS via a `secretRef`.

### Optional headers

Below are a list of optional defensive headers that users may choose to include.
In certain scenarios, these headers may be deployed to improve overall security from client responses.

```nginx
add_header Content-Type "text/plain; charset=utf-8" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Cache-Control "no-store" always;
```

Detailed header breakdown:

- Content-Type: "text/plain; charset=utf-8"
  - This header explicitly sets the body as plain text. This prevents browsers from treating the response as HTML or JavaScript, and is effective at mitigating Cross-site scripting (XSS) through error pages

- X-Content-Type-Options: "nosniff"
  - This header prevents content type confusion. This occurs when browsers guess HTML and JavaScript, and execute it despite a benign type.

- Cache-Control: "no-store"
  - This header informs browsers and proxies not to cache the response. Avoids sensitive, auth-related content from being stored and served later to unintended recipients.


### Validation

When referencing an `AuthenticationFilter` in either an HTTPRoute or GRPCRoute, it is important that we ensure all configurable fields are validated, and that the resulting NGINX configuration is correct and secure.

All fields in the `AuthenticationFilter` will be validated with OpenAPI Schema.
We should also include [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules) validation where required.

We should validate that only one `AuthenticationFilter` is referenced per-rule. Multiple references to an `AuthenticationFilter` in a single rule should result in an `Invalid` HTTPRoute/GRPCRoute, and the rule should be `Rejected`.

This scenario can use the status `RouteConditionPartiallyInvalid` defined in the Gateway API here: https://github.com/nginx/nginx-gateway-fabric/blob/3934c5c8c60b5aea91be4337d63d4e1d8640baa8/internal/controller/state/conditions/conditions.go#L402

## Alternatives

The Gateway API defines a means to standardise authentication through use of the [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter) available in the HTTPRoute specification.

This allows users to reference an external authentication service, such as Keycloak, to handle the authentication requests.
While this API is available in the experimental channel, it is subject to change.

Our decision to go forward with our own `AuthenticationFilter` was to ensure we could quickly provide authentication to our users while allowing us to closely monitor progress of the ExternalAuthFilter.

It is certainly possible for us to provide an External Authentication Service that leverages NGINX and is something we can further investigate as the API progresses.

## Additional considerations

### Documenting filter behavior

In regards to documentation of filter behavior with the `AuthenticationFilter`, the Gateway API documentation on filters states the following:

```text
Wherever possible, implementations SHOULD implement filters in the order they are specified.

Implementations MAY choose to implement this ordering strictly, rejecting
any combination or order of filters that cannot be supported.
If implementations choose a strict interpretation of filter ordering, they MUST clearly
document that behavior.
```

## Future updates

### Custom authentication failure response

By default, authentication failures return a 401 response.
If a user wanted to change this response code, or include additional headers in this response, we can include a custom named location that can be called by the [error_page](https://nginx.org/en/docs/http/ngx_http_core_module.html#error_page) directive.

Example AuthenticationFilter configuration:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: basic-auth
spec:
  type: Basic
  basic:
    secretRef:
      name: basic-auth-users
    realm: "Restricted"
    onFailure:
      statusCode: 401
      scheme: Basic
```

Example NGINX configuration:

```nginx
server{
  location /api {
      auth_basic "Restricted";
      auth_basic_user_file /etc/nginx/secrets/default/basic-auth-users/auth;

      # Calls named location
      error_page 401 = @basic_auth_failure;
    }

    location @basic_auth_failure {
        add_header WWW-Authenticate 'Basic realm="Restricted"' always;
        return 401 'Unauthorized';
    }
}
```

If we support this configuration, 3xx response codes should not be allowed and AuthenticationFilter.onFailure must not support redirect targets. This is to prevent open-redirect abuse.

We should only allow 401 and 403 response codes.

### Cross-namespace access

When referencing secrets for Basic Auth and JWT Auth, the initial implementation will use `LocalObjectReference`.

Future updates to this will use the `NamespacedSecretKeyReference` in conjunction with `ReferenceGrants` to support access to secrets in different namespaces.

Struct for `NamespacedSecretKeyReference`:

```go
// NamespacedSecretKeyReference references a Secret and optional key, with an optional namespace.
// If namespace differs from the filter's, a ReferenceGrant in the target namespace is required.
type NamespacedSecretKeyReference struct {
  // +optional
  Namespace *string `json:"namespace,omitempty"`
  Name      string  `json:"name"`
  // +optional
  Key       *string `json:"key,omitempty"`
}
```

For the initial implementation, both Basic Auth and Local JWKS will only have access to Secrets in the same namespace.

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
    realm: "Restricted"
```

### Additional Fields for JWT

`require`, `tokenSource` and `propagation` are some additional fields that may be included in future updates to the API.
These fields allow for more customization of how the JWT auth behaves, but aren't required for the minimal delivery of JWT Auth.

Example of what implementation of these fields might look like:

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
    # By default, reading from Authorization header (Bearer)
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
      stripAuthorization: true # Optionally remove client Authorization header before proxy_pass
```

Example Golang API changes:

```go
type JWTAuth struct {
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
  TokenSource *TokenSource `json:"tokenSource,omitempty"`

  // Propagation controls identity header propagation to upstream and header stripping.
  //
  // +optional
  Propagation *JWTPropagation `json:"propagation,omitempty"`
}

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

// JWTTokenSourceType selects where the JWT token is read from.
// +kubebuilder:validation:Enum=Header;Cookie;QueryArg
type TokenSourceType string

const (
  // Read from Authorization header (Bearer). Default.
  TokenSourceModeHeader TokenSourceMode = "Header"
  // Read from a cookie named tokenName.
  TokenSourceModeCookie TokenSourceMode = "Cookie"
  // Read from a query arg named tokenName.
  TokenSourceModeQueryArg TokenSourceMode = "QueryArg"
)

// JWTTokenSource specifies where tokens may be read from and the name when required.
type TokenSource struct {
  // Mode selects the token source.
  // +kubebuilder:default=Header
  Type TokenSourceType `json:"mode"`

  // TokenName is the cookie or query parameter name when Mode=Cookie or Mode=QueryArg.
  // Ignored when Mode=Header.
  //
  // +optional
  // +kubebuilder:default=access_token
  TokenName string `json:"tokenName,omitempty"`
}
// JWTPropagation controls identity header propagation and header stripping.
type JWTPropagation struct {
  // AddIdentityHeaders defines headers to add on success with values.
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
```

## References

- [Gateway API ExternalAuthFilter GEP](https://gateway-api.sigs.k8s.io/geps/gep-1494/)
- [HTTPExternalAuthFilter Specification](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter)
- [Kubernetes documentation on CEL validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules)
- [NGINX HTTP Basic Auth Module](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html)
- [NGINX JWT Auth Module](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html)
- [NGINX OIDC Module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)
