package telemetry

/*
This is a generated file. DO NOT EDIT.
*/

import (
	"go.opentelemetry.io/otel/attribute"

	ngxTelemetry "github.com/nginx/telemetry-exporter/pkg/telemetry"
)

func (d *Data) Attributes() []attribute.KeyValue {
	var attrs []attribute.KeyValue
	attrs = append(attrs, attribute.String("dataType", "ngf-product-telemetry"))
	attrs = append(attrs, attribute.String("ImageSource", d.ImageSource))
	attrs = append(attrs, d.Data.Attributes()...)
	attrs = append(attrs, attribute.StringSlice("FlagNames", d.FlagNames))
	attrs = append(attrs, attribute.StringSlice("FlagValues", d.FlagValues))
	attrs = append(attrs, attribute.StringSlice("SnippetsFiltersDirectives", d.SnippetsFiltersDirectives))
	attrs = append(attrs, attribute.Int64Slice("SnippetsFiltersDirectivesCount", d.SnippetsFiltersDirectivesCount))
	attrs = append(attrs, d.NGFResourceCounts.Attributes()...)
	attrs = append(attrs, attribute.Int64("NGFReplicaCount", d.NGFReplicaCount))

	return attrs
}

var _ ngxTelemetry.Exportable = (*Data)(nil)
