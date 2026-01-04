package stream

import (
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

// Server holds all configuration for a stream server.
type Server struct {
	Listen          string
	StatusZone      string
	ProxyPass       string
	Pass            string
	RewriteClientIP shared.RewriteClientIPSettings
	SSLPreread      bool
	IsSocket        bool
}

// Upstream holds all configuration for a stream upstream.
type Upstream struct {
	Name      string
	ZoneSize  string // format: 512k, 1m
	StateFile string
	Servers   []UpstreamServer
}

// UpstreamServer holds all configuration for a stream upstream server.
type UpstreamServer struct {
	Address string
	Resolve bool
	Weight  int32 // Weight for load balancing, default 1
}

// SplitClient holds configuration for a stream split_clients directive.
type SplitClient struct {
	VariableName  string
	Distributions []SplitClientDistribution
}

// SplitClientDistribution holds configuration for a split_clients distribution.
type SplitClientDistribution struct {
	Percent string
	Value   string
}

// ServerConfig holds configuration for a stream server and IP family to be used by NGINX.
type ServerConfig struct {
	DNSResolver  *dataplane.DNSResolverConfig
	Servers      []Server
	SplitClients []SplitClient
	IPFamily     shared.IPFamily
	Plus         bool
}
