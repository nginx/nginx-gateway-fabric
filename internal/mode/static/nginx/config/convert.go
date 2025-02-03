package config

import (
	"fmt"

	ngxclient "github.com/nginxinc/nginx-plus-go-client/client"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

// ConvertEndpoints converts a list of Endpoints into a list of NGINX Plus SDK UpstreamServers.
func ConvertEndpoints(eps []resolver.Endpoint) []ngxclient.UpstreamServer {
	servers := make([]ngxclient.UpstreamServer, 0, len(eps))

	for _, ep := range eps {
		port, format := getPortAndIPFormat(ep)

		server := ngxclient.UpstreamServer{
			Server: fmt.Sprintf(format, ep.Address, port),
		}

		servers = append(servers, server)
	}

	return servers
}

// ConvertStreamEndpoints converts a list of Endpoints into a list of NGINX Plus SDK StreamUpstreamServers.
func ConvertStreamEndpoints(eps []resolver.Endpoint) []ngxclient.StreamUpstreamServer {
	servers := make([]ngxclient.StreamUpstreamServer, 0, len(eps))

	for _, ep := range eps {
		port, format := getPortAndIPFormat(ep)

		server := ngxclient.StreamUpstreamServer{
			Server: fmt.Sprintf(format, ep.Address, port),
		}

		servers = append(servers, server)
	}

	return servers
}

func getPortAndIPFormat(ep resolver.Endpoint) (string, string) {
	var port string

	if ep.Port != 0 {
		port = fmt.Sprintf(":%d", ep.Port)
	}

	format := "%s%s"
	if ep.IPv6 {
		format = "[%s]%s"
	}

	return port, format
}
