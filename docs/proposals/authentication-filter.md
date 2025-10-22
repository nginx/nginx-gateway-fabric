# Enhancement Proposal-4052: Authentiation Filter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Provisional

## Preface

The Gatewy API currently has an `implementable` HTTP Auth mechanism exposed using the `ExternalAuth` field in the `HTTPRoute` resource. This uses Envoy's `ext_authz` protocol to reach out to an External Service to both `Authenticate` and `Authorize` requests.

See https://gateway-api.sigs.k8s.io/geps/gep-1494/ for more details

This GEP also describes a [two phased approach](https://gateway-api.sigs.k8s.io/geps/gep-1494/#why-two-phases), which includes starting first with a Filter at the `rule` level, and then providing a top level Policy attachment that can be overridden by lower level Auth Filters.

From initial discussion, we decided to go forward with exposing our own authentication filter.

This was decided for the following reasons:
- Given the timeline of `ExternalAuth` eventually being supported and implented by all other Gateway API implementations, it may be many months before this enchancement is available on the main channel within the Gateway API
- Exposing our own form of authentication through an authenticaiton filter does not exclude the possibility of eventually supporting the `ExternalAuth` field
- Given the expressed complexity of the [two phased approach](https://gateway-api.sigs.k8s.io/geps/gep-1494/#why-two-phases), `ExternalAuth` has the potential to go through may iterations before becoming stable
- Recent conversations with the Gateway API maintainers suggest that implementation of `ExternalAuth` is still speculative

## Summary

Design and implement a means for users of NGINX Gateway Fabric to enable authenticaiton on requests to their backend applications.
This new filter should eventually expose all forms of authentication avaialbe through NGINX, both Open Source and Plus.

## Goals

- Design a means of configuring authenticaiton for NGF
- Determine initial resource specification
- Evaluate filter early in request processing, occurring before URLRewrite, header modifiers and backend selection
- Authentication failures return appropriate status by default (e.g., 401/403)
- Ensure response codes are configurable


## Introduction

This document focus expliclty on Authentiaction (AuthN) and not Authorization (AuthZ). Authentiaction (AuthN) defines the verification of identiy. It asks the question, "Who are you?". This is different from Authorization (AuthZ), which preceeds Authentication. It asks the question, "What are you allowed to do".

This document also focus on Basic Authentication. Other authentication methods such as JWT and OAuth are mentioned, but are not part of the CRD design. These will be covered in future design and implementation tasks.


## Use Cases

- As an Application Developer, I want to secure access to my APIs and Backend Applications.
- As an Application Developer, I want to enforce authenticaiton on specific routes and matches.

### Understanding NGINX authentication methods

| **Authentication Method**     | **OSS**      | **Plus** | **NGINX Module**                | **Details**                                                        |
|-------------------------------|--------------|----------------|----------------------------------|--------------------------------------------------------------------|
| **HTTP Basic Authentication** | ✅           | ✅             | [ngx_http_auth_basic](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html) | Requires a username and password sent in an HTTP header.           |
| **JWT (JSON Web Token)**     | ❌           | ✅             | [ngx_http_auth_jwt_module](https://docs.nginx.com/nginx/admin-guide/security-controls/authentication/#jwt-authentication) | Tokens are used for stateless authentication between client and server. |
| **OpenID Connect**            | ❌           | ✅             | [ngx_http_oidc_module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)| Allows authentication through third-party providers like Google.   |

## Design and Proposal

When designing a means of configuring authentication for NGF, we can consider these approaches:
1. An `AuthenticationFilter` CRD which is responsible for providing a specification for each form of authentication within a single resource.
2. Indvidual CRDs responsbile for each authentication method. e.g. `BasicAuthFilter`, `JWTAuthFIlter`, etc...

This document will cover examples of both, including the pros and cons of each.

### 1. Single AuthenticationFilter

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AuthenticationFilter
metadata:
  name: basic-auth
spec:
  type: Basic
  basic:
    secret: basic-auth-users # Secret containing htpasswd data
    key: htpasswd            # key within the Secret
    realm: "Restricted"      # Optional. Helps with logging
    onFailure:               # Optional. These setting may be defaults.
      statusCode: 401
      wwwAuthenticate: 'Basic realm="Restricted"'
      responseBody: 'Unauthorized'
```

| **Pros**     | **Cons**      |
|-------------------------------|--------------|
| Single Resource to manage    | Resource updates may be difficult |
|                              | May require lots of internal logic |

### 2. Individual Filter for each Auth method

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: BasicAuthFilter
metadata:
  name: basic-auth
spec:
  secret: basic-auth-users # Secret containing htpasswd data
  key: htpasswd            # key within the Secret
  realm: "Restricted"      # Optional. Helps with logging
  onFailure:               # Optional. These setting may be defaults.
    statusCode: 401
    wwwAuthenticate: 'Basic realm="Restricted"'
    responseBody: 'Unauthorized'
```

| **Pros**     | **Cons**      |
|-------------------------------|--------------|
| Versioning per-resource is much easier    | Multiple resources to manage |
| Easier to map to `graph` in go code    | |

### Example integration

Secret referenced by Filter

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

HTTPRoute that will reference this filter

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-basic
  namespace: default
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
        kind: BasicAuthFilter
        name: basic-auth
    backendRefs:
    - name: backend
      port: 80
```

Generated NGINX config

```nginx
# ------------------------------------------
# http context
# ------------------------------------------
http {
    # NGF-managed upstream populated from Kubernetes Endpoints
    upstream backend_default {
        server 10.0.0.10:80;
        server 10.0.0.11:80;
    }

    server {
        listen 80;
        server_name api.example.com;

        # ------------------------------------------
        # Location generated for HTTPRoute rule: PathPrefix /v2
        # ------------------------------------------
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
            return 401 'Unauthorized';
        }
    }
}
```

## Reference Flow Diagram

![reference-1](/docs/images/authentication-filter/basic-auth-filter/reference-1.png)

## Path Type Behaviour

The `auth_basic` directive can be applied at the `http`, `server` and `location` contexts in NGINX.
As the intention is to provide this capabilitiy as a filter, which is attached at the `rules[].filters` level, this implementation aims to keep this behaviour at the `location` level.

As Filters are attached at the rule level, they are applied to every request that matches any of the rule’s match entries.
If a match uses `PathPrefix`, the filter applies to the entire prefix subtree (the prefix and all subsequent subpaths under it). If a match uses `Exact`, the filter applies only to that exact path.

Given this behaviour, we may need to consider to construct the final `http.conf` file given combinations of `Exact` and `PathPrefix` path types.

Example HTTPRoute
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-with-basic-auth
  namespace: default
spec:
  parentRefs:
  - name: gateway
  hostnames:
  - api.example.com
  rules:
  # Rule 1: Protect /v1 and a nested /v1/admin path
  - matches:
    - path:
        type: PathPrefix
        value: /v1
    - path:
        type: PathPrefix
        value: /v1/admin
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: BasicAuthFilter
        name: basic-auth
    backendRefs:
    - name: service-v1
      port: 80

  # Rule 2: Protect all /v2 paths
  - matches:
    - path:
        type: PathPrefix
        value: /v2
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: BasicAuthFilter
        name: basic-auth
    backendRefs:
    - name: service-v2
      port: 80

  # Rule 3: Protect an exact path with the same filter
  - matches:
    - path:
        type: Exact
        value: /reports
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org
        kind: BasicAuthFilter
        name: basic-auth
    backendRefs:
    - name: reports-svc
      port: 8080
```


## Potential HTTPRoute behaviour

An authentication based filter may be referenced by multiple HTTPRoutes, and multiple rules within those routes.

A HTTPRoute that is already deployed, and references an authentication based filter that does `not` exist, may remain unchanged and continue to serve the previous working config.

A HTTPRoute that is already deployed, and references an authentication based filter that has been delete, should update the NGINX configuration to reflect the removal of the filter.

A new HTTPRoute that has yet to be deployed, and is referencing a filter that does `not` exist may be `rejected` when attempting to apply it.
