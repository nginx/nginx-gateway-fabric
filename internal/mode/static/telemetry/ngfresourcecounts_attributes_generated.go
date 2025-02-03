package telemetry

/*
This is a generated file. DO NOT EDIT.
*/

import (
	"go.opentelemetry.io/otel/attribute"

	ngxTelemetry "github.com/nginx/telemetry-exporter/pkg/telemetry"
)

func (d *NGFResourceCounts) Attributes() []attribute.KeyValue {
	var attrs []attribute.KeyValue
	attrs = append(attrs, attribute.Int64("GatewayCount", d.GatewayCount))
	attrs = append(attrs, attribute.Int64("GatewayClassCount", d.GatewayClassCount))
	attrs = append(attrs, attribute.Int64("HTTPRouteCount", d.HTTPRouteCount))
	attrs = append(attrs, attribute.Int64("TLSRouteCount", d.TLSRouteCount))
	attrs = append(attrs, attribute.Int64("SecretCount", d.SecretCount))
	attrs = append(attrs, attribute.Int64("ServiceCount", d.ServiceCount))
	attrs = append(attrs, attribute.Int64("EndpointCount", d.EndpointCount))
	attrs = append(attrs, attribute.Int64("GRPCRouteCount", d.GRPCRouteCount))
	attrs = append(attrs, attribute.Int64("BackendTLSPolicyCount", d.BackendTLSPolicyCount))
	attrs = append(attrs, attribute.Int64("GatewayAttachedClientSettingsPolicyCount", d.GatewayAttachedClientSettingsPolicyCount))
	attrs = append(attrs, attribute.Int64("RouteAttachedClientSettingsPolicyCount", d.RouteAttachedClientSettingsPolicyCount))
	attrs = append(attrs, attribute.Int64("ObservabilityPolicyCount", d.ObservabilityPolicyCount))
	attrs = append(attrs, attribute.Int64("NginxProxyCount", d.NginxProxyCount))
	attrs = append(attrs, attribute.Int64("SnippetsFilterCount", d.SnippetsFilterCount))
	attrs = append(attrs, attribute.Int64("UpstreamSettingsPolicyCount", d.UpstreamSettingsPolicyCount))

	return attrs
}

var _ ngxTelemetry.Exportable = (*NGFResourceCounts)(nil)
