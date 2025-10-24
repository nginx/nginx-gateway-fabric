# Enhancement Proposal-4052: Authentiation Filter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Provisional

## Summary

Design and implement a means for users of NGINX Gateway Fabric to enable authenticaiton on requests to their backend applications.
This new filter should eventually expose all forms of authentication avaialbe through NGINX, both Open Source and Plus.

## Goals

- Design a means of configuring authenticaiton for NGF
- Determine initial resource specification
- Evaluate filter early in request processing, occurring before URLRewrite, header modifiers and backend selection
- Authentication failures return appropriate status by default (e.g., 401/403)
- Ensure response codes are configurable

## Non-Goals

- Design for all forms of authentication
- An Auth filter for GRPC, TCP and UDP routes
