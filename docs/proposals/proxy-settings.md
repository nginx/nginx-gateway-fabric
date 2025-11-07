
# Enhancement Proposal-4067: Proxy Settings Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4067
- Status: Provisional

## Summary

This Enhancement Proposal introduces the `ProxySettingsPolicy` API that allows Cluster Operators and Application Developers to configure NGINX's proxy buffering and connection settings between the NGINX Gateway Fabric dataplane and upstream applications.

## Goals

- Define proxy settings for buffering configuration.
- Define an API for proxy settings that is extensible to support additional proxy directives in the future.
- Outline the attachment points (Gateway and HTTPRoute/GRPCRoute) for the proxy settings policy.
- Describe the inheritance behavior of proxy settings when multiple policies exist at different levels.

## Non-Goals

- Define the complete set of all proxy directives (only buffering directives are in scope for initial implementation).
