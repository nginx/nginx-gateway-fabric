# Enhancement Proposal-1775: Gateway Settings

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/1775
- Status: Completed

## Summary

This Enhancement Proposal introduces the `NginxProxy` API, which allows Cluster Operators to define configuration at the GatewayClass level for all Gateways (proxies) under that Class. This configuration is attached via the GatewayClass `parametersRef` field.

## Goals

- Define the Gateway settings.
- Define an API for the NginxProxy CRD.

## Non-Goals

- Provide implementation details for implementing the NginxProxy configuration.

## Introduction

### Gateway Settings

Gateway settings are NGINX directives or configuration attached at the GatewayClass level that should be solely controlled by the Cluster Operator and should not be changed by the Application Developers. All Gateways attached to this GatewayClass will inherit these settings.

These settings apply to the `main`, `http`, and/or `stream` contexts of the NGINX config. The `NginxProxy` CRD will contain these settings.

To begin, the `NginxProxy` CRD will include the following NGINX directives (focusing on OpenTelemetry tracing):

- [`otel_exporter`](https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter)
- [`otel_service_name`](https://nginx.org/en/docs/ngx_otel_module.html#otel_service_name)
- [`otel_span_attr`](https://nginx.org/en/docs/ngx_otel_module.html#otel_span_attr): set global span attributes that will be merged with the span attributes set in the [Observability extension](observability.md).

In the future, this config will be extended to support other directives, such as those defined in the [NGINX Extensions Proposal](nginx-extensions.md#gateway-settings).

## API, Customer Driven Interfaces, and User Experience

The `NginxProxy` API is a CRD that is a part of the `gateway.nginx.org` Group. It will be referenced in the `parametersRef` field of a GatewayClass. It will live at the cluster scope.

This is a dynamic configuration that can be changed by a user at any time, and NGF will propagate those changes to NGINX. This is something we need to clearly document in our public documentation about this feature, so that users know that all Gateways under the Class can be updated by these settings.

For example, an `NginxProxy` named `proxy-settings` would be referenced as follows:

```yaml
kind: GatewayClass
metadata:
    name: nginx
spec:
    controllerName: gateway.nginx.org/nginx-gateway-controller
    parametersRef:
        group: gateway.nginx.org/v1alpha1
        kind: NginxProxy
        name: proxy-settings
```

Below is the Golang API for the `NginxProxy` API:

### Go

```go
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type NginxProxy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // Spec defines the desired state of the NginxProxy.
    Spec NginxProxySpec `json:"spec"`
}

type NginxProxySpec struct {
    // Telemetry specifies the OpenTelemetry configuration.
    //
    // +optional
    Telemetry *Telemetry `json:"telemetry,omitempty"`
}

// Telemetry specifies the OpenTelemetry configuration.
type Telemetry struct {
    // Exporter specifies OpenTelemetry export parameters.
    //
    // +optional
    Exporter *TelemetryExporter `json:"exporter,omitempty"`

    // ServiceName is the "service.name" attribute of the OpenTelemetry resource.
    // Default is 'ngf:<gateway-namespace>:<gateway-name>'. If a value is provided by the user,
    // then the default becomes a prefix to that value.
    //
    // +optional
    ServiceName *string `json:"serviceName,omitempty"`

    // SpanAttributes are custom key/value attributes that are added to each span.
    //
    // +optional
    SpanAttributes []SpanAttribute `json:"spanAttributes,omitempty"`
}

// TelemetryExporter specifies OpenTelemetry export parameters.
type TelemetryExporter struct {
    // Interval is the maximum interval between two exports, by default is 5 seconds.
    //
    // +optional
    Interval *Duration `json:"interval,omitempty"`

    // BatchSize is the maximum number of spans to be sent in one batch per worker, by default is 512.
    //
    // +optional
    BatchSize *int32 `json:"batchSize,omitempty"`

    // BatchCount is the number of pending batches per worker, spans exceeding the limit are dropped,
    // by default is 4.
    //
    // +optional
    BatchCount *int32 `json:"batchCount,omitempty"`

    // Endpoint is the address of OTLP/gRPC endpoint that will accept telemetry data.
    Endpoint string `json:"endpoint"`
}

// Duration is a string value representing a duration in time.
// The format is a subset of the syntax parsed by Golang time.ParseDuration.
// Examples: 1h, 12m, 30s, 150ms.
type Duration string

// SpanAttribute is a key value pair to be added to a tracing span.
type SpanAttribute struct {
	// Key is the key for a span attribute.
	Key string `json:"key"`

	// Value is the value for a span attribute.
	Value string `json:"value"`
}
```

### Status

#### GatewayClass

There are two Conditions on the GatewayClass status to consider when using the `parametersRef`. The first is a `ResolvedRefs` Condition. If the `NginxProxy` reference cannot be found, then this Condition is set to `False`.

NGINX Gateway Fabric must set this Condition on the GatewayClass affected by an `NginxProxy`.
Below is an example of what this Condition may look like:

```yaml
Conditions:
  Type:                  ResolvedRefs
  Message:               All references are resolved
  Observed Generation:   1
  Reason:                ResolvedRefs
  Status:                True
```

Some additional rules:

- This Condition should be added when the affected object starts being affected by an `NginxProxy`.
- When the `NginxProxy` affecting that object is removed, the Condition should be removed.
- The Observed Generation is the generation of the GatewayClass, not the generation of the `NginxProxy`.

The other condition is the existing `Accepted` condition on the GatewayClass. There is an existing reason for this Condition, `InvalidParameters`, that sets `Accepted` to `False` when the `parametersRef` fields are invalid. However, this could lead to downstream problems. For example, if a GatewayClass is `Accepted`, and then `NginxProxy` are updated to something invalid, then marking the GatewayClass as `not Accepted` would result in the entire downstream configuration tree being nullified. This is a large disruption.

The proposition is to instead keep the GatewayClass as `Accepted` even if the `NginxProxy` are invalid, but still set the reason to `InvalidParameters` and include a detailed message about the issue. In this case, default values will be reverted to for the settings, and downstream configuration will remain intact. There may be impact by reverting to defaults, but this impact is likely less than the impact of completely nullifying all Gateways/Routes/etc. that live under the GatewayClass if we instead marked it as `not Accepted`.

If this scenario occurs, we must be clear about what happened. Log an error, an event, and write the status. We should also set the generation to the last known good state of the resource.

## Use Cases

- As a Cluster Operator, I want to set global settings that will apply to all Gateways that are a part of a GatewayClass. These settings should not be overriden at a lower level.

## Testing

- Unit tests
- Functional tests that verify the attachment of the CRD to the GatewayClass, and that NGINX behaves properly based on the configuration. This includes verifying tracing works as expected.

## Security Considerations

Validating all fields in the `NginxProxy` is critical to ensuring that the NGINX config generated by NGINX Gateway Fabric is correct and secure.

All fields in the `NginxProxy` will be validated with Open API Schema. If the Open API Schema validation rules are not sufficient, we will use [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

RBAC via the Kubernetes API server will ensure that only authorized users can update the CRD.

## Alternatives

- ParametersRef with ConfigMap: A ConfigMap is another resource type where a user can provide configuration options. However, unlike CRDs, ConfigMaps do not have built-in schema validation, versioning, or conversion webhooks.
- Direct Policy: A Direct Policy may also work for Gateway settings. It can be attached to a Gateway and scoped to Cluster Operators through RBAC. It would allow Cluster Operators to apply settings for specific Gateways, instead of all Gateways.

## References

- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Attaching Policy to GatewayClass](https://gateway-api.sigs.k8s.io/geps/gep-713/#attaching-policy-to-gatewayclass)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
