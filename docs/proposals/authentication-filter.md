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
      - Understanding nested claim
      - Understand claim enforcement
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
// attached using as a filter via ExtensionRef.
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

// JWTKeyMode selects where JWT keys come from.
// +kubebuilder:validation:Enum=File;Remote
type JWTKeyMode string

const (
  // JWTKeyModeFile configures JWT to fetch JWKS from a local secret.
  JWTKeyModeFile   JWTKeyMode = "File"
  // JWTKeyModeRemote configures JWT to fetch JWKS from a remote source.
  JWTKeyModeRemote JWTKeyMode = "Remote"
)

// JWTAuth configures JWT-based authentication (NGINX Plus).
// +kubebuilder:validation:XValidation:message="mode File requires spec.file to be set.",rule="self.mode != 'File' || has(self.file)"
// +kubebuilder:validation:XValidation:message="mode Remote requires spec.remote to be set.",rule="self.mode != 'Remote' || has(self.remote)"
//
//nolint:lll
type JWTAuth struct {
  // File specifies local JWKS configuration.
  // Required when Mode == File.
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
  // Required when Mode == Remote.
  //
  // +optional
  Remote *RemoteKeySource `json:"remote,omitempty"`

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

  // Leeway is the acceptable clock skew for exp & nbf claims.
  // If exp & nbf claims are not defined, this directive takes no affect.
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

  // Mode selects how JWT keys are provided: local file or remote JWKS.
  Mode JWTKeyMode `json:"mode"`
}

// JWTFileKeySource specifies local JWKS key configuration.
type JWTFileKeySource struct {
  // SecretRef references a Secret containing the JWKS.
  SecretRef LocalObjectReference `json:"secretRef"`
}

 // RemoteKeySource specifies remote JWKS configuration.
type RemoteKeySource struct {
  // URI is the JWKS endpoint, e.g. "https://issuer.example.com/.well-known/jwks.json".
  URI string `json:"url"`

  // TLS defines HTTPS client parameters for retrieving JWKS.
  //
  // +optional
  TLS *RemoteTLSConfig `json:"tls,omitempty"`
}

// JWTRequiredClaims specifies exact-match requirements for JWT claims.
type JWTRequiredClaims struct {
  // Issuer (iss) required exact value.
  //
  // +optional
  Iss []string `json:"iss,omitempty"`

  // Audience (aud) required exact value.
  //
  // +optional
  Aud []string `json:"aud,omitempty"`

  // Subject (sub) required exact value
  //
  // +optional
  Sub []string `json:"sub,omitempty"`

  // User defined custom claims
  //
  // +optional
  Claims []JWTCustomClaim `json:"claims,omitempty"`
}

// JWTCustomClaim specifies custom user claims and values.
// +kubebuilder:validation:XValidation:message="exactly one of value or values must be set",rule="has(self.value) != has(self.values)"
// +kubebuilder:validation:XValidation:message="value must be non-empty when set",rule="!has(self.value) || size(self.value) > 0"
// +kubebuilder:validation:XValidation:message="values must be non-empty when set",rule="!has(self.values) || size(self.values) > 0"
type JWTCustomClaim struct {
  Name   string   `json:"name"`
  // Exactly one of Value or Values must be set.
  // +optional
  Value  *string  `json:"value,omitempty"`
  // +optional
  Values []string `json:"values,omitempty"`
}

// RemoteTLSConfig defines TLS settings for remote JWKS retrieval.
type RemoteTLSConfig struct {
  // SecretRef references a Secret containing client TLS cert and key.
  // Expectes secret type kubernetes.io/tls.
  //
  // +optional
  SecretRef *gatewayv1.SecretObjectReference `json:"secretRef,omitempty"`


  // Verify controls server certificate verification.
  //
  // +optional
  // +kubebuilder:default=true
  Verify *bool `json:"verify,omitempty"`
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

This mode will access the public JSON Web Key (JWK) from a Kubernetes secret.

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
        name: jwt-keys-securey
```

#### Spec for remote JWKS

This mode will access the public JSON Web Key Set (JWKS) from a remote server.
This could be a self-hosted server or a hosted identity provider (IdP).
To ensure a secure connection can be established to the remote JWKS URI, the `remote.tls` will allow users to define a secret of type `kubernetes.io/tls` with the TLS cert and key of their IdP.

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
      uri: https://issuer.example.com/.well-known/jwks.json
      tls:
        secretRef:
          name: cafe-secret
        verify: true # Defaults to true
        sni: true # Defaults to true
        sniName: foo.bar.com # Defaults to server name in proxy_pass
```

Optionally, users can also toggle and configure SNI capabilities through `remote.tls.sni` and `remote.tls.sniName`.
SNI will be enabled by default using the (proxy_ssl_server_name)[https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_server_name] directive.
By default, NGINX will use the server name defined in the `proxy_pass` when `proxy_ssl_server_name` is on.
Users can optionally set a specific host using `remote.tls.sniName`, which will configure the (proxy_ssl_name)[https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_name] directive

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

Below are `two` potential NGINX configurations based on the mode used.

1. NGINX Config when using `Mode: File` (i.e. locally referenced JWKS key)

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

1. NGINX Config when using `Mode: Remote`

When using the `Remote` mode, the `auth_jwt_key_request` directive is used in place of `auth_jwt_key_file`. This will call the `internal` NGINX location `/_ngf-internal-<namespace>_<name>_jwks_uri` to redirect the request to the external auth provider (e.g. Keycloak), In this example, the name will be `/_ngf-internal-default_api-jwt_jwks_uri`.
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

For JWT Remote authentication, NGINX will require a [resolver](https://nginx.org/en/docs/http/ngx_http_core_module.html#resolver) to be defined with one more resolver addresses.

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
To enable caching, users can set `keyCache` with the duration they wish the JWKS to be cached form:

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
The Expiration Time (`exp`), Not Before (`nbf`) and Issued At (`iat`) claims help with the lifecycle of a token. They ensure requests using tokens outside these time constrains are rejected. The (auth_jwt_leeway)[https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_leeway] directive interacts with the `exp` and `nbf` claims. When these two claims are verified, this directive will set a maximum allowable leeway to compensate for (clock skew)[https://en.wikipedia.org/wiki/Clock_skew].
The JWT ID (`jti`) claim is a unique identifier for the token.

NOTE: Both the Audience (`aud`) and Issuer (`iss`) claims in a JWT payload can be either a single string or an array. They will only ever be an array if it contains more than one value.

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

User defined variables can each be accessed through the `$jwt_claim_` variable, where the name the of the claim is appended to the end of the variable name.
For example, the `user` claim will be `$jwt_claim_used` with the value of `john doe`.

#### Understanding nested claims

It's possible that JWT payloads can contain nested claims. This is there certain, non-standard claims, like `roles` or `user`, are nested under other top-level claims.
Here is an example where the `roles`, claims is nested under the new `realm_access` claim, and the `user` claim now contains the `tenant` claim as a nested claim:

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

#### Understand claim enforcement

NGINX defines the `auth_jwt_require` directive to handle JWT claim enforcement.
The two most common claims to enforce as issuer `iss`, and audience `aud`.

When NGINX successfully validates a token, the `iss`, `aud` and `sub` claims are automatically exposed as variables. `$jwt_claim_iss`, `$jwt_claim_aud` and `$jwt_claim_sub`.

There are two ways to enforce claims.

1. The presence of the claim.

This is the simplest, and least recommended approach. By declaring `auth_jwt_require $jwt_claim_iss`, NGINX will check for the presence or absence of this claim.
If the claim is absent, NGINX throws an error. It will not validate the value of the claim.

2. Validate claim values.

This approach is provides a more secure and robust experience.

Let's say a user wants to enforce a token to contain one of two issuers, `https://issuer.example.com` or `https://issuer.example1.com`. Let's also say the value of audience can be either `api` or `cli`.

For NGINX to manage this, a map must be defined to check the values stored in `$jwt_claim_iss`, and `$jwt_claim_aud`, returning a 1 if there is a match, or a 0.

This map would look like this:

```nginx
http{
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
}
```

We would then set `$valid_jwt_iss` and `$valid_jwt_aud` as required claims within the location:

```nginx
http {
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

  location /api {
    # Other NGINX fields..

    auth_jwt_require $valid_jwt_iss;
    auth_jwt_require $valid_jwt_aud;


    proxy_pass ...
  }
}
```

#### Processing claims

This section will cover the proposed specification for JWT claim enforcement, as well as nested claims.
Claims can be required for both `File` and `Remote` modes.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    mode: Remote
      remote:
        uri: https://issuer.example.com/.well-known/jwks.json
      require:
        iss:
         - "https://issuer.example.com" # List with single value.
        aud:
         - "api"
         - "cli"
        sub: "user-12345"
        claims:
        - name: "tenant" # Set `auth_jwt_require $jwt_claim_tenant;`
          value: "acme-co"
        - name: "roles"
          values: # User defined list of roles.
          - "reader"
          - "admin"
```

This spec is configured to process a JWT payload with these claims:
```json
{
  // Standard registered claims
  "iss": "https://issuer.example.com",
  "aud": ["api", "cli"],
  "sub": "user-12345",
  // User defined claims
  "tenant": "acme-co",
  "roles": ["reader", "admin"],
}
```

#### Processing nested claims

The overall spec for nested claims will be similar to how standard claims are processed.
The main difference will be how NGINX expected them to be defined and processed.

Let's start with the JWT payload this time.
These are the claims we will process. This time `roles` is nested under `realm_access`:
```json
{
  // Standard registered claims
  "iss": "https://issuer.example.com",
  "aud": ["api", "cli"],
  "sub": "user-12345",
  // User defined claims
  "email": "user@example.com",
  "realm_access": {
    "roles": ["reader", "admin"]
  },
}
```

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: jwt-auth
spec:
  type: JWT
  jwt:
    realm: "Restricted"
    mode: Remote
      remote:
        uri: https://issuer.example.com/.well-known/jwks.json
      require:
        claims:
        - name: "realm_access/roles"
          values: # User defined list of roles.
          - "reader"
          - "admin"
        - name: "email"
          value: "user@example.com"
```

To process the nested claim, the names of bot the top-level and nested claim are specified as one string separate by a slash `/`.
It's important to note that [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519) does not explicitly define prohibited characters for JWT claim names.
Instead, it's advised to avoid characters that are reserved in URI such as slash `/`.
Given this, it feels safe to assume that we can separate these by the slash character when parsing the claim.

For nested claims and claims including a dot (“.”), the value of the variable cannot be evaluated by NGINX.
To handle these, the [`auth_jwt_claim_set`](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_claim_set) directive should be used instead.

In the case of the nested claim `realm_access/roles`, and `email` this should be defined like this:

```nginx
auth_jwt_claim_set $roles realm_access roles;
auth_jwt_claim_set $email email;
```

This will set the value of `$roles` to `["reader", "admin"]`, and the value of `$email` set to `user@example.com`.
Since the email contains a dot, this needs to be processed the same way.

### JWT Authentication Capabilities

The table below summarizes the capabilities enabled by the current JWT Authentication proposal.

| Capability | API fields | NGINX directive | Notes |
| --- | --- | --- | --- |
| Enable JWT authentication and set realm | `spec.type = "JWT"`; `spec.jwt.realm` | `auth_jwt "<realm>"` | Currently does not expose defining `token` |
| Provide JWT keys from local JWKS (Secret) | `spec.jwt.mode = "File"`; `spec.jwt.file.secretRef.name`; Secret type `nginx.org/jwt`; data key `auth` | `auth_jwt_key_file /etc/nginx/secrets/jwt_auth_<namespace>_<secret-name>` | Secret must exist in same namespace and must be of type `nginx.org/jwt` |
| Secret handling/validation for local JWKS | Secret type `nginx.org/jwt`; data key `auth`; `LocalObjectReference` | Validates presence/type/key; NGF loads JWKS into key file | Cross-namespace secrets not supported initially; future work may add `ReferenceGrant`-based access |
| Provide JWT keys from remote JWKS | `spec.jwt.mode = "Remote"`; `spec.jwt.remote.uri`; `spec.jwt.remote.tls.secretRef` (type `kubernetes.io/tls`); `spec.jwt.remote.tls.verify` (default `true`); `spec.jwt.remote.tls.sni` (default `true`); `spec.jwt.remote.tls.sniName` (optional; default to server name in `proxy_pass`) | `auth_jwt_key_request /_ngf-internal-<namespace>_<filter-name>_jwks_uri`; internal location `proxy_pass` to remote JWKS; optional client TLS. | Requires DNS resolver via `NginxProxy.spec.dnsResolver`; `verify` controls server cert verification; key caching optional |
| Configure DNS resolver for remote JWKS | `NginxProxy.spec.dnsResolver.addresses` (separate resource) | `resolver` set at `http` context for name resolution used by `auth_jwt_key_request` | Required for remote JWKS URIs; managed outside the filter |
| Configure JWT key cache duration | `spec.jwt.keyCache` (Duration) | `auth_jwt_key_cache <duration>` | Disabled by default to avoid stale keys |
| Configure acceptable clock skew for `exp`/`nbf` | `spec.jwt.leeway` (Duration) | `auth_jwt_leeway <duration>` | Applies only if `exp`/`nbf` claims are present; default `0s` |
| Require exact-match issuer (`iss`) values | `spec.jwt.require.iss: []string` | `map $jwt_claim_iss $valid_jwt_iss { ... }`; `auth_jwt_require $valid_jwt_iss` | Supports multiple allowed issuers; `iss` may be string or array in a JWT claim |
| Require exact-match audience (`aud`) values | `spec.jwt.require.aud: []string` | `map $jwt_claim_aud $valid_jwt_aud { ... }`; `auth_jwt_require $valid_jwt_aud` | Supports single or multiple audiences; `aud` may be string or array in a JWT claim |
| Require exact-match subject (`sub`) values | `spec.jwt.require.sub: []string` | `map $jwt_claim_sub $valid_jwt_sub { ... }`; `auth_jwt_require $valid_jwt_sub` | Multiple allowed subjects supported |
| Require exact-match custom claim values | `spec.jwt.require.claims[]`: `{ name, value \| values }` | For flat claims: `map $jwt_claim_<name> $valid_<name> { ... }`; `auth_jwt_require $valid_<name>` | Exactly one of `value` or `values` must be set and non-empty; enforces presence and allowed values |
| Require exact-match nested/dotted custom claims | `spec.jwt.require.claims[].name` accepts path like `parent/child` | `auth_jwt_claim_set $var parent child`; `map $var $valid_var { ... }`; `auth_jwt_require $valid_var` | Use `auth_jwt_claim_set` for nested or dotted claims; slash-separated path identifies nested segments |


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

NGINX allows multiple authentication methods such as `auth_basic` and `auth_jwt` to be defined together.
In a scenario where a user provides a JWT token for authentication, NGINX will first validate this against the `auth_basic` module.
If that authentication method fails, NGINX will then validate the request against the `auth_jwt` module.

NGINX does allow multiple of the same auth module defined.
However, NGINX will only resolve **one** of them.

In the example NGINX code below, where multiple `auth_basic` directives are defined, NGINX will only resolve the last one.
In this case, `auth_basic "Restricted Area 2";` will be used.

```nginx
location /path1 {
    # These directives are ignored
    auth_basic "Restricted Area 1";
    auth_basic_user_file /etc/nginx/.htpasswd1;

    # These directives are used
    auth_basic "Restricted Area 2";
    auth_basic_user_file /etc/nginx/.htpasswd2;
}
```

To ensure we avoid this scenario, only one `AuthenticationFilter` of the same `Type` may be referenced in a single rule.

In a scenario where a route rule references multiple `AuthenticationFilter` resources of the same `Type`, that route rule will set to `Invalid`.
The route resource will display the `UnresolvedRefs` message to inform the user that the rule has been `Rejected`.

Here is an example of an HTTPRoute that references multiple `AuthenticationFilter` resources of the same `Type` in a single rule.
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

Below is an example of a valid HTTPRoute that references multiple `AuthenticationFilter` resources of a different `Type` in a single rule.

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
        name: basic-auth
    - type: ExtensionRef
      extensionRef:
        # Type: JWT
        group: gateway.nginx.org
        kind: AuthenticationFilter
        name: jwt-auth
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
These invalid scenarios can occur for both `type: Basic` and `type: JWT`. For JWT, mode should be `File` in these scenarios.

When an `AuthenticationFilter` is described as invalid, it could be for these reasons:

- An `AuthenticationFilter` referencing a secret that does not exist
- An `AuthenticationFilter` referencing a secret in a different namespace
- An `AuthenticationFilter` referencing a secret with an incorrect type (e.g., Opaque)
- An `AuthenticationFilter` referencing a secret with an incorrect key
- An `AuthenticationFilter` set to `type: JWT` where there NGINX dataplane is using NGINX OSS, and not NGINX Plus

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

A route rule with a single path in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter` set to `type: JWT` and `mode: Remote` where the value of `remote.url` is a resolvable URL.
- Expected outcomes:
  The route rule referencing the `AuthenticationFilter` is marked as valid.
  Requests to any path in the invalid route rule will return a 200 response with the JSON web key set (JWKS) to validate the original JWT signature from the authentication request.
  This behavior is documented in the [auth_jwt_key_request](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_request) directive documentation.

A route rule referencing multiple `AuthenticationFilters` where each `AuthenticationFilters` is of a unique `Type`. (e.g. one with `Type: Basic` and one with `Type: JWT`)
- Expected outcomes:
  The route rule referencing multiple `AuthenticationFilters` where each `AuthenticationFilters` is of a unique `Type` is marked as valid.
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
  Requests to both paths in will return a 500 error.

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
  The route rule referencing the invalid `AuthentiationFilter` is marked as invalid.
  Requests to the path in the invalid route rule will return a 500 error.
  The route rule referencing the valid `AuthenticationFilter` is marked as valid.
  Requests to the path in the valid route rule will return a 200 response when correctly authenticated.
  Requests to the path in the valid route rule will return a 401 response when incorrectly authenticated.


Two or more route rules each with two or more paths in an HTTPRoute/GRPCRoute where one rule references a valid `AuthenticationFilter`, and the other references an invalid `AuthenticationFilter`
- Expected outcomes:
  The route rules referencing the invalid `AuthentiationFilter` is marked as invalid.
  Requests to any path in the invalid route rule will return a 500 error.
  The route rules referencing the valid `AuthenticationFilter` is marked as valid.
  Requests to any path in the valid route rule will return a 200 response when correctly authenticated.
  Requests to any path in the valid route rule will return a 401 response when incorrectly authenticated.


Two or more `AuthenticationFilters` of the same `Type` referenced in a route rule.
- Expected outcomes:
  The route rule referencing multiple `AuthenticationFilters` of the same `Type` is marked as invalid.
  Requests to any path in the invalid route rule will return a 500 error.

A route rule with a single path in an HTTPRoute/GRPCRoute referencing a valid `AuthenticationFilter` set to `type: JWT` and `mode: Remote` where the value of `remote.url` is an unresolvable URL.
- Expected outcomes:
  The route rule referencing the `AuthenticationFilter` is marked as valid.
  Requests to any path in the invalid route rule will return a 500 error.
  This behavior is documented in the [auth_jwt_key_request](https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html#auth_jwt_key_request) directive documentation.

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

## Future Updates

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
This document proposes initially supporting only `signed`, as both `encrypted` and `nested` types requires the Gateway to have access to private keys to decrypt the JWKS.

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
      mode: Remote
      remote:
        uri: https://issuer.example.com/.well-known/jwks.json

    # Required claims (exact matching done via maps in NGINX; see config)
    require:
      iss:
        - "https://issuer.example.com"
        - "https://issuer-2.example.com"
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
