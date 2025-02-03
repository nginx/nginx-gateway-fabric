# Enhancement Proposal-1778: Observability Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/1778
- Status: Completed

## Summary

This Enhancement Proposal introduces the `ObservabilityPolicy` API, which allows Application Developers to define settings related to tracing, metrics, or logging at the HTTPRoute level.

## Goals

- Define the Observability policy.
- Define an API for the Observability policy.

## Non-Goals

- Provide implementation details for implementing the Observability policy.

## Introduction

### Observability Policy

The Observability Policy contains settings to configure NGINX to expose information through tracing, metrics, and/or logging. This is a Direct Policy that is attached to an HTTPRoute by an Application Developer. It works in conjunction with an [NginxProxy](gateway-settings.md) configuration that contains higher level settings to enable Observability at this lower level. The [NginxProxy](gateway-settings.md) configuration is managed by a Cluster Operator.

Since this policy is attached to an HTTPRoute, the Observability settings should just apply to the relevant `location` contexts of the NGINX config for that route.

To begin, the Observability Policy will include the following NGINX directives (focusing on OpenTelemetry tracing):

- [`otel_trace`](https://nginx.org/en/docs/ngx_otel_module.html#otel_trace): enable tracing and set sampler rate
- [`otel_trace_context`](https://nginx.org/en/docs/ngx_otel_module.html#otel_trace_context): export, inject, propagate, ignore.
- [`otel_span_name`](https://nginx.org/en/docs/ngx_otel_module.html#otel_span_name)
- [`otel_span_attr`](https://nginx.org/en/docs/ngx_otel_module.html#otel_span_attr): these span attributes will be merged with any set at the global level in the `NginxProxy` config.

Tracing will be disabled by default. The Application Developer will be able to use this Policy to enable and configure tracing for their routes. This Policy will only be applied if the OTel endpoint has been set by the Cluster Operator on the [NginxProxy](gateway-settings.md).

Ratio and parent-based tracing should be supported as shown in the [nginx-otel examples](https://github.com/nginxinc/nginx-otel?tab=readme-ov-file#examples).

In the future, this config will be extended to support other functionality, such as those defined in the [NGINX Extensions Proposal](nginx-extensions.md#observability).

## API, Customer Driven Interfaces, and User Experience

The `ObservabilityPolicy` API is a CRD that is a part of the `gateway.nginx.org` Group. It is a namespaced resource that will reference an HTTPRoute as its target.

### Go

Below is the Golang API for the `ObservabilityPolicy` API:

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type ObservabilityPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // Spec defines the desired state of the ObservabilityPolicy.
    Spec ObservabilityPolicySpec `json:"spec"`

    // Status defines the state of the ObservabilityPolicy.
    Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

type ObservabilityPolicySpec struct {
    // TargetRefs identifies API object(s) to apply the policy to.
    // Objects must be in the same namespace as the policy.
    // Support: HTTPRoute
    TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`

    // Tracing allows for enabling and configuring tracing.
    //
    // +optional
    Tracing *Tracing `json:"tracing,omitempty"`
}

// Tracing allows for enabling and configuring OpenTelemetry tracing.
type Tracing struct {
    // Ratio is the percentage of traffic that should be sampled. Integer from 0 to 100.
    // By default, 100% of http requests are traced. Not applicable for parent-based tracing.
    //
    // +optional
    Ratio *int32 `json:"ratio,omitempty"`

    // Context specifies how to propagate traceparent/tracestate headers. By default is 'ignore'.
    //
    // +optional
    Context *TraceContext `json:"context,omitempty"`

    // SpanName defines the name of the Otel span. By default is the name of the location for a request.
    // If specified, applies to all locations that are created for a route.
    //
    // +optional
    SpanName *string `json:"spanName,omitempty"`

    // SpanAttributes are custom key/value attributes that are added to each span.
    //
    // +optional
    SpanAttributes []SpanAttribute `json:"spanAttributes,omitempty"`

    // Strategy defines if tracing is ratio-based or parent-based.
    Strategy TraceStrategy `json:"strategy"`
}

// TraceStrategy defines the tracing strategy.
type TraceStrategy string

const (
    // TraceStrategyRatio enables ratio-based tracing, defaulting to 100% sampling rate.
    TraceStrategyRatio TraceStrategy = "ratio"

    // TraceStrategyParent enables tracing and only records spans if the parent span was sampled.
    TraceStrategyParent TraceStrategy = "parent"
)

// TraceContext specifies how to propagate traceparent/tracestate headers.
type TraceContext string

const (
    // TraceContextExtract uses an existing trace context from the request, so that the identifiers
    // of a trace and the parent span are inherited from the incoming request.
    TraceContextExtract TraceContext = "extract"

    // TraceContextInject adds a new context to the request, overwriting existing headers, if any.
    TraceContextInject TraceContext = "inject"

    // TraceContextPropagate updates the existing context (combines extract and inject).
    TraceContextPropagate TraceContext = "propagate"

    // TraceContextIgnore skips context headers processing.
    TraceContextIgnore TraceContext = "ignore"
)

// SpanAttribute is a key value pair to be added to a tracing span.
type SpanAttribute struct {
	// Key is the key for a span attribute.
	Key string `json:"key"`

	// Value is the value for a span attribute.
	Value string `json:"value"`
}
```

### YAML

Below is an example YAML version of an `ObservabilityPolicy`:

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: ObservabilityPolicy
metadata:
  name: example-observability-policy
  namespace: default
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: example-route
  tracing:
    strategy: ratio
    ratio: 10
    context: inject
    spanName: example-span
    spanAttributes:
    - key: attribute1
      value: value1
    - key: attribute2
      value: value2
status:
  ancestors:
    ancestorRef:
      group: gateway.networking.k8s.io
      kind: Gateway
      name: example-gateway
      namespace: default
    conditions:
      - type: Accepted
        status: "True"
        reason: Accepted
        message: Policy is accepted
```

### Status

#### CRD Label

According to the [Policy and Metaresources GEP](https://gateway-api.sigs.k8s.io/geps/gep-713/), the `ObservabilityPolicy` CRD must have the `gateway.networking.k8s.io/policy: direct` label to specify that it is a direct policy.
This label will help with discoverability and will be used by the planned Gateway API Policy [kubectl plugin](https://gateway-api.sigs.k8s.io/geps/gep-713/#kubectl-plugin-or-command-line-tool).

#### Conditions/Policy Ancestor Status

According to the [Policy and Metaresources GEP](https://gateway-api.sigs.k8s.io/geps/gep-713/), the `ObservabilityPolicy` CRD must include a `status` stanza with a slice of Conditions.

The `Accepted` Condition must be populated on the `ObservabilityPolicy` CRD using the reasons defined in the [PolicyCondition API](https://github.com/kubernetes-sigs/gateway-api/blob/main/apis/v1alpha2/policy_types.go). If these reasons are not sufficient, we can add implementation-specific reasons.

One reason for being `not Accepted` would be the fact that the `NginxProxy` Policy is not configured, which is a requirement in order for the `ObservabilityPolicy` to work. This will be a custom reason `NginxProxyConfigNotSet`.

The Condition stanza may need to be namespaced using the `controllerName` if more than one controller could reconcile the Policy.

In the updated version of the [Policy and Metaresources GEP](https://github.com/kubernetes-sigs/gateway-api/pull/2813/files), which is still under review, the `PolicyAncestorStatus` applies to Direct Policies.
[`PolicyAncestorStatus`](https://github.com/kubernetes-sigs/gateway-api/blob/f1758d1bc233d78a3e1e6cfba34336526655d03d/apis/v1alpha2/policy_types.go#L156) contains a list of ancestor resources (usually Gateways) that are associated with the policy, and the status of the policy for each ancestor.
This status provides a view of the resources the policy is affecting. It is beneficial for policies implemented by multiple controllers (e.g., BackendTLSPolicy) or that attach to resources with different capabilities.

#### Setting Status on Objects Affected by a Policy

In the Policy and Metaresources GEP, there's a provisional status described [here](https://gateway-api.sigs.k8s.io/geps/gep-713/#standard-status-condition-on-policy-affected-objects) that involves adding a Condition or annotation to all objects affected by a Policy.

This solution gives the object owners some knowledge that their object is affected by a policy but minimizes status updates by limiting them to when the affected object starts or stops being affected by a policy.
Even though this status is provisional, implementing it now will help with discoverability and allow us to give feedback on the solution.

Implementing this involves defining a new Condition type and reason:

```go
package conditions

import (
    gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)


const (
    ObservabilityPolicyAffected gatewayv1alpha2.PolicyConditionType = "gateway.nginx.org/ObservabilityPolicyAffected"
    PolicyAffectedReason gatewayv1alpha2.PolicyConditionReason = "PolicyAffected"
)

```

NGINX Gateway Fabric must set this Condition on all HTTPRoutes affected by an `ObservabilityPolicy`.
Below is an example of what this Condition may look like:

```yaml
Conditions:
  Type:                  gateway.nginx.org/ObservabilityPolicyAffected
  Message:               Object affected by a ObservabilityPolicy.
  Observed Generation:   1
  Reason:                PolicyAffected
  Status:                True
```

Some additional rules:

- This Condition should be added when the affected object starts being affected by a `ObservabilityPolicy`.
- When the last `ObservabilityPolicy` affecting that object is removed, the Condition should be removed.
- The Observed Generation is the generation of the affected object, not the generation of the `ObservabilityPolicy`.

## Attachment

An `ObservabilityPolicy` can be attached to an HTTPRoute.

The policy will only take effect if an [NginxProxy](gateway-settings.md) configuration has been linked to the GatewayClass. Otherwise, the `ObservabilityPolicy` should not be `Accepted`.

Future: Attached to an HTTPRoute rule, using a [sectionName](https://gateway-api.sigs.k8s.io/geps/gep-713/#apply-policies-to-sections-of-a-resource).

### Creating the Effective Policy in NGINX Config

To determine how to reliably and consistently create the effective policy in NGINX config, we need to apply the policies for each attachment scenario to the three NGINX mappings described [here](/docs/developer/mapping.md).

The following examples use the `ClientSettingsPolicy`, but the rules are the same for the `ObservabilityPolicy`.

A. Distinct Hostname:
![example-a2](/docs/images/client-settings/example-a2.png)

B. Same Hostname:
![example-b2](/docs/images/client-settings/example-b2.png)

C. Internal Redirect
![example-c2](/docs/images/client-settings/example-c2.png)

For this attachment scenario, specifying the directives in the _final_ location blocks generated from the HTTPRoute with the policy attached achieves the effective policy. _Final_ means the location that ultimately handles the request.

## Use Cases

- As an Application Developer, I want to enable observability -- such as tracing -- for traffic flowing to my application, so I can easily debug issues or understand the use of my application.

## Testing

- Unit tests
- Functional tests that verify the attachment of the CRD to a Route, and that NGINX behaves properly based on the configuration. This includes verifying tracing works as expected.

## Security Considerations

Validating all fields in the `ObservabilityPolicy` is critical to ensuring that the NGINX config generated by NGINX Gateway Fabric is correct and secure.

All fields in the `ObservabilityPolicy` will be validated with Open API Schema. If the Open API Schema validation rules are not sufficient, we will use [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

RBAC via the Kubernetes API server will ensure that only authorized users can update the CRD.

## Alternatives

- Combine with OTel settings in `NginxProxy` for one OTel Policy: Rather than splitting tracing across two Policies, we could create a single tracing Policy. The issue with this approach is that some tracing settings -- such as exporter endpoint -- should be restricted to Cluster Operators, while settings like attributes should be available to Application Developers. If we combine these settings, RBAC will not be sufficient to restrict access across the settings. We will have to disallow certain fields based on the resource the Policy is attached to. This is a bad user experience.
- Inherited Policy: An Inherited Policy would be useful if there is a use case for the Cluster Operator enforcing or defaulting the OTel tracing settings included in this policy.


## References

- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Policy and Metaresources GEP](https://gateway-api.sigs.k8s.io/geps/gep-713/)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
