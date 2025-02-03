# Enhancement Proposal-929: Data Plane Configuration

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/929
- Status: [Replaced](nginx-extensions.md)

## Summary

This proposal is intended to contain the design for how to configure global settings for the data plane
of the NGINX Gateway Fabric (NGF) product. Similar to control plane configuration, we should be able to leverage
a custom resource definition to define data plane configuration, considering fields such as telemetry and
upstream zone size.

## Goals

Define a CRD to configure various global settings for the NGF data plane. The initial configurable
options will be for telemetry (tracing) and upstream zone size.

## Non-Goals

 1. This proposal is not defining every setting that needs to be present in the configuration.
 2. This proposal is not for any configuration related to control plane.

## Introduction

The NGF data plane will evolve to have various user-configurable options. These could include, but are not
limited to, tracing, logging, or metrics. For the best user experience, these options should be able to be
changed at runtime, to avoid having to restart NGF. The first set of options that we will allow users to
configure are tracing and upstream zone size. The easiest and most intuitive way to implement a Kubernetes-native
API is through a CRD.

The purpose of this CRD is to contain "global" configuration options for the data plane, and not focused on policy
per route or backend.

NGF will reload NGINX when configuration changes are made.

In this doc, the term "user" will refer to the cluster operator (the person who installs and manages NGF). The
cluster operator owns this CRD resource.

## API, Customer Driven Interfaces, and User Experience

The API would be provided in a CRD. An authorized user would interact with this CRD using `kubectl` to `get`
or `edit` the configuration.

Proposed configuration CRD example:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: NginxProxy
metadata:
    name: nginx-proxy-config
    namespace: nginx-gateway
spec:
    http:
        upstreamZoneSize: 512k # default
        telemetry:
            tracing:
                enabled: true # default false
                endpoint: my-otel-collector.svc:4317 # required
                interval: 5s # default
                batchSize: 512 # default
                batchCount: 4 # default
status:
    conditions:
    ...
```

- The CRD would be Namespace-scoped.
- CRD is initialized and created when NGF is deployed, in the `nginx-gateway` Namespace.
- CRD would be referenced in the [ParametersReference][ref] of the NGF GatewayClass.
- Conditions include `Accepted` if the CRD config is valid, and `Programmed` to determine if an nginx
reload was successful.

[ref]:https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.ParametersReference

## Use Cases

The high level use case is to configure options in the NGF data plane that are not currently configurable. The
CRD also allows for these to change without the need to restart the NGF Pod.

### Tracing

Users may want to observe how traffic is flowing through their applications. Tracing is a great way to achieve
this. By taking advantage of the OpenTelemetry standards, a user can deploy any OTLP-compliant tracing collector
to receive and visualize tracing data. Allowing a user to configure a tracing backend for NGF will forward
nginx tracing data to that backend for visualization.

For future considerations, a user may want to disable tracing for certain routes (or only enable it for certain
routes), in order to reduce the amount of data being collected. We would likely be able to implement a [per-route
Policy](https://gateway-api.sigs.k8s.io/geps/gep-713/#direct-policy-attachment)
that would include this switch. The proposed "global" CRD in this document would remain unchanged, though
could include an additional field to enable or disable tracing globally.

### Upstream Zone Size

As the number of servers within an upstream increases (in other words, Pod replicas for a Service), the
shared memory zone size needs to increase to accomodate this. A user can fine-tune this number to fit their
environment.

## Testing

Unit tests can be leveraged for verifying that NGF properly watches and acts on CRD changes. These tests would
be similar in behavior as the current unit tests that verify the control plane CRD resource processing.

We would need system level tests to ensure that tracing works as expected.

## Security Considerations

We need to ensure that any configurable fields that are exposed to a user are:

- Properly validated. This means that the fields should be the correct type (integer, string, etc.), have appropriate
length, and use regex patterns or enums to prevent any unwanted input. This will initially be done through
OpenAPI schema validation. If necessary as the CRD evolves, CEL or control plane validation could be used.
- Have a valid use case. The more fields we expose, the more attack vectors we create. We should only be exposing
fields that are genuinely useful for a user to change dynamically.

RBAC via the Kubernetes API server will ensure that only authorized users can update the CRD containing NGF data
plane configuration.

## Alternatives

- ConfigMap
A ConfigMap is another type of resource that a user can provide configuration options within, however it lacks the
benefits of a CRD, specifically built-in schema validation, versioning, and conversion webhooks.

- Custom API server
The NGF control plane could implement its own custom API server. However the overhead of implementing this, which
would include auth, validation, endpoints, and so on, would not be worth it due to the fact that the Kubernetes
API server already does all of these things for us.

- Policies CRD for granular control
Being that these are global settings, a user may have a need for more granular control, in other words, changing
the settings at a per-route or per-backend basis. A new Policy CRD could accomplish this in future work.

## References

- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
