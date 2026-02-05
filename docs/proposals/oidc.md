# Enhancement Proposal 4687: OpenID Connect

- Issue: [4687](https://github.com/nginx/nginx-gateway-fabric/issues/4687)
- Status: Provisional

## Summary

Enable NGINX Gateway Fabric to support centralized authentication enforcement using OpenID Connect (OIDC). This feature will be available only for NGINX Plus users.

## Goals

- Design a solution to support OIDC authentication that is compatible with multiple Identity Providers (IdPs).
- Extend the AuthenticationFilter CRD to allow users to configure OIDC authentication settings.

## Non-Goals

- Define implementation details for OIDC authorization.
- Provide role-based access control (RBAC) authorization mechanisms.
- Support authentication mechanisms outside of OIDC.

## Useful Links

- [NGINX OIDC Module](https://nginx.org/en/docs/http/ngx_http_oidc_module.html)
- [Single Sign-On with OpenID Connect and Identity Providers](https://docs.nginx.com/nginx/admin-guide/security-controls/configuring-oidc)
