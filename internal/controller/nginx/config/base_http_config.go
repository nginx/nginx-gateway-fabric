package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var baseHTTPTemplate = gotemplate.Must(gotemplate.New("baseHttp").Parse(baseHTTPTemplateText))

type httpConfig struct {
	DNSResolver             *dataplane.DNSResolverConfig
	LoggingSettings         *dataplane.LoggingSettings
	Includes                []shared.Include
	NginxReadinessProbePort int32
	IPFamily                shared.IPFamily
	HTTP2                   bool
}

func executeBaseHTTPConfig(conf dataplane.Configuration) []executeResult {
	includes := createIncludesFromSnippets(conf.BaseHTTPConfig.Snippets)

	hc := httpConfig{
		HTTP2:                   conf.BaseHTTPConfig.HTTP2,
		Includes:                includes,
		NginxReadinessProbePort: conf.BaseHTTPConfig.NginxReadinessProbePort,
		IPFamily:                getIPFamily(conf.BaseHTTPConfig),
		DNSResolver:             conf.BaseHTTPConfig.DNSResolver,
		LoggingSettings:         conf.Logging.LoggingSettings,
	}

	results := make([]executeResult, 0, len(includes)+1)
	results = append(results, executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(baseHTTPTemplate, hc),
	})
	results = append(results, createIncludeExecuteResults(includes)...)

	return results
}
