# Enhancement Proposal-5730: Health Checks

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5730
- Status: Provisional

## Summary

This Enhancement Proposal extends the `UpstreamSettingsPolicy` API to allow Application developers and Cluster
Operators to configure active (NGINX Plus) and passive (NGINX OSS and NGINX Plus) health checks for upstream
application endpoints, so that traffic is not routed to endpoints that are unable to serve requests.

## Goals

- Allow users to enable and configure passive health checks for an upstream, supported by both NGINX OSS and
  NGINX Plus.
- Allow users to enable and configure active health checks for an upstream, supported only by NGINX Plus.
- Support health checks for HTTP, HTTPS, and gRPC upstreams.
- Expose the active health-check parameters supported by NGINX Plus (e.g. path, interval, jitter, fails, passes,
  port, headers, status match, gRPC status/service, mandatory, persistent, keepalive-time).
- Stop routing traffic to endpoints that fail the configured health criteria, and restore them once they pass the
  configured recovery criteria.
- Support mandatory health checks so newly added endpoints must pass a health check before receiving traffic.
- Preserve endpoint health state across NGINX reloads when configured (persistent health checks).
- Clearly report through status conditions when an active health-check configuration is used with NGINX OSS, since
  it is not supported.

## Non-Goals

- Change or replace Kubernetes readiness/liveness probes.
- Define health checks for TLSRoute or other layer 4 routes.
