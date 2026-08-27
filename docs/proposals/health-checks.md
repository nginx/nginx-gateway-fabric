# Enhancement Proposal-5730: Health Checks

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5730
- Status: Implementable

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
  port, match criteria (initially status, with the API structured to support additional match types such
  as header/body in a future enhancement), gRPC status/service, mandatory, persistent, keepalive-time).
- Stop routing traffic to endpoints that fail the configured health criteria, and restore them once they pass the
  configured recovery criteria.
- Support mandatory health checks so newly added endpoints must pass a health check before receiving traffic.
- Preserve endpoint health state across NGINX reloads when configured (persistent health checks).
- Clearly report through status conditions when an active health-check configuration is used with NGINX OSS, since
  it is not supported.

## Non-Goals

- Change or replace Kubernetes readiness/liveness probes.
- Define health checks for TLSRoute or other layer 4 routes (stream upstreams).
- Expose a separate TLS configuration for health check requests. Health check requests will use the same
  protocol (HTTP or HTTPS) that NGINX Gateway Fabric already uses to connect to the upstream for regular traffic.
- Add support for `slow_start`. This is a related but separate NGINX Plus upstream feature (see
  [NGINX Extensions](nginx-extensions.md#upstream-settings)) and can be addressed in a future enhancement.

## Introduction

### Passive Health Checks

Passive health checks are supported by both NGINX OSS and NGINX Plus, and are configured by default in NGINX. NGINX
monitors transactions as they happen and tries to resume failed connections. If the transaction still cannot be
resumed, NGINX marks the server as unavailable and temporarily stops sending requests to it. The conditions under
which a server is marked unavailable are controlled by two parameters on the
[`server`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#server) directive in the `upstream` block:

- [`fail_timeout`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#fail_timeout) - Sets the time
  during which a number of failed attempts must happen for the server to be marked unavailable, and also the time
  for which the server is marked unavailable. Default: 10 seconds.
- [`max_fails`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#max_fails) - Sets the number of failed
  attempts that must occur during `fail_timeout` for the server to be marked unavailable. Default: 1.

```nginx
upstream backend {
    zone backend 64k;
    server 10.0.0.1:8080 max_fails=3 fail_timeout=5s;
    server 10.0.0.2:8080 max_fails=3 fail_timeout=5s;
}
```

### Active Health Checks

Active health checks are an NGINX Plus-only feature provided by the
[`ngx_http_upstream_hc_module`](https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html). NGF periodically
sends requests to each server in an upstream group and determines whether the server is healthy. Unhealthy servers
are temporarily removed from load balancing until they satisfy the configured recovery criteria. Active health
checks are configured with the
[`health_check`](https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#health_check) directive in the
`location` block that proxies to the upstream, and require the upstream to reside in a
[shared memory zone](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#zone).

Each upstream with an active health check enabled requires its own dedicated `location` for the `health_check`
directive. This location can inherit the TLS settings from a
[`BackendTLSPolicy`](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/) if one is attached to the
targeted Service.

```nginx
upstream backend {
    zone backend 64k;
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}

server {
    location @hc-backend {
        internal;
        proxy_pass http://backend;
        health_check interval=10s fails=3 passes=2 uri=/healthz mandatory persistent;
    }
}
```

Tests on the response (status code, headers, body) are configured separately with a
[`match`](https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#match) block, referenced from the
`health_check` directive's `match` parameter.

### Extending UpstreamSettingsPolicy

Both health check types will be added to the existing `UpstreamSettingsPolicy` API (introduced in
[Upstream Settings Policy](upstream-settings.md)), rather than introducing a new Policy CRD. This keeps all
upstream/backend-connection-related settings in a single Policy, which is consistent with how `keepAlive`,
`loadBalancingMethod`, and other upstream settings are already grouped (see
[NGINX Extensions](nginx-extensions.md#upstream-settings)).

## API, Customer Driven Interfaces, and User Experience

A single new optional field, `HealthCheck`, will be added to `UpstreamSettingsPolicySpec`. It contains two distinct
sub-fields, `Passive` and `Active`, which may be configured independently or together. For example, a Plus user
may configure `active` for fast, proactive failure detection while also tuning `passive` as a fallback/defense in
depth, since passive health checking is effectively always evaluated by NGINX regardless of whether active checks
are enabled. Internal validation ensures `active` may only be set when NGF is running with NGINX Plus; when set
while running NGINX OSS, the Policy is rejected with a status condition (see [Status](#status)).

### Go

```go
package v1alpha1

import (
    gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type UpstreamSettingsPolicySpec struct {
    // ... existing fields (ZoneSize, KeepAlive, LoadBalancingMethod, HashMethodKey, UseClusterIP, TargetRefs) ...

    // HealthCheck defines the health check settings for the upstream.
    //
    // +optional
    HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
}

// HealthCheck defines the passive and/or active health check settings for an upstream.
type HealthCheck struct {
    // Passive defines the passive health check settings for the upstream. Passive health checks are supported
    // by NGINX OSS and NGINX Plus.
    //
    // +optional
    Passive *PassiveHealthCheck `json:"passive,omitempty"`

    // Active defines the active health check settings for the upstream. Active health checks are only
    // supported by NGINX Plus. Setting this field while running NGINX OSS will cause the Policy to be
    // rejected.
    //
    // +optional
    Active *ActiveHealthCheck `json:"active,omitempty"`
}

// PassiveHealthCheck defines the passive health check settings for an upstream server.
type PassiveHealthCheck struct {
    // MaxFails sets the number of consecutive unsuccessful attempts to communicate with a server that must
    // happen during FailTimeout for the server to be considered unavailable. A value of 0 disables the
    // accounting of attempts entirely.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#max_fails
    //
    // +optional
    // +kubebuilder:validation:Minimum=0
    MaxFails *int32 `json:"maxFails,omitempty"`

    // FailTimeout sets the time during which the specified number of unsuccessful attempts to communicate
    // with a server must happen for the server to be considered unavailable. This is also the period of time
    // the server will be considered unavailable.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#fail_timeout
    //
    // +optional
    FailTimeout *Duration `json:"failTimeout,omitempty"`
}

// ActiveHealthCheck defines the active health check settings for an upstream. This is an NGINX Plus-only feature.
// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#health_check
//
// +kubebuilder:validation:XValidation:message="path must not be set when grpc is set",rule="!(has(self.path) && has(self.grpc))"
// +kubebuilder:validation:XValidation:message="match must not be set when grpc is set",rule="!(has(self.match) && has(self.grpc))"
type ActiveHealthCheck struct {
    // Interval sets the interval between two consecutive health checks.
    //
    // +optional
    Interval *Duration `json:"interval,omitempty"`

    // Jitter sets the time within which each health check will be randomly delayed.
    //
    // +optional
    Jitter *Duration `json:"jitter,omitempty"`

    // Fails sets the number of consecutive failed health checks of a particular server after which this
    // server will be considered unhealthy. Unlike PassiveHealthCheck's MaxFails, a value of 0 has no meaning
    // for active health checks (NGINX does not define a "disable" behavior for this parameter); active health
    // checking is disabled entirely by omitting ActiveHealthCheck, so a minimum of 1 is enforced here.
    //
    // +optional
    // +kubebuilder:validation:Minimum=1
    Fails *int32 `json:"fails,omitempty"`

    // Passes sets the number of consecutive passed health checks of a particular server after which the
    // server will be considered healthy.
    //
    // +optional
    // +kubebuilder:validation:Minimum=1
    Passes *int32 `json:"passes,omitempty"`

    // Path defines the URI used in health check requests, by default, "/". Mutually exclusive with GRPC: the
    // "type=grpc" parameter is not compatible with the "uri" parameter.
    //
    // +optional
    // +kubebuilder:validation:Pattern=`^[^\s{};$\\]*$`
    // +kubebuilder:validation:MaxLength=2048
    Path *string `json:"path,omitempty"`

    // Port defines the port used when connecting to a server to perform a health check.
    //
    // +optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=65535
    Port *int32 `json:"port,omitempty"`

    // Headers are the request headers to send with health check requests. NGINX Plus always sets the Host,
    // User-Agent, and Connection headers for health check requests, and these cannot be overridden.
    //
    // +optional
    // +kubebuilder:validation:MaxItems=16
    Headers []gatewayv1.HTTPHeader `json:"headers,omitempty"`

    // Match defines the response criteria that must be satisfied for a health check to pass. If not set, the
    // response must have a status code of 2xx or 3xx (NGINX's default). Mutually exclusive with GRPC.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#match
    //
    // +optional
    Match *Match `json:"match,omitempty"`

    // GRPC configures the health check to use the gRPC health checking protocol. Mutually exclusive with
    // Path and Match. Only valid for gRPC upstreams.
    //
    // +optional
    GRPC *GRPCHealthCheck `json:"grpc,omitempty"`

    // Mandatory requires every newly added server to pass the configured health check(s) before NGINX Plus
    // sends traffic to it.
    //
    // +optional
    Mandatory *bool `json:"mandatory,omitempty"`

    // Persistent sets the initial "up" state for a server after a reload, if the server was considered
    // healthy before the reload. Requires Mandatory to be set to true.
    //
    // +optional
    Persistent *bool `json:"persistent,omitempty"`

    // KeepAliveTime enables keepalive connections for health checks and specifies the time during which
    // requests can be processed through one keepalive connection.
    //
    // +optional
    KeepAliveTime *Duration `json:"keepAliveTime,omitempty"`

    // Timeout sets the connect/read/send timeouts used for health check requests. If not set, fall back to
    // NGINX's defaults.
    //
    // +optional
    Timeout *ProxyTimeout `json:"timeout,omitempty"`
}

// Match defines the criteria used to determine whether a response to a health check request is considered
// successful. This corresponds to an NGINX "match" block
// (https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#match), which can test the response status
// code, response headers, and response body.
type Match struct {
    // Status defines the expected response status code(s) for a health check to pass. By default, the
    // response must have a status code of 2xx or 3xx.
    // Must be an optional "!" negation followed by one or more space-separated status codes or status code
    // ranges (each 3 digits), matching the syntax of the "status" test in an NGINX match block.
    // Examples: "200", "! 500", "200 204", "! 301 302", "200-399", "! 400-599", "301-303 307".
    //
    // +optional
    // +kubebuilder:validation:Pattern=`^(!\s+)?\d{3}(-\d{3})?(\s+\d{3}(-\d{3})?)*$`
    // +kubebuilder:validation:MaxLength=64
    Status *string `json:"status,omitempty"`
}

// GRPCHealthCheck configures a gRPC-specific active health check.
type GRPCHealthCheck struct {
    // Service is the gRPC service to be monitored on the upstream server, corresponding to the "service"
    // field of the gRPC Health Checking Protocol's HealthCheckRequest. If not set, the health of the overall
    // server is monitored using the gRPC Health Checking Protocol.
    //
    // +optional
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=253
    // +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`
    Service *string `json:"service,omitempty"`

    // Status is the gRPC status code expected in the response to the Check method, treated as healthy.
    // Configure this field only if the gRPC service does not implement the gRPC Health Checking Protocol.
    // Accepts either the numeric code (e.g. "12") or its canonical name (e.g. "UNIMPLEMENTED"), matching the
    // two forms accepted by NGINX's grpc_status parameter.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#health_check
    //
    // +optional
    Status *GRPCStatus `json:"status,omitempty"`
}

// GRPCStatus is a gRPC status code, expressed as either its numeric code ("12") or its canonical name
// ("UNIMPLEMENTED"). "OK"/"0" is intentionally omitted: it is already
// the default healthy response and NGINX's grpc_status parameter requires a non-zero code.
//
// +kubebuilder:validation:Enum=1;2;3;4;5;6;7;8;9;10;11;12;13;14;15;16;CANCELLED;UNKNOWN;INVALID_ARGUMENT;DEADLINE_EXCEEDED;NOT_FOUND;ALREADY_EXISTS;PERMISSION_DENIED;RESOURCE_EXHAUSTED;FAILED_PRECONDITION;ABORTED;OUT_OF_RANGE;UNIMPLEMENTED;INTERNAL;UNAVAILABLE;DATA_LOSS;UNAUTHENTICATED
type GRPCStatus string

const (
    GRPCStatusCancelled          GRPCStatus = "CANCELLED"
    GRPCStatusUnknown            GRPCStatus = "UNKNOWN"
    GRPCStatusInvalidArgument    GRPCStatus = "INVALID_ARGUMENT"
    GRPCStatusDeadlineExceeded   GRPCStatus = "DEADLINE_EXCEEDED"
    GRPCStatusNotFound           GRPCStatus = "NOT_FOUND"
    GRPCStatusAlreadyExists      GRPCStatus = "ALREADY_EXISTS"
    GRPCStatusPermissionDenied   GRPCStatus = "PERMISSION_DENIED"
    GRPCStatusResourceExhausted  GRPCStatus = "RESOURCE_EXHAUSTED"
    GRPCStatusFailedPrecondition GRPCStatus = "FAILED_PRECONDITION"
    GRPCStatusAborted            GRPCStatus = "ABORTED"
    GRPCStatusOutOfRange         GRPCStatus = "OUT_OF_RANGE"
    GRPCStatusUnimplemented      GRPCStatus = "UNIMPLEMENTED"
    GRPCStatusInternal           GRPCStatus = "INTERNAL"
    GRPCStatusUnavailable        GRPCStatus = "UNAVAILABLE"
    GRPCStatusDataLoss           GRPCStatus = "DATA_LOSS"
    GRPCStatusUnauthenticated    GRPCStatus = "UNAUTHENTICATED"

    // Numeric equivalents (1-16), accepted as an alternative to the named constants above.
    GRPCStatusCode1  GRPCStatus = "1"
    GRPCStatusCode2  GRPCStatus = "2"
    GRPCStatusCode3  GRPCStatus = "3"
    GRPCStatusCode4  GRPCStatus = "4"
    GRPCStatusCode5  GRPCStatus = "5"
    GRPCStatusCode6  GRPCStatus = "6"
    GRPCStatusCode7  GRPCStatus = "7"
    GRPCStatusCode8  GRPCStatus = "8"
    GRPCStatusCode9  GRPCStatus = "9"
    GRPCStatusCode10 GRPCStatus = "10"
    GRPCStatusCode11 GRPCStatus = "11"
    GRPCStatusCode12 GRPCStatus = "12"
    GRPCStatusCode13 GRPCStatus = "13"
    GRPCStatusCode14 GRPCStatus = "14"
    GRPCStatusCode15 GRPCStatus = "15"
    GRPCStatusCode16 GRPCStatus = "16"
)
```

### NGINX OSS Support

Passive health checks (`healthCheck.passive`) are supported on both NGINX OSS and NGINX Plus and generate the
`max_fails`/`fail_timeout` parameters on the `server` directive in the `upstream` block.

Active health checks (`healthCheck.active`) require NGINX Plus. If a user sets `healthCheck.active` while running
NGF with NGINX OSS, the Policy will be rejected: the `Accepted` condition will be set to `False` with reason
`Invalid`, and the message will clearly state that active health checks require NGINX Plus. No partial
configuration will be applied to NGINX in this case, consistent with how other Plus-only settings (e.g.
`least_time` load balancing) are validated today.

### Status

No new Condition types are required for the "affected object" status mechanism already described in
[Upstream Settings Policy](upstream-settings.md#setting-status-on-objects-affected-by-a-policy); it applies
unchanged.

A new reason for the existing `Accepted`/Invalid condition path will be added to clearly convey the NGINX
Plus-only requirement, for example:

```yaml
Conditions:
  Type:                  Accepted
  Status:                False
  Reason:                Invalid
  Message:               "spec.healthCheck.active: Forbidden: active health checks are only supported with NGINX Plus"
  Observed Generation:   1
```

### YAML

Passive health check example (NGINX OSS or NGINX Plus):

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: example-passive-hc
  namespace: default
spec:
  targetRefs:
  - group: core
    kind: Service
    name: backend-svc
  healthCheck:
    passive:
      maxFails: 3
      failTimeout: 5s
```

Generated NGINX configuration:

```nginx
upstream backend-svc {
    server 10.244.0.5:8080 max_fails=3 fail_timeout=5s;
    server 10.244.0.6:8080 max_fails=3 fail_timeout=5s;
}
```

Active health check example (NGINX Plus only):

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: example-active-hc
  namespace: default
spec:
  targetRefs:
  - group: core
    kind: Service
    name: backend-svc
  zoneSize: 1m
  healthCheck:
    active:
      interval: 10s
      jitter: 3s
      fails: 3
      passes: 2
      path: /healthz
      match:
        status: "! 500"
      mandatory: true
      persistent: true
      keepAliveTime: 60s
      timeout:
        connect: 2s
        read: 2s
        send: 2s
```

Generated NGINX configuration:

```nginx
upstream backend-svc {
    zone backend-svc 1m;
    server 10.244.0.5:8080;
    server 10.244.0.6:8080;
}

server {
    location @hc-backend-svc {
        internal;
        proxy_connect_timeout 2s;
        proxy_read_timeout 2s;
        proxy_send_timeout 2s;
        proxy_pass http://backend-svc;
        health_check interval=10s jitter=3s fails=3 passes=2 uri=/healthz mandatory persistent
                     keepalive_time=60s match=backend-svc_match;
    }
}

match backend-svc_match {
    status ! 500;
}
```

Combined example (NGINX Plus only) configuring both active and passive health checks simultaneously:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: example-combined-hc
  namespace: default
spec:
  targetRefs:
  - group: core
    kind: Service
    name: backend-svc
  zoneSize: 1m
  healthCheck:
    passive:
      maxFails: 3
      failTimeout: 5s
    active:
      interval: 10s
      fails: 3
      passes: 2
      path: /healthz
      mandatory: true
      persistent: true
```

Generated NGINX configuration:

```nginx
upstream backend-svc {
    zone backend-svc 1m;
    server 10.244.0.5:8080 max_fails=3 fail_timeout=5s;
    server 10.244.0.6:8080 max_fails=3 fail_timeout=5s;
}

server {
    location @hc-backend-svc {
        internal;
        proxy_pass http://backend-svc;
        health_check interval=10s fails=3 passes=2 uri=/healthz mandatory persistent;
    }
}
```

gRPC active health check example:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: example-grpc-hc
  namespace: default
spec:
  targetRefs:
  - group: core
    kind: Service
    name: grpc-backend-svc
  zoneSize: 1m
  healthCheck:
    active:
      interval: 10s
      fails: 3
      passes: 2
      grpc:
        service: my.grpc.Service
```

Generated NGINX configuration:

```nginx
upstream grpc-backend-svc {
    zone grpc-backend-svc 1m;
    server 10.244.0.7:50051;
    server 10.244.0.8:50051;
}

server {
    location @hc-grpc-backend-svc {
        internal;
        grpc_pass grpc://grpc-backend-svc;
        health_check interval=10s fails=3 passes=2 type=grpc grpc_service=my.grpc.Service;
    }
}
```

## Use Cases

- As an Application Operator, I want NGF to probe my application's health endpoint so that traffic is not sent to
  pods that are running but unable to serve requests (e.g. a dependency failure causes a health endpoint to fail).
- As a Platform Administrator, I want newly created endpoints to pass a health check before receiving traffic
  (`mandatory`) so that applications have time to initialize and warm up.
- As a Platform Administrator running NGINX Plus, I want endpoint health state to persist across NGINX reloads
  (`persistent`) so that reloads do not cause a thundering-herd of health checks or send traffic to servers that
  were previously known to be unhealthy.
- As an Application Operator with a gRPC backend, I want to configure an active health check that uses the gRPC
  Health Checking Protocol (or a custom gRPC status) to validate my service is healthy.
- As a Cluster Operator, I want a clear, actionable status message when a user configures active health checks, so misconfigurations are easy to diagnose.

## Testing

- Unit tests
- Functional tests that test the attachment and inheritance behavior outlined in this document. The details of these tests are out of scope for this document.

## Security Considerations

Validating all fields in the extended `UpstreamSettingsPolicy` is critical to ensuring that the NGINX config
generated by NGINX Gateway Fabric is correct and does not allow for injection into the NGINX configuration file.

All fields will be validated with OpenAPI Schema validation where possible. Internal validation can be done on string fields for an extra layer of security to prevent config injection.

RBAC via the Kubernetes API server will continue to ensure that only authorized users can create or update the
`UpstreamSettingsPolicy` CRD.

## Future Work

- Add support for `slow_start`, which is closely related to `mandatory` active health checks (giving a newly healthy
  server a ramp-up period before receiving full traffic).
- Add support for health checks on stream (TLSRoute/TCPRoute/UDPRoute) upstreams, mirroring the pattern used for
  the HTTP `UpstreamSettingsPolicy` extension described here.
- Extend `Match` with additional NGINX `match` block criteria (`header`, `body`) as new optional sub-fields, if
  there is demonstrated user need. The `Match` struct introduced in this proposal is intentionally modeled after
  NGINX's own `match` block (rather than as a flat `statusMatch` field) so that this can be done without a
  breaking API change.

## References

- [Upstream Settings Policy Enhancement Proposal](upstream-settings.md)
- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [NGINX passive health check documentation](https://docs.nginx.com/nginx/admin-guide/load-balancer/http-health-check/#passive-health-checks)
- [NGINX active health check module (`ngx_http_upstream_hc_module`)](https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html)
- [NGINX Ingress Controller `Healthcheck` type](https://docs.nginx.com/nginx-ingress-controller/configuration/policy-resource/#healthcheck)
- [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/)
