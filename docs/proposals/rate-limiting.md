# Enhancement Proposal-4059: Rate Limit Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4059
- Status: Provisional

## Summary

This Enhancement Proposal introduces the "RateLimitPolicy" API that allows Cluster Operators and Application Developers to configure NGINX's rate limiting settings for Local Rate Limiting (RL per instance) and Global Rate Limiting (RL across all instances). Local Rate Limiting will be available on OSS through the `ngx_http_limit_req_module` while Global Rate Limiting will only be available through NGINX Plus, building off the OSS implementation but also using the `ngx_stream_zone_sync_module` to share state between NGINX instances. In addition to rate limiting on a key, which tells NGINX which rate limit bucket a request goes to, users should also be able to define Conditions on the RateLimitPolicy which decide if the request should be affected by the policy. This will allow for rate limiting on JWT Claim and other NGINX variables. 

## Goals

- Define rate limiting settings.
- Outline attachment points (Gateway and HTTPRoute/GRPCRoute) for the rate limit policy.
- Describe inheritance behavior of rate limiting settings when multiple policies exist at different levels.
- Define how Conditions on the rate limit policy work. 

## Non-Goals

- Championing a Rate Limiting Gateway API contribution.
- Exposing Zone Sync settings.
- Support for attachment to TLSRoute. 
