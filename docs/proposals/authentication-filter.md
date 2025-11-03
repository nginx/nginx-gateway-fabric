# Enhancement Proposal-4052: Authentiation Filter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Provisional

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
- Implementation of the Gateway API's ExternalAuth filter
