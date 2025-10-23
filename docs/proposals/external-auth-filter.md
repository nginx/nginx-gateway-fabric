
# Enhancement Proposal-4052: External AuthFilter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Provisional

## Summary

[GEP-1494](https://gateway-api.sigs.k8s.io/geps/gep-1494/) defines an API for HTTPRoute to standardize Authentication and Authorization within the Gateway API.

This proposal aim to provider users of the Gateway API with a native form of Authenticaiton through Gateway API's [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter)

## Goals

- Integrate with the [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter) for HTTPRoute in the Gateway API
- Ensure this capability is available only when users enable experimental features in NGF
- Support only [HTTPAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#httpauthconfig)
- Provide users with a helm chart to deploy, manage and configure NGINX for Authentication use cases. i.e. the External Service
- Allow users to configure an exteranl IdP such as Keycloak or AzureAD

## Non-Goals

- Support for [GRPCAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#grpcauthconfig)

## Introduction

This document focus on an approach to have NGF integate with the HTTPExternalAuthFilter in the Gateway API.
This filter defines a mean to communicate with an external authentication services that can be responsible for both Authenticaiton and Authroization of requests to a backend application.

> ⚠️ This filter is currently part of the Gateway API experimental channel. The API is subject to changes that may break implementations.

## API, Customer Driven Interfaces, and User Experience

### API

Below is an example of an HTTPRoute with one path configured to route request to an external auth service named `ext-authz-svc`
This service exposes the `/auth` endpoint which is responsible for authentication of any request to `/api`
The protocol field is set to `HTTP`. This defines the `ExternalAuth` filter as a [HTTPAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#httpauthconfig)

This filter will send authentication requests to an the ExternalAuth service. This service may be an IdP such as Keycloak. This may also be or our own [NGINX External Auth Service](https://github.com/nginx/nginx-external-auth-service) deployable as a Helm chart.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-with-external-auth
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
        value: /api
    filters:
    - type: ExternalAuth
      externalAuth:
        protocol: HTTP
        backendRef:
          # BackendObjectReference: defaults to core group and kind=Service if omitted
          name: ext-authz-svc
          port: 8080
        http:
          # Prepend a prefix when forwarding the client path to the auth server
          path: /auth
          # Additional request headers to send to the auth server (core headers are always sent)
          allowedHeaders:
            - X-Request-Id
            - X-User-Agent
            - X-Correlation-Id
          # Headers from the auth server response to copy into the backend request
          allowedResponseHeaders:
            - X-Authz-Trace
            - WWW-Authenticate
            - Set-Cookie
        forwardBody:
          # Buffer and forward up to 16 KiB of the client request body to the auth server
          maxSize: 16384
    backendRefs:
    - name: backend-svc
      port: 80
```

### Configuration flow

Configuration flow with one HTTPRoute with a single path rule referencing an externalAuth service. In this case the NGINX Auth Service. This service could be Keycloak and any other IdP.

![configuration-flow](/docs/images/external-auth-filter/configuration-flow.png)

## Use Cases

- As a Cluter Administrator, I want to define specific authentication configurations for each application within the cluster so that each application developer can select the appropriate auth menchanisim for their endpoints.
- As an Application Developer, I want to secure access to my APIs and Backend Applications.
- As an Application Developer, I want to enforce authenticaiton on specific routes and matches.

### Understanding NGINX authentication methods

| **Authentication Method**     | **OSS**      | **Plus** | **NGINX Module**                | **Details**                                                        |
|-------------------------------|--------------|----------------|----------------------------------|--------------------------------------------------------------------|
| **HTTP Basic Authentication** | ✅           | ✅             | [ngx_http_auth_basic](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html) | Requires a username and password sent in an HTTP header.           |
| **JWT (JSON Web Token)**     | ❌           | ✅             | [ngx_http_auth_jwt_module](https://docs.nginx.com/nginx/admin-guide/security-controls/authentication/#jwt-authentication) | Tokens are used for stateless authentication between client and server. |
| **OpenID Connect**            | ❌           | ✅             | [ngx_http_oidc_module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)| Allows authentication through third-party providers like Google.   |

## Testing

- Unit tests
- Functional tests to validate behaviour of the external auth service.
  - In cases where users choose to deploy and manage our NGINX Auth Service, we will want to ensure any configurations applied to NGINX are robust and secure.
  - In cases where users choose to reference a seperate IdP such as Keycloak, we will want to ensure NGF responds accordingly to the appropriate response code returned. This will also be the case for responses returned from the NGINX Auth Service.

## Security Considerations

It's important we consider a means to secure connect from NGF to the ExternalAuth service.
A user may choose to deploy a `BackendTLSPolicy` configured with SNI/CA trust.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: auth-backend-tls
  namespace: security
spec:
  targetRefs:
  - group: ""
    kind: Service
    name: ext-authz-svc
  validation:
    caCertificateRefs:
    - kind: ConfigMap
      name: auth-ca
    hostname: auth.internal.example
```

### Validation

As this approach also includes the option to provide users with an NGINX Auth Service to deploy and manage, it is important that all configuration fields are validated for that deployment option, and that configurations applied to NGINX are robust and secure.

## Alternatives

The [Authentication Filter](docs/proposals/authentication-filter.md) document proposes to develop our own `AuthenticationFilter` CRD to expose the various auth capabilies through NGINX. Please refer to that proposal for details on how that approach may be implemented.

## References

- [Authentication Filter Proposal](docs/proposals/authentication-filter.md)
- [NGINX External Auth Service](https://github.com/nginx/nginx-external-auth-service)
- [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter)
- [HTTPAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#httpauthconfig)
- [GRPCAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#grpcauthconfig)
- [ngx_http_auth_basic](https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html)
- [ngx_http_auth_jwt_module](https://docs.nginx.com/nginx/admin-guide/security-controls/authentication/#jwt-authentication)
- [ngx_http_oidc_module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)
