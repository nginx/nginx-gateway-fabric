# Enhancement Proposal-4052: Authentication Filter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Implementable

## Summary

Design and implement a means for users of NGINX Gateway Fabric to enable authentication on requests to their backend applications.
This filter should eventually expose all forms of authentication available through NGINX, both Open Source and Plus.

## Goals

- Design a means of configuring authentication for NGF
- Design Authentication CRD with Basic Auth and JWT Auth in mind
- Determine initial resource specification
- Evaluate filter early in request processing, occurring before URLRewrite, header modifiers and backend selection
- Authentication failures return 401 Unauthorized by default

## Non-Goals

- Design for OIDC Auth
- Allow multiple authentication mechanisms per route rule
- An Auth filter for TCP and UDP routes
- Ensure response codes are configurable
- Design for integration with [ExternalAuth in the Gateway API](https://gateway-api.sigs.k8s.io/geps/gep-1494/)

## Introduction

This document focuses explicitly on Authentication (AuthN) and not Authorization (AuthZ). Authentication (AuthN) defines the verification of identity. It asks the question, "Who are you?". This is different from Authorization (AuthZ), which follows Authentication. It asks the question, "What are you allowed to do?"

This document also focuses on HTTP Basic Authentication and JWT Authentication. Other authentication methods such as OpenID Connect (OIDC) are mentioned but are not part of the CRD design. These will be covered in future design and implementation tasks.

## Use Cases

- As an Application Developer, I want to secure access to my APIs and Backend Applications.
- As an Application Developer, I want to enforce authentication on specific routes and matches.

### Understanding NGINX Authentication Methods

| **Authentication Method** | **OSS** | **Plus** | **NGINX Module** | **Details** |
| ------------------------------- | -------------- | ---------------- | ---------------------------------- | -------------------------------------------------------------------- |
| **HTTP Basic Authentication** | ✅ | ✅ | [ngx_http_auth_basic](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html) | Requires a username and password sent in an HTTP header. |
| **JWT (JSON Web Token)** | ❌ | ✅ | [ngx_http_auth_jwt_module](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html) | Tokens are used for stateless authentication between client and server. |
| **OpenID Connect** | ❌ | ✅ | [ngx_http_oidc_module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html) | Allows authentication through third-party providers like Google. |

### Understanding Authentication Terminology

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

- The Golang API
- Basic Auth
    - Proposed spec for Basic Auth
    - Secret creation and reference for Basic Auth
    - Example HTTPRoute resource
    - Generated NGINX configuration
- JWT Auth
    - Proposed spec for JWT Auth
    - Secret creation and reference for JWT Auth
    - Example HTTPRoute resource
    - Generated NGINX configuration
    - JWT claims
      - Understanding JWT claims
      - Understanding nested claims
      - Understanding claim enforcement
      - Processing claims
      - Processing nested claims
    - JWT Authentication Capabilities
- Route Attachment
- Resource status

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

// AuthenticationFilter configures request authentication and is
// attached as a filter via ExtensionRef.
type AuthenticationFilter struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`

  // Spec defines the desired state of the AuthenticationFilter.
  Spec AuthenticationFilterSpec `json:"spec"`

  // Status defines the state of the AuthenticationFilter.
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
// +kubebuilder:validation:XValidation:message="type Basic requires spec.basic to be set.",rule="self.type != 'Basic' || has(self.basic)"
// +kubebuilder:validation:XValidation:message="type Basic must not set spec.jwt.", rule="self.type != 'Basic' || !has(self.jwt)"
// +kubebuilder:validation:XValidation:message="type JWT requires spec.jwt to be set.",rule="self.type != 'JWT' || has(self.jwt)"
// +kubebuilder:validation:XValidation:message="type JWT must not set spec.basic.", rule="self.type != 'JWT' || !has(self.basic)"
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
// +kubebuilder:validation:Enum=Basic;JWT
type AuthType string

const (
  // AuthTypeBasic is the HTTP Basic Authentication mechanism.
  AuthTypeBasic AuthType = "Basic"
  // AuthTypeJWT is the JWT Authentication mechanism.
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

// JWTKeySource selects where JWT keys come from.
// +kubebuilder:validation:Enum=File;Remote
type JWTKeySource string

const (
  // JWTKeySourceFile configures JWT to fetch JWKS from a local secret.
  JWTKeySourceFile   JWTKeySource = "File"
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

  // Authorization defines the authorization (authz) specification.
  // Enables configuration of token claim validation.
  //
  // +optional
  Authorization *Authorization `json:"authorization,omitempty"`

  // Leeway is the acceptable clock skew for exp & nbf claims.
  // If exp & nbf claims are not defined, this directive takes no effect.
  // Configures `auth_jwt_leeway` directive.
  // https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_leeway
  // Example: "auth_jwt_leeway 60s".
  // Default: 0s.
  //
  // +optional
  Leeway *Duration `json:"leeway,omitempty"`

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
  // URI is the JWKS endpoint.
  //
  //nolint:lll
  // +kubebuilder:validation:Pattern=`^https:\/\/[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(:[0-9]{1,5})?(\/[a-zA-Z0-9._~:\/?@!&'()*+,=-]*)?$`
  URI string `json:"uri"`
  // CACertificateRefs references a list of secrets containing trusted CA certificates
  // in PEM format used to verify the server certificate of the JWKS endpoint.
  // The referenced secrets must contain an entry with the key "ca.crt".
  // Only one secret can be referenced currently.
  // If not specified, the system CA bundle is used.
  //
  // Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_trusted_certificate
  //
  // +optional
  // +kubebuilder:validation:MaxItems=1
  CACertificateRefs []LocalObjectReference `json:"caCertificateRefs,omitempty"`
}

// RequireType defines how JWT Claims are validated.
// +kubebuilder:validation:Enum=All,Any
type RequireType string

const (
  // RequireTypeAll authorizes claims that satisfy all requirements.
  RequireTypeAll  RequireType  = "All"
  // RequireTypeAny authorizes claims that satisfy any requirement.
  RequireTypeAny  RequireType  = "Any"
)

// ClaimMatchType defines how claim values are parsed.
// +kubebuilder:validation:Enum=Exact,Regex
type ClaimMatchType string

const (
  // ClaimMatchTypeExact treats claim values as their exact value.
  ClaimMatchTypeExact ClaimMatchType = "Exact"
  // ClaimMatchTypeRegex treats claim values as a regex value.
  ClaimMatchTypeRegex ClaimMatchType = "Regex"
)

// Authorization specifies a set of required claim rules
// that a token's claim must match to be authorized, given the require type defined.
type Authorization struct {
  // Rules defines a list of claims and their specific authorization requirements.
  Rules []Rule `json:"rules,omitempty"`

  // Require sets top level authorization requirement.
  // When set to All, the requirements for all claims in a rule must be met.
  // When set to Any, the requirements for any one claim in a rule must be met.
  //
  // +optional
  // +kubebuilder:default=Any
  Require *RequireType `json:"require,omitempty"`
}

// Rule defines a list of claims, and authorization rules for those claims.
type Rule struct {
  // Claims defines a list of claims required by users.
  // +kubebuilder:validation:MinItems=1
  Claims []Claim `json:"claims,omitempty"`

  // Require sets the authorization mode for a specific claim within a rule.
  // When set to All, a token's claim must match all values within that claim.
  // When set to Any, a token's claim must match at least one value with that claim.
  //
  // +optional
  // +kubebuilder:default=Any
  Require *RequireType `json:"require,omitempty"`
}

// Claim describes the exact name/value pair of claims that must be matched.
type Claim struct {
  // Name is the name of the claim within the token.
  // +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_/-]+$`
  Name   string   `json:"name"`

  // Values are the values within the claim.
  // When more than one value is set, the claim must match any of these values.
  // +kubebuilder:validation:items:Pattern=`^[^\\n\\r;#\\$\\{\\}\\|&><'\"]+$`
  // +kubebuilder:validation:MinItems=1
  Values []string `json:"values"`

  // ProxySetHeader sets both the name and variable for `proxy_set_header`
  // Example: For claim name `sub` for JWT auth
  //
  // proxy_set_header X-JWT-Claim-Sub $jwt_claim_sub;
  // +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_/-]+$`
  ProxySetHeader *string `json:"proxySetHeader,omitempty"`

  // Match sets the match type for the claim.
  // +kubebuilder:default=Exact
  Match ClaimMatchType `json:"match,omitempty"`
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

## Basic Auth

### Proposed Spec for Basic Auth

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

### Secret creation and reference for Basic Auth

For Basic Auth, we will process a custom secret type of `nginx.org/htpasswd`.
This will allow us to be more confident that the user is providing us with the appropriate kind of secret for this use case.

To create this kind of secret for Basic Auth, first run this command:

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

Note: `auth` will be the default key for secrets referenced by `AuthenticationFilters` of `Type: Basic` and `Type: JWT`.

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

### Example HTTPRoute resource

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

### Generated NGINX configuration

For Basic Auth, NGF will store the file used by `auth_basic_user_file` in `/etc/nginx/secrets/`
The full path to the file will be `/etc/nginx/secrets/basic_auth_<secret-namespace>_<secret-name>`
In this case, the full path will be `/etc/nginx/secrets/basic_auth_default_basic-auth-user`

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
            auth_basic_user_file /etc/nginx/secrets/basic_auth_default_basic_auth_users;

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

## JWT Auth

### Proposed spec for JWT Auth

For JWT Auth, there are two options.

1. Local JWKS file stored as a Secret of type `nginx.org/jwt`
2. Remote JWKS from an external identity provider (IdP) such as Keycloak

#### Spec for local JWKS

This configuration will access the public JSON Web Key (JWK) from a Kubernetes secret.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    # JWK source. Local file or Remote JWKS
    source: File
    file:
      secretRef:
        name: jwt-keys-secure
```

#### Spec for remote JWKS

This configuration will access the public JSON Web Key Set (JWKS) from a remote server.
This could be a self-hosted server or a hosted identity provider (IdP).
The `remote.uri` must use HTTPS. To verify the JWKS endpoint's server certificate with a custom CA, users can optionally reference a Secret containing the CA certificate in PEM format (key `ca.crt`) via `remote.caCertificateRefs`. If omitted, the system CA bundle is used.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    # JWK source. Local file or Remote JWKS
    source: Remote
    remote:
      uri: https://issuer.example.com/.well-known/jwks.json
      # Optional: CA certificate for verifying the JWKS endpoint's TLS certificate.
      # If omitted, the system CA bundle is used.
      caCertificateRefs:
        - name: idp-ca-secret
```

The `remote.uri` must use HTTPS. To verify the JWKS endpoint's server certificate with a custom CA, reference a Secret containing the CA certificate in PEM format with the key `ca.crt` via `remote.caCertificateRefs`. If `caCertificateRefs` is omitted, the system CA bundle is used. Up to one CA certificate secret may be referenced.

### Secret creation and reference for JWT Auth

For JWT Auth, we will process a custom secret type of `nginx.org/jwt`.
This will allow us to be more confident that the user is providing us with the appropriate kind of secret for this use case.

Before creating the secret, you will need to generate a JSON Web Key Set (JWKS).

Below is an example of what a JWKS looks like:

```json
{
  "keys": [
    {
      "kty": "RSA",
      "alg": "RS256",
      "use": "sig",
      "kid": "my-key-id",
      "n": "$modulus",
      "e": "$exponent"
    }
  ]
}
```

To generate a JWKS, an RSA public key is required. For users attempting to create this secret for production use, they may need to have this key file generated by their system administrator.

Once the application developer has their public key, the following commands can be run to extract both the `modulus` and `exponent` values from the public key:

```bash
# Extract Modulus
modulus=$(openssl rsa -in public_key.pem -pubin -noout -modulus | cut -d'=' -f2 | tr -d '\n' | base64)

# Extract Exponent
exponent=$(openssl rsa -in public_key.pem -pubin -text -noout | grep 'Exponent' | awk '{print $2; exit}')

# Convert Exponent to Base64
exponent_base64=$(echo -n "$exponent" | xxd -r -p | base64)
```

For context, the `modulus` value extracted from the public key is used in both the encryption and decryption processes. In public-key encryption, it helps define the space within which encryption and decryption operations are performed.

The `exponent` determines how modular arithmetic is applied in the encryption process.

In order for the JWKS to verify a JWT provided by the user, both of these entries are required.

With the values of `modulus` and `exponent` saved, we can now create the file containing the JWKS.
In this case, we will name the file `auth` as this will be used later to create the JWT secret.

```bash
cat <<EOF > auth
{
  "keys": [
    {
      "kty": "RSA",
      "alg": "RS256",
      "use": "sig",
      "kid": "my-key-id",
      "n": "$modulus",
      "e": "$exponent_base64"
    }
  ]
}
EOF
```

You can then run this command to generate the secret from the `auth` file which contains your JSON Web Key Set (JWKS):

```bash
kubectl create secret generic jwt-keys-secure --type='nginx.org/jwt' --from-file=auth
```

Note: Similar to Basic Auth, `auth` is the required key for secrets referenced by `AuthenticationFilters` of `Type: JWT`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: jwt-keys-secure
type: nginx.org/jwt
data:
  auth: ewogICJrZXlzIjogWwogICAgewogICAgICAia3R5IjogIlJTQSIsCiAgICAgICJhbGciOiAiUlMyNTYiLAogICAgICAidXNlIjogInNpZyIsCiAgICAgICJraWQiOiAibXkta2V5LWlkIiwKICAgICAgIm4iOiAiUVVFek5FVkJOMFkyUmpORE4wWkZOelUxTjBKRU4wUkdNelUxTkVFelJqWkJRakl3TWtSRE16azVSVFF3TVRGRVEwRXdRa00yUmpoRU1qWTRNa1pHTURrME16QXhOekpETUVOQlFUUXpSa014TWtRMU5FTTVPVFV6UkRjM05UTTJSRUkyT0VVMU9EWXdRalUxTWtFMk4wUXlORUkwUmtRMk1qQTBORFJETTBJMk5FWkJNemhETmprM05qTkVRekl5T0RORVFVRTRORVE0T1VOR05rUTJNVGxDUmpCRU1USkNSamxCTVRFNVJEZzBNRGRET0RNMVEwRTBOa0ZCTWtNeU16VTBSRGs0TjBORE9VTkJRek5GTlRZMFEwUXdRVEJGUTBVd1FqZEZOakV4UVVGR05EaENSRVF6UkROQk0wSkJNa1F4TkVVM1F6RXpRVUpGUkVFNVJURkNNelk0T0RrMlEwTTBRemRDTkRZMk9URkRPRFpCUlRoRVFqazVNMEl4TlVWQ1FUZ3hNRFl5UWtRM09FUTNSRVZDTURrM09UQXhNa1JFTUVZMFFqVkZRVGxHUWpORk5EVkNORUUwT0VaR09UYzFPVVF6TTBFNU9FTTJNRE15UmpFMU9UY3lRekJHT0VOQlFUUkNNVEl3TkRJeE5VRkdOVUl3TVRJeVJESkRNVU0yTkVGR1JFVXpSVFl4TlVVd1FqSXdNMEZFTXpZNE5UUXpSVEJHTmpnMU9EQkNSRUk1TXpNMFFUUkRSREU1TWtZMU1EaEdOVVl3T1RBeVJVUkVOVVpFTVVKQk1VSkZSakJDTmtZMlJEVTJRalZDTWpjM1JEZENSRVJETmpaQk0wRkdOVEpDUVRFNVEwTXpPVFF6UVVZNFF6VTRNMEk1UmpNMFJFRTNRVVE9IiwKICAgICAgImUiOiAiWlZNPSIKICAgIH0KICBdCn0K
```

### Example HTTPRoute resource

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

### Generated NGINX configuration

Below are `two` potential NGINX configurations based on the source defined.

1. NGINX Config when using `source: File` (i.e. locally referenced JWKS key)

For JWT Auth, NGF will store the file used by `auth_jwt_key_file` in `/etc/nginx/secrets/`.
The full path to the file will be `/etc/nginx/secrets/jwt_auth_<secret-namespace>_<secret-name>`.
In this case, the full path will be `/etc/nginx/secrets/jwt_auth_default_jwt-keys-secure`.

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
            auth_jwt "Restricted";

            # File-based JWKS
            # Path is generated by NGF using the name and key from the secret
            auth_jwt_key_file /etc/nginx/secrets/jwt_auth_default_jwt-keys-secure;

            # Optional: key cache duration
            auth_jwt_key_cache 10m;

            # Required claims (enforced via maps above)
            auth_jwt_require $valid_jwt_iss;
            auth_jwt_require $valid_jwt_aud;

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

1. NGINX Config when using `source: Remote`

When using the `Remote` source, the `auth_jwt_key_request` directive is used in place of `auth_jwt_key_file`. This will call the `internal` NGINX location `/_ngf-internal-<namespace>_<name>_jwks_uri` to redirect the request to the external auth provider (e.g. Keycloak). In this example, the name will be `/_ngf-internal-default_api-jwt_jwks_uri`.
To improve the overall performance of remote requests, `auth_jwt_key_cache` can be specified to locally cache the JWKS received from the IdP. This prevents repeated calls to the IdP for a period of time.

Here is an example of what the NGINX configuration would look like:

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
            auth_jwt "Restricted";
            # Remote JWKS
            auth_jwt_key_request /_ngf-internal-default_api-jwt_jwks_uri;

            # Optional: key cache duration
            auth_jwt_key_cache 10m;

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
        location = /_ngf-internal-default_api-jwt_jwks_uri {
            internal;
            proxy_pass https://issuer.example.com/.well-known/jwks.json;
        }
    }
}
```

### Resolving remote IdP JWKS URI

For JWT Remote authentication, NGINX will require a [resolver](https://nginx.org/en/docs/http/ngx_http_core_module.html#resolver) to be defined with one or more resolver addresses.

Currently, the `NginxProxy` resource is the only way to define resolvers.
This will set the resolvers at the `http` context, which will affect all configurations that require a resolver to function.

Here is an example of an `NginxProxy` with an IPAddress resolver defined:

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: nginx-proxy
spec:
  dnsResolver:
    addresses:
    - type: IPAddress
      value: "8.8.8.8"
```

It will be the responsibility of the infrastructure provider to manage and configure these resolvers.
We will need to document this aspect of the JWT Remote use case.

### Cache specification

JWT key caching will be `disabled` by default.
This is to ensure that, by default, users don't encounter scenarios where stale keys are served.
To enable caching, users can set `keyCache` with the duration they wish the JWKS to be cached for:

```yaml
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    source: Remote
    remote:
      uri: https://issuer.example.com/.well-known/jwks.json
    keyCache: 10m
```

### JWT claims

A common use case for JWT authentication is to enforce fields within a JWT payload to be required.
These fields are referred to as claims.

This section will discuss JWT claims, as well as a proposed specification for configuring to enforce NGINX to require the presence of specific claims and/or values of those claims.

#### Understanding JWT claims

JWT claims are fields/attributes contained within a JSON Web Token. These handle authorization (AuthZ).

Here is an example of a JWT payload containing the standard registered claims outlined in [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519#page-9):

```json
{
  "iss": "https://issuer.example.com",
  "sub": "user-12345",
  "aud": ["api", "cli"],
  "exp": 1924992000,
  "nbf": 1737931200,
  "iat": 1737930900,
  "jti": "3f4c2f2a-2e02-4f7b-bb4b-0a1b2c3d4e5f",
}
```

The Subject (`sub`) claim typically contains details on the user such as their username.
The Audience (`aud`) claim identifies what access the claim is intended for. For example, this JWT is claiming to have API and CLI access.
The Issuer (`iss`) claim identifies who issued this token.
The Expiration Time (`exp`), Not Before (`nbf`) and Issued At (`iat`) claims help with the lifecycle of a token. They ensure requests using tokens outside these time constraints are rejected. The [auth_jwt_leeway](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_leeway) directive interacts with the `exp` and `nbf` claims. When these two claims are verified, this directive will set a maximum allowable leeway to compensate for [clock skew](https://en.wikipedia.org/wiki/Clock_skew).
The JWT ID (`jti`) claim is a unique identifier for the token.

NOTE: For the Audience (`aud`), as outlined in [RFC-7519 Section 4.1.3](https://datatracker.ietf.org/doc/html/rfc7519#section-4.1.3) the value is an array of case-sensitive strings, each containing a StringOrURI value. This results in the `aud` always being processed as an array by NGINX, even when containing a single value.

Users may also choose to set custom claims. Common ones are `email` and `name`.

```json
{
  // User defined claims
  "name": "john doe",
  "roles": ["reader", "admin"],
  "tenant": "acme-co",

  // Standard registered claims
  "iss": "https://issuer.example.com",
  "sub": "user-12345",
  "aud": "api",
  "exp": 1924992000,
  "nbf": 1737931200,
  "iat": 1737930900,
  "jti": "3f4c2f2a-2e02-4f7b-bb4b-0a1b2c3d4e5f",
}
```

User defined variables can each be accessed through the `$jwt_claim_` variable, where the name of the claim is appended to the end of the variable name.
For example, the `name` claim will be `$jwt_claim_name` with the value of `john doe`.

#### Understanding nested claims

It's possible that JWT payloads can contain nested claims. This is where certain, non-standard claims, like `roles` or `user`, are nested under other top-level claims.
Here is an example where the `roles` claim is nested under the new `realm_access` claim, and the `user` claim now contains the `tenant` claim as a nested claim:

```json
{
  "realm_access": {
    "roles": ["reader", "admin"]
  },
  "user": {
    "tenant": "acme-eu"
  },
}
```

Claims provide a means to enhance the security of JWT authentication and improve access control through requiring specific claims and claim values.

#### Understanding claim enforcement

NGINX defines the `auth_jwt_require` directive to handle JWT claim enforcement.
The two most common claims to enforce are issuer `iss`, and audience `aud`.

When NGINX successfully validates a token, the `iss`, `aud` and `sub` claims are automatically exposed as variables. `$jwt_claim_iss`, `$jwt_claim_aud` and `$jwt_claim_sub`.

There are two ways to enforce claims.

- The presence of the claim.

This is the simplest, and least recommended approach. By declaring `auth_jwt_require $jwt_claim_iss`, NGINX will check for the presence or absence of this claim.
If the claim is absent, NGINX returns a `401` error code. It will not validate the value of the claim.

- Validate claim values.

This approach provides a more secure and robust experience.

For this, we need to combine the `auth_jwt_require` directive with a `map`.
Using the `iss` claim as an example again, if we wanted to enforce the claim contains `https://issuer.example.com` as the claim value for `iss`, we can use a configuration like this:

```nginx
    map $jwt_claim_iss $valid_jwt_iss {
      "https://issuer.example.com" 1;
      default 0;
    }
    auth_jwt_require $valid_jwt_iss;
```

In this case, we use `auth_jwt_require $valid_jwt_iss` to validate the value returned from the map.

#### Processing claims

Claim validation can be defined for both `JWT` and `OIDC` auth types.
This section covers the proposed specification for `JWT` auth claim validation.
To understand the specifics of how claim validation works for `OIDC` auth in NGINX, see the [Claim validation](oidc.md#claim-validation) section of the OIDC auth proposal.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "ngf"
    source: File
    file:
      secretRef:
        name: jwks-secret
    # Top level authorization spec.
    authorization:
      require: Any # Top level requirement for all rules. Default: Any
      rules:
      # rule[0] requires all claims to match.
      - require: All
        claims:
          - name: "iss"
            values:
            - "https://issuer.example-1.com"
          - name: "aud"
            values:
            - "cloud"
            - "admin"
          - name: "tenant"
            values:
            - "acme-co"
            match: Exact # Default. Match values exactly.
            # Send value of $claim_tenant in X-Tenant header to upstream.
            proxySetHeader: X-Tenant
      # rule[1] requires any claim to match.
      - require: Any
        claims:
          - name: "iss"
            values:
            - "https://issuer.example-2.com"
            - "https://issuer.example-3.com"
          - name: "aud"
            values:
            - "cli"
          - name: "tenant"
            values:
            - "a(.*)ws"
            match: Regex # Allow values to contain regex patterns.
```

Here is a breakdown each component of this specification:
1. `spec.jwt.authorization`: This is the top level spec for the claim validation
2. `spec.jwt.authorization.require`: Defaults to `Any`. Can be set to `Any` or `All`. This defines the top level rule requirement for all claims. When set to `All`, all claims within `rules` must be satisfied for a request to be authorized. When set to `Any`, one or more claims may be satisfied within `rules` for the request to be authorized.
3. `spec.jwt.authorization.rules[]`: This defines the list of rules that a JWT claim must satisfy, depending on the `require` modes set.
4. `spec.jwt.authorization.rules[].claims[]`: This defines a list of `name/value(s)` pair, that relate to the key/value(s) pair of a JWT claim. If the `claims` array contains `name: "aud"` and `values: ["cloud"]`, NGINX will expect this claim `name/value(s)` to be present in the JWT claim presented.
5. `spec.jwt.authorization.rules[].claims[].match`: Defaults to `Exact`. Can be set to `Exact` or `Regex` This defines how NGINX will attempt to match the value(s) in a JWT claim. When set to `Exact`, NGINX will match to claim value(s) provided against the exact expected string defined in `claims[].values`. When set to `Regex`, the value in a `claims[].values` array may contain regex patterns. e.g. `name: "aud" values: ["a(.*)pi"]`.
6. `spec.jwt.authorization.rules[].claims[].proxySetHeader`: This configuration relates to a specific `name/values` within `claims`. When set, a `proxy_set_header` directive will be set for that claim within the route the `AuthenticationFilter` is referenced by. e.g. If we define `proxySetHeader: X-Tenant` for `name: "tenant": values: ["acme-co"]`, we get `proxy_set_header X-Tenant $claim_tenant`.

The `authorization` spec above will generate this NGINX configuration.
To help make this more digestible, the NGINX configuration contain multiple comment blocks for the fields they relate to.

```nginx
http {
    # For every `rules[].claims[].name` we define an `auth_jwt_claim_set` directive.
    # This is required by NGINX to parse array-type claims.
    # It's safer and more consistent to treat every `rule[].claim[].values`
    # as an array in NGINX, even if it contains a single value.
    auth_jwt_claim_set $claim_aud aud;
    auth_jwt_claim_set $claim_iss iss;
    auth_jwt_claim_set $claim_tenant tenant;

    #################################################
    #####       Map for `rules[0].claims`     #####
    #################################################
    map $claim_iss+$claim_aud+$claim_tenant $rule_0_all {
        ~^(?:.*,)?https://issuer\.example-1\.com(?:,.*)?\+(?:.*,)?(cloud|admin)(?:,.*)?\+(?:.*,)?acme-co(?:,.*)?$ 1;
        default 0;
    }
    #################################################


    #################################################
    #####      Maps for `rules[1].claims`     #####
    #################################################
    map $claim_iss $iss_rule_1 {
        ~(?:^|,)?(https://issuer\.example-2\.com|https://issuer\.example-3\.com)(?:,|$) 1;
        default 0;
    }

    map $claim_aud $aud_rule_1 {
        ~(?:^|,)cli(?:,|$) 1;
        default 0;
    }

    map $claim_tenant $tenant_rule_1 {
        ~(?:^|,)a(.*)ws(?:,|$) 1;
        default 0;
    }

    map $iss_rule_1$aud_rule_1$tenant_rule_1 $rule_1_any {
        ~1 1;
        default 0;
    }
    #################################################


    #################################################
    #####  Map for `authorization.require: Any` #####
    #################################################
    # When `authorization.require` is set to `Any`
    # This map configuration is used.
    map $rule_0_all$rule_1_any $require_any {
        ~1 1;
        default 0;
    }
    #################################################


    #################################################
    #####  Map for `authorization.require: All` #####
    #################################################
    # When `authorization.require` is set to `All`
    # This map configuration is used.
    map $rule_0_all$rule_1_any $require_all {
        11 1;
        default 0;
    }
    #################################################


    server {
        listen 80;

        # Requests to /require_all must match all rules.
        location /require_all {
            auth_jwt "ngf";
            auth_jwt_key_file /etc/nginx/secrets/jwt_auth_default_jwt-keys-secure;

            auth_jwt_require $require_all;

            proxy_set_header X-Tenant $claim_tenant;

            proxy_pass http://nginx-hello-backend;
        }

        # Requests to /require_any may match any one rule.
        location /require_any {
            auth_jwt "ngf";
            auth_jwt_key_file /etc/nginx/secrets/jwt_auth_default_jwt-keys-secure;

            auth_jwt_require $require_any;

            proxy_pass http://nginx-hello-backend;
        }
    }

    upstream nginx-hello-backend {
        server nginx-hello-service:8080;
    }
}
```

Here is break a down each component of this configuration, and why they are necessary:
1. `auth_jwt_claim_set $claim_aud aud`: NGINX requires this directive to parse a list of claims. This uses the `aud` claim as an example. If `aud` is configured as to have multiple `values`, NGINX will represent them as a comma separated string. e.g. `name: "aud": values: ["api", "cli"]`, becomes `"api,cli"`. Since we don't know if a user will set one or more `value`, it's safer and more consistent to use `auth_jwt_claim_set`, even if only a single value is present.
1. Map for `rules[0].claims[]`: This section represents the map required for these claims, and is configured with `require: All`. For this to work, all required values are represented as a single string in the map, separated by a plus (`+`) symbol. If a claim presented contains all required values, this map will return `1`.
**Note**: The symbol combining each claim can be any symbol. The plus (`+`) symbol was chosen, as it seemed to clearly delineate between claim values. It's important though that this symbol is escaped using `\+` to ensure it's treated as the exact symbol, and not used as part of a regular expression.
1. Maps for `rules[1].claims[]`: This section represents the map required for these claims, and is configured with `require: Any`. For this to work, each claim requires its own `map`, which will return `1` if a token's claim matches that value. The values returned from each of these maps are then evaluated together, to confirm if any one of these maps returned `1`.

```nginx
    map $iss_rule_1$aud_rule_1$tenant_rule_1 $rule_1_any {
        ~1 1;
        default 0;
    }
```

1. Map for `authorization.require: Any`: By default, `authorization.require` is set to `Any`. When set, the final results of each `rules[].claims[]` are evaluated together to see if any result contains a `1`

```nginx
    # Returns `1` if either `rules[0].claims[].require: All` OR `rules[1].claims[].require: Any` was satisfied.
    map $rule_0_all$rule_1_any $require_any {
        ~1 1;
        default 0;
    }
```

**Note**: There are several regex patterns surrounding each value in each map. These all server a very specific function. To make it easier to understand, we'll look at each pattern based on the map they are used in.

Regex patterns for `rules[1].claims`:

```regex
~(?:^|,)cli(?:,|$) 1;
```

The starting pattern `(?:^|,)` validates that the value "cli", is either at the start `^` of the string, or has a comma `,` at the start.
The ending pattern `(?:,|$)` does the opposite, and validates that the value "cli" has a comma `,` at the end, or is at the end of a string `$`.
This would match any one of these patterns:
- `api,cli,ops`
- `cli,api,ops`
- `api,ops,cli`

Regex patterns for `rules[0].claims`.

```regex
~^(?:.*,)?https://issuer\.example-1\.com(?:,.*)?\+(?:.*,)?(cloud|admin)(?:,.*)?\+(?:.*,)?acme-co(?:,.*)?$
```

The starting pattern for all values is `(?:.*,)?`:
- This is a non-capture group `?:`, meaning we don't store the results.
- We then match any character, and then a comma `.*,`. This captures the value within a comma separated list.
- Lastly, the question mark `?` outside the brackets will match the pattern in the brackets "zero or one times, as many times as possible". This allows the captured value to be both within a comma separated list, and at the start and end of that list.

The ending pattern for all vales is `(?:,.*)?`:
- This operates almost the same as the starting pattern. Instead of `.*,`, which matches any value **before** a comma, we use `,.*`. This allows us to match a comma first, and then any value **after** that comma.

The goal of these patterns is to allow these values to be found anywhere within a comma separated list, while also allowing them to be matched as a full string. This is why the regex symbols hat `^` and dollar sign `$`, to capture the start and end, are not part of the start and end patterns, like they are for the regex patterns for `rules[0].claims`.


#### Processing nested claims

The overall spec for nested claims will be similar to how standard claims are processed.
The main difference will be how NGINX expects them to be defined and processed.

Let's start with the JWT payload.
These are the claims we will process. This time `roles` is nested under `realm_access`:

```json
{
  // Standard registered claims
  "iss": "https://issuer.example.com",
  "sub": "user-12345",
  "aud": "api",
  "tenant": "acme-co",
  // User defined claim
  "email": "user@example.com",
  // User defined nested claim
  "realm_access": {
    "roles": ["reader", "admin"]
  },
}
```

This is what the `AuthenticationFilter` for this may look like:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "ngf"
    source: Remote
    remote:
      uri: https://issuer.example.com/.well-known/jwks.json
    authorization:
     rules:
      - claims:
        - name: "realm_access/roles" # Nested claim.
          values: # User defined list of roles.
          - "reader"
          - "admin"
```

To process the nested claim, the names of both the top-level and nested claim are specified as one string separated by a slash `/`.
It's important to note that [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519) does not explicitly define prohibited characters for JWT claim names.
Instead, it's advised that the value of a claim should avoid containing characters that are reserved for URIs, such as slash `/`.
Given this, it feels safe to assume that we can separate these by the slash character when parsing the claim.

For nested claims and claims including a dot (“.”), the value of the variable cannot be evaluated by NGINX.
To handle these, the [`auth_jwt_claim_set`](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_claim_set) directive should be used instead.

In the case of the nested claim `realm_access/roles` and `email`, this should be defined like this:

```nginx
auth_jwt_claim_set $claim_roles realm_access roles;
auth_jwt_claim_set $claim_email email;
```

This will set the value of `$claim_roles` to `["reader", "admin"]`, and the value of `$claim_email` to `user@example.com`.
Since the email contains a dot, this needs to be processed the same way.

### Route Attachment

Filters must be attached to a route resource (HTTPRoute, GRPCRoute, etc...) at the `rules.matches` level.
An `AuthenticationFilter` MAY be referenced by multiple rules.
An `AuthenticationFilter` MUST NOT be referenced more than once within a single rule.
This is expanded upon in the **Resource status** section.

#### Basic example

This example shows a single HTTPRoute, with a single `filter` defined in a `rule`

![reference-1](/docs/images/authentication-filter/reference-1.png)

### Resource status

#### Referencing multiple AuthenticationFilter resources in a single rule

Only one `AuthenticationFilter` may be specified per route rule, ensuring each route rule defines only one authentication method.

In a scenario where a route rule references multiple `AuthenticationFilter` resources, that route rule will be set to `Invalid`.
The route resource will display the `UnresolvedRefs` message to inform the user that the rule has been `Rejected`.

Here is an example of an HTTPRoute that references multiple `AuthenticationFilter` resources in a single rule.
In this scenario, the route rule for `/api` will be marked as `Invalid`.

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
        # Type: Basic
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: basic-auth-1
    - type: ExtensionRef
      extensionRef:
         # Type: Basic
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: basic-auth-2
    backendRefs:
    - name: backend
      port: 80
```

#### Referencing an AuthenticationFilter Resource that is Invalid

Note: With appropriate use of CEL validation, we are less likely to encounter a scenario where an `AuthenticationFilter` has been deployed to the cluster with an invalid configuration.
If this does happen, and a route rule references this `AuthenticationFilter`, the route rule will be set to `Invalid` and the route resource will display the `UnresolvedRef` status.

#### Attaching a JWT AuthenticationFilter to a Route When Using NGINX OSS

If a user attempts to attach a JWT type `AuthenticationFilter` while using NGINX OSS, the rule referencing the filter will be `Rejected`.

This can use the status `RouteConditionPartiallyInvalid` defined in the Gateway API here: https://github.com/nginx/nginx-gateway-fabric/blob/3934c5c8c60b5aea91be4337d63d4e1d8640baa8/internal/controller/state/conditions/conditions.go#L402

## Testing

- Unit tests
- Functional tests to validate behavioral scenarios when referencing filters in different combinations.

## Functional Test Cases

### Invalid AuthenticationFilter Scenarios

This section covers configuration deployment scenarios for an `AuthenticationFilter` resource that would be considered invalid.
These typically occur when the secret referenced by the `AuthenticationFilter` is misconfigured.
These invalid scenarios can occur for both `type: Basic` and `type: JWT`. For JWT, source should be `File` in these scenarios.

When an `AuthenticationFilter` is described as invalid, it could be for these reasons:

- An `AuthenticationFilter` referencing a secret that does not exist
- An `AuthenticationFilter` referencing a secret in a different namespace
- An `AuthenticationFilter` referencing a secret with an incorrect type (e.g., Opaque)
- An `AuthenticationFilter` referencing a secret with an incorrect key
- An `AuthenticationFilter` set to `type: JWT` where the NGINX dataplane is using NGINX OSS, and not NGINX Plus

### Valid Scenarios

This section covers deployment scenarios that are considered valid

Single route rule with a single path in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter`
- Expected outcomes:
  The route rule is marked as valid.
  Request to the path will return a 200 response when correctly authenticated.
  Request to the path will return a 401 response when incorrectly authenticated.

Single route rule with two or more paths in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter`
- Expected outcomes:
  The route rule is marked as valid.
  Requests to any path in the valid route rule return a 200 response when correctly authenticated.
  Requests to any path in the valid route rule return a 401 response when incorrectly authenticated.

Two or more route rules each with a single path in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter`
- Expected outcomes:
  All route rules are marked as valid.
  Request to a path in each route rule will return a 200 response when correctly authenticated.
  Request to a path in each route rule will return a 401 response when incorrectly authenticated.

Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter`
- Expected outcomes:
  All route rules are marked as valid.
  Requests to any path in the valid route rule return a 200 response when correctly authenticated.
  Requests to any path in the valid route rule return a 401 response when incorrectly authenticated.

A route rule with a single path in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter` set to `type: JWT` and `source: Remote` where the value of `remote.url` is a resolvable URL.
- Expected outcomes:
  The route rule referencing the `AuthenticationFilter` is marked as valid.
  Requests to any path in the valid route rule will return a 200 response with the JSON web key set (JWKS) to validate the original JWT signature from the authentication request.
  This behavior is documented in the [auth_jwt_key_request](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_request) directive documentation.

A route rule referencing multiple `AuthenticationFilters` where each `AuthenticationFilter` is of a unique `Type`. (e.g. one with `Type: Basic` and one with `Type: JWT`)
- Expected outcomes:
  The route rule referencing multiple `AuthenticationFilters` where each `AuthenticationFilter` is of a unique `Type` is marked as valid.
  Requests to any path in the valid route rule return a 200 response when correctly authenticated.
  Requests to any path in the valid route rule return a 401 response when incorrectly authenticated.

### Invalid scenarios

This section covers deployment scenarios that are considered invalid

Single route rule with a single path in an HTTPRoute/GRPCRoute referencing an invalid `AuthenticationFilter`
- Expected outcomes:
  The route rule is marked as invalid.
  Request to the path will return a 500 error.

Single route rule with two or more paths in an HTTPRoute/GRPCRoute where each route rule references an invalid `AuthenticationFilter`
- Expected outcomes:
  The route rules are marked as invalid.
  Requests to both paths will return a 500 error.

Two or more route rules each with a single path in an HTTPRoute/GRPCRoute referencing an invalid `AuthenticationFilter`
- Expected outcomes:
  Both route rules are marked as invalid.
  Requests to each path in each route rule will return a 500 error.

Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute referencing an invalid `AuthenticationFilter`
- Expected outcomes:
  Both route rules are marked as invalid.
  Requests to each path in each route rule will return a 500 error.


Two or more route rules each with a single path in an HTTPRoute/GRPCRoute, where one rule references a valid `AuthenticationFilter`, and the other references an invalid `AuthenticationFilter`
- Expected outcomes:
  The route rule referencing the invalid `AuthenticationFilter` is marked as invalid.
  Requests to the path in the invalid route rule will return a 500 error.
  The route rule referencing the valid `AuthenticationFilter` is marked as valid.
  Requests to the path in the valid route rule will return a 200 response when correctly authenticated.
  Requests to the path in the valid route rule will return a 401 response when incorrectly authenticated.


Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute where one rule references a valid `AuthenticationFilter`, and the other references an invalid `AuthenticationFilter`
- Expected outcomes:
  The route rules referencing the invalid `AuthenticationFilter` are marked as invalid.
  Requests to any path in the invalid route rule will return a 500 error.
  The route rules referencing the valid `AuthenticationFilter` are marked as valid.
  Requests to any path in the valid route rule will return a 200 response when correctly authenticated.
  Requests to any path in the valid route rule will return a 401 response when incorrectly authenticated.


Two or more `AuthenticationFilters` of the same `Type` referenced in a route rule.
- Expected outcomes:
  The route rule referencing multiple `AuthenticationFilters` of the same `Type` is marked as invalid.
  Requests to any path in the invalid route rule will return a 500 error.

A route rule with a single path in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter` set to `type: JWT` and `source: Remote` where the value of `remote.url` is an unresolvable URL.
- Expected outcomes:
  The route rule referencing the `AuthenticationFilter` is marked as valid.
  Requests to any path in the invalid route rule will return a 500 error.
  This behavior is documented in the [auth_jwt_key_request](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_request) directive documentation.

## Claim testing

This section assumes that the `AuthenticationFilter` attached to the route is valid.
It explicitly covers the expected outcomes for required claims.

`spec.jwt.authorization.require` set to `Any`
- Input:
  A JWT claim(s) that matches any combination of claim requirements defined in `spec.jwt.authorize.rule[].claims[]`.

- Expected outcome:
  Request is authorized.
  NGINX returns a 200 response code.

`spec.jwt.authorization.require` set to `All`
- Input:
  A JWT claim(s) that match all claim requirements defined in `spec.jwt.authorize.rule[].claims[]`.

- Expected outcome:
  Request is authorized.
  NGINX returns a 200 response code.

`spec.jwt.authorization.require` set to `Any`
- Input:
  A JWT claim(s) that match none of claim requirements defined in `spec.jwt.authorize.rule[].claims[]`.

- Expected outcome:
  Request is not authorized.
  NGINX returns a 401 response code.

`spec.jwt.authorization.require` set to `All`
- Input:
  A JWT claim(s) that match none of claim requirements defined in `spec.jwt.authorize.rule[].claims[]`.

- Expected outcome:
  Request is not authorized.
  NGINX returns a 401 response code.

`spec.jwt.authorization.require` set to `All`
- Input:
  A JWT claim(s) that match one or more, but not all, of the claim requirements defined in `spec.jwt.authorize.rule[].claims[]`.

- Expected outcome:
  Request is not authorized.
  NGINX returns a 401 response code.

## Security Considerations

### Basic Auth and Local JWKS

Basic Auth sends credentials in an Authorization header which is `base64` encoded.
JWT Auth requires users to provide a bearer token through the Authorization header.

Both methods can be easily intercepted over HTTP.

Users that attach an `AuthenticationFilter` to an HTTPRoute/GRPCRoute should be advised to enable HTTPS traffic at the Gateway level for the routes.

Any example configurations and deployments for the `AuthenticationFilter` should enable HTTPS at the Gateway level by default.

### Remote JWKS

Proxy cache TTL should be configurable and set to a reasonable default, reducing periods of stale cached JWKs.

### Key Rotation

Users should be advised to regularly rotate their JWKS keys in cases where they choose to reference a local JWKS via a `secretRef`.

### Optional Headers

Below is a list of optional defensive headers that users may choose to include.
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

When defining `name` and `values` fields for `spec.jwt.authorization.rules[].claims[]`, we need to ensure potentially malicious NGINX configuration cannot be injected, since these fields are used directly in the NGINX config,

We can do this at the API level, by ensuring `claims[].name` and `claims[].values[]` contain a regex allow pattern.

For `claims[].name`, we use `^[a-zA-Z0-9_\/-]+$`. This allows uppercase and lowercase letters (A-Z, a-z), Digits (0-9), underscores (`_`), hyphens (`-`), and slashes (`/`) for nested claims. This will prevent special NGINX characters like curly brackets, semicolons, and escape characters from being entered. i.e. block anything that does **not** match the pattern.

For `claims[].values[]`, we use `^[^\n\r;#\$\\{\\}\\|&><'"]+$`. This regex is designed to match any character **not** in found in the pattern. Blocks multi-line values and config injection (`\n`, `\r`), semicolons (`;`), hashes (`#`), dollar sign (`$`), curly braces (`{`, `}`), pipes (`|`), ampersands (`&`), angle brackets (`>`, `<`), single and double quotes (`'` & `"`).

## Alternatives

The Gateway API defines a means to standardize authentication through use of the [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter) available in the HTTPRoute specification.

This allows users to reference an external authentication service, such as Keycloak, to handle the authentication requests.
While this API is available in the experimental channel, it is subject to change.

Our decision to go forward with our own `AuthenticationFilter` was to ensure we could quickly provide authentication to our users while allowing us to closely monitor progress of the ExternalAuthFilter.

It is certainly possible for us to provide an External Authentication Service that leverages NGINX and is something we can further investigate as the API progresses.

## Additional Considerations

### Documenting Filter Behavior

In regard to documentation of filter behavior with the `AuthenticationFilter`, the Gateway API documentation on filters states the following:

```text
Wherever possible, implementations SHOULD implement filters in the order they are specified.

Implementations MAY choose to implement this ordering strictly, rejecting
any combination or order of filters that cannot be supported.
If implementations choose a strict interpretation of filter ordering, they MUST clearly
document that behavior.
```

## Future updates

### Multiple authentication methods

NGINX allows multiple authentication methods such as `auth_basic` and `auth_jwt` to be defined together.
By default, NGINX requires `all` specified authentication methods to be satisfied for a request to be considered authenticated.
This behavior is defined by the [satisfy](https://nginx.org/en/docs/http/ngx_http_core_module.html#satisfy) directive, which defaults to `all`.

Although NGINX allows this, it is not a goal of this proposal to enable multiple authentication mechanisms per route rule.

If we were to update the API to support this use case, the following changes would need to happen:

1. Allow each authentication method (i.e. `basic`, `jwt` and `oidc`) to be specified in a single `AuthenticationFilter` resource.
2. Removal of the `Type` field.
3. Ability to configure the [satisfy](https://nginx.org/en/docs/http/ngx_http_core_module.html#satisfy) directive.

Below is an example of what this new API would look like:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: basic-and-jwt-auth
spec:
  satisfy: any # all | any. Defaults to all.
  basic: # No need to specify Type
    secretRef:
      name: basic-auth-users
    realm: "Basic Restricted"
  jwt: # JWT defined alongside basic
    source: File
    file:
      secretRef:
        name: jwks-secret
```

### Custom Authentication Failure Response

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
      auth_basic_user_file /etc/nginx/secrets/basic_auth_default_basic_auth_users;

      # Calls named location
      error_page 401 = @basic_auth_failure;
    }

    location @basic_auth_failure {
        add_header WWW-Authenticate 'Basic realm="Restricted"' always;
        return 401 'Unauthorized';
    }
}
```

If we support this configuration, 3xx response codes should not be allowed and `AuthenticationFilter.onFailure` must not support redirect targets. This is to prevent open-redirect abuse.

We should only allow 401 and 403 response codes.

### Cross-Namespace Access

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

### JWT auth types

NGINX provides a directive called `auth_jwt_type`, which can be set to `signed` (default), `encrypted` or `nested`
This document proposes initially supporting only `signed`, as both `encrypted` and `nested` types require the Gateway to have access to private keys to decrypt the JWKS.

#### Use case for encrypted and nested

In scenarios where tokens pass through components such as a CDN, WAF, or a shared gateway, encrypted and nested tokens will not expose their claim contents.
JWEs (Encrypted JSON Web Token) will keep their claims unreadable to anyone without the private key.

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
      source: Remote
      remote:
        uri: https://issuer.example.com/.well-known/jwks.json
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
  // Source selects the token source.
  // +kubebuilder:default=Header
  Type TokenSourceType `json:"source"`

  // TokenName is the cookie or query parameter name when Source=Cookie or Source=QueryArg.
  // Ignored when Source=Header.
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
