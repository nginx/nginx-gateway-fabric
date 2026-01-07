package config

import (
	"fmt"
	"strings"
	gotemplate "text/template"

	"github.com/go-logr/logr"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var streamServersTemplate = gotemplate.Must(gotemplate.New("streamServers").Parse(streamServersTemplateText))

func (g GeneratorImpl) executeStreamServers(conf dataplane.Configuration) []executeResult {
	streamServers := createStreamServers(g.logger, conf)
	splitClients := createStreamSplitClients(conf)

	streamServerConfig := stream.ServerConfig{
		Servers:      streamServers,
		SplitClients: splitClients,
		IPFamily:     getIPFamily(conf.BaseHTTPConfig),
		Plus:         g.plus,
		DNSResolver:  buildDNSResolver(conf.BaseStreamConfig.DNSResolver),
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

	for _, server := range conf.TLSPassthroughServers {
		if len(server.Upstreams) > 0 {
			upstreamName := server.Upstreams[0].Name
			if u, ok := upstreams[upstreamName]; ok && server.Hostname != "" && len(u.Endpoints) > 0 {
				streamServer := stream.Server{
					Listen:     getSocketNameTLS(server.Port, server.Hostname),
					StatusZone: server.Hostname,
					ProxyPass:  upstreamName,
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
	processLayer4Servers(logger, conf.TCPServers, upstreams, portSet, &streamServers, string(v1.TCPProtocolType))
	processLayer4Servers(logger, conf.UDPServers, upstreams, portSet, &streamServers, string(v1.UDPProtocolType))

	return streamServers
}

// processLayer4Servers processes TCP and UDP servers to create stream servers.
func processLayer4Servers(
	logger logr.Logger,
	servers []dataplane.Layer4VirtualServer,
	upstreams map[string]dataplane.Upstream,
	portSet map[int32]struct{},
	streamServers *[]stream.Server,
	protocol string,
) {
	protocolSuffix := ""
	if protocol == string(v1.UDPProtocolType) {
		protocolSuffix = " " + strings.ToLower(string(v1.UDPProtocolType))
	}

	for i, server := range servers {
		if _, inPortSet := portSet[server.Port]; inPortSet {
			continue
		}

		if len(server.Upstreams) == 0 {
			logger.V(1).Info(
				fmt.Sprintf("%s Server skipped - no upstreams", protocol),
				"serverIndex", i,
				"port", server.Port,
			)
			continue
		}

		var proxyPass string
		if len(server.Upstreams) > 1 {
			proxyPass = fmt.Sprintf("$backend_%d", server.Port)
			hasValidUpstreams := false
			for _, upstream := range server.Upstreams {
				if u, ok := upstreams[upstream.Name]; ok && len(u.Endpoints) > 0 {
					hasValidUpstreams = true
					break
				}
			}
			if !hasValidUpstreams {
				logger.V(1).Info(
					fmt.Sprintf("%s Server skipped - no valid upstreams with endpoints", protocol),
					"serverIndex", i,
					"port", server.Port,
				)
				continue
			}
		} else {
			upstreamName := server.Upstreams[0].Name
			if u, ok := upstreams[upstreamName]; ok && len(u.Endpoints) > 0 {
				proxyPass = upstreamName
			} else {
				logger.V(1).Info(
					fmt.Sprintf("%s Server skipped - upstream not found or no endpoints", protocol),
					"serverIndex", i,
					"port", server.Port,
					"upstreamName", upstreamName,
				)
				continue
			}
		}

		streamServer := stream.Server{
			Listen:     fmt.Sprintf("%d%s", server.Port, protocolSuffix),
			StatusZone: fmt.Sprintf("%s_%d", protocol, server.Port),
			ProxyPass:  proxyPass,
		}
		*streamServers = append(*streamServers, streamServer)
		portSet[server.Port] = struct{}{}
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

// createStreamSplitClients creates split_clients configurations for Layer4 servers with multiple backends.
func createStreamSplitClients(conf dataplane.Configuration) []stream.SplitClient {
	var splitClients []stream.SplitClient

	// Process TCP servers
	for _, server := range conf.TCPServers {
		if server.NeedsWeightDistribution() {
			splitClient := createSplitClientForL4Server(server)
			if splitClient != nil {
				splitClients = append(splitClients, *splitClient)
			}
		}
	}

	// Process UDP servers
	for _, server := range conf.UDPServers {
		if server.NeedsWeightDistribution() {
			splitClient := createSplitClientForL4Server(server)
			if splitClient != nil {
				splitClients = append(splitClients, *splitClient)
			}
		}
	}

	return splitClients
}

// createSplitClientForL4Server creates a split_clients configuration for a Layer4 server with multiple backends.
func createSplitClientForL4Server(server dataplane.Layer4VirtualServer) *stream.SplitClient {
	if !server.NeedsWeightDistribution() {
		return nil
	}

	// Calculate total weight
	totalWeight := int32(0)
	for _, upstream := range server.Upstreams {
		totalWeight += upstream.Weight
	}

	if totalWeight == 0 {
		return nil
	}

	distributions := make([]stream.SplitClientDistribution, 0, len(server.Upstreams))
	availablePercentage := float64(100)

	// Process all upstreams except the last one
	for i := range len(server.Upstreams) - 1 {
		upstream := server.Upstreams[i]
		percentage := percentOf(upstream.Weight, totalWeight)
		availablePercentage -= percentage

		distributions = append(distributions, stream.SplitClientDistribution{
			Percent: fmt.Sprintf("%.2f", percentage),
			Value:   upstream.Name,
		})
	}

	// The last upstream gets the remaining percentage
	lastUpstream := server.Upstreams[len(server.Upstreams)-1]
	distributions = append(distributions, stream.SplitClientDistribution{
		Percent: fmt.Sprintf("%.2f", availablePercentage),
		Value:   lastUpstream.Name,
	})

	return &stream.SplitClient{
		VariableName:  fmt.Sprintf("backend_%d", server.Port),
		Distributions: distributions,
	}
}
