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
	SSL                    *SSL
	MisdirectedRequestVars *MisdirectedRequestVars
	ServerName             string
	Listen                 string
	Locations              []Location
	Includes               []shared.Include
	IsDefaultHTTP          bool
	IsDefaultSSL           bool
	GRPC                   bool
	IsSocket               bool
}

// MisdirectedRequestVars holds the per-port NGINX variable names used
// for misdirected request (421) detection via SNI-to-Host comparison maps.
type MisdirectedRequestVars struct {
	// SNIVar is the NGINX variable for the SNI-derived listener ID (e.g., "$sni_listener_id_443").
	SNIVar string
	// HostVar is the NGINX variable for the Host-derived listener ID (e.g., "$host_listener_id_443").
	HostVar string
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
)

// Location holds all configuration for an HTTP location.
type Location struct {
	// AuthBasic contains the configuration for basic authentication.
	AuthBasic *AuthBasic
	// ProxySSLVerify controls SSL verification for upstreams when proxying requests.
	ProxySSLVerify *ProxySSLVerify
	// Inference holds configuration for the Rust inference module (EPP directives).
	Inference *InferenceConfig
	// AuthJWT contains the configuration for JWT authentication.
	AuthJWT *AuthJWT
	// Return specifies a return directive (e.g., HTTP status or redirect) for this location block.
	Return *Return
	// HTTPMatchKey is the key for associating HTTP match rules, used for routing and NJS module logic.
	HTTPMatchKey string
	// Type indicates the type of location (external, internal, redirect, etc).
	Type LocationType
	// Path is the NGINX location path.
	Path string
	// MirrorSplitClientsVariableName is the variable name for split_clients, used in traffic mirroring scenarios.
	MirrorSplitClientsVariableName string
	// AuthOIDCProviderName is the name of the oidc_provider to be referenced in this location.
	AuthOIDCProviderName string
	// ProxyPass is the upstream backend (URL or name) to which requests are proxied.
	ProxyPass string
	// ResponseHeaders are custom response headers to be sent.
	ResponseHeaders ResponseHeaders
	// CORSHeaders are the CORS headers to be added for this location.
	CORSHeaders []Header
	// ProxySetHeaders are headers to set when proxying requests upstream.
	ProxySetHeaders []Header
	// Rewrites are rewrite rules for modifying request paths.
	Rewrites []string
	// MirrorPaths are paths to which requests are mirrored.
	MirrorPaths []string
	// Includes are additional NGINX config snippets or policies to include in this location.
	Includes []shared.Include
	// GRPC indicates if this location proxies gRPC traffic.
	GRPC bool
}

// InferenceConfig holds configuration for the Rust inference dynamic module.
// The Rust module runs in the NGINX access phase of the location and calls the EPP
// server via gRPC to select an endpoint before proxy_pass executes.
type InferenceConfig struct {
	// EPPEndpoint is the host:port of the Endpoint Picker Protocol server.
	EPPEndpoint string
	// FailopenUpstream is the upstream name to use if the EPP call fails (optional).
	FailopenUpstream string
	// UseTLS indicates whether TLS should be used when connecting to the EPP server.
	UseTLS bool
	// TLSSkipVerify indicates whether to skip certificate verification when using TLS.
	TLSSkipVerify bool
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
	Protocols           string
	Ciphers             string
	ClientCertificate   string
	VerifyClient        string
	Certificates        []string
	CertificateKeys     []string
	RequireVerifiedCert bool
	PreferServerCiphers bool
}

// StatusCode is an HTTP status code.
type StatusCode int

const (
	// StatusOK is the HTTP 200 status code.
	StatusOK StatusCode = 200
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

// AuthJWT holds the configuration for JWT authentication using the auth_jwt directive.
// See https://nginx.org/en/docs/http/ngx_http_auth_jwt_module.html
type AuthJWT struct {
	KeyCache *ngfAPI.Duration
	Remote   *AuthJWTRemote
	Realm    string
	File     string
}

// AuthJWTRemote holds configuration for remote JWKS retrieval.
type AuthJWTRemote struct {
	TrustedCertificate string
	URI                string
	Path               string
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
