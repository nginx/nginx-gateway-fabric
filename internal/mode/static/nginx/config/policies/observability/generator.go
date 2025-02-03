package observability

import (
	"fmt"
	"text/template"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var (
	tmpl            = template.Must(template.New("observability policy").Parse(observabilityTemplate))
	tmplInternal    = template.Must(template.New("observability policy internal").Parse(internalTemplate))
	tmplExtRedirect = template.Must(template.New("observability policy ext redirect").Parse(externalRedirectTemplate))
)

const observabilityTemplate = `
{{- if .Tracing }}
otel_trace {{ .Strategy }};
  {{- if .Tracing.Context }}
otel_trace_context {{ .Tracing.Context }};
  {{- end }}
  {{- if .Tracing.SpanName }}
otel_span_name "{{ .Tracing.SpanName }}";
  {{- end }}
  {{- range $attr := .Tracing.SpanAttributes }}
otel_span_attr "{{ $attr.Key }}" "{{ $attr.Value }}";
  {{- end }}
  {{- range $attr := .GlobalSpanAttributes }}
otel_span_attr "{{ $attr.Key }}" "{{ $attr.Value }}";
  {{- end }}
{{- end }}
`

const internalTemplate = `
{{- if .Tracing }}
  {{- if .Tracing.SpanName }}
otel_span_name "{{ .Tracing.SpanName }}";
  {{- else }}
otel_span_name $request_uri_path;
  {{- end }}
  {{- range $attr := .Tracing.SpanAttributes }}
otel_span_attr "{{ $attr.Key }}" "{{ $attr.Value }}";
  {{- end }}
  {{- range $attr := .GlobalSpanAttributes }}
otel_span_attr "{{ $attr.Key }}" "{{ $attr.Value }}";
  {{- end }}
{{- end }}
`

const externalRedirectTemplate = `
{{- if .Tracing }}
otel_trace {{ .Strategy }};
  {{- if .Tracing.Context }}
otel_trace_context {{ .Tracing.Context }};
  {{- end }}
{{- end }}
`

// Generator generates nginx configuration based on an observability policy.
type Generator struct {
	policies.UnimplementedGenerator

	telemetryConf dataplane.Telemetry
}

// NewGenerator returns a new instance of Generator.
func NewGenerator(telemetry dataplane.Telemetry) *Generator {
	return &Generator{telemetryConf: telemetry}
}

// GenerateForLocation generates policy configuration for a normal location block.
// For a normal location, all directives are applied.
// When the configuration involves a normal location redirecting to an internal location,
// only otel_trace and otel_trace_context are applied to the normal location.
func (g Generator) GenerateForLocation(pols []policies.Policy, location http.Location) policies.GenerateResultFiles {
	buildTemplate := func(
		tmplate *template.Template,
		fileSuffix string,
		includeGlobalAttrs bool,
	) policies.GenerateResultFiles {
		for _, pol := range pols {
			obs, ok := pol.(*ngfAPIv1alpha2.ObservabilityPolicy)
			if !ok {
				continue
			}

			fields := map[string]interface{}{
				"Tracing":  obs.Spec.Tracing,
				"Strategy": getStrategy(obs),
			}
			if includeGlobalAttrs {
				fields["GlobalSpanAttributes"] = g.telemetryConf.SpanAttributes
			}

			return policies.GenerateResultFiles{
				{
					Name:    fmt.Sprintf("ObservabilityPolicy_%s_%s_%s.conf", obs.Namespace, obs.Name, fileSuffix),
					Content: helpers.MustExecuteTemplate(tmplate, fields),
				},
			}
		}
		return nil
	}

	if location.Type == http.ExternalLocationType {
		return buildTemplate(tmpl, "ext", true)
	}

	return buildTemplate(tmplExtRedirect, "redirect", false)
}

// GenerateForInternalLocation generates policy configuration for an internal location block.
// otel_span_attr and otel_span_name are set in the internal location, with otel_trace and otel_trace_context
// being specified in the external location that redirects to the internal location.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	for _, pol := range pols {
		obs, ok := pol.(*ngfAPIv1alpha2.ObservabilityPolicy)
		if !ok {
			continue
		}

		fields := map[string]interface{}{
			"Tracing":              obs.Spec.Tracing,
			"GlobalSpanAttributes": g.telemetryConf.SpanAttributes,
		}

		return policies.GenerateResultFiles{
			{
				Name:    fmt.Sprintf("ObservabilityPolicy_%s_%s_int.conf", obs.Namespace, obs.Name),
				Content: helpers.MustExecuteTemplate(tmplInternal, fields),
			},
		}
	}

	return nil
}

func getStrategy(obs *ngfAPIv1alpha2.ObservabilityPolicy) string {
	var strategy string
	if obs.Spec.Tracing != nil {
		switch obs.Spec.Tracing.Strategy {
		case ngfAPIv1alpha2.TraceStrategyParent:
			strategy = "$otel_parent_sampled"
		case ngfAPIv1alpha2.TraceStrategyRatio:
			strategy = "on"
			if obs.Spec.Tracing.Ratio != nil {
				if *obs.Spec.Tracing.Ratio > 0 {
					strategy = dataplane.CreateRatioVarName(*obs.Spec.Tracing.Ratio)
				} else {
					strategy = "off"
				}
			}
		default:
			strategy = "off"
		}
	}

	return strategy
}
