package framework_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

func TestGetPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		defaultPort  int
		portFwdPort  int
		expectedPort int
	}{
		{
			name:         "uses default when port-forward is inactive",
			defaultPort:  80,
			portFwdPort:  0,
			expectedPort: 80,
		},
		{
			name:         "uses port-forward port when set",
			defaultPort:  80,
			portFwdPort:  8080,
			expectedPort: 8080,
		},
		{
			name:         "zero default with inactive port-forward",
			defaultPort:  0,
			portFwdPort:  0,
			expectedPort: 0,
		},
		{
			name:         "zero default with active port-forward",
			defaultPort:  0,
			portFwdPort:  9090,
			expectedPort: 9090,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(framework.GetPort(tc.defaultPort, tc.portFwdPort)).To(Equal(tc.expectedPort))
		})
	}
}

func TestGetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseURL     string
		port        int
		expectedURL string
	}{
		{
			name:        "omits port when zero",
			baseURL:     "http://hello.example.com",
			port:        0,
			expectedURL: "http://hello.example.com",
		},
		{
			name:        "inserts port without path",
			baseURL:     "http://hello.example.com",
			port:        8080,
			expectedURL: "http://hello.example.com:8080",
		},
		{
			name:        "inserts port before path",
			baseURL:     "http://foo.example.com/hello",
			port:        8080,
			expectedURL: "http://foo.example.com:8080/hello",
		},
		{
			name:        "inserts default HTTP port",
			baseURL:     "http://cafe.example.com/coffee",
			port:        80,
			expectedURL: "http://cafe.example.com:80/coffee",
		},
		{
			name:        "preserves query string encoding",
			baseURL:     "http://cafe.example.com/coffee?x=%3C%2Fscript%3E",
			port:        80,
			expectedURL: "http://cafe.example.com:80/coffee?x=%3C%2Fscript%3E",
		},
		{
			name:        "preserves query and fragment",
			baseURL:     "http://cafe.example.com/coffee?x=%3C%2Fscript%3E#menu",
			port:        80,
			expectedURL: "http://cafe.example.com:80/coffee?x=%3C%2Fscript%3E#menu",
		},
		{
			name:        "preserves fragment without path",
			baseURL:     "http://cafe.example.com#menu",
			port:        80,
			expectedURL: "http://cafe.example.com:80#menu",
		},
		{
			name:        "https scheme",
			baseURL:     "https://cafe.example.com/tea",
			port:        8443,
			expectedURL: "https://cafe.example.com:8443/tea",
		},
		{
			name:        "replaces existing port",
			baseURL:     "http://cafe.example.com:80/coffee",
			port:        8080,
			expectedURL: "http://cafe.example.com:8080/coffee",
		},
		{
			name:        "path only slash",
			baseURL:     "http://hello.example.com/",
			port:        8080,
			expectedURL: "http://hello.example.com:8080/",
		},
		{
			name:        "bracketed IPv6 host",
			baseURL:     "http://[::1]/hello",
			port:        8080,
			expectedURL: "http://[::1]:8080/hello",
		},
		{
			name:        "replaces existing port on IPv6 host",
			baseURL:     "http://[::1]:80/hello",
			port:        8080,
			expectedURL: "http://[::1]:8080/hello",
		},
		{
			name:        "preserves IPv6 query and fragment",
			baseURL:     "http://[2001:db8::1]/coffee?size=large#menu",
			port:        8080,
			expectedURL: "http://[2001:db8::1]:8080/coffee?size=large#menu",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(framework.GetURL(tc.baseURL, tc.port)).To(Equal(tc.expectedURL))
		})
	}
}

func TestGetURLPanicsOnInvalidBaseURL(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{
		"not-a-url",
		"http://[::1",
		"http:///missing-host",
	} {
		baseURL := baseURL
		t.Run(baseURL, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(func() { framework.GetURL(baseURL, 80) }).To(Panic())
		})
	}
}
