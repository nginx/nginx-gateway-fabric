package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

func TestExecuteStreamServers(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname: "example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "cafe.example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend2", Weight: 0},
				},
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name: "backend1",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
			{
				Name: "backend2",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
		},
	}

	expSubStrings := map[string]int{
		"pass $dest8081;": 1,
		"pass $dest8080;": 1,
		"ssl_preread on;": 2,
		"proxy_pass":      3,
		"status_zone":     0,
	}
	g := NewWithT(t)

	gen := GeneratorImpl{}
	results := gen.executeStreamServers(conf)
	g.Expect(results).To(HaveLen(1))
	result := results[0]

	g.Expect(result.dest).To(Equal(streamConfigFile))
	for expSubStr, expCount := range expSubStrings {
		g.Expect(strings.Count(string(result.data), expSubStr)).To(Equal(expCount))
	}
}

func TestExecuteStreamServers_Plus(t *testing.T) {
	t.Parallel()
	config := dataplane.Configuration{
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname: "example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "cafe.example.com",
				Port:     8082,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend2", Weight: 0},
				},
			},
		},
	}
	expectedHTTPConfig := map[string]int{
		"status_zone example.com;":      2,
		"status_zone cafe.example.com;": 1,
	}

	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}
	results := gen.executeStreamServers(config)
	g.Expect(results).To(HaveLen(1))

	serverConf := string(results[0].data)

	for expSubStr, expCount := range expectedHTTPConfig {
		g.Expect(strings.Count(serverConf, expSubStr)).To(Equal(expCount))
	}
}

func TestCreateStreamServers(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname: "example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "cafe.example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend2", Weight: 0},
				},
			},
			{
				Hostname:  "blank-upstream.example.com",
				Port:      8081,
				Upstreams: []dataplane.Layer4Upstream{},
			},
			{
				Hostname: "dne-upstream.example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "dne", Weight: 0},
				},
			},
			{
				Hostname: "no-endpoints.example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "no-endpoints", Weight: 0},
				},
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name: "backend1",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
			{
				Name: "backend2",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
			{
				Name:      "no-endpoints",
				Endpoints: nil,
			},
		},
	}

	logger := logr.Discard()
	streamServers := createStreamServers(logger, conf)

	g := NewWithT(t)

	expectedStreamServers := []stream.Server{
		{
			Listen:     getSocketNameTLS(conf.TLSPassthroughServers[0].Port, conf.TLSPassthroughServers[0].Hostname),
			ProxyPass:  conf.TLSPassthroughServers[0].Upstreams[0].Name,
			StatusZone: conf.TLSPassthroughServers[0].Hostname,
			SSLPreread: false,
			IsSocket:   true,
		},
		{
			Listen:     getSocketNameTLS(conf.TLSPassthroughServers[1].Port, conf.TLSPassthroughServers[1].Hostname),
			ProxyPass:  conf.TLSPassthroughServers[1].Upstreams[0].Name,
			StatusZone: conf.TLSPassthroughServers[1].Hostname,
			SSLPreread: false,
			IsSocket:   true,
		},
		{
			Listen:     getSocketNameTLS(conf.TLSPassthroughServers[2].Port, conf.TLSPassthroughServers[2].Hostname),
			ProxyPass:  conf.TLSPassthroughServers[2].Upstreams[0].Name,
			StatusZone: conf.TLSPassthroughServers[2].Hostname,
			SSLPreread: false,
			IsSocket:   true,
		},
		{
			Listen:     fmt.Sprint(8081),
			Pass:       getTLSPassthroughVarName(8081),
			StatusZone: "example.com",
			SSLPreread: true,
		},
		{
			Listen:     fmt.Sprint(8080),
			Pass:       getTLSPassthroughVarName(8080),
			StatusZone: "example.com",
			SSLPreread: true,
		},
	}
	g.Expect(streamServers).To(ConsistOf(expectedStreamServers))
}

func TestExecuteStreamServersForIPFamily(t *testing.T) {
	t.Parallel()
	passThroughServers := []dataplane.Layer4VirtualServer{
		{
			Hostname: "cafe.example.com",
			Port:     8443,
			Upstreams: []dataplane.Layer4Upstream{
				{Name: "backend1", Weight: 0},
			},
		},
	}
	streamUpstreams := []dataplane.Upstream{
		{
			Name: "backend1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "1.1.1.1",
				},
			},
		},
	}
	tests := []struct {
		msg                  string
		expectedServerConfig map[string]int
		config               dataplane.Configuration
	}{
		{
			msg: "tls servers with IPv4 IP family",
			config: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.IPv4,
				},
				TLSPassthroughServers: passThroughServers,
				StreamUpstreams:       streamUpstreams,
			},
			expectedServerConfig: map[string]int{
				"listen 8443;": 1,
				"listen unix:/var/run/nginx/cafe.example.com-8443.sock;": 1,
			},
		},
		{
			msg: "tls servers with IPv6 IP family",
			config: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.IPv6,
				},
				TLSPassthroughServers: passThroughServers,
				StreamUpstreams:       streamUpstreams,
			},
			expectedServerConfig: map[string]int{
				"listen [::]:8443;": 1,
				"listen unix:/var/run/nginx/cafe.example.com-8443.sock;": 1,
			},
		},
		{
			msg: "tls servers with dual IP family",
			config: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.Dual,
				},
				TLSPassthroughServers: passThroughServers,
				StreamUpstreams:       streamUpstreams,
			},
			expectedServerConfig: map[string]int{
				"listen 8443;":      1,
				"listen [::]:8443;": 1,
				"listen unix:/var/run/nginx/cafe.example.com-8443.sock;": 1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gen := GeneratorImpl{}
			results := gen.executeStreamServers(test.config)
			g.Expect(results).To(HaveLen(1))
			serverConf := string(results[0].data)

			for expSubStr, expCount := range test.expectedServerConfig {
				g.Expect(strings.Count(serverConf, expSubStr)).To(Equal(expCount))
			}
		})
	}
}

func TestExecuteStreamServers_RewriteClientIP(t *testing.T) {
	t.Parallel()
	passThroughServers := []dataplane.Layer4VirtualServer{
		{
			Hostname: "cafe.example.com",
			Port:     8443,
			Upstreams: []dataplane.Layer4Upstream{
				{Name: "backend1", Weight: 0},
			},
		},
	}
	streamUpstreams := []dataplane.Upstream{
		{
			Name: "backend1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "1.1.1.1",
				},
			},
		},
	}
	tests := []struct {
		msg                  string
		expectedStreamConfig map[string]int
		config               dataplane.Configuration
	}{
		{
			msg: "rewrite client IP not configured",
			config: dataplane.Configuration{
				TLSPassthroughServers: passThroughServers,
				StreamUpstreams:       streamUpstreams,
			},
			expectedStreamConfig: map[string]int{
				"listen 8443;":      1,
				"listen [::]:8443;": 1,
				"listen unix:/var/run/nginx/cafe.example.com-8443.sock;": 1,
			},
		},
		{
			msg: "rewrite client IP configured with proxy protocol",
			config: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					RewriteClientIPSettings: dataplane.RewriteClientIPSettings{
						Mode:             dataplane.RewriteIPModeProxyProtocol,
						TrustedAddresses: []string{"10.1.1.22/32", "::1/128", "3.4.5.6"},
						IPRecursive:      false,
					},
				},
				TLSPassthroughServers: passThroughServers,
				StreamUpstreams:       streamUpstreams,
			},
			expectedStreamConfig: map[string]int{
				"listen 8443;":      1,
				"listen [::]:8443;": 1,
				"listen unix:/var/run/nginx/cafe.example.com-8443.sock proxy_protocol;": 1,
				"set_real_ip_from 10.1.1.22/32;":                                        1,
				"set_real_ip_from ::1/128;":                                             1,
				"set_real_ip_from 3.4.5.6;":                                             1,
				"real_ip_recursive on;":                                                 0,
			},
		},
		{
			msg: "rewrite client IP configured with xforwardedfor",
			config: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					RewriteClientIPSettings: dataplane.RewriteClientIPSettings{
						Mode:             dataplane.RewriteIPModeXForwardedFor,
						TrustedAddresses: []string{"1.1.1.1/32"},
						IPRecursive:      true,
					},
				},
				TLSPassthroughServers: passThroughServers,
				StreamUpstreams:       streamUpstreams,
			},
			expectedStreamConfig: map[string]int{
				"listen 8443;":      1,
				"listen [::]:8443;": 1,
				"listen unix:/var/run/nginx/cafe.example.com-8443.sock;": 1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gen := GeneratorImpl{}
			results := gen.executeStreamServers(test.config)
			g.Expect(results).To(HaveLen(1))
			serverConf := string(results[0].data)

			for expSubStr, expCount := range test.expectedStreamConfig {
				g.Expect(strings.Count(serverConf, expSubStr)).To(Equal(expCount))
			}
		})
	}
}

func TestCreateStreamServersWithNone(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		TLSPassthroughServers: nil,
	}

	logger := logr.Discard()
	streamServers := createStreamServers(logger, conf)

	g := NewWithT(t)

	g.Expect(streamServers).To(BeNil())
}

func TestExecuteStreamServersWithResolver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		expectedConfig string
		conf           dataplane.Configuration
	}{
		{
			name: "stream servers with DNS resolver",
			conf: dataplane.Configuration{
				BaseStreamConfig: dataplane.BaseStreamConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses:   []string{"8.8.8.8", "8.8.4.4"},
						Timeout:     "10s",
						Valid:       "60s",
						DisableIPv6: true,
					},
				},
			},
			expectedConfig: `
# DNS resolver configuration for ExternalName services
resolver 8.8.8.8 8.8.4.4 valid=60s ipv6=off;
resolver_timeout 10s;

server {
    listen unix:/var/run/nginx/connection-closed-server.sock;
    return "";
}
`,
		},
		{
			name: "stream servers without DNS resolver",
			conf: dataplane.Configuration{
				BaseStreamConfig: dataplane.BaseStreamConfig{
					DNSResolver: nil,
				},
			},
			expectedConfig: `

server {
    listen unix:/var/run/nginx/connection-closed-server.sock;
    return "";
}
`,
		},
		{
			name: "stream servers with DNS resolver IPv6 enabled",
			conf: dataplane.Configuration{
				BaseStreamConfig: dataplane.BaseStreamConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses:   []string{"2001:4860:4860::8888"},
						Timeout:     "5s",
						Valid:       "30s",
						DisableIPv6: false,
					},
				},
			},
			expectedConfig: `
# DNS resolver configuration for ExternalName services
resolver [2001:4860:4860::8888] valid=30s;
resolver_timeout 5s;

server {
    listen unix:/var/run/nginx/connection-closed-server.sock;
    return "";
}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			generator := GeneratorImpl{}
			results := generator.executeStreamServers(test.conf)

			g.Expect(results).To(HaveLen(1))
			g.Expect(string(results[0].data)).To(Equal(test.expectedConfig))
		})
	}
}

func TestCreateSplitClientForL4Server(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expected *stream.SplitClient
		name     string
		server   dataplane.Layer4VirtualServer
	}{
		{
			name: "single upstream returns nil",
			server: dataplane.Layer4VirtualServer{
				Port:      8080,
				Upstreams: []dataplane.Layer4Upstream{{Name: "upstream1", Weight: 100}},
			},
			expected: nil,
		},
		{
			name: "two upstreams with 80/20 weights",
			server: dataplane.Layer4VirtualServer{
				Port: 9000,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "upstream1", Weight: 80},
					{Name: "upstream2", Weight: 20},
				},
			},
			expected: &stream.SplitClient{
				VariableName: "backend_9000",
				Distributions: []stream.SplitClientDistribution{
					{Percent: "80.00", Value: "upstream1"},
					{Percent: "20.00", Value: "upstream2"},
				},
			},
		},
		{
			name: "totalWeight zero returns nil",
			server: dataplane.Layer4VirtualServer{
				Port: 8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "upstream1", Weight: 0},
					{Name: "upstream2", Weight: 0},
				},
			},
			expected: nil,
		},
		{
			name: "three upstreams with remainder",
			server: dataplane.Layer4VirtualServer{
				Port: 8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "upstream1", Weight: 33},
					{Name: "upstream2", Weight: 33},
					{Name: "upstream3", Weight: 34},
				},
			},
			expected: &stream.SplitClient{
				VariableName: "backend_8080",
				Distributions: []stream.SplitClientDistribution{
					{Percent: "33.00", Value: "upstream1"},
					{Percent: "33.00", Value: "upstream2"},
					{Percent: "34.00", Value: "upstream3"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createSplitClientForL4Server(tt.server)

			if tt.expected == nil {
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(result).ToNot(BeNil())
				g.Expect(result.VariableName).To(Equal(tt.expected.VariableName))
				g.Expect(result.Distributions).To(Equal(tt.expected.Distributions))
			}
		})
	}
}

func TestCreateStreamSplitClients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expectedVars []string
		conf         dataplane.Configuration
	}{
		{
			name:         "no servers",
			conf:         dataplane.Configuration{},
			expectedVars: nil,
		},
		{
			name: "TCP with single backend - no split",
			conf: dataplane.Configuration{
				TCPServers: []dataplane.Layer4VirtualServer{
					{Port: 8080, Upstreams: []dataplane.Layer4Upstream{{Name: "tcp1", Weight: 100}}},
				},
			},
			expectedVars: nil,
		},
		{
			name: "TCP and UDP with weights",
			conf: dataplane.Configuration{
				TCPServers: []dataplane.Layer4VirtualServer{
					{Port: 8080, Upstreams: []dataplane.Layer4Upstream{
						{Name: "tcp1", Weight: 50}, {Name: "tcp2", Weight: 50},
					}},
				},
				UDPServers: []dataplane.Layer4VirtualServer{
					{Port: 5353, Upstreams: []dataplane.Layer4Upstream{
						{Name: "udp1", Weight: 70}, {Name: "udp2", Weight: 30},
					}},
				},
			},
			expectedVars: []string{"backend_8080", "backend_5353"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createStreamSplitClients(tt.conf)

			if tt.expectedVars == nil {
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(result).To(HaveLen(len(tt.expectedVars)))
				for i, varName := range tt.expectedVars {
					g.Expect(result[i].VariableName).To(Equal(varName))
				}
			}
		})
	}
}

func TestExecuteStreamServersWithTCPUDPWeights(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Test split_clients generation with weighted TCP backends
	splitClients := createStreamSplitClients(dataplane.Configuration{
		TCPServers: []dataplane.Layer4VirtualServer{
			{
				Port: 8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "tcp_v1", Weight: 80},
					{Name: "tcp_v2", Weight: 20},
				},
			},
		},
	})

	g.Expect(splitClients).To(HaveLen(1))
	g.Expect(splitClients[0].VariableName).To(Equal("backend_8080"))
	g.Expect(splitClients[0].Distributions).To(HaveLen(2))
	g.Expect(splitClients[0].Distributions[0].Percent).To(Equal("80.00"))
	g.Expect(splitClients[0].Distributions[0].Value).To(Equal("tcp_v1"))
	g.Expect(splitClients[0].Distributions[1].Percent).To(Equal("20.00"))
	g.Expect(splitClients[0].Distributions[1].Value).To(Equal("tcp_v2"))
}

func TestProcessLayer4Servers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		upstreams      map[string]dataplane.Upstream
		portSet        map[int32]struct{}
		expectedServer *stream.Server
		name           string
		protocol       string
		servers        []dataplane.Layer4VirtualServer
		expectedCount  int
	}{
		{
			name:          "empty servers",
			servers:       []dataplane.Layer4VirtualServer{},
			upstreams:     map[string]dataplane.Upstream{},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 0,
		},
		{
			name: "TCP server with single upstream",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 8080, Upstreams: []dataplane.Layer4Upstream{{Name: "backend1"}}},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend1": {
					Name:      "backend1",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.1", Port: 8080}},
				},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 1,
			expectedServer: &stream.Server{
				Listen:     "8080",
				StatusZone: "TCP_8080",
				ProxyPass:  "backend1",
			},
		},
		{
			name: "UDP server with single upstream",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 5353, Upstreams: []dataplane.Layer4Upstream{{Name: "dns-backend"}}},
			},
			upstreams: map[string]dataplane.Upstream{
				"dns-backend": {
					Name:      "dns-backend",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.2", Port: 53}},
				},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.UDPProtocolType),
			expectedCount: 1,
			expectedServer: &stream.Server{
				Listen:     "5353 udp",
				StatusZone: "UDP_5353",
				ProxyPass:  "dns-backend",
			},
		},
		{
			name: "server with multiple upstreams",
			servers: []dataplane.Layer4VirtualServer{
				{
					Port: 9000,
					Upstreams: []dataplane.Layer4Upstream{
						{Name: "backend-v1", Weight: 80},
						{Name: "backend-v2", Weight: 20},
					},
				},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend-v1": {
					Name:      "backend-v1",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.3", Port: 9000}},
				},
				"backend-v2": {
					Name:      "backend-v2",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.4", Port: 9000}},
				},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 1,
			expectedServer: &stream.Server{
				Listen:     "9000",
				StatusZone: "TCP_9000",
				ProxyPass:  "$backend_9000",
			},
		},
		{
			name: "skip server on port already in portSet",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 8080, Upstreams: []dataplane.Layer4Upstream{{Name: "backend1"}}},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend1": {
					Name:      "backend1",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.1", Port: 8080}},
				},
			},
			portSet:       map[int32]struct{}{8080: {}},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 0,
		},
		{
			name: "skip server with no upstreams",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 8080, Upstreams: []dataplane.Layer4Upstream{}},
			},
			upstreams:     map[string]dataplane.Upstream{},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 0,
		},
		{
			name: "skip server with upstream not found",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 8080, Upstreams: []dataplane.Layer4Upstream{{Name: "missing-backend"}}},
			},
			upstreams:     map[string]dataplane.Upstream{},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 0,
		},
		{
			name: "skip server with upstream having no endpoints",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 8080, Upstreams: []dataplane.Layer4Upstream{{Name: "backend1"}}},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend1": {
					Name:      "backend1",
					Endpoints: []resolver.Endpoint{},
				},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 0,
		},
		{
			name: "skip server with multiple upstreams all having no endpoints",
			servers: []dataplane.Layer4VirtualServer{
				{
					Port: 9000,
					Upstreams: []dataplane.Layer4Upstream{
						{Name: "backend-v1", Weight: 80},
						{Name: "backend-v2", Weight: 20},
					},
				},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend-v1": {Name: "backend-v1", Endpoints: []resolver.Endpoint{}},
				"backend-v2": {Name: "backend-v2", Endpoints: []resolver.Endpoint{}},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 0,
		},
		{
			name: "accept multiple upstreams with at least one having endpoints",
			servers: []dataplane.Layer4VirtualServer{
				{
					Port: 9000,
					Upstreams: []dataplane.Layer4Upstream{
						{Name: "backend-v1", Weight: 80},
						{Name: "backend-v2", Weight: 20},
					},
				},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend-v1": {
					Name:      "backend-v1",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.3", Port: 9000}},
				},
				"backend-v2": {Name: "backend-v2", Endpoints: []resolver.Endpoint{}},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 1,
			expectedServer: &stream.Server{
				Listen:     "9000",
				StatusZone: "TCP_9000",
				ProxyPass:  "$backend_9000",
			},
		},
		{
			name: "multiple servers on different ports",
			servers: []dataplane.Layer4VirtualServer{
				{Port: 8080, Upstreams: []dataplane.Layer4Upstream{{Name: "backend1"}}},
				{Port: 9000, Upstreams: []dataplane.Layer4Upstream{{Name: "backend2"}}},
			},
			upstreams: map[string]dataplane.Upstream{
				"backend1": {
					Name:      "backend1",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.1", Port: 8080}},
				},
				"backend2": {
					Name:      "backend2",
					Endpoints: []resolver.Endpoint{{Address: "10.0.0.2", Port: 9000}},
				},
			},
			portSet:       map[int32]struct{}{},
			protocol:      string(v1.TCPProtocolType),
			expectedCount: 2,
			expectedServer: &stream.Server{
				Listen:     "8080",
				StatusZone: "TCP_8080",
				ProxyPass:  "backend1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			streamServers := []stream.Server{}
			portSet := tt.portSet
			if portSet == nil {
				portSet = map[int32]struct{}{}
			}

			logger := logr.Discard()
			processLayer4Servers(logger, tt.servers, tt.upstreams, portSet, &streamServers, tt.protocol)

			g.Expect(streamServers).To(HaveLen(tt.expectedCount))

			if tt.expectedServer != nil && len(streamServers) > 0 {
				g.Expect(streamServers[0].Listen).To(Equal(tt.expectedServer.Listen))
				g.Expect(streamServers[0].StatusZone).To(Equal(tt.expectedServer.StatusZone))
				g.Expect(streamServers[0].ProxyPass).To(Equal(tt.expectedServer.ProxyPass))
			}
		})
	}
}
