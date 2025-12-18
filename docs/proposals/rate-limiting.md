# Enhancement Proposal-4059: Rate Limit Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4059
- Status: Implementable

## Summary

This Enhancement Proposal introduces the "RateLimitPolicy" API that allows Cluster Operators and Application Developers to configure NGINX's rate limiting settings for Local Rate Limiting (RL per instance) and Global Rate Limiting (RL across all instances). Local Rate Limiting will be available on OSS through the `ngx_http_limit_req_module`. Global Rate Limiting will only be available through NGINX Plus, building off the OSS implementation but also using the `ngx_stream_zone_sync_module` to share state between NGINX instances, however that is out of scope for the current design.

## Goals

- Define rate limiting settings.
- Outline attachment points (Gateway and HTTPRoute/GRPCRoute) for the rate limit policy.
- Describe inheritance behavior of rate limiting settings when multiple policies exist at different levels.

## Non-Goals

- Champion a Rate Limiting Gateway API contribution.
- Support for attachment to TLSRoute.
- Support Global Rate Limiting
- Support Conditional Rate Limiting

## Introduction

Rate limiting is a feature in NGINX which allows users to limit the request processing rate per a defined key, which usually refers to processing rate of requests coming from a single IP address. However, this key can contain text, variables, or a combination of them. Rate limiting through a reverse proxy can be broadly broken down into two different categories: Local Rate Limiting, and Global Rate Limiting. Global Rate Limiting is out of scope for this enhancement proposal.

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

## Use Cases

- As a Cluster Operator:
  - I want to set Local Rate Limits on NGINX instances to:
    - Provide a default for NGINX instances.
    - Create protection for non-critical paths that don't need expensive Global Rate Limits.
- As an Application Operator:
  - I want to set Local Rate Limits for my specific application to:
    - Act as a circuit-breaker for heavy endpoints.
    - Enable Canary / blue-green safety.
    - Add additional security to developer namespaces.

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

// RateLimitPolicy is an Inherited Attached Policy. It provides a way to set local rate limiting rules in NGINX.
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

// RateLimit contains settings for Rate Limiting.
type RateLimit struct {
    // Local defines the local rate limit rules for this policy.
    //
    // +optional
    Local *LocalRateLimit `json:"local,omitempty"`
}

// LocalRateLimit contains the local rate limit rules.
type LocalRateLimit struct {
    // Rules contains the list of rate limit rules.
    //
    // +optional
    Rules *RateLimitRule[] `json:"rules,omitempty"`
}

// RateLimitRule contains settings for a RateLimit Rule.
type RateLimitRule struct {
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
    //
    // +optional
    ZoneSize *Size `json:"zoneSize"`

    // Delay specifies a limit at which excessive requests become delayed. Default value is zero, which means all excessive requests are delayed.
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
}

// Size is a string value representing a size. Size can be specified in bytes, kilobytes (k), megabytes (m).
// Examples: 1024, 8k, 1m.
//
// +kubebuilder:validation:Pattern=`^\d{1,4}(k|m)?$`
type Size string

// Rate is a string value representing a rate. Rate can be specified in r/s or r/m.
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
      rules:
      - rate: 5r/s
        key: $binary_remote_addr
        zoneSize: 10m
        delay: 5
        noDelay: false
        burst: 5
        dryRun: false
        logLevel: error
        rejectCode: 503
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
  name: example-rl-policy-routes
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
      - rate: 5r/s
        key: $binary_remote_addr
        zoneSize: 10m
        delay: 5
        noDelay: false
        burst: 5
        dryRun: false
        logLevel: error
        rejectCode: 503
```

## Attachment and Inheritance

The `RateLimitPolicy` may be attached to Gateways, HTTPRoutes, and GRPCRoutes.

There are three possible attachment scenarios:

**1. Gateway Attachment**

When a `RateLimitPolicy` is attached to a Gateway only, all the HTTPRoutes and GRPCRoutes attached to the Gateway inherit the rate limit settings. A singular rate limit zone is created for the Gateway, and `limit_req` directives targeting the zone are propagated downwards to the `location` directives of the HTTPRoutes and GRPCRoutes attached to the Gateway.

**2: Route Attachment**

When a `RateLimitPolicy` is attached to an HTTPRoute or GRPCRoute only, the settings in that policy apply to that Route only. The rate limit zone in the policy will be created at the top level `http` directive, but the rate limit rules in the `location` directives of the route will only exist on routes with the `RateLimitPolicy` attached. Other Routes attached to the same Gateway will not have the rate limit rules applied to them.

**3: Gateway and Route Attachment**

When a `RateLimitPolicy` is attached to a Gateway and one or more of the Routes that are attached to that Gateway, there is no conflict in policies. The `RateLimitPolicy` attached to the Gateway will generate its rate limit rule that gets applied to all the Routes attached to the Gateway and the `RateLimitPolicy` attached to the Route will generate its rate limit rule that gets applied to its specific `location` directives. In this case, the Route would end up with its own rate limit rule, in addition to the rate limit rule passed down from the Gateway.

As a consequence, there is no way to overwrite / negate a `RateLimitPolicy` from a Gateway by attaching another policy to the Route.

### Creating the Effective Policy in NGINX Config

The strategy for implementing the effective policy is:

- When a `RateLimitPolicy` is attached to a Gateway, generate a singular `limit_req_zone` directive, unique to that policy and Gateway, at the `http` block, and a `limit_req` directive at each of the `location` blocks generated by Routes attached to the Gateway.
- When a `RateLimitPolicy` is attached to an HTTPRoute or GRPCRoute, generate a singular `limit_req_zone`, unique to that policy and Route, directive at the `http` block, and a `limit_req` directive at each of the `location` blocks generated for the Route.
- When multiple `RateLimitPolicies` are attached to a Gateway, generate a unique `limit_req_zone` for each policy-gateway pair.
- When a `RateLimitPolicy` is attached to a Gateway, and there exists a Route which is attached to that Gateway which also has a `RateLimitPolicy` attached to it, the `location` blocks generated for that Route will have the `limit_req` directive with the Gateway `RateLimitPolicy` zone, and whatever `limit_req` directives generated by the `RateLimitPolicy` attached to the Route.
- When a `RateLimitPolicy` is targeting a Gateway and Routes that are attached to the same Gateway, only a singular `limit_req_zone`, unique to that policy and Gateway is generated, and the `location` blocks from the Routes contain a `limit_req` directive targeting that zone.

NGINX rate limit configuration should not be generated on internal location blocks generated for the purpose of internal rewriting logic. If done so, a request directed to an external location might be counted multiple times if there are internal locations.

## Testing

- Unit tests for the API validation.
- Functional tests that test the attachment and inheritance behavior, including:
  - Policy attached to Gateway only
  - Policy attached to Route only
  - Policy attached to both Gateway and Route
  - Policy with various rate rule configurations
  - Validation tests for invalid configurations

## Security Considerations

### Validation

Validating all fields in the `RateLimitPolicy` is critical to ensuring that the NGINX config generated by NGINX Gateway Fabric is correct and secure.

All fields in the `RateLimitPolicy` will be validated with OpenAPI Schema validation. If the OpenAPI Schema validation rules are not sufficient, we will use [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

Key validation rules:

- `Size` fields must match the pattern `^\d{1,4}(k|m)?$` to ensure valid NGINX size values
- TargetRef must reference Gateway, HTTPRoute, or GRPCRoute only

## Alternatives

- **Direct Policy**: If there's no strong use case for the Cluster Operator setting defaults for these settings on a Gateway, we could use a Direct Policy. However, since Rate Limit rules should be able to be defined on both Gateway and Routes, an Inherited Policy is the only Policy type for our solution.
- **ExtensionRef approach**: We could use Gateway API's extensionRef, aka Filter option, mechanism instead of a Policy. However, Policy attachment is more appropriate for this use case as it follows the established pattern in NGINX Gateway Fabric, provides better status reporting, and allows for Rate Limit rules to be set by the Cluster Operator on a Gateway.
- Allow `RateLimitPolicies` attached at the Route level to overwrite rules set at the Gateway level. Currently if a Route `location` inherits a rate limit rule from a Gateway, there is no way to disable it or override it. The workaround around this problem is to either remove the Route from the Gateway, or remove the `RateLimitPolicy` from attaching at the Gateway level, and instead attach to the Routes on the Gateway. However, this is inconvinient and may be a common scenario warranting supporting through either a field in the `RateLimitRule` or changing how `RateLimitPolicies` interact with each other.

## Future Work

- Add support for global rate limiting. In NGINX Plus, this can be done by using the `ngx_stream_zone_sync_module` to extend the solution for Local Rate Limiting and provide a way for synchronizing contents of shared memory zones across NGINX Plus instances. Support for `zone_sync` is a separate enhancement and can either be completed along side global rate limiting support or separately.
- Add Conditional Rate Limiting. Users would also like to set conditions for a rate limit policy, where if a certain condition isn't met, the request would either go to a default rate limit policy, or would not be rate limited. This is designed to be used in combination with one or more rate limit policies. For example, multiple rate limit policies with that condition on JWT level can be used to apply different tiers of rate limit based on the value of a JWT claim (ie. more req/s for a higher level, less req/s for a lower level).
- Add some sort of Scale field for local rate limiting. This would dynamically calculate the rate of a `RateLimitPolicy` based on number of NGINX replicas.

## References

- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/)
- [NGINX limit_req documentation](https://nginx.org/en/docs/http/ngx_http_limit_req_module.html)
- [NGINX Plus guide on Rate Limiting](https://docs.nginx.com/nginx/admin-guide/security-controls/controlling-access-proxied-http/#limiting-the-request-rate)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
