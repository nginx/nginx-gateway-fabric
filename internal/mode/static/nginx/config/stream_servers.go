package config

import (
	"fmt"
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var streamServersTemplate = gotemplate.Must(gotemplate.New("streamServers").Parse(streamServersTemplateText))

func (g GeneratorImpl) executeStreamServers(conf dataplane.Configuration) []executeResult {
	streamServers := createStreamServers(conf)

	streamServerConfig := stream.ServerConfig{
		Servers:  streamServers,
		IPFamily: getIPFamily(conf.BaseHTTPConfig),
		Plus:     g.plus,
	}

	streamServerResult := executeResult{
		dest: streamConfigFile,
		data: helpers.MustExecuteTemplate(streamServersTemplate, streamServerConfig),
	}

	return []executeResult{
		streamServerResult,
	}
}

func createStreamServers(conf dataplane.Configuration) []stream.Server {
	if len(conf.TLSPassthroughServers) == 0 {
		return nil
	}

	streamServers := make([]stream.Server, 0, len(conf.TLSPassthroughServers)*2)
	portSet := make(map[int32]struct{})
	upstreams := make(map[string]dataplane.Upstream)

	for _, u := range conf.StreamUpstreams {
		upstreams[u.Name] = u
	}

	for _, server := range conf.TLSPassthroughServers {
		if u, ok := upstreams[server.UpstreamName]; ok && server.UpstreamName != "" {
			if server.Hostname != "" && len(u.Endpoints) > 0 {
				streamServer := stream.Server{
					Listen:     getSocketNameTLS(server.Port, server.Hostname),
					StatusZone: server.Hostname,
					ProxyPass:  server.UpstreamName,
					IsSocket:   true,
				}
				// set rewriteClientIP settings as this is a socket stream server
				streamServer.RewriteClientIP = getRewriteClientIPSettingsForStream(
					conf.BaseHTTPConfig.RewriteClientIPSettings,
				)
				streamServers = append(streamServers, streamServer)
			}
		}

		if _, inPortSet := portSet[server.Port]; inPortSet {
			continue
		}

		portSet[server.Port] = struct{}{}

		// we do not evaluate rewriteClientIP settings for non-socket stream servers
		streamServer := stream.Server{
			Listen:     fmt.Sprint(server.Port),
			StatusZone: server.Hostname,
			Pass:       getTLSPassthroughVarName(server.Port),
			SSLPreread: true,
		}
		streamServers = append(streamServers, streamServer)
	}
	return streamServers
}

func getRewriteClientIPSettingsForStream(
	rewriteConfig dataplane.RewriteClientIPSettings,
) shared.RewriteClientIPSettings {
	proxyEnabled := rewriteConfig.Mode == dataplane.RewriteIPModeProxyProtocol
	if proxyEnabled {
		return shared.RewriteClientIPSettings{
			ProxyProtocol: shared.ProxyProtocolDirective,
			RealIPFrom:    rewriteConfig.TrustedAddresses,
		}
	}

	return shared.RewriteClientIPSettings{}
}
