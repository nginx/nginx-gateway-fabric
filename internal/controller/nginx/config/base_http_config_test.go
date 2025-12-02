package config

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

func TestLoggingSettingsTemplate(t *testing.T) {
	t.Parallel()

	logFormat := "$remote_addr - [$time_local] \"$request\" $status $body_bytes_sent"

	tests := []struct {
		name              string
		accessLog         *dataplane.AccessLog
		expectedOutputs   []string
		unexpectedOutputs []string
	}{
		{
			name:      "Log format and access log with custom format",
			accessLog: &dataplane.AccessLog{Format: logFormat},
			expectedOutputs: []string{
				fmt.Sprintf("log_format %s '%s'", dataplane.DefaultLogFormatName, logFormat),
				fmt.Sprintf("access_log %s %s", dataplane.DefaultAccessLogPath, dataplane.DefaultLogFormatName),
			},
		},
		{
			name:      "Empty format",
			accessLog: &dataplane.AccessLog{Format: ""},
			unexpectedOutputs: []string{
				fmt.Sprintf("log_format %s '%s'", dataplane.DefaultLogFormatName, logFormat),
				fmt.Sprintf("access_log %s %s", dataplane.DefaultAccessLogPath, dataplane.DefaultLogFormatName),
			},
		},
		{
			name:      "Access log off while format presented",
			accessLog: &dataplane.AccessLog{Disable: true, Format: logFormat},
			expectedOutputs: []string{
				`access_log off;`,
			},
			unexpectedOutputs: []string{
				fmt.Sprintf("access_log off %s", dataplane.DefaultLogFormatName),
			},
		},
		{
			name:      "Access log off",
			accessLog: &dataplane.AccessLog{Disable: true},
			expectedOutputs: []string{
				`access_log off;`,
			},
			unexpectedOutputs: []string{
				`log_format`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conf := dataplane.Configuration{
				Logging: dataplane.Logging{AccessLog: tt.accessLog},
			}

			res := executeBaseHTTPConfig(conf)
			g.Expect(res).To(HaveLen(1))
			httpConfig := string(res[0].data)
			for _, expectedOutput := range tt.expectedOutputs {
				g.Expect(httpConfig).To(ContainSubstring(expectedOutput))
			}
			for _, unexpectedOutput := range tt.unexpectedOutputs {
				g.Expect(httpConfig).ToNot(ContainSubstring(unexpectedOutput))
			}
		})
	}
}

func TestExecuteBaseHttp_HTTP2(t *testing.T) {
	t.Parallel()
	confOn := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			HTTP2: true,
		},
	}

	confOff := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			HTTP2: false,
		},
	}

	expSubStr := "http2 on;"

	tests := []struct {
		name     string
		conf     dataplane.Configuration
		expCount int
	}{
		{
			name:     "http2 on",
			conf:     confOn,
			expCount: 1,
		},
		{
			name:     "http2 off",
			expCount: 0,
			conf:     confOff,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf)
			g.Expect(res).To(HaveLen(1))
			g.Expect(test.expCount).To(Equal(strings.Count(string(res[0].data), expSubStr)))
			g.Expect(strings.Count(string(res[0].data), "map $http_host $gw_api_compliant_host {")).To(Equal(1))
			g.Expect(strings.Count(string(res[0].data), "map $http_upgrade $connection_upgrade {")).To(Equal(1))
			g.Expect(strings.Count(string(res[0].data), "map $request_uri $request_uri_path {")).To(Equal(1))
		})
	}
}

func TestExecuteBaseHttp_Snippets(t *testing.T) {
	t.Parallel()

	conf := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			Snippets: []dataplane.Snippet{
				{
					Name:     "snippet1",
					Contents: "contents1",
				},
				{
					Name:     "snippet2",
					Contents: "contents2",
				},
			},
		},
	}

	g := NewWithT(t)

	res := executeBaseHTTPConfig(conf)
	g.Expect(res).To(HaveLen(3))

	sort.Slice(
		res, func(i, j int) bool {
			return res[i].dest < res[j].dest
		},
	)

	/*
		Order of files:
		/etc/nginx/conf.d/http.conf
		/etc/nginx/includes/snippet1.conf
		/etc/nginx/includes/snippet2.conf
	*/

	httpRes := string(res[0].data)
	g.Expect(httpRes).To(ContainSubstring("map $http_host $gw_api_compliant_host {"))
	g.Expect(httpRes).To(ContainSubstring("map $http_upgrade $connection_upgrade {"))
	g.Expect(httpRes).To(ContainSubstring("map $request_uri $request_uri_path {"))
	g.Expect(httpRes).To(ContainSubstring("include /etc/nginx/includes/snippet1.conf;"))
	g.Expect(httpRes).To(ContainSubstring("include /etc/nginx/includes/snippet2.conf;"))

	snippet1IncludeRes := string(res[1].data)
	g.Expect(snippet1IncludeRes).To(ContainSubstring("contents1"))

	snippet2IncludeRes := string(res[2].data)
	g.Expect(snippet2IncludeRes).To(ContainSubstring("contents2"))
}

func TestExecuteBaseHttp_NginxReadinessProbePort(t *testing.T) {
	t.Parallel()

	defaultConfig := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: dataplane.DefaultNginxReadinessProbePort,
		},
	}

	customPortConfig := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: 9090,
		},
	}

	customIPv4Config := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: dataplane.DefaultNginxReadinessProbePort,
			IPFamily:                dataplane.IPv4,
		},
	}

	customIPv6Config := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: dataplane.DefaultNginxReadinessProbePort,
			IPFamily:                dataplane.IPv6,
		},
	}

	tests := []struct {
		name             string
		expectedPort     string
		expectedListen   string
		expectedNoListen string
		conf             dataplane.Configuration
	}{
		{
			name:           "default nginx readiness probe port",
			conf:           defaultConfig,
			expectedPort:   "8081",
			expectedListen: "listen 8081;",
		},
		{
			name:           "default nginx readiness probe port on ipv6",
			conf:           defaultConfig,
			expectedPort:   "8081",
			expectedListen: "listen [::]:8081;",
		},
		{
			name:           "custom nginx readiness probe 9090",
			conf:           customPortConfig,
			expectedPort:   "9090",
			expectedListen: "listen 9090;",
		},
		{
			name:           "custom nginx readiness probe 9090 on ipv6",
			conf:           customPortConfig,
			expectedPort:   "9090",
			expectedListen: "listen [::]:9090;",
		},
		{
			name:             "custom ipv4 nginx readiness probe does not have ipv6 listen",
			conf:             customIPv4Config,
			expectedPort:     "8081",
			expectedListen:   "listen 8081;",
			expectedNoListen: "listen [::]:8081;",
		},
		{
			name:             "custom ipv6 nginx readiness probe does not have ipv4 listen",
			conf:             customIPv6Config,
			expectedPort:     "8081",
			expectedListen:   "listen [::]:8081;",
			expectedNoListen: "listen 8081;",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf)
			g.Expect(res).To(HaveLen(1))

			httpConfig := string(res[0].data)

			// check that the listen directive contains the expected port
			g.Expect(httpConfig).To(ContainSubstring(test.expectedListen))

			// check that an additional listen directive is NOT set
			if test.expectedNoListen != "" {
				g.Expect(httpConfig).ToNot(ContainSubstring(test.expectedNoListen))
			}

			// check that the health check server block is present
			g.Expect(httpConfig).To(ContainSubstring("server {"))
			g.Expect(httpConfig).To(ContainSubstring("access_log off;"))
			g.Expect(httpConfig).To(ContainSubstring("location = /readyz {"))
			g.Expect(httpConfig).To(ContainSubstring("return 200;"))
		})
	}
}

func TestExecuteBaseHttp_DNSResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expectedConfig string
		conf           dataplane.Configuration
	}{
		{
			name: "DNS resolver with all options",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses:   []string{"8.8.8.8", "8.8.4.4"},
						Timeout:     "10s",
						Valid:       "60s",
						DisableIPv6: true,
					},
				},
			},
			expectedConfig: "resolver 8.8.8.8 8.8.4.4 valid=60s ipv6=off;\nresolver_timeout 10s;",
		},
		{
			name: "DNS resolver with single address and IPv6 enabled",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses:   []string{"8.8.8.8"},
						DisableIPv6: false,
					},
				},
			},
			expectedConfig: "resolver 8.8.8.8;",
		},
		{
			name: "DNS resolver with single IPv6 address",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses: []string{"2606:4700:4700::64"},
					},
				},
			},
			expectedConfig: "resolver [2606:4700:4700::64];",
		},
		{
			name: "DNS resolver with one IPv6 address and one IPv4 address",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses: []string{"2606:4700:4700::64", "8.8.8.8"},
					},
				},
			},
			expectedConfig: "resolver [2606:4700:4700::64] 8.8.8.8;",
		},
		{
			name: "no DNS resolver",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: nil,
				},
			},
			expectedConfig: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf)
			g.Expect(res).To(HaveLen(1))

			httpConfig := string(res[0].data)

			if test.expectedConfig != "" {
				// Check that the resolver directive is present
				g.Expect(httpConfig).To(ContainSubstring(test.expectedConfig))
				// Check that the comment is present
				g.Expect(httpConfig).To(ContainSubstring("# DNS resolver configuration for ExternalName services"))
			} else {
				// Check that no resolver directive is present
				g.Expect(httpConfig).ToNot(ContainSubstring("resolver"))
				g.Expect(httpConfig).ToNot(ContainSubstring("# DNS resolver configuration for ExternalName services"))
			}

			// Verify that standard config elements are still present
			g.Expect(httpConfig).To(ContainSubstring("map $http_host $gw_api_compliant_host {"))
			g.Expect(httpConfig).To(ContainSubstring("map $http_upgrade $connection_upgrade {"))
			g.Expect(httpConfig).To(ContainSubstring("map $request_uri $request_uri_path {"))
		})
	}
}

func TestExecuteBaseHttp_GatewaySecretID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expectedConfig string
		conf           dataplane.Configuration
	}{
		{
			name: "with GatewaySecretID",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					GatewaySecretID: "client-secret",
				},
			},
			expectedConfig: "proxy_ssl_certificate /etc/nginx/secrets/client-secret.pem;" +
				"\nproxy_ssl_certificate_key /etc/nginx/secrets/client-secret.pem;",
		},
		{
			name: "without GatewaySecretID",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					GatewaySecretID: "",
				},
			},
			expectedConfig: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf)
			g.Expect(res).To(HaveLen(1))

			httpConfig := string(res[0].data)

			if test.expectedConfig != "" {
				g.Expect(httpConfig).To(ContainSubstring(test.expectedConfig))
			}
		})
	}
}
