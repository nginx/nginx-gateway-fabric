package config

import (
	"fmt"
	gotemplate "text/template"

	"github.com/go-logr/logr"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var streamServersTemplate = gotemplate.Must(gotemplate.New("streamServers").Parse(streamServersTemplateText))

func (g GeneratorImpl) executeStreamServers(conf dataplane.Configuration) []executeResult {
	streamServers := createStreamServers(g.logger, conf)

	streamServerConfig := stream.ServerConfig{
		Servers:     streamServers,
		IPFamily:    getIPFamily(conf.BaseHTTPConfig),
		Plus:        g.plus,
		DNSResolver: conf.BaseStreamConfig.DNSResolver,
	}

	streamServerResult := executeResult{
		dest: streamConfigFile,
		data: helpers.MustExecuteTemplate(streamServersTemplate, streamServerConfig),
	}

	return []executeResult{
		streamServerResult,
	}
}

func createStreamServers(logger logr.Logger, conf dataplane.Configuration) []stream.Server {
	totalServers := len(conf.TLSPassthroughServers) + len(conf.TCPServers) + len(conf.UDPServers)
	if totalServers == 0 {
		return nil
	}

	streamServers := make([]stream.Server, 0, totalServers*2)
	portSet := make(map[int32]struct{})
	upstreams := make(map[string]dataplane.Upstream)

	for _, u := range conf.StreamUpstreams {
		upstreams[u.Name] = u
	}
	for _, u := range conf.TCPUpstreams {
		upstreams[u.Name] = u
	}
	for _, u := range conf.UDPUpstreams {
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

	// Process Layer4 servers (TCP and UDP)
	processLayer4Servers(logger, conf.TCPServers, conf.UDPServers, upstreams, portSet, &streamServers)

	return streamServers
}

// processLayer4Servers processes TCP and UDP servers to create stream servers.
func processLayer4Servers(
	logger logr.Logger,
	tcpServers []dataplane.Layer4VirtualServer,
	udpServers []dataplane.Layer4VirtualServer,
	upstreams map[string]dataplane.Upstream,
	portSet map[int32]struct{},
	streamServers *[]stream.Server,
) {
	// Process TCP servers
	for i, server := range tcpServers {
		if _, inPortSet := portSet[server.Port]; inPortSet {
			continue // Skip if port already in use
		}

		if u, ok := upstreams[server.UpstreamName]; ok && server.UpstreamName != "" && len(u.Endpoints) > 0 {
			streamServer := stream.Server{
				Listen:     fmt.Sprint(server.Port),
				StatusZone: fmt.Sprintf("tcp_%d", server.Port),
				ProxyPass:  server.UpstreamName,
			}
			*streamServers = append(*streamServers, streamServer)
			portSet[server.Port] = struct{}{}
		} else {
			logger.V(1).Info("TCP Server skipped - upstream not found or no endpoints",
				"serverIndex", i,
				"port", server.Port,
				"upstreamName", server.UpstreamName,
			)
		}
	}

	// Process UDP servers
	for _, server := range udpServers {
		if _, inPortSet := portSet[server.Port]; inPortSet {
			continue // Skip if port already in use
		}

		if u, ok := upstreams[server.UpstreamName]; ok && server.UpstreamName != "" && len(u.Endpoints) > 0 {
			streamServer := stream.Server{
				Listen:     fmt.Sprintf("%d udp", server.Port),
				StatusZone: fmt.Sprintf("udp_%d", server.Port),
				ProxyPass:  server.UpstreamName,
				Protocol:   "udp",
				UDPConfig: &stream.UDPConfig{
					ProxyTimeout: "1s",
				},
			}
			*streamServers = append(*streamServers, streamServer)
			portSet[server.Port] = struct{}{}
		}
	}
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
