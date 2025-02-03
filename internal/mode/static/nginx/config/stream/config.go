package stream

import "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"

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
}

// ServerConfig holds configuration for a stream server and IP family to be used by NGINX.
type ServerConfig struct {
	Servers  []Server
	IPFamily shared.IPFamily
	Plus     bool
}
