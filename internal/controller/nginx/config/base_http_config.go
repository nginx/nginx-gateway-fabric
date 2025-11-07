package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var baseHTTPTemplate = gotemplate.Must(gotemplate.New("baseHttp").Parse(baseHTTPTemplateText))

type AccessLog struct {
	Format     string // User's format string
	Path       string // Where to write logs (/dev/stdout)
	FormatName string // Internal format name (ngf_user_defined_log_format)
	Disabled   bool   // User's disabled flag
}
type httpConfig struct {
	DNSResolver             *dataplane.DNSResolverConfig
	AccessLog               *AccessLog
	DefaultAccessLogPath    string
	DefaultLogFormatName    string
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
		AccessLog:               buildAccessLog(conf.Logging.AccessLog),
	}

	results := make([]executeResult, 0, len(includes)+1)
	results = append(results, executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(baseHTTPTemplate, hc),
	})
	results = append(results, createIncludeExecuteResults(includes)...)

	return results
}

func buildAccessLog(accessLogConfig *dataplane.AccessLog) *AccessLog {
	if accessLogConfig != nil {
		accessLog := &AccessLog{
			Path:       dataplane.DefaultAccessLogPath,
			FormatName: dataplane.DefaultLogFormatName,
		}
		if accessLogConfig.Format != "" {
			accessLog.Format = accessLogConfig.Format
		}

		if accessLogConfig.Disabled {
			accessLog.Disabled = accessLogConfig.Disabled
		}

		return accessLog
	}
	return nil
}
