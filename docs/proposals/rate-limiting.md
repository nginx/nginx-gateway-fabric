# Enhancement Proposal-4059: Rate Limit Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4059
- Status: Implementable

## Summary

This Enhancement Proposal introduces the "RateLimitPolicy" API that allows Cluster Operators and Application Developers to configure NGINX's rate limiting settings for Local Rate Limiting (RL per instance) and Global Rate Limiting (RL across all instances). Local Rate Limiting will be available on OSS through the `ngx_http_limit_req_module` while Global Rate Limiting will only be available through NGINX Plus, building off the OSS implementation but also using the `ngx_stream_zone_sync_module` to share state between NGINX instances. In addition to rate limiting on a key, which tells NGINX which rate limit bucket a request goes to, users should also be able to define Conditions on the RateLimitPolicy which decide if the request should be affected by the policy. This will allow for rate limiting on JWT Claim and other NGINX variables.

## Goals

- Define rate limiting settings.
- Outline attachment points (Gateway and HTTPRoute/GRPCRoute) for the rate limit policy.
- Describe inheritance behavior of rate limiting settings when multiple policies exist at different levels.
- Define how Conditions on the rate limit policy work.

## Non-Goals

- Champion a Rate Limiting Gateway API contribution.
- Expose Zone Sync settings.
- Support for attachment to TLSRoute.

## Introduction

Rate limiting is a feature in NGINX which allows users to limit the request processing rate per a defined key, which usually refers to processing rate of requests coming from a single IP address. However, this key can contain text, variables, or a combination of them. Rate limiting through a reverse proxy can be broadly broken down into two different categories: Local Rate Limiting, and Global Rate Limiting.

### Local Rate Limiting

Local Rate Limiting refers to rate limiting per NGINX instance. Meaning each NGINX instance will have independent limits and these limits are not affected by requests sent to other NGINX instances in a replica fleet.

In NGINX, this can be done using the `ngx_http_limit_req_module`, using the `limit_req_zone` and `limit_req` directives. Below is a simple example configuration where a `zone` named `one` is created with a size of `10 megabytes` and an average request processing rate for this zone cannot exceed 1 request per second. This zone keys on the variable `$binary_remote_addr`, which is the client IP address, meaning each client IP address will be tracked by a separate rate limit. Finally, the `limit_req` directive is used in the `location /search/` to put a limit on requests targeting that path.

```nginx
limit_req_zone $binary_remote_addr zone=one:10m rate=1r/s;

server {
    location /search/ {
        limit_req zone=one;
    }
    ...
```

Benefits of local limiting:

- Lightweight and does not require any external state tracking
- Fast enforcement with rate limiting at the edge
- Effective as a first line of defense against traffic bursts

Downsides:

- Harder to reason about capacity of fleet, especially when auto-scaling is enabled

### Global Rate Limiting

Global Rate Limiting refers to rate limiting across an entire NGINX Plus fleet. Meaning NGINX Plus instances will share state and centralize their limits.

In NGINX Plus, this can be done by using the `ngx_stream_zone_sync_module` to extend the solution for Local Rate Limiting and provide a way for synchronizing contents of shared memory zones across NGINX Plus instances. Below is a simple example configuration where the `sync` parameter is attached to the `limit_req_zone` directive. The other `zone_sync` directives, living in a separate `stream` block, start the global synchronization engine and lets this NGINX Plus instance connect and share state with the other specified NGINX Plus instances.

```nginx
stream {
    server {
        listen 0.0.0.0:12345;      # any free TCP port for sync traffic
        zone_sync;                 # turns the engine on

        # full list of cluster peers (including yourself is harmless)
        zone_sync_server  nginx-0.example.com:12345;
        zone_sync_server  nginx-1.example.com:12345;
        zone_sync_server  nginx-2.example.com:12345;
    }
}

http {

    limit_req_zone $binary_remote_addr zone=one:10m rate=1r/s sync;

    server {
        location /search/ {
            limit_req zone=one;
        }
        ...
}
```

Benefits of global limiting:

- Centralized control across instances
- Fair sharing of backend capacity
- Burst resistance during autoscaling

Downsides:

- Additional resource consumption, the NGINX Plus sync module is complicated and when instances scale, memory consumption is greatly increased
- Eventually consistent, the sync module does not work on a real-time timeline, but instead propogates state every few seconds
- As NGINX Plus instances scale, zone_sync settings may need to be tuned
- NGINX Plus only

### Combining Local and Global Rate Limiting

NGINX Gateway Fabric will support configuring both global and local rate limits simultaneously on the same route. When combined, local and global rate limiting should work together, where a request is evaluated first at the local rate limit, then gets evaluated at the global rate limit, and only if both pass does the request be allowed through.

This should provide comprehensive protection by combining the benefits of both strategies.

## Use Cases

- As a Cluster Operator:
  - I want to set Global Rate Limits on NGINX Plus instances to:
    - Protect the whole Kubernetes Cluster.
    - Fit my commercial API license caps.
    - Ensure autoscaling is handled correctly.
    - Create Multi-tenant fairness.
  - I want to set Local Rate Limits on NGINX instances to:
    - Provide a default for NGINX instances.
    - Create protection for non-critical paths that don't need expensive Global Rate Limits.
- As an Application Operator:
  - I want to set Global Rate Limits for my specific application to:
    - Align with my specific End-user API plans. (Only 10 req/s per API key no matter which gateway replica the user hits).
    - Login / Auth brute-force defense.
    - Shared micro-service budget.
    - Fit my specific needs.
  - I want to set Local Rate Limits for my specific application to:
    - Act as a circuit-breaker for heavy endpoints.
    - Enable Canary / blue-green saftey.
    - Add additional security to developer namespaces.
    - Fit my specific needs.
  - I want to override the defaults for Local and Global Rate Limits set by the Cluster Operator because they do not satisfy my application's requirements or behaviors

## Design

Rate limiting allows users to limit the request processing rate per a defined key or bucket, and this can all be achieved through native NGINX OSS and Plus modules as shown above. However, users would also like to set conditions for a rate limit policy, where if a certain condition isn't met, the request would either go to a default rate limit policy, or would not be rate limited. This is designed to be used in combination with one or more rate limit policies. For example, multiple rate limit policies with that condition on JWT level can be used to apply different tiers of rate limit based on the value of a JWT claim (ie. more req/s for a higher level, less req/s for a lower level).

### Variable Condition

Variable Condition on a RateLimitPolicy would define a condition for a rate limit by NGINX variable. For example, a condition could be on the variable `$request_method` and the match could be `GET`, meaning this RateLimitPolicy would only apply to requests with the request method with a value `GET`.

### JWT Claim Condition

JWT Claim Condition on a RateLimitPolicy would define a condition for a rate limit by JWT claim. For example, a condition could be on the claim `user_details.level` and the match could be `premium`, meaning this RateLimitPolicy would only apply to requests with a JWT claim `user_details.level` with a value `premium`. The following JWT payload would match the condition:

```JSON
{
  "user_details": {
    "level": "premium"
  },
  "sub": "client1"
}
```

### NJS Support

Adding support for Conditions on the RateLimitPolicy will not be possible through native NGINX OSS and Plus modules and will need to be done through a separate NJS module.

## API

The `RateLimitPolicy` API is a CRD that is part of the `gateway.nginx.org` Group. It adheres to the guidelines and requirements of an Inherited Policy as defined in the [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/).

The policy uses `targetRefs` (plural) to support targeting multiple resources with a single policy instance. This follows the current GEP-713 guidance and provides better user experience by:

- Avoiding policy duplication when applying the same settings to multiple targets
- Reducing maintenance burden and risk of configuration inconsistencies
- Preventing future migration challenges from singular to plural forms

Below is the Golang API for the `RateLimitPolicy` API:

### Go

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// RateLimitPolicy is an Inherited Attached Policy. It provides a way to set local and global rate limiting rules in NGINX.
//
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=gateway-api,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"
type RateLimitPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // Spec defines the desired state of the RateLimitPolicy.
    Spec RateLimitPolicySpec `json:"spec"`

    // Status defines the state of the RateLimitPolicy.
    Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// RateLimitPolicySpec defines the desired state of the RateLimitPolicy.
type RateLimitPolicySpec struct {
    // TargetRefs identifies API object(s) to apply the policy to.
    // Objects must be in the same namespace as the policy.
    //
    // Support: Gateway, HTTPRoute, GRPCRoute
    //
    // +kubebuilder:validation:MinItems=1
    // +kubebuilder:validation:MaxItems=16
    // +kubebuilder:validation:XValidation:message="TargetRefs entries must have kind Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind == 'Gateway' || t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute')"
    // +kubebuilder:validation:XValidation:message="TargetRefs entries must have group gateway.networking.k8s.io",rule="self.all(t, t.group == 'gateway.networking.k8s.io')"
    // +kubebuilder:validation:XValidation:message="TargetRefs must be unique",rule="self.all(t1, self.exists_one(t2, t1.group == t2.group && t1.kind == t2.kind && t1.name == t2.name))"
    TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`

    // RateLimit defines the Rate Limit settings.
    //
    // +optional
    RateLimit *RateLimit `json:"rateLimit,omitempty"`
}

// RateLimit contains settings for Rate Limitting.
type RateLimit struct {
    // Local defines the local rate limit rules for this policy.
    //
    // +optional
    Local *LocalRateLimit `json:"local,omitempty"`

    // Global defines the global rate limit rules for this policy.
    //
    // +optional
    Global *GlobalRateLimit `json:"global,omitempty"`
}

// LocalRateLimit contains the local rate limit rules.
type LocalRateLimit struct {
    // Rules contains the list of rate limit rules.
    //
    // +optional
    Rules *RateLimitRule[] `json:"rules,omitempty"`

    // Zones contains the list of rate limit zones. Multiple rate limit rules can target the same zone.
    //
    // +optional
    Zones *RateLimitZone[] `json:"zones,omitempty"`
}

// GlobalRateLimit contains the global rate limit rules.
type GlobalRateLimit struct {
    // Rules contains the list of rate limit rules.
    Rules *RateLimitRule[] `json:"rules,omitempty"`

    // Zones contains the list of rate limit zones. Multiple rate limit rules can target the same zone.
    //
    // +optional
    Zones *RateLimitZone[] `json:"zones,omitempty"`
}

// RateLimitZone contains the settings for a rate limit zone. Multiple rate limit rules can target the same zone.
type RateLimitZone struct {
    // Rate represents the rate of requests permitted. The rate is specified in requests per second (r/s)
    // or requests per minute (r/m).
    //
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
    Rate Rate `json:"rate"`

    // Key represents the key to which the rate limit is applied.
    //
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
    Key string `json:"key"`

    // ZoneSize is the size of the shared memory zone.
    //
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
    ZoneSize Size `json:"zoneSize"`

    // ZoneName is the name of the zone.
    //
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
    ZoneName string `json:"zoneName"`
}

// RateLimitRule contains settings for a RateLimit Rule.
type RateLimitRule struct {
    // ZoneName is the name of the zone.
    //
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
    ZoneName string `json:"zoneName"`

    // Delay specifies a limit at which excessive requests become delayed. Default value is zero, i.e. all excessive requests are delayed.
    //
    // Default: 0
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req
    //
    // +optional
    Delay *int32 `json:"delay,omitempty"`

    // NoDelay disables the delaying of excessive requests while requests are being limited. Overrides delay if both are set.
    //
    // Default: false
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req
    //
    // +optional
    NoDelay *bool `json:"noDelay,omitempty"`

    // Burst sets the maximum burst size of requests. If the requests rate exceeds the rate configured for a zone,
    // their processing is delayed such that requests are processed at a defined rate. Excessive requests are delayed
    // until their number exceeds the maximum burst size in which case the request is terminated with an error.
    //
    // Default: 0
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req
    //
    // +optional
    Burst *int32 `json:"burst,omitempty"`

    // DryRun enables the dry run mode. In this mode, the rate limit is not actually applied, but the number of excessive requests is accounted as usual in the shared memory zone.
    //
    // Default: false
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_dry_run
    //
    // +optional
    DryRun *bool `json:"dryRun,omitempty"`

    // LogLevel sets the desired logging level for cases when the server refuses to process requests due to rate exceeding, or delays request processing. Allowed values are info, notice, warn or error.
    //
    // Default: error
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_log_level
    //
    // +optional
    LogLevel *RateLimitLogLevel `json:"logLevel,omitempty"`

    // RejectCode sets the status code to return in response to rejected requests. Must fall into the range 400..599.
    //
    // Default: 503
    // Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_status
    //
    // +optional
    // +kubebuilder:validation:Minimum=400
    // +kubebuilder:validation:Maximum=599
    RejectCode *int32 `json:"rejectCode,omitempty"`

    // Condition represents a condition to determine if the request should be rate limited by this rule.
    //
    // +optional
    Condition *RateLimitCondition `json:"condition,omitempty"`
}

// RateLimitCondition represents a condition to determine if the request should be rate limited.
type RateLimitCondition struct {
    // JWT defines a JWT condition to determine if the request should be rate limited.
    //
    // +optional
    JWT *RateLimitJWTCondition `json:"jwt,omitempty"`
    // Variable defines a Variable condition to determine if the request should be rate limited.
    //
    // +optional
    Variable *RateLimitVariableCondition `json:"variable,omitempty"`
    // Default sets the rate limit in this policy to be the default if no conditions are met. In a group of policies with the same condition,
    // only one policy can be the default.
    //
    // +optional
    Default *bool `json:"default,omitempty"`
}

// RateLimitJWTCondition represents a condition against a JWT claim.
type RateLimitJWTCondition struct {
    // Claim is the JWT claim that the conditional will check against. Nested claims should be separated by ".".
    Claim string `json:"claim"`
    // Match is the value of the claim to match against.
    Match string `json:"match"`
}

// RateLimitVariableCondition represents a condition against an NGINX variable.
type RateLimitVariableCondition struct {
    // Name is the name of the NGINX variable that the conditional will check against.
    Name string `json:"name"`
    // Match is the value of the NGINX variable to match against. Values prefixed with the ~ character denote the following is a regular expression.
    Match string `json:"match"`
}

// Size is a string value representing a size. Size can be specified in bytes, kilobytes (k), megabytes (m).
// Examples: 1024, 8k, 1m.
//
// +kubebuilder:validation:Pattern=`^\d{1,4}(k|m)?$`
type Size string

// Rate is a string value representing a rate. Rate can be specifid in r/s or r/m.
//
// +kubebuilder:validation:Pattern=`^\d+r/[sm]$`
type Rate string

// RateLimitPolicyList contains a list of RateLimitPolicies.
//
// +kubebuilder:object:root=true
type RateLimitPolicyList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []RateLimitPolicy `json:"items"`
}

// RateLimitLogLevel defines the log level for cases when the server refuses
// to process requests due to rate exceeding, or delays request processing.
//
// +kubebuilder:validation:Enum=info;notice;warn;error
type RateLimitLogLevel string

const (
	// RateLimitLogLevelInfo is the info level rate limit logs.
	RateLimitLogLevelInfo RateLimitLogLevel = "info"

	// RateLimitLogLevelNotice is the notice level rate limit logs.
	RateLimitLogLevelNotice RateLimitLogLevel = "notice"

	// RateLimitLogLevelWarn is the warn level rate limit logs.
	RateLimitLogLevelWarn RateLimitLogLevel = "warn"

	// RateLimitLogLevelError is the error level rate limit logs.
	RateLimitLogLevelError RateLimitLogLevel = "error"
)
```

### Versioning and Installation

The version of the `RateLimitPolicy` API will be `v1alpha1`.

The `RateLimitPolicy` CRD will be installed by the Cluster Operator via Helm or with manifests. It will be required, and if the `RateLimitPolicy` CRD does not exist in the cluster, NGINX Gateway Fabric will log errors until it is installed.

### Status

#### CRD Label

According to the [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/), the `RateLimitPolicy` CRD must have the `gateway.networking.k8s.io/policy: inherited` label to specify that it is an inherited policy.
This label will help with discoverability and will be used by Gateway API tooling.

#### Conditions

According to the [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/), the `RateLimitPolicy` CRD must include a `status` stanza with a slice of Conditions.

The following Conditions must be populated on the `RateLimitPolicy` CRD:

- `Accepted`: Indicates whether the policy has been accepted by the controller. This condition uses the reasons defined in the [PolicyCondition API](https://github.com/kubernetes-sigs/gateway-api/blob/main/apis/v1alpha2/policy_types.go).
- `Programmed`: Indicates whether the policy configuration has been propagated to the data plane. This helps users understand if their policy changes are active.

Note: The `Programmed` condition is part of the updated GEP-713 specification and should be implemented for this policy. Existing policies (ClientSettingsPolicy, UpstreamSettingsPolicy, ObservabilityPolicy) may not have implemented this condition yet and should be updated in future work.

##### ZoneName conflicts

Since the `limit_req_zone` directive can only be at the `http` context level, and `zoneNames` need to be unique, this introduces the potential for `zoneName` conflicts in `RateLimitPolicies` that contain a `RateLimitZone` with the same `zoneName`. The general strategy for resolving conflicts is that `RateLimitZones` defined at the Gateway-level will take priority over Route-level `RateLimitZones`, and conflicting `RateLimitZones` defined at equal `targetRef` level will prioritize the `RateLimitZone` of the `RateLimitPolicy` that was created before the other. When a `RateLimitZone` is taken "priority" over another, its settings will be generated in the NGINX configuration, and a `PartiallyInvalid` Condition will be shown on the "ignored/losing" `RateLimitPolicy`.

When a Route-level policy specifies a `RateLimitZone` that has a `zoneName` that conflicts with an existing `zoneName` set at the Gateway level, the following condition will be set on the `RateLimitPolicy` attached to the Route:

- **Condition Type**: `Programmed`
- **Status**: `False`
- **Reason**: `PartiallyInvalid` (implementation-specific reason)
- **Message**: "Policy is not fully programmed: RateLimitZone(s) with zoneName(s): (zoneName(s) here) are ignored because of a conflict by an ancestor policy. Remove this conflict to enable the RateLimitZone(s)."

When there is a `zoneName` conflict between two `RateLimitPolicies` at the same level (Gateway-Gateway, or Route-Route), the following condition will be set on the `RateLimitPolicy` which was created after the other:

- **Condition Type**: `Programmed`
- **Status**: `False`
- **Reason**: `PartiallyInvalid` (implementation-specific reason)
- **Message**: "Policy is not fully programmed: RateLimitZone(s) with zoneName(s): (zoneName(s) here) are in conflict with an existing RateLimitZone with the same zoneName. Remove this conflict to enable the RateLimitZone(s)"

No condition will be set on the winning `RateLimitPolicy` which has its `RateLimitZone` generated in NGINX configuration.

These conditions informs users that their policy configuration has not been fully programmed to the data plane due to `RateLimitZone` conflicts.

#### Setting Status on Objects Affected by a Policy

In the Policy Attachment GEP, there's a provisional status described [here](https://gateway-api.sigs.k8s.io/geps/gep-713/#target-object-status) that involves adding a Condition to all objects affected by a Policy.

This solution gives the object owners some knowledge that their object is affected by a policy but minimizes status updates by limiting them to when the affected object starts or stops being affected by a policy.

Implementing this involves defining a new Condition type and reason:

```go
package conditions

import (
    gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
    RateLimitPolicyAffected gatewayv1alpha2.PolicyConditionType = "gateway.nginx.org/RateLimitPolicyAffected"
    PolicyAffectedReason gatewayv1alpha2.PolicyConditionReason = "RateLimitPolicyAffectedAffected"
)
```

NGINX Gateway Fabric must set this Condition on all HTTPRoutes, GRPCRoutes, and Gateways affected by a `RateLimitPolicyAffected`.
Below is an example of what this Condition may look like:

```yaml
Conditions:
  Type:                  gateway.nginx.org/RateLimitPolicyAffected
  Message:               The RateLimitPolicy is applied to the resource.
  Observed Generation:   1
  Reason:                PolicyAffected
  Status:                True
```

Some additional rules:

- This Condition should be added when the affected object starts being affected by a `RateLimitPolicy`.
- If an object is affected by multiple `RateLimitPolicy` instances, only one Condition should exist.
- When the last `RateLimitPolicy` affecting that object is removed, the Condition should be removed.
- The Observed Generation is the generation of the affected object, not the generation of the `RateLimitPolicy`.

### YAML

Below is an example of `RateLimitPolicy` YAML definition:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: RateLimitPolicy
metadata:
  name: example-rl-policy
  namespace: default
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: example-gateway
  rateLimit:
    local:
      zones:
      - zoneName: zone_one
        rate: 5r/s
        key: $binary_remote_addr
        zoneSize: 10m
      rules:
      - zoneName: zone_one
        delay: 5
        noDelay: false
        burst: 5
        dryRun: false
        logLevel: error
        rejectCode: 503
        condition:
          jwt:
            claim: user_details.level
            match: premium
          default: false
    global:
      zones:
      - zoneName: global_zone_one
        rate: 100r/s
        key: $binary_remote_addr
        zoneSize: 10m
      rules:
      - zoneName: global_zone_one
        delay: 5
        noDelay: false
        burst: 5
        dryRun: false
        logLevel: error
        rejectCode: 503
        condition:
          jwt:
            claim: user_details.level
            match: premium
          default: false
status:
  ancestors:
  - ancestorRef:
      group: gateway.networking.k8s.io
      kind: Gateway
      name: example-gateway
      namespace: default
    conditions:
    - type: Accepted
      status: "True"
      reason: Accepted
      message: Policy is accepted
    - type: Programmed
      status: "True"
      reason: Programmed
      message: Policy is programmed
```

And an example attached to an HTTPRoute and GRPCRoute:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: RateLimitPolicy
metadata:
  name: example-rl-policy
  namespace: default
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: http-route
  - group: gateway.networking.k8s.io
    kind: GRPCRoute
    name: grpc-route
  rateLimit:
    local:
      rules:
      - zoneName: zone_one
        delay: 5
        noDelay: false
        burst: 5
        dryRun: false
        logLevel: error
        rejectCode: 503
        condition:
          variable:
            name: $request_method
            match: GET
          default: false
```

## Attachment and Inheritance

The `RateLimitPolicy` may be attached to Gateways, HTTPRoutes, and GRPCRoutes.

There are three possible attachment scenarios:

**1. Gateway Attachment**

When a `RateLimitPolicy` is attached to a Gateway only, all the HTTPRoutes and GRPCRoutes attached to the Gateway inherit the rate limit settings. However, the rate limit zone in the policy is only created once at the top level `http` directive. All the rate limit rules are propogated downwards to the `location` directives of the HTTPRoutes and GRPCRoutes attached to the Gateway.

**2: Route Attachment**

When a `RateLimitPolicy` is attached to an HTTPRoute or GRPCRoute only, the settings in that policy apply to that Route only. The rate limit zone in the policy will be created at the top level `http` directive, but the rate limit rules in the `location` directives of the route will only exist on routes with the `RateLimitPolicy` attached. Other Routes attached to the same Gateway will not have the rate limit rules applied to them.

**3: Gateway and Route Attachment**

When a `RateLimitPolicy` is attached to a Gateway and one or more of the Routes that are attached to that Gateway, the effective policy is calculated by doing a Patch overrides merge strategy for rate limit zones based on conflicts in `zoneName`, and an Atomic defaults merge strategy for rate limit rules if there exist rate limit rules defined in both the Gateway and Route level.

When calculating conflicts in `zoneName` for a rate limit zone between a policy attached on a Gateway and a different one attached to the Route, the policy attached to the Gateway will have it's defined rate limit zone be the effective one for that `zoneName`.

However for rate limit rules, when there exists a rate limit rule in a policy attached on a Gateway and a different one attached to the Route, the policy attached to the Route will have it's defined rate limit rule(s) be the effective one(s).

This allows a `RateLimitPolicy` attached to a Route to overwrite any settings on a rate limit rule for their specific upstreams, while protecting any rate limit zones set by a `RateLimitPolicy` on a Gateway. If a `RateLimitPolicy` on a Route needs to define a new zone, it will need to find a name that does not conflict with a `RateLimitPolicy` on another Gateway or Route, meaning it can create a separate zone and rate limit rule if a zone created by a `RateLimitPolicy` attached to a Gateway or different Route don't fit its needs.

For example:

- When there is a a Route with a `RateLimitPolicy` attached that sets a rate limit zone named `zone_one` with `rate = 3r/s` and `zoneSize = 5m`, and a Gateway that also has a `RateLimitPolicy` attached that sets a rate limit zone named `zone_one` with `rate = 5/rs` and `zoneSize = 100m`, the effective policy will choose the rate limit zone settings from the Gateway.
- When there is a Route with a `RateLimitPolicy` attached that sets a rate limit rule with `zoneName = default_zone_five` and `burst=5`, and a Gateway that also has a `RateLimitPolicy` attached that sets a rate limit rule with `zoneName = default_zone_three` and `burst = 2` and `noDelay = true`, the effective policy will choose the rate limit rule settings from the HTTPRoute.
- A Route without a policy attached will inherit all settings from the Gateway's policy.

For more information on how to calculate effective policies, see the [hierarchy](https://gateway-api.sigs.k8s.io/geps/gep-713/#hierarchy-of-target-kinds) and [merge strategies](https://gateway-api.sigs.k8s.io/geps/gep-713/#designing-a-merge-strategy) sections in the Policy Attachment GEP. This merge strategy falls into the [custom merge strategy](https://gateway-api.sigs.k8s.io/geps/gep-713/#custom-merge-strategies)

### NGINX Inheritance Behavior

The `limit_req_zone` directive is only available at the `http` context, so any `RateLimitPolicy` that is attached to a Gateway or Route will have that zone inherited/available to other RateLimitPolicies in the same context.

Although the `limit_req` directive is available at these three NGINX contexts: `http`, `server`, and `location`, the directive will only be placed in the `location` directive. This is to make inheritance from the Gateway level downwards to the final locations easier if there are other `limit_req` directives added at the Route level which want to overwrite the Gateway level one.

### Creating the Effective Policy in NGINX Config

The strategy for implementing the effective policy is:

- When a `RateLimitPolicy` is attached to a Gateway, add the `limit_req_zone` directive at the `http` block, and the `limit_req` directive at each of the `location` blocks generated by Routes attached to the Gateway.
- When a `RateLimitPolicy` is attached to an HTTPRoute or GRPCRoute, add the `limit_req_zone` directive at the `http` block, and the `limit_req` directive at each of the `location` blocks generated for the Route.

For both local and global rate limiting, NGINX rate limit configuration should not be generated on internal location blocks generated for the purpose of internal rewriting logic. If done so, a request directed to an external location might be counted multiple times if there are internal locations.

## Testing

- Unit tests for the API validation.
- Functional tests that test the attachment and inheritance behavior, including:
  - Policy attached to Gateway only
  - Policy attached to Route only
  - Policy attached to both Gateway and Route (with inheritance and override scenarios)
  - Policy with various rate limit zone and rules configurations
  - Validation tests for invalid configurations

## Security Considerations

### Validation

Validating all fields in the `RateLimitPolicy` is critical to ensuring that the NGINX config generated by NGINX Gateway Fabric is correct and secure.

All fields in the `RateLimitPolicy` will be validated with OpenAPI Schema validation. If the OpenAPI Schema validation rules are not sufficient, we will use [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

Key validation rules:

- `Size` fields must match the pattern `^\d{1,4}(k|m)?$` to ensure valid NGINX size values
- TargetRef must reference Gateway, HTTPRoute, or GRPCRoute only

### Resource Limits

Due to how NGINX Plus configures zone synchronization under the hood, users with many NGINX Plus instances sharing state through Global Rate Limiting could an increase in consumed network bandwidth and CPU usage. More details on zone synchronization are provided in the official NGINX Plus documentation in [How NGINX Plus Performs Zone Synchronization](https://docs.nginx.com/nginx/admin-guide/high-availability/zone_sync_details/#scaling).

## Alternatives

- **Direct Policy**: If there's no strong use case for the Cluster Operator setting defaults for these settings on a Gateway, we could use a Direct Policy. However, since Rate Limit rules should be able to be defined on both Gateway and Routes, an Inherited Policy is the only Policy type for our solution.
- **ExtensionRef approach**: We could use Gateway API's extensionRef mechanism instead of a Policy. However, Policy attachment is more appropriate for this use case as it follows the established pattern in NGINX Gateway Fabric, provides better status reporting, and allows for Rate Limit rules to be set by the Cluster Operator on a Gateway.

## Future Work

- Add support for configuring NGINX Plus `zone_sync` settings. The defaults we set may not be a one size fits all for users with ranging sizes of NGINX Plus instances.

## References

- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/)
- [NGINX limit_req documentation](https://nginx.org/en/docs/http/ngx_http_limit_req_module.html)
- [NGINX zone_sync documentation](https://nginx.org/en/docs/stream/ngx_stream_zone_sync_module.html)
- [NGINX Plus guide on runtime state sharing](https://docs.nginx.com/nginx/admin-guide/high-availability/zone_sync)
- [NGINX Plus guide detailing how zone sync works](https://docs.nginx.com/nginx/admin-guide/high-availability/zone_sync_details)
- [NGINX Plus guide on Rate Limiting](https://docs.nginx.com/nginx/admin-guide/security-controls/controlling-access-proxied-http/#limiting-the-request-rate)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
