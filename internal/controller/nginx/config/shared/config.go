package shared

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
