package dataplane

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

// PathType is the type of the path in a PathRule.
type PathType string

const (
	// PathTypePrefix indicates that the path is a prefix.
	PathTypePrefix PathType = "prefix"
	// PathTypeExact indicates that the path is exact.
	PathTypeExact PathType = "exact"
)

// Configuration is an intermediate representation of dataplane configuration.
type Configuration struct {
	// SSLKeyPairs holds all unique SSLKeyPairs.
	SSLKeyPairs map[SSLKeyPairID]SSLKeyPair
	// CertBundles holds all unique Certificate Bundles.
	CertBundles map[CertBundleID]CertBundle
	// HTTPServers holds all HTTPServers.
	HTTPServers []VirtualServer
	// SSLServers holds all SSLServers.
	SSLServers []VirtualServer
	// TLSPassthroughServers hold all TLSPassthroughServers
	TLSPassthroughServers []Layer4VirtualServer
	// Upstreams holds all unique http Upstreams.
	Upstreams []Upstream
	// DeploymentContext contains metadata about NGF and the cluster.
	DeploymentContext DeploymentContext
	// AuxiliarySecrets contains additional secret data, like certificates/keys/tokens that are not related to
	// Gateway API resources.
	AuxiliarySecrets map[graph.SecretFileType][]byte
	// StreamUpstreams holds all unique stream Upstreams
	StreamUpstreams []Upstream
	// BackendGroups holds all unique BackendGroups.
	BackendGroups []BackendGroup
	// MainSnippets holds all the snippets that apply to the main context.
	MainSnippets []Snippet
	// Telemetry holds the Otel configuration.
	Telemetry Telemetry
	// Logging defines logging related settings for NGINX.
	Logging Logging
	// NginxPlus specifies NGINX Plus additional settings.
	NginxPlus NginxPlus
	// BaseHTTPConfig holds the configuration options at the http context.
	BaseHTTPConfig BaseHTTPConfig
	// Version represents the version of the generated configuration.
	Version int
}

// SSLKeyPairID is a unique identifier for a SSLKeyPair.
// The ID is safe to use as a file name.
type SSLKeyPairID string

// CertBundleID is a unique identifier for a Certificate bundle.
// The ID is safe to use as a file name.
type CertBundleID string

// CertBundle is a Certificate bundle.
type CertBundle []byte

// SSLKeyPair is an SSL private/public key pair.
type SSLKeyPair struct {
	// Cert is the certificate.
	Cert []byte
	// Key is the private key.
	Key []byte
}

// VirtualServer is a virtual server.
type VirtualServer struct {
	// SSL holds the SSL configuration for the server.
	SSL *SSL
	// Hostname is the hostname of the server.
	Hostname string
	// PathRules is a collection of routing rules.
	PathRules []PathRule
	// Policies is a list of Policies that apply to the server.
	Policies []policies.Policy
	// Port is the port of the server.
	Port int32
	// IsDefault indicates whether the server is the default server.
	IsDefault bool
}

// Layer4VirtualServer is a virtual server for Layer 4 traffic.
type Layer4VirtualServer struct {
	// Hostname is the hostname of the server.
	Hostname string
	// UpstreamName refers to the name of the upstream that is used.
	UpstreamName string
	// Port is the port of the server.
	Port int32
	// IsDefault refers to whether this server is created for the default listener hostname.
	IsDefault bool
}

// Upstream is a pool of endpoints to be load balanced.
type Upstream struct {
	// Name is the name of the Upstream. Will be unique for each service/port combination.
	Name string
	// ErrorMsg contains the error message if the Upstream is invalid.
	ErrorMsg string
	// Endpoints are the endpoints of the Upstream.
	Endpoints []resolver.Endpoint
	// Policies holds all the valid policies that apply to the Upstream.
	Policies []policies.Policy
}

// SSL is the SSL configuration for a server.
type SSL struct {
	// KeyPairID is the ID of the corresponding SSLKeyPair for the server.
	KeyPairID SSLKeyPairID
}

// PathRule represents routing rules that share a common path.
type PathRule struct {
	// Path is a path. For example, '/hello'.
	Path string
	// PathType is the type of the path.
	PathType PathType
	// MatchRules holds routing rules.
	MatchRules []MatchRule
	// Policies contains the list of policies that are applied to this PathRule.
	Policies []policies.Policy
	// GRPC indicates if this is a gRPC rule
	GRPC bool
}

// InvalidHTTPFilter is a special filter for handling the case when configured filters are invalid.
type InvalidHTTPFilter struct{}

// HTTPFilters hold the filters for a MatchRule.
type HTTPFilters struct {
	// InvalidFilter is a special filter that indicates whether the filters are invalid. If this is the case,
	// the data plane must return 500 error, and all other filters are nil.
	InvalidFilter *InvalidHTTPFilter
	// RequestRedirect holds the HTTPRequestRedirectFilter.
	RequestRedirect *HTTPRequestRedirectFilter
	// RequestURLRewrite holds the HTTPURLRewriteFilter.
	RequestURLRewrite *HTTPURLRewriteFilter
	// RequestHeaderModifiers holds the HTTPHeaderFilter.
	RequestHeaderModifiers *HTTPHeaderFilter
	// ResponseHeaderModifiers holds the HTTPHeaderFilter.
	ResponseHeaderModifiers *HTTPHeaderFilter
	// SnippetsFilters holds all the SnippetsFilters for the MatchRule.
	// Unlike the core and extended filters, there can be more than one SnippetsFilters defined on a routing rule.
	SnippetsFilters []SnippetsFilter
}

// SnippetsFilter holds the location and server snippets in a SnippetsFilter.
// The main and http snippets are stored separately in Configuration.MainSnippets and BaseHTTPConfig.Snippets.
type SnippetsFilter struct {
	// LocationSnippet holds the snippet for the location context.
	LocationSnippet *Snippet
	// ServerSnippet holds the snippet for the server context.
	ServerSnippet *Snippet
}

// HTTPHeader represents an HTTP header.
type HTTPHeader struct {
	// Name is the name of the header.
	Name string
	// Value is the value of the header.
	Value string
}

// HTTPHeaderFilter manipulates HTTP headers.
type HTTPHeaderFilter struct {
	// Set adds or replaces headers.
	Set []HTTPHeader
	// Add adds headers. It appends to any existing values associated with a header name.
	Add []HTTPHeader
	// Remove removes headers.
	Remove []string
}

// HTTPRequestRedirectFilter redirects HTTP requests.
type HTTPRequestRedirectFilter struct {
	// Scheme is the scheme of the redirect.
	Scheme *string
	// Hostname is the hostname of the redirect.
	Hostname *string
	// Port is the port of the redirect.
	Port *int32
	// StatusCode is the HTTP status code of the redirect.
	StatusCode *int
	// Path is the path of the redirect.
	Path *HTTPPathModifier
}

// HTTPURLRewriteFilter rewrites HTTP requests.
type HTTPURLRewriteFilter struct {
	// Hostname is the hostname of the rewrite.
	Hostname *string
	// Path is the path of the rewrite.
	Path *HTTPPathModifier
}

// PathModifierType is the type of the PathModifier in a redirect or rewrite rule.
type PathModifierType string

const (
	// ReplaceFullPath indicates that we replace the full path.
	ReplaceFullPath PathModifierType = "ReplaceFullPath"
	// ReplacePrefixMatch indicates that we replace a prefix match.
	ReplacePrefixMatch PathModifierType = "ReplacePrefixMatch"
)

// HTTPPathModifier defines configuration for path modifiers.
type HTTPPathModifier struct {
	// Replacement specifies the value with which to replace the full path or prefix match of a request during
	// a rewrite or redirect.
	Replacement string
	// Type indicates the type of path modifier.
	Type PathModifierType
}

// HTTPHeaderMatch matches an HTTP header.
type HTTPHeaderMatch struct {
	// Name is the name of the header to match.
	Name string
	// Value is the value of the header to match.
	Value string
}

// HTTPQueryParamMatch matches an HTTP query parameter.
type HTTPQueryParamMatch struct {
	// Name is the name of the query parameter to match.
	Name string
	// Value is the value of the query parameter to match.
	Value string
}

// MatchRule represents a routing rule. It corresponds directly to a Match in the HTTPRoute resource.
// An HTTPRoute is guaranteed to have at least one rule with one match.
// If no rule or match is specified by the user, the default rule {{path:{ type: "PathPrefix", value: "/"}}}
// is set by the schema.
type MatchRule struct {
	// Filters holds the filters for the MatchRule.
	Filters HTTPFilters
	// Source is the ObjectMeta of the resource that includes the rule.
	Source *metav1.ObjectMeta
	// Match holds the match for the rule.
	Match Match
	// BackendGroup is the group of Backends that the rule routes to.
	BackendGroup BackendGroup
}

// Match represents a match for a routing rule which consist of matches against various HTTP request attributes.
type Match struct {
	// Method matches against the HTTP method.
	Method *string
	// Headers matches against the HTTP headers.
	Headers []HTTPHeaderMatch
	// QueryParams matches against the HTTP query parameters.
	QueryParams []HTTPQueryParamMatch
}

// BackendGroup represents a group of Backends for a routing rule in an HTTPRoute.
type BackendGroup struct {
	// Source is the NamespacedName of the HTTPRoute the group belongs to.
	Source types.NamespacedName
	// Backends is a list of Backends in the Group.
	Backends []Backend
	// RuleIdx is the index of the corresponding rule in the HTTPRoute.
	RuleIdx int
}

// Name returns the name of the backend group.
// This name must be unique across all HTTPRoutes and all rules within the same HTTPRoute.
// It is prefixed with `group_` for cases when namespace name starts with a digit. Variable names
// in nginx configuration cannot start with a digit.
// The RuleIdx is used to make the name unique across all rules within the same HTTPRoute.
// The RuleIdx may change for a given rule if an update is made to the HTTPRoute, but it will always match the index
// of the rule in the stored HTTPRoute.
func (bg *BackendGroup) Name() string {
	return fmt.Sprintf("group_%s__%s_rule%d", bg.Source.Namespace, bg.Source.Name, bg.RuleIdx)
}

// Backend represents a Backend for a routing rule.
type Backend struct {
	// VerifyTLS holds the backend TLS verification configuration.
	VerifyTLS *VerifyTLS
	// UpstreamName is the name of the upstream for this backend.
	UpstreamName string
	// Weight is the weight of the BackendRef.
	// The possible values of weight are 0-1,000,000.
	// If weight is 0, no traffic should be forwarded for this entry.
	Weight int32
	// Valid indicates whether the Backend is valid.
	Valid bool
}

// VerifyTLS holds the backend TLS verification configuration.
type VerifyTLS struct {
	CertBundleID CertBundleID
	Hostname     string
	RootCAPath   string
}

// Telemetry represents global Otel configuration for the dataplane.
type Telemetry struct {
	// Endpoint specifies the address of OTLP/gRPC endpoint that will accept telemetry data.
	Endpoint string
	// ServiceName is the “service.name” attribute of the OTel resource.
	ServiceName string
	// Interval specifies the export interval.
	Interval string
	// Ratios is a list of tracing sampling ratios.
	Ratios []Ratio
	// SpanAttributes are global custom key/value attributes that are added to each span.
	SpanAttributes []SpanAttribute
	// BatchSize specifies the maximum number of spans to be sent in one batch per worker.
	BatchSize int32
	// BatchCount specifies the number of pending batches per worker, spans exceeding the limit are dropped.
	BatchCount int32
}

// SpanAttribute is a key value pair to be added to a tracing span.
type SpanAttribute struct {
	// Key is the key for a span attribute.
	Key string
	// Value is the value for a span attribute.
	Value string
}

// BaseHTTPConfig holds the configuration options at the http context.
type BaseHTTPConfig struct {
	// IPFamily specifies the IP family for all servers.
	IPFamily IPFamilyType
	// Snippets contain the snippets that apply to the http context.
	Snippets []Snippet
	// RewriteIPSettings defines configuration for rewriting the client IP to the original client's IP.
	RewriteClientIPSettings RewriteClientIPSettings
	// HTTP2 specifies whether http2 should be enabled for all servers.
	HTTP2 bool
}

// Snippet is a snippet of configuration.
type Snippet struct {
	// Name is the name of the snippet.
	Name string
	// Contents is the content of the snippet.
	Contents string
}

// RewriteClientIPSettings defines configuration for rewriting the client IP to the original client's IP.
type RewriteClientIPSettings struct {
	// Mode specifies the mode for rewriting the client IP.
	Mode RewriteIPModeType
	// TrustedAddresses specifies the addresses that are trusted to provide the client IP.
	TrustedAddresses []string
	// IPRecursive specifies whether a recursive search is used when selecting the client IP.
	IPRecursive bool
}

// RewriteIPModeType specifies the mode for rewriting the client IP.
type RewriteIPModeType string

const (
	// RewriteIPModeProxyProtocol specifies that client IP will be rewrritten using the Proxy-Protocol header.
	RewriteIPModeProxyProtocol RewriteIPModeType = "proxy_protocol"
	// RewriteIPModeXForwardedFor specifies that client IP will be rewrritten using the X-Forwarded-For header.
	RewriteIPModeXForwardedFor RewriteIPModeType = "X-Forwarded-For"
)

// IPFamilyType specifies the IP family to be used by NGINX.
type IPFamilyType string

const (
	// Dual specifies that the server will use both IPv4 and IPv6.
	Dual IPFamilyType = "dual"
	// IPv4 specifies that the server will use only IPv4.
	IPv4 IPFamilyType = "ipv4"
	// IPv6 specifies that the server will use only IPv6.
	IPv6 IPFamilyType = "ipv6"
)

// Ratio represents a tracing sampling ratio used in an nginx config with the otel_module.
type Ratio struct {
	// Name is based on the associated ObservabilityPolicy's NamespacedName,
	// and is used as the nginx variable name for this ratio.
	Name string
	// Value is the value of the ratio.
	Value int32
}

// Logging defines logging related settings for NGINX.
type Logging struct {
	// ErrorLevel defines the error log level.
	ErrorLevel string
}

// NginxPlus specifies NGINX Plus additional settings.
type NginxPlus struct {
	// AllowedAddresses specifies IPAddresses or CIDR blocks to the allow list for accessing the NGINX Plus API.
	AllowedAddresses []string
}

// DeploymentContext contains metadata about NGF and the cluster.
// This is JSON marshaled into a file created by the generator, hence the json tags.
type DeploymentContext struct {
	// ClusterID is the ID of the kube-system namespace.
	ClusterID *string `json:"cluster_id,omitempty"`
	// InstallationID is the ID of the NGF deployment.
	InstallationID *string `json:"installation_id,omitempty"`
	// ClusterNodeCount is the count of nodes in the cluster.
	ClusterNodeCount *int `json:"cluster_node_count,omitempty"`
	// Integration is "ngf".
	Integration string `json:"integration"`
}
