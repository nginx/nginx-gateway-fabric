package http //nolint:revive,nolintlint // ignoring conflicting package name

import (
	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
)

const (
	InternalRoutePathPrefix       = "/_ngf-internal"
	InternalMirrorRoutePathPrefix = InternalRoutePathPrefix + "-mirror"
	HTTPSScheme                   = "https"
	KeepAliveConnectionDefault    = int32(16)
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
	// InternalLocationType defines an internal location that is only accessible within NGINX.
	InternalLocationType LocationType = "internal"
	// ExternalLocationType defines a normal external location that is accessible by clients.
	ExternalLocationType LocationType = "external"
	// RedirectLocationType defines an external location that redirects to an internal location
	// based on HTTP matching conditions.
	RedirectLocationType LocationType = "redirect"
	// InferenceExternalLocationType defines an external location that is used for calling NJS
	// to get the inference workload endpoint and redirects to the internal location that will proxy_pass
	// to that endpoint.
	InferenceExternalLocationType LocationType = "inference-external"
	// InferenceInternalLocationType defines an internal location that is used for calling NJS
	// to get the inference workload endpoint and redirects to the internal location that will proxy_pass
	// to that endpoint. This is used when an HTTP redirect location is also defined that redirects
	// to this internal inference location.
	InferenceInternalLocationType LocationType = "inference-internal"
)

// Location holds all configuration for an HTTP location.
type Location struct {
	// Return specifies a return directive (e.g., HTTP status or redirect) for this location block.
	Return *Return
	// ProxySSLVerify controls SSL verification for upstreams when proxying requests.
	ProxySSLVerify *ProxySSLVerify
	// ProxyPass is the upstream backend (URL or name) to which requests are proxied.
	ProxyPass string
	// HTTPMatchKey is the key for associating HTTP match rules, used for routing and NJS module logic.
	HTTPMatchKey string
	// MirrorSplitClientsVariableName is the variable name for split_clients, used in traffic mirroring scenarios.
	MirrorSplitClientsVariableName string
	// EPPInternalPath is the internal path for the inference NJS module to redirect to.
	EPPInternalPath string
	// EPPHost is the host for the EndpointPicker, used for inference routing.
	EPPHost string
	// Type indicates the type of location (external, internal, redirect, etc).
	Type LocationType
	// Path is the NGINX location path.
	Path string
	// ResponseHeaders are custom response headers to be sent.
	ResponseHeaders ResponseHeaders
	// ProxySetHeaders are headers to set when proxying requests upstream.
	ProxySetHeaders []Header
	// Rewrites are rewrite rules for modifying request paths.
	Rewrites []string
	// MirrorPaths are paths to which requests are mirrored.
	MirrorPaths []string
	// Includes are additional NGINX config snippets or policies to include in this location.
	Includes []shared.Include
	// EPPPort is the port for the EndpointPicker, used for inference routing.
	EPPPort int
	// GRPC indicates if this location proxies gRPC traffic.
	GRPC bool
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
	SessionPersistence  UpstreamSessionPersistence
	Name                string
	ZoneSize            string // format: 512k, 1m
	StateFile           string
	LoadBalancingMethod string
	HashMethodKey       string
	KeepAlive           UpstreamKeepAlive
	Servers             []UpstreamServer
}

// UpstreamSessionPersistence holds the session persistence configuration for an upstream.
type UpstreamSessionPersistence struct {
	Name        string
	Expiry      string
	Path        string
	SessionType string
}

// UpstreamKeepAlive holds the keepalive configuration for an HTTP upstream.
type UpstreamKeepAlive struct {
	Connections *int32
	Time        string
	Timeout     string
	Requests    int32
}

// UpstreamServer holds all configuration for an HTTP upstream server.
type UpstreamServer struct {
	Address string
	Resolve bool
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
	Servers                  []Server
	RewriteClientIP          shared.RewriteClientIPSettings
	IPFamily                 shared.IPFamily
	Plus                     bool
	DisableSNIHostValidation bool
}

var (
	OSSAllowedLBMethods = map[ngfAPI.LoadBalancingType]struct{}{
		ngfAPI.LoadBalancingTypeRoundRobin:               {},
		ngfAPI.LoadBalancingTypeLeastConnection:          {},
		ngfAPI.LoadBalancingTypeIPHash:                   {},
		ngfAPI.LoadBalancingTypeRandom:                   {},
		ngfAPI.LoadBalancingTypeHash:                     {},
		ngfAPI.LoadBalancingTypeHashConsistent:           {},
		ngfAPI.LoadBalancingTypeRandomTwo:                {},
		ngfAPI.LoadBalancingTypeRandomTwoLeastConnection: {},
	}

	PlusAllowedLBMethods = map[ngfAPI.LoadBalancingType]struct{}{
		ngfAPI.LoadBalancingTypeRoundRobin:                 {},
		ngfAPI.LoadBalancingTypeLeastConnection:            {},
		ngfAPI.LoadBalancingTypeIPHash:                     {},
		ngfAPI.LoadBalancingTypeRandom:                     {},
		ngfAPI.LoadBalancingTypeHash:                       {},
		ngfAPI.LoadBalancingTypeHashConsistent:             {},
		ngfAPI.LoadBalancingTypeRandomTwo:                  {},
		ngfAPI.LoadBalancingTypeRandomTwoLeastConnection:   {},
		ngfAPI.LoadBalancingTypeLeastTimeHeader:            {},
		ngfAPI.LoadBalancingTypeLeastTimeLastByte:          {},
		ngfAPI.LoadBalancingTypeLeastTimeHeaderInflight:    {},
		ngfAPI.LoadBalancingTypeLeastTimeLastByteInflight:  {},
		ngfAPI.LoadBalancingTypeRandomTwoLeastTimeHeader:   {},
		ngfAPI.LoadBalancingTypeRandomTwoLeastTimeLastByte: {},
	}
)
