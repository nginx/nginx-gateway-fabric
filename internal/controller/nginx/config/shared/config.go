package shared //nolint:revive,nolintlint // ignoring meaningless package name

// Map defines an NGINX map.
type Map struct {
	Source       string
	Variable     string
	Parameters   []MapParameter
	UseHostnames bool
}

// MapParameter defines a Value and Result pair in a Map.
type MapParameter struct {
	Value  string
	Result string
}

const (
	// ProxyCacheZoneName is the name of the shared cache zone used for content caching.
	ProxyCacheZoneName = "ngf_cache"
	// ProxyCachePath is the directory in which the shared cache zone stores cached responses.
	// It is located in the writable emptyDir volume mounted at /var/cache/nginx in the NGINX container.
	ProxyCachePath = "/var/cache/nginx/ngf_cache"
)

// IPFamily holds the IP family configuration to be used by NGINX.
type IPFamily struct {
	IPv4 bool
	IPv6 bool
}

// RewriteClientIPSettings holds the configuration for the rewrite client IP settings.
type RewriteClientIPSettings struct {
	RealIPHeader  string
	ProxyProtocol string
	RealIPFrom    []string
	Recursive     bool
}

const (
	ProxyProtocolDirective = " proxy_protocol"
)

// Include defines a file that's included via the include directive.
type Include struct {
	Name    string
	Content []byte
}
