# Enhancement Proposal-2467: Upstream Settings Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/2467
- Status: Implementable

## Summary

This Enhancement Proposal introduces the `UpstreamSettingsPolicy` API that allows Application developers to configure the behavior of the connection between NGINX and their upstream applications. This Policy will attach to a Service that is referenced in an HTTPRoute or GRPCRoute.

## Goals

- Define upstream settings.
- Define an API for upstream settings.

## Non-Goals

- Provide implementation details for implementing the upstream settings policy.
- Define an API for upstream settings for TLSRoute or other layer 4 routes.

## Introduction

### Upstream Settings

Upstream settings are NGINX directives that affect requests sent from NGINX Gateway Fabric to an upstream application.

To begin, the Upstream Settings Policy will include the following NGINX directives:

- [`zone`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#zone)
- [`keepalive`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive)
- [`keepalive_requests`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_requests)
- [`keepalive_time`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_time)
- [`keepalive_timeout`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_timeout)

In the future, we can extend the Upstream Settings Policy to include more [upstream-related directives](nginx-extensions.md#upstream-settings).

## API, Customer Driven Interfaces, and User Experience

The `UpstreamSettingsPolicy` API is a CRD that is a part of the `gateway.nginx.org` Group. It adheres to the guidelines and requirements of a Direct Policy as outlined in the [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/). It will target and be attached to a Service which is referenced in an HTTPRoute or GRPCRoute.

Below is the Golang API for the `UpstreamSettingsPolicy` API:

### Go

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type UpstreamSettingsPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // Spec defines the desired state of the UpstreamSettingsPolicy.
    Spec UpstreamSettingsPolicySpec `json:"spec"`

    // Status defines the state of the UpstreamSettingsPolicy.
    Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

type UpstreamSettingsPolicySpec struct {
    // TargetRefs identifies API object(s) to apply the policy to.
    // Objects must be in the same namespace as the policy.
    // Support: Service
    TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`

    // ZoneSize is the size of the shared memory zone used by the upstream. This memory zone is used to share
    // the upstream configuration between nginx worker processes. The more servers that an upstream has,
    // the larger memory zone is required.
    // Default: OSS: 512k, Plus: 1m.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#zone
    //
    // +optional
    ZoneSize *Size `json:"zoneSize,omitempty"`

    // KeepAlive defines the keep-alive settings.
    //
    // +optional
    KeepAlive *UpstreamKeepAlive `json:"keepAlive,omitempty"`
}

// UpstreamKeepAlive defines the keep-alive settings for upstreams.
type UpstreamKeepAlive struct {
    // Connections sets the maximum number of idle keep-alive connections to upstream servers that are preserved
    // in the cache of each nginx worker process. When this number is exceeded, the least recently used
    // connections are closed.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive
    //
    // +optional
    Connections *int32 `json:"connections,omitempty"`

    // Requests sets the maximum number of requests that can be served through one keep-alive connection.
    // After the maximum number of requests are made, the connection is closed.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_requests
    //
    // +optional
    Requests *int32 `json:"requests,omitempty"`

    // Time defines the maximum time during which requests can be processed through one keep-alive connection.
    // After this time is reached, the connection is closed following the subsequent request processing.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_time
    //
    // +optional
    Time *Duration `json:"time,omitempty"`

    // Timeout defines the keep-alive timeout for upstreams.
    // Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_timeout
    //
    // +optional
    Timeout *Duration `json:"timeout,omitempty"`
}

// Duration is a string value representing a duration in time.
// The format is a subset of the syntax parsed by Golang time.ParseDuration.
// Examples: 1h, 12m, 30s, 150ms.
type Duration string

// Size is a string value representing a size. Size can be specified in bytes, kilobytes (suffix k),
// or megabytes (suffix m).
// Examples: 1024, 8k, 1m.
type Size string
```

### Global zone size setting

Since this Policy only applies to `http` upstreams, there's no way to set the zone size for `stream` upstreams. For now, we can introduce a global `zoneSize` variable in the `NginxProxy` resource that will set the zone size for all upstreams. Then this Policy would override that global setting on upstreams that it attaches to.

### Versioning and Installation

The version of the `UpstreamSettingsPolicy` API will be `v1alpha1`.

The `UpstreamSettingsPolicy` CRD will be installed by the Cluster Operator via Helm or with manifests. It will be required, and if the `UpstreamSettingsPolicy` CRD does not exist in the cluster, NGINX Gateway Fabric will log errors until it is installed.

### Status

#### CRD Label

According to the [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/), the `UpstreamSettingsPolicy` CRD must have the `gateway.networking.k8s.io/policy: direct` label to specify that it is a direct policy.
This label will help with discoverability and will be used by the planned Gateway API Policy [kubectl plugin](https://gateway-api.sigs.k8s.io/geps/gep-713/#kubectl-plugin-or-command-line-tool).

#### Conditions

#### Conditions/Policy Ancestor Status

According to the [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/), the `UpstreamSettingsPolicy` CRD must include a `status` stanza with a slice of Conditions.

The `Accepted` Condition must be populated on the `UpstreamSettingsPolicy` CRD using the reasons defined in the [PolicyCondition API](https://github.com/kubernetes-sigs/gateway-api/blob/main/apis/v1alpha2/policy_types.go). If these reasons are not sufficient, we can add implementation-specific reasons.

If the Policy is attached to a Service that is applied to a TLSRoute, it will not be Accepted.

The Condition stanza must be namespaced using the `controllerName` since more than one controller could reconcile the Policy.

In the [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/), the `PolicyAncestorStatus` applies to Direct Policies.
[`PolicyAncestorStatus`](https://github.com/kubernetes-sigs/gateway-api/blob/f1758d1bc233d78a3e1e6cfba34336526655d03d/apis/v1alpha2/policy_types.go#L156) contains a list of ancestor resources that are associated with the policy, and the status of the policy for each ancestor.
This status provides a view of the resources the policy is affecting. It is beneficial for policies implemented by multiple controllers (e.g., BackendTLSPolicy) or that attach to resources with different capabilities.

#### Setting Status on Objects Affected by a Policy

The [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/) mentions adding a Condition or label to all objects affected by a Policy.

This solution gives the object owners some knowledge that their object is affected by a policy but minimizes status updates by limiting them to when the affected object starts or stops being affected by a policy.

The first step is adding the `gateway.nginx.org/UpstreamSettingsPolicyAffected: true` label to the affected Service. We also must set this Condition on all Routes that reference a Service affected by an `UpstreamSettingsPolicy`.
Below is an example of what this Condition may look like:

```yaml
Conditions:
  Type:                  gateway.nginx.org/UpstreamSettingsPolicyAffected
  Message:               Object affected by an UpstreamSettingsPolicy.
  Observed Generation:   1
  Reason:                PolicyAffected
  Status:                True
```

Implementing this involves defining a new Condition type and reason:

```go
package conditions

import (
    gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
    UpstreamSettingsPolicyAffected gatewayv1alpha2.PolicyConditionType = "gateway.nginx.org/UpstreamSettingsPolicyAffected"
    PolicyAffectedReason gatewayv1alpha2.PolicyConditionReason = "PolicyAffected"
)
```

Some additional rules:

- This Condition and label should be added when the affected object starts being affected by an `UpstreamSettingsPolicy`.
- When the last `UpstreamSettingsPolicy` affecting that object is removed, the Condition and label should be removed.
- The Observed Generation is the generation of the affected object, not the generation of the `UpstreamSettingsPolicy`.

## Use Cases

- As an Application Developer, I want to be able to configure upstream settings for my application based on its behavior or requirements.
  - I may have a large number of Pods for my Service and therefore need a larger memory zone for it.
  - I may want to alter the keepalive settings for my upstream.

## Testing

- Unit tests
- Functional tests that verify the attachment of the CRD to a Service, and that NGINX behaves properly based on the configuration.

## Security Considerations

Validating all fields in the `UpstreamSettingsPolicy` is critical to ensuring that the NGINX config generated by NGINX Gateway Fabric is correct and secure.

All fields in the `UpstreamSettingsPolicy` will be validated with Open API Schema. If the Open API Schema validation rules are not sufficient, we will use [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

RBAC via the Kubernetes API server will ensure that only authorized users can update the CRD.

## Future Work

- Add support for more [upstream-related directives](nginx-extensions.md#upstream-settings).
- Add a `StreamUpstreamSettingsPolicy` that configures upstream settings for stream servers (TLSRoute, TCPRoute).

## Alternatives

- Single policy that contains `http` and `stream` fields for both types of upstreams. It was decided that this isn't as good of a user experience versus just having separate policies with a more targeted purpose.
- Single policy with all top level fields, where some would only be applied depending on the type of upstream, and have conditions that say which fields were ignored. This was rejected for similar reasoning.

## References

- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Policy and Metaresources GEP](https://gateway-api.sigs.k8s.io/geps/gep-713/)
- [Direct Policy Attachment GEP](https://gateway-api.sigs.k8s.io/geps/gep-2648/)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
