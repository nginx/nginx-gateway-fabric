---
title: "Custom policies"
weight: 300
toc: true
docs: "DOCS-000"
---

## Overview

Custom policies are NGINX Gateway Fabric CRDs (Custom Resource Definitions) that allow users to configure NGINX data plane features that are unavailable in the Gateway API.
These custom policies follow the Gateway API [Policy Attachment](https://gateway-api.sigs.k8s.io/reference/policy-attachment/) pattern, which allows users to extend the Gateway API functionality by creating implementation-specific policies and attaching them to Kubernetes objects such as HTTPRoutes, Gateways, and Services.

Policies are a Kubernetes object that augments the behavior of an object in a standard way. Policies can be attached to one object ([Direct Policy Attachment](#direct-policy-attachment)) or objects in a hierarchy ([Inherited Policy Attachment](#inherited-policy-attachment)).
The following table summarizes NGINX Gateway Fabric custom policies:

{{< bootstrap-table "table table-striped table-bordered" >}}

| Policy                                                                                    | Description                                                           | Attachment Type | Supported Target Object(s)    | Supports Multiple Target Refs | Mergeable | API Version |
|-------------------------------------------------------------------------------------------|-----------------------------------------------------------------------|-----------------|-------------------------------|-------------------------------|-----------|-------------|
| [ClientSettingsPolicy]({{<relref "/how-to/traffic-management/client-settings.md" >}})     | Configure connection behavior between client and NGINX                | Inherited       | Gateway, HTTPRoute, GRPCRoute | No                            | Yes       | v1alpha1    |
| [ObservabilityPolicy]({{<relref "/how-to/monitoring/tracing.md" >}})                      | Define settings related to tracing, metrics, or logging               | Direct          | HTTPRoute, GRPCRoute          | Yes                           | No        | v1alpha2    |
| [UpstreamSettingsPolicy]({{<relref "/how-to/traffic-management/upstream-settings.md" >}}) | Configure connection behavior between NGINX and upstream applications | Direct          | Service                       | Yes                           | Yes       | v1alpha1    |

{{</bootstrap-table>}}

{{< important >}}
If attaching a Policy to a Route, that Route must not share a hostname:port/path combination with any other Route that is not referenced by the same Policy. If it does, the Policy will be rejected. This is because the Policy would end up affecting other Routes that it is not attached to.
{{< /important >}}

## Terminology

- _Attachment Type_. How the policy attaches to an object. Attachment type can be "direct" or "inherited".
- _Supported Target Object(s)_. API objects the policy can be applied to.
- _Supports Multiple Target Refs_. Whether a single policy can target multiple objects.
- _Mergeable_. Whether policies that target the same object can be merged.

## Direct Policy Attachment

A Direct Policy Attachment is a policy that references a single object, such as a Gateway or HTTPRoute. It is tightly bound to one instance of a particular Kind within a single Namespace or an instance of a single Kind at the cluster-scope. It affects _only_ the object specified in its TargetRef.

This diagram uses a fictional retry policy to show how Direct Policy Attachment works:

{{<img src="img/direct-policy-attachment.png" alt="">}}

The policy targets the HTTPRoute `baz` and sets `retries` to `3` and `timeout` to `60s`. Since this policy is a Direct Policy Attachment, its settings are only applied to the `baz` HTTPRoute.

## Inherited Policy Attachment

Inherited Policy Attachment allows settings to cascade down a hierarchy. The hierarchy for Gateway API resources looks like this:

{{<img src="img/hierarchy.png" alt="">}}

Settings defined in a policy attached to an object in this hierarchy may be inherited by the resources below it. For example, the settings defined in a policy attached to a Gateway may be inherited by all the HTTPRoutes attached to that Gateway.

Settings in an Inherited Policy can be Defaults or Overrides. Defaults set the default value for something and can be overridden by policies on a lower object. Overrides cannot be overridden by lower objects.
All settings in NGINX Gateway Fabric Inherited Policies are Defaults.

Default values are given precedence from the bottom up. Therefore, a policy setting attached to a Backend will have the highest precedence over the one attached to higher objects.

The following diagram shows how Inherited Policies work in NGINX Gateway Fabric using a fictional retry policy:

{{<img src="img/inherited-policy-attachment.png" alt="">}}

There are three policies defined:

- `dev-policy` that targets the `dev` Gateway
- `baz-policy` that targets the `baz` HTTPRoute
- `foo-policy` that targets the `foo` HTTPRoute

The settings in `dev-policy` affect the `dev` Gateway and are inherited by all the HTTPRoutes attached to `dev`.
The `baz-policy` and `foo-policy` are attached to the `baz` and `foo` HTTPRoutes. Since HTTPRoutes are lower than Gateways in the hierarchy, the settings defined in them override those in the `dev` policy.
Since the `foo-policy` only defines the `retries` setting, it still inherits the `timeout` setting from `dev-policy`.
The `bar` HTTPRoute has no policy attached to it and inherits all the settings from `dev-policy`.

## Merging Policies

With some NGINX Gateway Fabric Policies, it is possible to create multiple policies that target the same resource as long as the fields in those policies do not conflict.

For example, consider the following fictional policies:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: ExamplePolicy
metadata:
  name: retries
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: foo
  retries: 10
```


```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: ExamplePolicy
metadata:
  name: timeout
spec:
  targetRef:
    kind: HTTPRoute
    name: foo
  timeout: 60s
```

The `retries` ExamplePolicy defines the number of retries for the `foo` HTTPRoute, and the `timeout` ExamplePolicy defines the timeout for the `foo` HTTPRoute.
NGINX Gateway Fabric will merge the fields defined in the policies and apply the following settings to the `foo` HTTPRoute:

```yaml
retries: 10
timeout: 60s
```

However, if both policies had the `retries` field set, then the policies cannot be merged. In this case, NGINX Gateway Fabric will choose which policy to configure based on the following criteria (continuing on ties):

1. The oldest policy by creation timestamp
1. The policy appearing first in alphabetical order by "{namespace}/{name}"

If a policy conflicts with a configured policy, NGINX Gateway Fabric will set the policy `Accepted` status to false with a reason of `Conflicted`. See [Policy Status](#policy-status) for more details.

## Policy Status

NGINX Gateway Fabric sets the [PolicyStatus](https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1alpha2.PolicyStatus) on all policies.

`PolicyStatus` fields:

- `ancestors`: describes the status of a route with respect to the ancestor.
  - `ancestorRef`: the object that the policy targets in `spec.targetRef`.
  - `controllerName`: the controller name of NGINX Gateway Fabric.
  - `conditions`: (Condition/Status/Reason):
    - `Accepted/True/Accepted`: the policy is accepted by the ancestor.
    - `Accepted/False/Invalid`: the policy is not accepted because it is semantically or syntactically invalid.
    - `Accepted/False/Conflicted`: the policy is not accepted because it conflicts with another policy.
    - `Accepted/False/TargetNotFound`: the policy is not accepted because it targets a resource that is invalid or does not exist.
    - `Accepted/False/NginxProxyNotSet`: the policy is not accepted because it relies on the NginxProxy configuration which is missing or invalid.

To check the status of a policy, use `kubectl describe`. This example checks the status of the `foo` ObservabilityPolicy, which is accepted:

```shell
kubectl describe observabilitypolicies.gateway.nginx.org foo -n default
```

```text
Status:
  Ancestors:
    Ancestor Ref:
      Group:      gateway.networking.k8s.io
      Kind:       HTTPRoute
      Name:       foo
      Namespace:  default
    Conditions:
      Last Transition Time:  2024-05-23T18:13:03Z
      Message:               Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
```
