package config

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

func TestExecuteStreamServers(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname:     "example.com",
				Port:         8081,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "example.com",
				Port:         8080,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "cafe.example.com",
				Port:         8080,
				UpstreamName: "backend2",
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
				Hostname:     "example.com",
				Port:         8081,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "example.com",
				Port:         8080,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "cafe.example.com",
				Port:         8082,
				UpstreamName: "backend2",
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
				Hostname:     "example.com",
				Port:         8081,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "example.com",
				Port:         8080,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "cafe.example.com",
				Port:         8080,
				UpstreamName: "backend2",
			},
			{
				Hostname:     "blank-upstream.example.com",
				Port:         8081,
				UpstreamName: "",
			},
			{
				Hostname:     "dne-upstream.example.com",
				Port:         8081,
				UpstreamName: "dne",
			},
			{
				Hostname:     "no-endpoints.example.com",
				Port:         8081,
				UpstreamName: "no-endpoints",
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

	streamServers := createStreamServers(conf)

	g := NewWithT(t)

	expectedStreamServers := []stream.Server{
		{
			Listen:     getSocketNameTLS(conf.TLSPassthroughServers[0].Port, conf.TLSPassthroughServers[0].Hostname),
			ProxyPass:  conf.TLSPassthroughServers[0].UpstreamName,
			StatusZone: conf.TLSPassthroughServers[0].Hostname,
			SSLPreread: false,
			IsSocket:   true,
		},
		{
			Listen:     getSocketNameTLS(conf.TLSPassthroughServers[1].Port, conf.TLSPassthroughServers[1].Hostname),
			ProxyPass:  conf.TLSPassthroughServers[1].UpstreamName,
			StatusZone: conf.TLSPassthroughServers[1].Hostname,
			SSLPreread: false,
			IsSocket:   true,
		},
		{
			Listen:     getSocketNameTLS(conf.TLSPassthroughServers[2].Port, conf.TLSPassthroughServers[2].Hostname),
			ProxyPass:  conf.TLSPassthroughServers[2].UpstreamName,
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
			UpstreamName: "backend1",
			Hostname:     "cafe.example.com",
			Port:         8443,
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
			UpstreamName: "backend1",
			Hostname:     "cafe.example.com",
			Port:         8443,
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

	streamServers := createStreamServers(conf)

	g := NewWithT(t)

	g.Expect(streamServers).To(BeNil())
}
