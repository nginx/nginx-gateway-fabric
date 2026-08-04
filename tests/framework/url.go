package framework

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// GetPort returns portFwdPort when it is non-zero, otherwise defaultPort.
// Used by system tests that always include a port in request URLs (for example
// default HTTP port 80) but override it when a port-forward is active.
func GetPort(defaultPort, portFwdPort int) int {
	if portFwdPort != 0 {
		return portFwdPort
	}

	return defaultPort
}

// GetURL builds a request URL, inserting port between the host and path when port is non-zero.
// If port is 0, baseURL is returned unchanged.
//
// baseURL should be of the form "scheme://host[/path][?query][#fragment]".
// Path, query, and fragment are preserved exactly.
//
// Examples:
//
//	GetURL("http://hello.example.com", 0)          -> "http://hello.example.com"
//	GetURL("http://hello.example.com", 8080)       -> "http://hello.example.com:8080"
//	GetURL("http://hello.example.com/hello", 8080) -> "http://hello.example.com:8080/hello"
//	GetURL("http://cafe.example.com/coffee", 80)   -> "http://cafe.example.com:80/coffee"
func GetURL(baseURL string, port int) string {
	if port == 0 {
		return baseURL
	}

	schemeSep := strings.Index(baseURL, "://")
	if schemeSep < 0 {
		panic(fmt.Sprintf("GetURL: invalid baseURL %q: missing scheme", baseURL))
	}

	hostStart := schemeSep + len("://")
	hostEnd := hostStart
	for hostEnd < len(baseURL) {
		switch baseURL[hostEnd] {
		case '/', '?', '#':
			// end of authority
		default:
			hostEnd++
			continue
		}
		break
	}

	parsed, err := url.Parse(baseURL[:hostEnd])
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" {
		panic(fmt.Sprintf("GetURL: invalid baseURL %q", baseURL))
	}

	authority := net.JoinHostPort(parsed.Hostname(), strconv.Itoa(port))
	if parsed.User != nil {
		authority = parsed.User.String() + "@" + authority
	}

	return baseURL[:hostStart] + authority + baseURL[hostEnd:]
}
