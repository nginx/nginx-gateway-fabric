package proxysettings

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
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
{{- if .ProxyConnectTimeout }}
proxy_connect_timeout {{ .ProxyConnectTimeout }};
{{- end }}
{{- if .ProxyReadTimeout }}
proxy_read_timeout {{ .ProxyReadTimeout }};
{{- end }}
{{- if .ProxySendTimeout }}
proxy_send_timeout {{ .ProxySendTimeout }};
{{- end }}
{{- if .ProxyCache }}
proxy_cache {{ .ProxyCache }};
{{- end }}
{{- range .ProxyCacheValid }}
proxy_cache_valid {{ . }};
{{- end }}
{{- if .ProxyCacheKey }}
proxy_cache_key {{ .ProxyCacheKey }};
{{- end }}
{{- if .ProxyCacheBypass }}
proxy_cache_bypass {{ .ProxyCacheBypass }};
{{- end }}
{{- if .ProxyNoCache }}
proxy_no_cache {{ .ProxyNoCache }};
{{- end }}
`

type proxySettings struct {
	ProxyCache              string
	ProxyCacheKey           string
	ProxyCacheBypass        string
	ProxyNoCache            string
	ProxyBufferingDirective string
	ProxyBufferSize         string
	ProxyBuffers            string
	ProxyBusyBuffersSize    string
	ProxyConnectTimeout     string
	ProxyReadTimeout        string
	ProxySendTimeout        string
	ProxyCacheValid         []string
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

	if spec.Timeout != nil {
		if spec.Timeout.Connect != nil {
			settings.ProxyConnectTimeout = string(*spec.Timeout.Connect)
		}

		if spec.Timeout.Read != nil {
			settings.ProxyReadTimeout = string(*spec.Timeout.Read)
		}

		if spec.Timeout.Send != nil {
			settings.ProxySendTimeout = string(*spec.Timeout.Send)
		}
	}

	if spec.Caching != nil {
		addCachingSettings(&settings, *spec.Caching)
	}

	return settings
}

func addCachingSettings(settings *proxySettings, caching ngfAPI.ProxyCaching) {
	if caching.Enable != nil {
		if *caching.Enable {
			settings.ProxyCache = shared.ProxyCacheZoneName
		} else {
			settings.ProxyCache = "off"
		}
	}

	for _, valid := range caching.Valid {
		parts := make([]string, 0, len(valid.StatusCodes)+1)
		for _, code := range valid.StatusCodes {
			parts = append(parts, strconv.Itoa(int(code)))
		}
		parts = append(parts, string(valid.Duration))

		settings.ProxyCacheValid = append(settings.ProxyCacheValid, strings.Join(parts, " "))
	}

	// The key variables are concatenated without a separator, for example $scheme$host$request_uri.
	settings.ProxyCacheKey = strings.Join(caching.Key, "")
	settings.ProxyCacheBypass = strings.Join(caching.Bypass, " ")
	settings.ProxyNoCache = strings.Join(caching.NoCache, " ")
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
