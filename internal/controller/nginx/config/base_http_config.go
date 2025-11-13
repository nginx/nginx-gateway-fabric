package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var baseHTTPTemplate = gotemplate.Must(gotemplate.New("baseHttp").Parse(baseHTTPTemplateText))

type AccessLog struct {
	Format     string // User's format string
	Path       string // Where to write logs (/dev/stdout)
	FormatName string // Internal format name (ngf_user_defined_log_format)
	Disable    bool   // User's disable flag
}
type httpConfig struct {
	DNSResolver             *dataplane.DNSResolverConfig
	AccessLog               *AccessLog
	GatewaySecretID         dataplane.SSLKeyPairID
	Includes                []shared.Include
	NginxReadinessProbePort int32
	IPFamily                shared.IPFamily
	HTTP2                   bool
}

func newExecuteBaseHTTPConfigFunc(generator policies.Generator) executeFunc {
	return func(conf dataplane.Configuration) []executeResult {
		return executeBaseHTTPConfig(conf, generator)
	}
}

func executeBaseHTTPConfig(conf dataplane.Configuration, generator policies.Generator) []executeResult {
	snippetIncludes := createIncludesFromSnippets(conf.BaseHTTPConfig.Snippets)
	policyIncludes := createIncludesFromPolicyGenerateResult(
		generator.GenerateForHTTP(conf.BaseHTTPConfig.Policies),
	)

	includes := make([]shared.Include, 0, len(snippetIncludes)+len(policyIncludes))
	includes = append(includes, snippetIncludes...)
	includes = append(includes, policyIncludes...)

	hc := httpConfig{
		HTTP2:                   conf.BaseHTTPConfig.HTTP2,
		Includes:                includes,
		NginxReadinessProbePort: conf.BaseHTTPConfig.NginxReadinessProbePort,
		IPFamily:                getIPFamily(conf.BaseHTTPConfig),
		DNSResolver:             conf.BaseHTTPConfig.DNSResolver,
		AccessLog:               buildAccessLog(conf.Logging.AccessLog),
		GatewaySecretID:         conf.BaseHTTPConfig.GatewaySecretID,
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
		accessLog.Disable = accessLogConfig.Disable

		return accessLog
	}
	return nil
}
