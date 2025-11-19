package proxysettings

import (
	"fmt"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var tmpl = template.Must(template.New("proxy settings policy").Parse(proxySettingsTemplate))

const proxySettingsTemplate = `
{{- if .ProxyBufferingDirective }}
proxy_buffering {{ .ProxyBufferingDirective }};
{{- end }}
{{- if .ProxyBufferSize }}
proxy_buffer_size {{ .ProxyBufferSize }};
{{- end }}
{{- if .ProxyBuffers }}
proxy_buffers {{ .ProxyBuffers }};
{{- end }}
{{- if .ProxyBusyBuffersSize }}
proxy_busy_buffers_size {{ .ProxyBusyBuffersSize }};
{{- end }}
`

type proxySettings struct {
	ProxyBufferingDirective string
	ProxyBufferSize         string
	ProxyBuffers            string
	ProxyBusyBuffersSize    string
}

func getProxySettings(spec ngfAPI.ProxySettingsPolicySpec) proxySettings {
	settings := proxySettings{}

	if spec.Buffering != nil {
		if spec.Buffering.Disable != nil {
			if *spec.Buffering.Disable {
				settings.ProxyBufferingDirective = "off"
			} else {
				settings.ProxyBufferingDirective = "on"
			}
		}

		if spec.Buffering.BufferSize != nil {
			settings.ProxyBufferSize = string(*spec.Buffering.BufferSize)
		}

		if spec.Buffering.Buffers != nil {
			settings.ProxyBuffers = fmt.Sprintf("%d %s", spec.Buffering.Buffers.Number, spec.Buffering.Buffers.Size)
		}

		if spec.Buffering.BusyBuffersSize != nil {
			settings.ProxyBusyBuffersSize = string(*spec.Buffering.BusyBuffersSize)
		}
	}

	return settings
}

// Generator generates nginx configuration based on a ProxySettingsPolicy.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForHTTP generates policy configuration for the http block.
func (g Generator) GenerateForHTTP(pols []policies.Policy) policies.GenerateResultFiles {
	return generate(pols)
}

// GenerateForLocation generates policy configuration for a normal location block.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generate(pols)
}

// GenerateForInternalLocation generates policy configuration for an internal location block.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return generate(pols)
}

func generate(pols []policies.Policy) policies.GenerateResultFiles {
	files := make(policies.GenerateResultFiles, 0, len(pols))

	for _, pol := range pols {
		psp, ok := pol.(*ngfAPI.ProxySettingsPolicy)
		if !ok {
			continue
		}

		settings := getProxySettings(psp.Spec)

		files = append(files, policies.File{
			Name:    fmt.Sprintf("ProxySettingsPolicy_%s_%s.conf", psp.Namespace, psp.Name),
			Content: helpers.MustExecuteTemplate(tmpl, settings),
		})
	}

	return files
}
