package http

// Server holds all configuration for an HTTP server.
type Server struct {
	SSL           *SSL
	ServerName    string
	Locations     []Location
	IsDefaultHTTP bool
	IsDefaultSSL  bool
	Port          int32
}

// Location holds all configuration for an HTTP location.
type Location struct {
	Return          *Return
	Rewrites        []string
	Path            string
	ProxyPass       string
	HTTPMatchVar    string
	ProxySetHeaders []Header
	Internal        bool
}

// Header defines a HTTP header to be passed to the proxied server.
type Header struct {
	Name  string
	Value string
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
	Name    string
	Servers []UpstreamServer
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

// Map defines an NGINX map.
type Map struct {
	Source     string
	Variable   string
	Parameters []MapParameter
}

// Parameter defines a Value and Result pair in a Map.
type MapParameter struct {
	Value  string
	Result string
}
