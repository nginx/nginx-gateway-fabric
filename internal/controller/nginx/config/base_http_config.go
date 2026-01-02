package config

import (
	"fmt"
	"net"
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

func executeBaseHTTPConfig(conf dataplane.Configuration) []executeResult {
	includes := createIncludesFromSnippets(conf.BaseHTTPConfig.Snippets)

	hc := httpConfig{
		HTTP2:                   conf.BaseHTTPConfig.HTTP2,
		Includes:                includes,
		NginxReadinessProbePort: conf.BaseHTTPConfig.NginxReadinessProbePort,
		IPFamily:                getIPFamily(conf.BaseHTTPConfig),
		DNSResolver:             buildDNSResolver(conf.BaseHTTPConfig.DNSResolver),
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

func buildDNSResolver(dnsResolver *dataplane.DNSResolverConfig) *dataplane.DNSResolverConfig {
	if dnsResolver == nil {
		return nil
	}

	fixed := &dataplane.DNSResolverConfig{
		Timeout:     dnsResolver.Timeout,
		Valid:       dnsResolver.Valid,
		DisableIPv6: dnsResolver.DisableIPv6,
	}

	for _, address := range dnsResolver.Addresses {
		ip := net.ParseIP(address)
		if ip == nil {
			continue
		}

		if ip.To4() == nil {
			// nginx expects IPv6 DNS resolvers to be passed with brackets
			fixed.Addresses = append(fixed.Addresses, fmt.Sprintf("[%s]", address))
		} else {
			fixed.Addresses = append(fixed.Addresses, address)
		}
	}

	return fixed
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
