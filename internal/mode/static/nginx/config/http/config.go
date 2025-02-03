package http

import (
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
)

const (
	InternalRoutePathPrefix = "/_ngf-internal"
	HTTPSScheme             = "https"
)

// Server holds all configuration for an HTTP server.
type Server struct {
	SSL           *SSL
	ServerName    string
	Listen        string
	Locations     []Location
	Includes      []shared.Include
	IsDefaultHTTP bool
	IsDefaultSSL  bool
	GRPC          bool
	IsSocket      bool
}

type LocationType string

const (
	InternalLocationType LocationType = "internal"
	ExternalLocationType LocationType = "external"
	RedirectLocationType LocationType = "redirect"
)

// Location holds all configuration for an HTTP location.
type Location struct {
	Path            string
	ProxyPass       string
	HTTPMatchKey    string
	Type            LocationType
	ProxySetHeaders []Header
	ProxySSLVerify  *ProxySSLVerify
	Return          *Return
	ResponseHeaders ResponseHeaders
	Rewrites        []string
	Includes        []shared.Include
	GRPC            bool
}

// Header defines an HTTP header to be passed to the proxied server.
type Header struct {
	Name  string
	Value string
}

// ResponseHeaders holds all response headers to be added, set, or removed.
type ResponseHeaders struct {
	Add    []Header
	Set    []Header
	Remove []string
}

// Return represents an HTTP return.
type Return struct {
	Body string
	Code StatusCode
}

// SSL holds all SSL related configuration.
type SSL struct {
	Certificate    string
	CertificateKey string
}

// StatusCode is an HTTP status code.
type StatusCode int

const (
	// StatusFound is the HTTP 302 status code.
	StatusFound StatusCode = 302
	// StatusNotFound is the HTTP 404 status code.
	StatusNotFound StatusCode = 404
	// StatusInternalServerError is the HTTP 500 status code.
	StatusInternalServerError StatusCode = 500
)

// Upstream holds all configuration for an HTTP upstream.
type Upstream struct {
	Name      string
	ZoneSize  string // format: 512k, 1m
	StateFile string
	KeepAlive UpstreamKeepAlive
	Servers   []UpstreamServer
}

// UpstreamKeepAlive holds the keepalive configuration for an HTTP upstream.
type UpstreamKeepAlive struct {
	Time        string
	Timeout     string
	Connections int32
	Requests    int32
}

// UpstreamServer holds all configuration for an HTTP upstream server.
type UpstreamServer struct {
	Address string
}

// SplitClient holds all configuration for an HTTP split client.
type SplitClient struct {
	VariableName  string
	Distributions []SplitClientDistribution
}

// SplitClientDistribution maps Percentage to Value in a SplitClient.
type SplitClientDistribution struct {
	Percent string
	Value   string
}

// ProxySSLVerify holds the proxied HTTPS server verification configuration.
type ProxySSLVerify struct {
	TrustedCertificate string
	Name               string
}

// ServerConfig holds configuration for an HTTP server and IP family to be used by NGINX.
type ServerConfig struct {
	Servers         []Server
	RewriteClientIP shared.RewriteClientIPSettings
	IPFamily        shared.IPFamily
	Plus            bool
}
