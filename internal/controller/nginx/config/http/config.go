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
	Return                         *Return
	ProxySSLVerify                 *ProxySSLVerify
	CORSHeaders                    map[string]string
	AuthBasic                      *AuthBasic
	Path                           string
	ProxyPass                      string
	EPPHost                        string
	Type                           LocationType
	MirrorSplitClientsVariableName string
	HTTPMatchKey                   string
	EPPInternalPath                string
	ResponseHeaders                ResponseHeaders
	ProxySetHeaders                []Header
	MirrorPaths                    []string
	Includes                       []shared.Include
	Rewrites                       []string
	EPPPort                        int
	GRPC                           bool
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
	Certificate         string
	CertificateKey      string
	Protocols           string
	Ciphers             string
	PreferServerCiphers bool
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

// AuthBasic holds the values for the auth_basic and auth_basic_user_file directives.
// See https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html
type AuthBasic struct {
	Realm string
	File  string
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
