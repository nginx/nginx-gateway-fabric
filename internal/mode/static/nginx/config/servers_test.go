package config

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var (
	httpBaseHeaders             = createBaseProxySetHeaders(httpUpgradeHeader, httpConnectionHeader)
	grpcBaseHeaders             = createBaseProxySetHeaders(grpcAuthorityHeader)
	alwaysFalseKeepAliveChecker = func(_ string) bool { return false }
)

func TestExecuteServers(t *testing.T) {
	t.Parallel()

	conf := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{
			{
				IsDefault: true,
				Port:      8080,
			},
			{
				Hostname: "example.com",
				Port:     8080,
			},
			{
				Hostname: "cafe.example.com",
				Port:     8080,
				Policies: []policies.Policy{
					&policiesfakes.FakePolicy{},
				},
				PathRules: []dataplane.PathRule{
					{
						Path:     "/",
						PathType: dataplane.PathTypePrefix,
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									SnippetsFilters: []dataplane.SnippetsFilter{
										{
											LocationSnippet: &dataplane.Snippet{
												Name:     "location-snippet",
												Contents: "location snippet contents",
											},
											ServerSnippet: &dataplane.Snippet{
												Name:     "server-snippet",
												Contents: "server snippet contents",
											},
										},
									},
								},
								Match: dataplane.Match{},
								BackendGroup: dataplane.BackendGroup{
									Source:  types.NamespacedName{Namespace: "test", Name: "route1"},
									RuleIdx: 0,
									Backends: []dataplane.Backend{
										{
											UpstreamName: "test_foo_443",
											Valid:        true,
											Weight:       1,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				IsDefault: true,
				Port:      8443,
			},
			{
				Hostname: "example.com",
				SSL: &dataplane.SSL{
					KeyPairID: "test-keypair",
				},
				Port: 8443,
			},
			{
				Hostname: "cafe.example.com",
				SSL: &dataplane.SSL{
					KeyPairID: "test-keypair",
				},
				Port: 8443,
				PathRules: []dataplane.PathRule{
					{
						Path:     "/",
						PathType: dataplane.PathTypePrefix,
						MatchRules: []dataplane.MatchRule{
							{
								Match: dataplane.Match{},
								BackendGroup: dataplane.BackendGroup{
									Source:  types.NamespacedName{Namespace: "test", Name: "route1"},
									RuleIdx: 0,
									Backends: []dataplane.Backend{
										{
											UpstreamName: "test_foo_443",
											Valid:        true,
											Weight:       1,
											VerifyTLS: &dataplane.VerifyTLS{
												CertBundleID: "test-foo",
												Hostname:     "test-foo.example.com",
											},
										},
									},
								},
							},
						},
					},
				},
				Policies: []policies.Policy{
					&policiesfakes.FakePolicy{},
				},
			},
		},
	}

	expSubStrings := map[string]int{
		"listen 8080 default_server;":                              1,
		"listen 8080;":                                             2,
		"listen 8443 ssl;":                                         2,
		"listen 8443 ssl default_server;":                          1,
		"server_name example.com;":                                 2,
		"server_name cafe.example.com;":                            2,
		"ssl_certificate /etc/nginx/secrets/test-keypair.pem;":     2,
		"ssl_certificate_key /etc/nginx/secrets/test-keypair.pem;": 2,
		"proxy_ssl_server_name on;":                                1,
		"status_zone":                                              0,
		"include /etc/nginx/includes/location-snippet.conf":        1,
		"include /etc/nginx/includes/server-snippet.conf":          1,
	}

	type assertion func(g *WithT, data string)

	expectedResults := map[string]assertion{
		httpConfigFile: func(g *WithT, data string) {
			for expSubStr, expCount := range expSubStrings {
				g.Expect(strings.Count(data, expSubStr)).To(Equal(expCount))
			}
		},
		httpMatchVarsFile: func(g *WithT, data string) {
			g.Expect(data).To(Equal("{}"))
		},
		includesFolder + "/include-1.conf": func(g *WithT, data string) {
			g.Expect(data).To(Equal("include-1"))
		},
		includesFolder + "/include-2.conf": func(g *WithT, data string) {
			g.Expect(data).To(Equal("include-2"))
		},
		includesFolder + "/location-snippet.conf": func(g *WithT, data string) {
			g.Expect(data).To(Equal("location snippet contents"))
		},
		includesFolder + "/server-snippet.conf": func(g *WithT, data string) {
			g.Expect(data).To(Equal("server snippet contents"))
		},
	}

	g := NewWithT(t)

	fakeGenerator := &policiesfakes.FakeGenerator{}
	fakeGenerator.GenerateForServerReturns(
		policies.GenerateResultFiles{
			{
				Name:    "include-1.conf",
				Content: []byte("include-1"),
			},
			{
				Name:    "include-2.conf",
				Content: []byte("include-2"),
			},
		},
	)

	gen := GeneratorImpl{}
	results := gen.executeServers(conf, fakeGenerator, alwaysFalseKeepAliveChecker)
	g.Expect(results).To(HaveLen(len(expectedResults)))

	for _, res := range results {
		g.Expect(expectedResults).To(HaveKey(res.dest), "executeServers returned unexpected result destination")

		assertData := expectedResults[res.dest]
		assertData(g, string(res.data))
	}
}

func TestExecuteServers_IPFamily(t *testing.T) {
	t.Parallel()
	httpServers := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      8080,
		},
		{
			Hostname: "example.com",
			Port:     8080,
		},
	}
	sslServers := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      8443,
		},
		{
			Hostname: "example.com",
			SSL: &dataplane.SSL{
				KeyPairID: "test-keypair",
			},
			Port: 8443,
		},
	}
	sslServers443 := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      443,
		},
		{
			Hostname: "example.com",
			SSL: &dataplane.SSL{
				KeyPairID: "test-keypair",
			},
			Port: 443,
		},
	}
	passThroughServers := []dataplane.Layer4VirtualServer{
		{
			IsDefault: true,
			Hostname:  "*.example.com",
			Port:      8443,
		},
	}
	tests := []struct {
		msg                string
		expectedHTTPConfig map[string]int
		config             dataplane.Configuration
	}{
		{
			msg: "http and ssl servers with IPv4 IP family",
			config: dataplane.Configuration{
				HTTPServers: httpServers,
				SSLServers:  sslServers,
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.IPv4,
				},
			},
			expectedHTTPConfig: map[string]int{
				"listen 8080 default_server;":                              1,
				"listen 8080;":                                             1,
				"listen 8443 ssl default_server;":                          1,
				"listen 8443 ssl;":                                         1,
				"server_name example.com;":                                 2,
				"ssl_certificate /etc/nginx/secrets/test-keypair.pem;":     1,
				"ssl_certificate_key /etc/nginx/secrets/test-keypair.pem;": 1,
				"ssl_reject_handshake on;":                                 1,
			},
		},
		{
			msg: "http, ssl servers, and tls servers with IPv6 IP family",
			config: dataplane.Configuration{
				HTTPServers: httpServers,
				SSLServers:  append(sslServers, sslServers443...),
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.IPv6,
				},
				TLSPassthroughServers: passThroughServers,
			},
			expectedHTTPConfig: map[string]int{
				"listen [::]:8080 default_server;":                              1,
				"listen [::]:8080;":                                             1,
				"listen [::]:443 ssl default_server;":                           1,
				"listen [::]:443 ssl;":                                          1,
				"listen unix:/var/run/nginx/https8443.sock ssl;":                1,
				"listen unix:/var/run/nginx/https8443.sock ssl default_server;": 1,
				"server_name example.com;":                                      3,
				"ssl_certificate /etc/nginx/secrets/test-keypair.pem;":          2,
				"ssl_certificate_key /etc/nginx/secrets/test-keypair.pem;":      2,
				"ssl_reject_handshake on;":                                      2,
			},
		},
		{
			msg: "http and ssl servers with Dual IP family",
			config: dataplane.Configuration{
				HTTPServers: httpServers,
				SSLServers:  sslServers,
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.Dual,
				},
			},
			expectedHTTPConfig: map[string]int{
				"listen 8080 default_server;":                              1,
				"listen 8080;":                                             1,
				"listen 8443 ssl default_server;":                          1,
				"listen 8443 ssl;":                                         1,
				"server_name example.com;":                                 2,
				"ssl_certificate /etc/nginx/secrets/test-keypair.pem;":     1,
				"ssl_certificate_key /etc/nginx/secrets/test-keypair.pem;": 1,
				"ssl_reject_handshake on;":                                 1,
				"listen [::]:8080 default_server;":                         1,
				"listen [::]:8080;":                                        1,
				"listen [::]:8443 ssl default_server;":                     1,
				"listen [::]:8443 ssl;":                                    1,
				"status_zone":                                              0,
				"real_ip_header proxy-protocol;":                           0,
				"real_ip_recursive on;":                                    0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gen := GeneratorImpl{}
			results := gen.executeServers(test.config, &policiesfakes.FakeGenerator{}, alwaysFalseKeepAliveChecker)

			g.Expect(results).To(HaveLen(2))
			serverConf := string(results[0].data)
			httpMatchConf := string(results[1].data)
			g.Expect(httpMatchConf).To(Equal("{}"))

			for expSubStr, expCount := range test.expectedHTTPConfig {
				g.Expect(strings.Count(serverConf, expSubStr)).To(Equal(expCount))
			}
		})
	}
}

func TestExecuteServers_RewriteClientIP(t *testing.T) {
	t.Parallel()
	httpServers := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      8080,
		},
		{
			Hostname: "example.com",
			Port:     8080,
		},
	}

	sslServers := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      8443,
		},
		{
			Hostname: "example.com",
			SSL: &dataplane.SSL{
				KeyPairID: "test-keypair",
			},
			Port: 8443,
		},
	}
	tests := []struct {
		msg                string
		expectedHTTPConfig map[string]int
		config             dataplane.Configuration
	}{
		{
			msg: "rewrite client IP settings configured with proxy protocol",
			config: dataplane.Configuration{
				HTTPServers: httpServers,
				SSLServers:  sslServers,
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.Dual,
					RewriteClientIPSettings: dataplane.RewriteClientIPSettings{
						Mode:             dataplane.RewriteIPModeProxyProtocol,
						TrustedAddresses: []string{"10.56.73.51/32"},
						IPRecursive:      false,
					},
				},
			},
			expectedHTTPConfig: map[string]int{
				"set_real_ip_from 10.56.73.51/32;":                         4,
				"real_ip_header proxy_protocol;":                           4,
				"listen 8080 default_server proxy_protocol;":               1,
				"listen 8080 proxy_protocol;":                              1,
				"listen 8443 ssl default_server proxy_protocol;":           1,
				"listen 8443 ssl proxy_protocol;":                          1,
				"server_name example.com;":                                 2,
				"ssl_certificate /etc/nginx/secrets/test-keypair.pem;":     1,
				"ssl_certificate_key /etc/nginx/secrets/test-keypair.pem;": 1,
				"ssl_reject_handshake on;":                                 1,
				"listen [::]:8080 default_server proxy_protocol;":          1,
				"listen [::]:8080 proxy_protocol;":                         1,
				"listen [::]:8443 ssl default_server proxy_protocol;":      1,
				"listen [::]:8443 ssl proxy_protocol;":                     1,
				"real_ip_recursive on;":                                    0,
			},
		},
		{
			msg: "rewrite client IP settings configured with x-forwarded-for",
			config: dataplane.Configuration{
				HTTPServers: httpServers,
				SSLServers:  sslServers,
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					IPFamily: dataplane.Dual,
					RewriteClientIPSettings: dataplane.RewriteClientIPSettings{
						Mode:             dataplane.RewriteIPModeXForwardedFor,
						TrustedAddresses: []string{"10.1.1.3/32", "2.2.2.2", "2001:db8::/32"},
						IPRecursive:      true,
					},
				},
			},
			expectedHTTPConfig: map[string]int{
				"set_real_ip_from 10.1.1.3/32;":                            4,
				"set_real_ip_from 2.2.2.2;":                                4,
				"set_real_ip_from 2001:db8::/32;":                          4,
				"real_ip_header X-Forwarded-For;":                          4,
				"real_ip_recursive on;":                                    4,
				"listen 8080 default_server;":                              1,
				"listen 8080;":                                             1,
				"listen 8443 ssl default_server;":                          1,
				"listen 8443 ssl;":                                         1,
				"server_name example.com;":                                 2,
				"ssl_certificate /etc/nginx/secrets/test-keypair.pem;":     1,
				"ssl_certificate_key /etc/nginx/secrets/test-keypair.pem;": 1,
				"ssl_reject_handshake on;":                                 1,
				"listen [::]:8080 default_server;":                         1,
				"listen [::]:8080;":                                        1,
				"listen [::]:8443 ssl default_server;":                     1,
				"listen [::]:8443 ssl;":                                    1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gen := GeneratorImpl{}
			results := gen.executeServers(test.config, &policiesfakes.FakeGenerator{}, alwaysFalseKeepAliveChecker)
			g.Expect(results).To(HaveLen(2))
			serverConf := string(results[0].data)
			httpMatchConf := string(results[1].data)
			g.Expect(httpMatchConf).To(Equal("{}"))

			for expSubStr, expCount := range test.expectedHTTPConfig {
				g.Expect(strings.Count(serverConf, expSubStr)).To(Equal(expCount))
			}
		})
	}
}

func TestExecuteServers_Plus(t *testing.T) {
	t.Parallel()
	config := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{
			{
				Hostname: "example.com",
			},
			{
				Hostname: "example2.com",
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				Hostname: "example.com",
				SSL: &dataplane.SSL{
					KeyPairID: "test-keypair",
				},
			},
		},
	}

	expectedHTTPConfig := map[string]int{
		"status_zone example.com;":  2,
		"status_zone example2.com;": 1,
	}

	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}
	results := gen.executeServers(config, &policiesfakes.FakeGenerator{}, alwaysFalseKeepAliveChecker)
	g.Expect(results).To(HaveLen(2))

	serverConf := string(results[0].data)

	for expSubStr, expCount := range expectedHTTPConfig {
		g.Expect(strings.Count(serverConf, expSubStr)).To(Equal(expCount))
	}
}

func TestExecuteForDefaultServers(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		msg       string
		httpPorts []int
		sslPorts  []int
		conf      dataplane.Configuration
	}{
		{
			conf: dataplane.Configuration{},
			msg:  "no default servers",
		},
		{
			conf: dataplane.Configuration{
				HTTPServers: []dataplane.VirtualServer{
					{
						IsDefault: true,
						Port:      80,
					},
				},
			},
			httpPorts: []int{80},
			msg:       "only HTTP default server",
		},
		{
			conf: dataplane.Configuration{
				SSLServers: []dataplane.VirtualServer{
					{
						IsDefault: true,
						Port:      443,
					},
				},
			},
			sslPorts: []int{443},
			msg:      "only HTTPS default server",
		},
		{
			conf: dataplane.Configuration{
				HTTPServers: []dataplane.VirtualServer{
					{
						IsDefault: true,
						Port:      80,
					},
					{
						IsDefault: true,
						Port:      8080,
					},
				},
				SSLServers: []dataplane.VirtualServer{
					{
						IsDefault: true,
						Port:      443,
					},
					{
						IsDefault: true,
						Port:      8443,
					},
				},
			},
			httpPorts: []int{80, 8080},
			sslPorts:  []int{443, 8443},
			msg:       "multiple HTTP and HTTPS default servers",
		},
	}

	sslDefaultFmt := "listen %d ssl default_server"
	httpDefaultFmt := "listen %d default_server"

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gen := GeneratorImpl{}
			serverResults := gen.executeServers(tc.conf, &policiesfakes.FakeGenerator{}, alwaysFalseKeepAliveChecker)
			g.Expect(serverResults).To(HaveLen(2))
			serverConf := string(serverResults[0].data)
			httpMatchConf := string(serverResults[1].data)
			g.Expect(httpMatchConf).To(Equal("{}"))

			for _, expPort := range tc.httpPorts {
				g.Expect(serverConf).To(ContainSubstring(fmt.Sprintf(httpDefaultFmt, expPort)))
			}

			for _, expPort := range tc.sslPorts {
				g.Expect(serverConf).To(ContainSubstring(fmt.Sprintf(sslDefaultFmt, expPort)))
			}
		})
	}
}

func TestCreateServers(t *testing.T) {
	t.Parallel()
	const (
		sslKeyPairID = "test-keypair"
	)

	hrNsName := types.NamespacedName{Namespace: "test", Name: "route1"}

	fooGroup := dataplane.BackendGroup{
		Source:  hrNsName,
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_foo_80",
				Valid:        true,
				Weight:       1,
			},
		},
	}

	// barGroup has two backends, which should generate a proxy pass with a variable.
	barGroup := dataplane.BackendGroup{
		Source:  hrNsName,
		RuleIdx: 1,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_bar_80",
				Valid:        true,
				Weight:       50,
			},
			{
				UpstreamName: "test_bar2_80",
				Valid:        true,
				Weight:       50,
			},
		},
	}

	// baz group has an invalid backend, which should generate a proxy pass to the invalid ref backend.
	bazGroup := dataplane.BackendGroup{
		Source:  hrNsName,
		RuleIdx: 2,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_baz_80",
				Valid:        false,
				Weight:       1,
			},
		},
	}

	btpGroup := dataplane.BackendGroup{
		Source:  hrNsName,
		RuleIdx: 3,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_btp_80",
				Valid:        true,
				Weight:       1,
				VerifyTLS: &dataplane.VerifyTLS{
					CertBundleID: "test-btp",
					Hostname:     "test-btp.example.com",
				},
			},
		},
	}

	keepAliveGroup := dataplane.BackendGroup{
		Source:  hrNsName,
		RuleIdx: 4,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_keep_alive_80",
				Valid:        true,
				Weight:       1,
			},
		},
	}

	filterGroup1 := dataplane.BackendGroup{Source: hrNsName, RuleIdx: 3}

	filterGroup2 := dataplane.BackendGroup{Source: hrNsName, RuleIdx: 4}

	invalidFilterGroup := dataplane.BackendGroup{Source: hrNsName, RuleIdx: 5}

	cafePathRules := []dataplane.PathRule{
		{
			Path:     "/",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{
						Method: helpers.GetPointer("POST"),
					},
					BackendGroup: fooGroup,
				},
				{
					Match: dataplane.Match{
						Method: helpers.GetPointer("PATCH"),
					},
					BackendGroup: fooGroup,
				},
				{
					// should generate an "any" httpmatch since other matches exists for /
					Match:        dataplane.Match{},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/test",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					// A match with all possible fields set
					Match: dataplane.Match{
						Method: helpers.GetPointer("GET"),
						Headers: []dataplane.HTTPHeaderMatch{
							{
								Name:  "Version",
								Value: "V1",
							},
							{
								Name:  "test",
								Value: "foo",
							},
							{
								Name:  "my-header",
								Value: "my-value",
							},
						},
						QueryParams: []dataplane.HTTPQueryParamMatch{
							{
								// query names and values should not be normalized to lowercase
								Name:  "GrEat",
								Value: "EXAMPLE",
							},
							{
								Name:  "test",
								Value: "foo=bar",
							},
						},
					},
					BackendGroup: barGroup,
				},
			},
		},
		{
			Path:     "/path-only",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: bazGroup,
				},
			},
		},
		{
			Path:     "/backend-tls-policy",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: btpGroup,
				},
			},
		},
		{
			Path:     "/redirect-implicit-port",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{},
					Filters: dataplane.HTTPFilters{
						RequestRedirect: &dataplane.HTTPRequestRedirectFilter{
							Hostname: helpers.GetPointer("foo.example.com"),
						},
					},
					BackendGroup: filterGroup1,
				},
			},
		},
		{
			Path:     "/redirect-explicit-port",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{},
					Filters: dataplane.HTTPFilters{
						RequestRedirect: &dataplane.HTTPRequestRedirectFilter{
							Hostname: helpers.GetPointer("bar.example.com"),
							Port:     helpers.GetPointer[int32](8080),
						},
					},
					BackendGroup: filterGroup2,
				},
			},
		},
		{
			Path:     "/redirect-with-headers",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{
						Headers: []dataplane.HTTPHeaderMatch{
							{
								Name:  "redirect",
								Value: "this",
							},
						},
					},
					Filters: dataplane.HTTPFilters{
						RequestRedirect: &dataplane.HTTPRequestRedirectFilter{
							Hostname: helpers.GetPointer("foo.example.com"),
							Port:     helpers.GetPointer[int32](8080),
						},
					},
					BackendGroup: filterGroup1,
				},
			},
		},
		{
			Path:     "/rewrite",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{},
					Filters: dataplane.HTTPFilters{
						RequestURLRewrite: &dataplane.HTTPURLRewriteFilter{
							Hostname: helpers.GetPointer("new.example.com"),
							Path: &dataplane.HTTPPathModifier{
								Type:        dataplane.ReplaceFullPath,
								Replacement: "/replacement",
							},
						},
					},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/rewrite-with-headers",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{
						Headers: []dataplane.HTTPHeaderMatch{
							{
								Name:  "rewrite",
								Value: "this",
							},
						},
					},
					Filters: dataplane.HTTPFilters{
						RequestURLRewrite: &dataplane.HTTPURLRewriteFilter{
							Hostname: helpers.GetPointer("new.example.com"),
							Path: &dataplane.HTTPPathModifier{
								Type:        dataplane.ReplacePrefixMatch,
								Replacement: "/prefix-replacement",
							},
						},
					},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/invalid-filter",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{},
					Filters: dataplane.HTTPFilters{
						InvalidFilter: &dataplane.InvalidHTTPFilter{},
					},
					BackendGroup: invalidFilterGroup,
				},
			},
		},
		{
			Path:     "/invalid-filter-with-headers",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{
						Headers: []dataplane.HTTPHeaderMatch{
							{
								Name:  "filter",
								Value: "this",
							},
						},
					},
					Filters: dataplane.HTTPFilters{
						InvalidFilter: &dataplane.InvalidHTTPFilter{},
					},
					BackendGroup: invalidFilterGroup,
				},
			},
		},
		{
			Path:     "/exact",
			PathType: dataplane.PathTypeExact,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/test",
			PathType: dataplane.PathTypeExact,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{
						Method: helpers.GetPointer("GET"),
					},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/proxy-set-headers",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: fooGroup,
					Filters: dataplane.HTTPFilters{
						RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Add: []dataplane.HTTPHeader{
								{
									Name:  "my-header",
									Value: "some-value-123",
								},
							},
						},
						ResponseHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Add: []dataplane.HTTPHeader{
								{
									Name:  "my-header-response",
									Value: "some-value-response-123",
								},
							},
						},
					},
				},
			},
		},
		{
			Path:     "/grpc/method",
			PathType: dataplane.PathTypeExact,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: fooGroup,
				},
			},
			GRPC: true,
		},
		{
			Path:     "/grpc-with-backend-tls-policy/method",
			PathType: dataplane.PathTypeExact,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: btpGroup,
				},
			},
			GRPC: true,
		},
		{
			Path:     "/include-path-only-match",
			PathType: dataplane.PathTypeExact,
			Policies: []policies.Policy{
				&policiesfakes.FakePolicy{},
			},
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/include-header-match",
			PathType: dataplane.PathTypeExact,
			Policies: []policies.Policy{
				&policiesfakes.FakePolicy{},
			},
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{
						Method: helpers.GetPointer("GET"),
					},
					BackendGroup: fooGroup,
				},
			},
		},
		{
			Path:     "/keep-alive-enabled",
			PathType: dataplane.PathTypeExact,
			MatchRules: []dataplane.MatchRule{
				{
					Match:        dataplane.Match{},
					BackendGroup: keepAliveGroup,
				},
			},
		},
		{
			Path:     "/redirect-with-path",
			PathType: dataplane.PathTypePrefix,
			MatchRules: []dataplane.MatchRule{
				{
					Match: dataplane.Match{},
					Filters: dataplane.HTTPFilters{
						RequestRedirect: &dataplane.HTTPRequestRedirectFilter{
							Hostname:   helpers.GetPointer("redirect.example.com"),
							StatusCode: helpers.GetPointer[int](301),
							Port:       helpers.GetPointer[int32](8080),
							Path: &dataplane.HTTPPathModifier{
								Type:        dataplane.ReplaceFullPath,
								Replacement: "/replacement",
							},
						},
					},
					BackendGroup: fooGroup,
				},
			},
		},
	}

	conf := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{
			{
				IsDefault: true,
				Port:      8080,
			},
			{
				Hostname:  "cafe.example.com",
				PathRules: cafePathRules,
				Port:      8080,
				Policies: []policies.Policy{
					&policiesfakes.FakePolicy{},
					&policiesfakes.FakePolicy{},
				},
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				IsDefault: true,
				Port:      8443,
			},
			{
				Hostname:  "cafe.example.com",
				SSL:       &dataplane.SSL{KeyPairID: sslKeyPairID},
				PathRules: cafePathRules,
				Port:      8443,
				Policies: []policies.Policy{
					&policiesfakes.FakePolicy{},
					&policiesfakes.FakePolicy{},
				},
			},
		},
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname:     "app.example.com",
				Port:         8443,
				UpstreamName: "sup",
			},
		},
	}

	expMatchPairs := httpMatchPairs{
		"1_0": {
			{Method: "POST", RedirectPath: "/_ngf-internal-rule0-route0"},
			{Method: "PATCH", RedirectPath: "/_ngf-internal-rule0-route1"},
			{RedirectPath: "/_ngf-internal-rule0-route2", Any: true},
		},
		"1_1": {
			{
				Method:       "GET",
				Headers:      []string{"Version:V1", "test:foo", "my-header:my-value"},
				QueryParams:  []string{"GrEat=EXAMPLE", "test=foo=bar"},
				RedirectPath: "/_ngf-internal-rule1-route0",
			},
		},
		"1_6": {
			{RedirectPath: "/_ngf-internal-rule6-route0", Headers: []string{"redirect:this"}},
		},
		"1_8": {
			{
				Headers:      []string{"rewrite:this"},
				RedirectPath: "/_ngf-internal-rule8-route0",
			},
		},
		"1_10": {
			{
				Headers:      []string{"filter:this"},
				RedirectPath: "/_ngf-internal-rule10-route0",
			},
		},
		"1_12": {
			{
				Method:       "GET",
				RedirectPath: "/_ngf-internal-rule12-route0",
				Headers:      nil,
				QueryParams:  nil,
				Any:          false,
			},
		},
		"1_17": {
			{
				Method:       "GET",
				RedirectPath: "/_ngf-internal-rule17-route0",
			},
		},
	}

	allExpMatchPair := make(httpMatchPairs)
	maps.Copy(allExpMatchPair, expMatchPairs)
	modifiedMatchPairs := modifyMatchPairs(expMatchPairs)
	maps.Copy(allExpMatchPair, modifiedMatchPairs)

	rewriteProxySetHeaders := []http.Header{
		{
			Name:  "Host",
			Value: "new.example.com",
		},
		{
			Name:  "X-Forwarded-For",
			Value: "$proxy_add_x_forwarded_for",
		},
		{
			Name:  "X-Real-IP",
			Value: "$remote_addr",
		},
		{
			Name:  "X-Forwarded-Proto",
			Value: "$scheme",
		},
		{
			Name:  "X-Forwarded-Host",
			Value: "$host",
		},
		{
			Name:  "X-Forwarded-Port",
			Value: "$server_port",
		},
		{
			Name:  "Upgrade",
			Value: "$http_upgrade",
		},
		{
			Name:  "Connection",
			Value: "$connection_upgrade",
		},
	}

	externalIncludes := []shared.Include{
		{Name: "/etc/nginx/includes/include-1.conf", Content: []byte("include-1")},
	}

	internalIncludes := []shared.Include{
		{Name: "/etc/nginx/includes/internal-include-1.conf", Content: []byte("include-1")},
	}

	getExpectedLocations := func(isHTTPS bool) []http.Location {
		port := 8080
		ssl := ""
		if isHTTPS {
			port = 8443
			ssl = "SSL_"
		}

		return []http.Location{
			{
				Path:         "/",
				HTTPMatchKey: ssl + "1_0",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule0-route0",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule0-route1",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule0-route2",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path:         "/test/",
				HTTPMatchKey: ssl + "1_1",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule1-route0",
				ProxyPass:       "http://$group_test__route1_rule1$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path:            "/path-only/",
				ProxyPass:       "http://invalid-backend-ref$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:            "= /path-only",
				ProxyPass:       "http://invalid-backend-ref$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:            "/backend-tls-policy/",
				ProxyPass:       "https://test_btp_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				ProxySSLVerify: &http.ProxySSLVerify{
					Name:               "test-btp.example.com",
					TrustedCertificate: "/etc/nginx/secrets/test-btp.crt",
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path:            "= /backend-tls-policy",
				ProxyPass:       "https://test_btp_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				ProxySSLVerify: &http.ProxySSLVerify{
					Name:               "test-btp.example.com",
					TrustedCertificate: "/etc/nginx/secrets/test-btp.crt",
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path: "/redirect-implicit-port/",
				Return: &http.Return{
					Code: 302,
					Body: fmt.Sprintf("$scheme://foo.example.com:%d$request_uri", port),
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path: "= /redirect-implicit-port",
				Return: &http.Return{
					Code: 302,
					Body: fmt.Sprintf("$scheme://foo.example.com:%d$request_uri", port),
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path: "/redirect-explicit-port/",
				Return: &http.Return{
					Code: 302,
					Body: "$scheme://bar.example.com:8080$request_uri",
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path: "= /redirect-explicit-port",
				Return: &http.Return{
					Code: 302,
					Body: "$scheme://bar.example.com:8080$request_uri",
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path:         "/redirect-with-headers/",
				HTTPMatchKey: ssl + "1_6",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:         "= /redirect-with-headers",
				HTTPMatchKey: ssl + "1_6",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path: "/_ngf-internal-rule6-route0",
				Return: &http.Return{
					Body: "$scheme://foo.example.com:8080$request_uri",
					Code: 302,
				},
				Type:     http.InternalLocationType,
				Includes: internalIncludes,
			},
			{
				Path:            "/rewrite/",
				Rewrites:        []string{"^ /replacement break"},
				ProxyPass:       "http://test_foo_80",
				ProxySetHeaders: rewriteProxySetHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:            "= /rewrite",
				Rewrites:        []string{"^ /replacement break"},
				ProxyPass:       "http://test_foo_80",
				ProxySetHeaders: rewriteProxySetHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:         "/rewrite-with-headers/",
				HTTPMatchKey: ssl + "1_8",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:         "= /rewrite-with-headers",
				HTTPMatchKey: ssl + "1_8",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule8-route0",
				Rewrites:        []string{"^ $request_uri", "^/rewrite-with-headers([^?]*)? /prefix-replacement$1?$args? break"},
				ProxyPass:       "http://test_foo_80",
				ProxySetHeaders: rewriteProxySetHeaders,
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path: "/invalid-filter/",
				Return: &http.Return{
					Code: http.StatusInternalServerError,
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path: "= /invalid-filter",
				Return: &http.Return{
					Code: http.StatusInternalServerError,
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path:         "/invalid-filter-with-headers/",
				HTTPMatchKey: ssl + "1_10",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:         "= /invalid-filter-with-headers",
				HTTPMatchKey: ssl + "1_10",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path: "/_ngf-internal-rule10-route0",
				Return: &http.Return{
					Code: http.StatusInternalServerError,
				},
				Type:     http.InternalLocationType,
				Includes: internalIncludes,
			},
			{
				Path:            "= /exact",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:         "= /test",
				HTTPMatchKey: ssl + "1_12",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule12-route0",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path:      "/proxy-set-headers/",
				ProxyPass: "http://test_foo_80$request_uri",
				ProxySetHeaders: append([]http.Header{
					{
						Name:  "my-header",
						Value: "${my_header_header_var}some-value-123",
					},
				}, httpBaseHeaders...),
				ResponseHeaders: http.ResponseHeaders{
					Add: []http.Header{
						{
							Name:  "my-header-response",
							Value: "some-value-response-123",
						},
					},
					Set:    []http.Header{},
					Remove: []string{},
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path:      "= /proxy-set-headers",
				ProxyPass: "http://test_foo_80$request_uri",
				ProxySetHeaders: append([]http.Header{
					{
						Name:  "my-header",
						Value: "${my_header_header_var}some-value-123",
					},
				}, httpBaseHeaders...),
				ResponseHeaders: http.ResponseHeaders{
					Add: []http.Header{
						{
							Name:  "my-header-response",
							Value: "some-value-response-123",
						},
					},
					Set:    []http.Header{},
					Remove: []string{},
				},
				Type:     http.ExternalLocationType,
				Includes: externalIncludes,
			},
			{
				Path:            "= /grpc/method",
				ProxyPass:       "grpc://test_foo_80",
				GRPC:            true,
				ProxySetHeaders: grpcBaseHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:      "= /grpc-with-backend-tls-policy/method",
				ProxyPass: "grpcs://test_btp_80",
				ProxySSLVerify: &http.ProxySSLVerify{
					Name:               "test-btp.example.com",
					TrustedCertificate: "/etc/nginx/secrets/test-btp.crt",
				},
				GRPC:            true,
				ProxySetHeaders: grpcBaseHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:            "= /include-path-only-match",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path:         "= /include-header-match",
				HTTPMatchKey: ssl + "1_17",
				Type:         http.RedirectLocationType,
				Includes:     externalIncludes,
			},
			{
				Path:            "/_ngf-internal-rule17-route0",
				ProxyPass:       "http://test_foo_80$request_uri",
				ProxySetHeaders: httpBaseHeaders,
				Rewrites:        []string{"^ $request_uri break"},
				Type:            http.InternalLocationType,
				Includes:        internalIncludes,
			},
			{
				Path:            "= /keep-alive-enabled",
				ProxyPass:       "http://test_keep_alive_80$request_uri",
				ProxySetHeaders: createBaseProxySetHeaders(httpUpgradeHeader, unsetHTTPConnectionHeader),
				Type:            http.ExternalLocationType,
				Includes:        externalIncludes,
			},
			{
				Path: "/redirect-with-path/",
				Type: http.ExternalLocationType,
				Return: &http.Return{
					Code: 301,
					Body: "$scheme://redirect.example.com:8080$uri$is_args$args",
				},
				Rewrites: []string{"^ /replacement"},
				Includes: externalIncludes,
			},
			{
				Path: "= /redirect-with-path",
				Type: http.ExternalLocationType,
				Return: &http.Return{
					Code: 301,
					Body: "$scheme://redirect.example.com:8080$uri$is_args$args",
				},
				Rewrites: []string{"^ /replacement"},
				Includes: externalIncludes,
			},
		}
	}

	expectedPEMPath := fmt.Sprintf("/etc/nginx/secrets/%s.pem", sslKeyPairID)

	expectedServers := []http.Server{
		{
			IsDefaultHTTP: true,
			Listen:        "8080",
		},
		{
			ServerName: "cafe.example.com",
			Locations:  getExpectedLocations(false),
			Includes:   []shared.Include{},
			Listen:     "8080",
			GRPC:       true,
		},
		{
			IsDefaultSSL: true,
			Listen:       getSocketNameHTTPS(8443),
			IsSocket:     true,
		},
		{
			ServerName: "cafe.example.com",
			SSL: &http.SSL{
				Certificate:    expectedPEMPath,
				CertificateKey: expectedPEMPath,
			},
			Locations: getExpectedLocations(true),
			Includes:  []shared.Include{},
			Listen:    getSocketNameHTTPS(8443),
			IsSocket:  true,
			GRPC:      true,
		},
	}

	g := NewWithT(t)

	fakeGenerator := &policiesfakes.FakeGenerator{}
	fakeGenerator.GenerateForLocationReturns(policies.GenerateResultFiles{
		{
			Name:    "include-1.conf",
			Content: []byte("include-1"),
		},
	})
	fakeGenerator.GenerateForInternalLocationReturns(policies.GenerateResultFiles{
		{
			Name:    "internal-include-1.conf",
			Content: []byte("include-1"),
		},
	})

	keepAliveEnabledUpstream := http.Upstream{
		Name: "test_keep_alive_80",
		KeepAlive: http.UpstreamKeepAlive{
			Connections: 1,
		},
	}
	keepAliveCheck := newKeepAliveChecker([]http.Upstream{keepAliveEnabledUpstream})

	result, httpMatchPair := createServers(conf, fakeGenerator, keepAliveCheck)

	g.Expect(httpMatchPair).To(Equal(allExpMatchPair))
	g.Expect(helpers.Diff(expectedServers, result)).To(BeEmpty())
}

func modifyMatchPairs(matchPairs httpMatchPairs) httpMatchPairs {
	modified := make(httpMatchPairs)
	for k, v := range matchPairs {
		modifiedKey := "SSL_" + k
		modified[modifiedKey] = v
	}

	return modified
}

func TestCreateServersConflicts(t *testing.T) {
	t.Parallel()
	fooGroup := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "route"},
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_foo_80",
				Valid:        true,
				Weight:       1,
			},
		},
	}
	barGroup := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "route"},
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_bar_80",
				Valid:        true,
				Weight:       1,
			},
		},
	}
	bazGroup := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "route"},
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_baz_80",
				Valid:        true,
				Weight:       1,
			},
		},
	}

	tests := []struct {
		name    string
		rules   []dataplane.PathRule
		expLocs []http.Location
	}{
		{
			name: "/coffee prefix, /coffee exact",
			rules: []dataplane.PathRule{
				{
					Path:     "/coffee",
					PathType: dataplane.PathTypePrefix,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: fooGroup,
						},
					},
				},
				{
					Path:     "/coffee",
					PathType: dataplane.PathTypeExact,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: barGroup,
						},
					},
				},
			},
			expLocs: []http.Location{
				{
					Path:            "/coffee/",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "= /coffee",
					ProxyPass:       "http://test_bar_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				createDefaultRootLocation(),
			},
		},
		{
			name: "/coffee prefix, /coffee/ prefix",
			rules: []dataplane.PathRule{
				{
					Path:     "/coffee",
					PathType: dataplane.PathTypePrefix,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: fooGroup,
						},
					},
				},
				{
					Path:     "/coffee/",
					PathType: dataplane.PathTypePrefix,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: barGroup,
						},
					},
				},
			},
			expLocs: []http.Location{
				{
					Path:            "= /coffee",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "/coffee/",
					ProxyPass:       "http://test_bar_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				createDefaultRootLocation(),
			},
		},
		{
			name: "/coffee prefix, /coffee/ prefix, /coffee exact",
			rules: []dataplane.PathRule{
				{
					Path:     "/coffee",
					PathType: dataplane.PathTypePrefix,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: fooGroup,
						},
					},
				},
				{
					Path:     "/coffee/",
					PathType: dataplane.PathTypePrefix,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: barGroup,
						},
					},
				},
				{
					Path:     "/coffee",
					PathType: dataplane.PathTypeExact,
					MatchRules: []dataplane.MatchRule{
						{
							Match:        dataplane.Match{},
							BackendGroup: bazGroup,
						},
					},
				},
			},
			expLocs: []http.Location{
				{
					Path:            "/coffee/",
					ProxyPass:       "http://test_bar_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "= /coffee",
					ProxyPass:       "http://test_baz_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				createDefaultRootLocation(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			httpServers := []dataplane.VirtualServer{
				{
					IsDefault: true,
					Port:      8080,
				},
				{
					Hostname:  "cafe.example.com",
					PathRules: test.rules,
					Port:      8080,
				},
			}
			expectedServers := []http.Server{
				{
					IsDefaultHTTP: true,
					Listen:        "8080",
				},
				{
					ServerName: "cafe.example.com",
					Locations:  test.expLocs,
					Listen:     "8080",
					Includes:   []shared.Include{},
				},
			}

			g := NewWithT(t)

			result, _ := createServers(
				dataplane.Configuration{HTTPServers: httpServers},
				&policiesfakes.FakeGenerator{},
				alwaysFalseKeepAliveChecker,
			)
			g.Expect(helpers.Diff(expectedServers, result)).To(BeEmpty())
		})
	}
}

func TestCreateServers_Includes(t *testing.T) {
	t.Parallel()

	pathRules := []dataplane.PathRule{
		{
			Path:     "/",
			PathType: dataplane.PathTypeExact,
			MatchRules: []dataplane.MatchRule{
				{
					Filters: dataplane.HTTPFilters{
						SnippetsFilters: []dataplane.SnippetsFilter{
							{
								LocationSnippet: &dataplane.Snippet{
									Name:     "location-snippet",
									Contents: "location snippet contents",
								},
								ServerSnippet: &dataplane.Snippet{
									Name:     "server-snippet",
									Contents: "server snippet contents",
								},
							},
						},
					},
				},
			},
		},
	}

	httpServers := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      8080,
		},
		{
			Hostname:  "http.example.com",
			PathRules: pathRules,
			Port:      8080,
			Policies: []policies.Policy{
				&policiesfakes.FakePolicy{},
			},
		},
	}

	sslServers := []dataplane.VirtualServer{
		{
			IsDefault: true,
			Port:      8443,
		},
		{
			Hostname:  "ssl.example.com",
			SSL:       &dataplane.SSL{KeyPairID: "test-keypair"},
			PathRules: pathRules,
			Port:      8443,
			Policies: []policies.Policy{
				&policiesfakes.FakePolicy{},
			},
		},
	}

	fakeGenerator := &policiesfakes.FakeGenerator{}
	fakeGenerator.GenerateForLocationReturns(policies.GenerateResultFiles{
		{
			Name:    "ext-policy.conf",
			Content: []byte("external policy conf"),
		},
	})
	fakeGenerator.GenerateForServerReturns(policies.GenerateResultFiles{
		{
			Name:    "server-policy.conf",
			Content: []byte("server policy conf"),
		},
	})

	expServers := []http.Server{
		{
			IsDefaultHTTP: true,
		},
		{
			ServerName: "http.example.com",
			Locations: []http.Location{
				{
					Path: "= /",
					Includes: []shared.Include{
						{
							Name:    includesFolder + "/location-snippet.conf",
							Content: []byte("location snippet contents"),
						},
						{
							Name:    includesFolder + "/ext-policy.conf",
							Content: []byte("external policy conf"),
						},
					},
				},
			},
			Includes: []shared.Include{
				{
					Name:    includesFolder + "/server-policy.conf",
					Content: []byte("server policy conf"),
				},
				{
					Name:    includesFolder + "/server-snippet.conf",
					Content: []byte("server snippet contents"),
				},
			},
			Listen: "8080",
			GRPC:   true,
		},
		{
			IsDefaultSSL: true,
		},
		{
			ServerName: "ssl.example.com",
			Locations: []http.Location{
				{
					Path: "= /",
					Includes: []shared.Include{
						{
							Name:    includesFolder + "/location-snippet.conf",
							Content: []byte("location snippet contents"),
						},
						{
							Name:    includesFolder + "/ext-policy.conf",
							Content: []byte("external policy conf"),
						},
					},
				},
			},
			Includes: []shared.Include{
				{
					Name:    includesFolder + "/server-policy.conf",
					Content: []byte("server policy conf"),
				},
				{
					Name:    includesFolder + "/server-snippet.conf",
					Content: []byte("server snippet contents"),
				},
			},
		},
	}

	g := NewWithT(t)

	conf := dataplane.Configuration{HTTPServers: httpServers, SSLServers: sslServers}

	actualServers, matchPairs := createServers(conf, fakeGenerator, alwaysFalseKeepAliveChecker)
	g.Expect(matchPairs).To(BeEmpty())
	g.Expect(actualServers).To(HaveLen(len(expServers)))

	for i, expServer := range expServers {
		g.Expect(actualServers[i].ServerName).To(Equal(expServer.ServerName))

		if actualServers[i].IsDefaultHTTP || actualServers[i].IsDefaultSSL {
			g.Expect(actualServers[i].Includes).To(BeEmpty())
		} else {
			g.Expect(actualServers[i].Includes).To(ConsistOf(expServer.Includes))
			g.Expect(actualServers[i].Locations).To(HaveLen(1))
			g.Expect(actualServers[i].Locations[0].Path).To(Equal(expServer.Locations[0].Path))
			g.Expect(actualServers[i].Locations[0].Includes).To(ConsistOf(expServer.Locations[0].Includes))
		}
	}
}

func TestCreateLocations_Includes(t *testing.T) {
	t.Parallel()

	httpServer := dataplane.VirtualServer{
		Hostname: "example.com",
		PathRules: []dataplane.PathRule{
			{
				Path:     "/",
				PathType: dataplane.PathTypeExact,
				MatchRules: []dataplane.MatchRule{
					{
						Filters: dataplane.HTTPFilters{
							SnippetsFilters: []dataplane.SnippetsFilter{
								{
									LocationSnippet: &dataplane.Snippet{
										Name:     "location-snippet",
										Contents: "location snippet contents",
									},
									ServerSnippet: &dataplane.Snippet{
										Name:     "server-snippet",
										Contents: "server snippet 2 contents",
									},
								},
							},
						},
					},
				},
			},
			{
				Path:     "/snippets-prefix-path",
				PathType: dataplane.PathTypePrefix,
				MatchRules: []dataplane.MatchRule{
					{
						Filters: dataplane.HTTPFilters{
							SnippetsFilters: []dataplane.SnippetsFilter{
								{
									LocationSnippet: &dataplane.Snippet{
										Name:     "prefix-path-location-snippet",
										Contents: "prefix path location snippet contents",
									},
								},
							},
						},
					},
				},
			},
			{
				Path:     "/snippets-with-method-match",
				PathType: dataplane.PathTypeExact,
				MatchRules: []dataplane.MatchRule{
					{
						Match: dataplane.Match{
							Method: helpers.GetPointer("GET"),
						},
						Filters: dataplane.HTTPFilters{
							SnippetsFilters: []dataplane.SnippetsFilter{
								{
									LocationSnippet: &dataplane.Snippet{
										Name:     "method-match-location-snippet",
										Contents: "method match location snippet contents",
									},
								},
							},
						},
					},
				},
			},
		},
		Port: 80,
	}

	externalPolicyInclude := shared.Include{
		Name:    includesFolder + "/ext-policy.conf",
		Content: []byte("external policy conf"),
	}

	internalPolicyInclude := shared.Include{
		Name:    includesFolder + "/int-policy.conf",
		Content: []byte("internal policy conf"),
	}

	// this test only covers the includes generated for locations, it does not test other location fields.
	expLocations := []http.Location{
		{
			Path: "= /",
			Includes: []shared.Include{
				{
					Name:    includesFolder + "/location-snippet.conf",
					Content: []byte("location snippet contents"),
				},
				externalPolicyInclude,
			},
		},
		{
			Path: "/snippets-prefix-path/",
			Includes: []shared.Include{
				{
					Name:    includesFolder + "/prefix-path-location-snippet.conf",
					Content: []byte("prefix path location snippet contents"),
				},
				externalPolicyInclude,
			},
		},
		{
			Path: "= /snippets-prefix-path",
			Includes: []shared.Include{
				{
					Name:    includesFolder + "/prefix-path-location-snippet.conf",
					Content: []byte("prefix path location snippet contents"),
				},
				externalPolicyInclude,
			},
		},
		{
			Path:     "= /snippets-with-method-match",
			Includes: []shared.Include{externalPolicyInclude},
		},
		{
			Path: "/_ngf-internal-rule2-route0",
			Includes: []shared.Include{
				{
					Name:    includesFolder + "/method-match-location-snippet.conf",
					Content: []byte("method match location snippet contents"),
				},
				internalPolicyInclude,
			},
		},
	}

	fakeGenerator := &policiesfakes.FakeGenerator{}
	fakeGenerator.GenerateForLocationReturns(policies.GenerateResultFiles{
		{
			Name:    "ext-policy.conf",
			Content: []byte("external policy conf"),
		},
	})
	fakeGenerator.GenerateForInternalLocationReturns(policies.GenerateResultFiles{
		{
			Name:    "int-policy.conf",
			Content: []byte("internal policy conf"),
		},
	})

	locations, matches, grpc := createLocations(&httpServer, "1", fakeGenerator, alwaysFalseKeepAliveChecker)

	g := NewWithT(t)
	g.Expect(grpc).To(BeFalse())
	g.Expect(matches).To(HaveLen(1))
	g.Expect(locations).To(HaveLen(len(expLocations)))
	for i, location := range locations {
		g.Expect(location.Path).To(Equal(expLocations[i].Path))
		g.Expect(location.Includes).To(ConsistOf(expLocations[i].Includes))
	}
}

func TestCreateLocationsRootPath(t *testing.T) {
	t.Parallel()
	hrNsName := types.NamespacedName{Namespace: "test", Name: "route1"}

	fooGroup := dataplane.BackendGroup{
		Source:  hrNsName,
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{
				UpstreamName: "test_foo_80",
				Valid:        true,
				Weight:       1,
			},
		},
	}

	getPathRules := func(rootPath bool, grpc bool) []dataplane.PathRule {
		rules := []dataplane.PathRule{
			{
				Path: "/path-1",
				MatchRules: []dataplane.MatchRule{
					{
						Match:        dataplane.Match{},
						BackendGroup: fooGroup,
					},
				},
			},
			{
				Path: "/path-2",
				MatchRules: []dataplane.MatchRule{
					{
						Match:        dataplane.Match{},
						BackendGroup: fooGroup,
					},
				},
			},
		}

		if rootPath {
			rules = append(rules, dataplane.PathRule{
				Path: "/",
				MatchRules: []dataplane.MatchRule{
					{
						Match:        dataplane.Match{},
						BackendGroup: fooGroup,
					},
				},
			})
		}

		if grpc {
			rules = append(rules, dataplane.PathRule{
				Path: "/grpc",
				GRPC: true,
				MatchRules: []dataplane.MatchRule{
					{
						Match:        dataplane.Match{},
						BackendGroup: fooGroup,
					},
				},
			})
		}

		return rules
	}

	tests := []struct {
		name         string
		pathRules    []dataplane.PathRule
		expLocations []http.Location
		grpc         bool
	}{
		{
			name:      "path rules with no root path should generate a default 404 root location",
			pathRules: getPathRules(false /* rootPath */, false /* grpc */),
			expLocations: []http.Location{
				{
					Path:            "/path-1",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "/path-2",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path: "/",
					Return: &http.Return{
						Code: http.StatusNotFound,
					},
				},
			},
		},
		{
			name:      "path rules with grpc & with no root path should generate a default 404 root location and GRPC true",
			pathRules: getPathRules(false /* rootPath */, true /* grpc */),
			grpc:      true,
			expLocations: []http.Location{
				{
					Path:            "/path-1",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "/path-2",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "/grpc",
					ProxyPass:       "grpc://test_foo_80",
					GRPC:            true,
					ProxySetHeaders: grpcBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path: "/",
					Return: &http.Return{
						Code: http.StatusNotFound,
					},
				},
			},
		},
		{
			name:      "path rules with a root path should not generate a default 404 root path",
			pathRules: getPathRules(true /* rootPath */, false /* grpc */),
			expLocations: []http.Location{
				{
					Path:            "/path-1",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "/path-2",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
				{
					Path:            "/",
					ProxyPass:       "http://test_foo_80$request_uri",
					ProxySetHeaders: httpBaseHeaders,
					Type:            http.ExternalLocationType,
				},
			},
		},
		{
			name:      "nil path rules should generate a default 404 root path",
			pathRules: nil,
			expLocations: []http.Location{
				{
					Path: "/",
					Return: &http.Return{
						Code: http.StatusNotFound,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			locs, httpMatchPair, grpc := createLocations(
				&dataplane.VirtualServer{
					PathRules: test.pathRules,
					Port:      80,
				},
				"1",
				&policiesfakes.FakeGenerator{},
				alwaysFalseKeepAliveChecker,
			)
			g.Expect(locs).To(Equal(test.expLocations))
			g.Expect(httpMatchPair).To(BeEmpty())
			g.Expect(grpc).To(Equal(test.grpc))
		})
	}
}

func TestCreateReturnValForRedirectFilter(t *testing.T) {
	t.Parallel()
	const listenerPortCustom = 123
	const listenerPortHTTP = 80
	const listenerPortHTTPS = 443

	createBasicHTTPRequestRedirectFilter := func() *dataplane.HTTPRequestRedirectFilter {
		return &dataplane.HTTPRequestRedirectFilter{
			Scheme:     helpers.GetPointer("http"),
			Hostname:   helpers.GetPointer("foo.example.com"),
			StatusCode: helpers.GetPointer(301),
		}
	}

	modifiedHTTPRequestRedirectFilter := func(
		mod func(
			filter *dataplane.HTTPRequestRedirectFilter,
		) *dataplane.HTTPRequestRedirectFilter,
	) *dataplane.HTTPRequestRedirectFilter {
		return mod(createBasicHTTPRequestRedirectFilter())
	}

	tests := []struct {
		filter          *dataplane.HTTPRequestRedirectFilter
		expectedReturn  *http.Return
		expectedRewrite *rewriteConfig
		msg             string
		path            string
		listenerPort    int32
	}{
		{
			filter:         nil,
			expectedReturn: nil,
			listenerPort:   listenerPortCustom,
			msg:            "filter is nil",
		},
		{
			filter:       &dataplane.HTTPRequestRedirectFilter{},
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: http.StatusFound,
				Body: "$scheme://$host:123$request_uri",
			},
			expectedRewrite: &rewriteConfig{},
			msg:             "all fields are empty",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Scheme = helpers.GetPointer("https")
				filter.Port = helpers.GetPointer[int32](2022)
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplaceFullPath,
					Replacement: "/full-path",
				}
				return filter
			}),
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "https://foo.example.com:2022$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^ /full-path",
			},
			msg: "all fields are set",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Scheme = helpers.GetPointer("https")
				return filter
			}),
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "https://foo.example.com$request_uri",
			},
			expectedRewrite: &rewriteConfig{},
			msg:             "listenerPort is custom, scheme is set, no port",
		},
		{
			filter: &dataplane.HTTPRequestRedirectFilter{
				Hostname:   helpers.GetPointer("foo.example.com"),
				StatusCode: helpers.GetPointer(301),
			},
			listenerPort: listenerPortHTTPS,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "$scheme://foo.example.com:443$request_uri",
			},
			expectedRewrite: &rewriteConfig{},
			msg:             "no scheme, listenerPort https, no port is set",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Scheme = helpers.GetPointer("https")
				return filter
			}),
			listenerPort: listenerPortHTTPS,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "https://foo.example.com$request_uri",
			},
			expectedRewrite: &rewriteConfig{},
			msg:             "scheme is https, listenerPort https, no port is set",
		},
		{
			filter:       createBasicHTTPRequestRedirectFilter(),
			listenerPort: listenerPortHTTP,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "http://foo.example.com$request_uri",
			},
			expectedRewrite: &rewriteConfig{},
			msg:             "scheme is http, listenerPort http, no port is set",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Port = helpers.GetPointer[int32](80)
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "",
				}
				return filter
			}),
			path:         "/original",
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "http://foo.example.com$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^/original(?:/([^?]*))? /$1?$args?",
			},
			msg: "scheme is http, port http, prefix path is empty",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Port = helpers.GetPointer[int32](443)
				filter.Scheme = helpers.GetPointer("https")
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/",
				}
				return filter
			}),
			path:         "/original",
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "https://foo.example.com$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^/original(?:/([^?]*))? /$1?$args?",
			},
			msg: "scheme is https, port https and prefix path is /",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Port = helpers.GetPointer[int32](80)
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/prefix-path",
				}
				return filter
			}),
			path:         "/original",
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "http://foo.example.com$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^/original([^?]*)? /prefix-path$1?$args?",
			},
			msg: "scheme is http, port http, prefix path with no trailing slashes",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Port = helpers.GetPointer[int32](80)
				filter.StatusCode = helpers.GetPointer(302)
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/trailing/",
				}
				return filter
			}),
			path:         "/original",
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 302,
				Body: "http://foo.example.com$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^/original(?:/([^?]*))? /trailing/$1?$args?",
			},
			msg: "scheme is http, port http, prefix path replacement with trailing /",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Port = helpers.GetPointer[int32](80)
				filter.StatusCode = helpers.GetPointer(301)
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/trailing",
				}
				return filter
			}),
			path:         "/original/",
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "http://foo.example.com$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^/original/([^?]*)? /trailing/$1?$args?",
			},
			msg: "scheme is http, port http, prefix path original with trailing /",
		},
		{
			filter: modifiedHTTPRequestRedirectFilter(func(
				filter *dataplane.HTTPRequestRedirectFilter,
			) *dataplane.HTTPRequestRedirectFilter {
				filter.Port = helpers.GetPointer[int32](80)
				filter.StatusCode = helpers.GetPointer(301)
				filter.Path = &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/trailing/",
				}
				return filter
			}),
			path:         "/original/",
			listenerPort: listenerPortCustom,
			expectedReturn: &http.Return{
				Code: 301,
				Body: "http://foo.example.com$uri$is_args$args",
			},
			expectedRewrite: &rewriteConfig{
				MainRewrite: "^/original/([^?]*)? /trailing/$1?$args?",
			},
			msg: "scheme is http, port http, prefix path both with trailing slashes",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result, rewriteConfig := createReturnAndRewriteConfigForRedirectFilter(test.filter, test.listenerPort, test.path)
			g.Expect(helpers.Diff(test.expectedReturn, result)).To(BeEmpty())
			g.Expect(helpers.Diff(test.expectedRewrite, rewriteConfig)).To(BeEmpty())
		})
	}
}

func TestCreateRewritesValForRewriteFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filter   *dataplane.HTTPURLRewriteFilter
		expected *rewriteConfig
		msg      string
		path     string
	}{
		{
			filter:   nil,
			expected: nil,
			msg:      "filter is nil",
		},
		{
			filter:   &dataplane.HTTPURLRewriteFilter{},
			expected: &rewriteConfig{},
			msg:      "all fields are empty",
		},
		{
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplaceFullPath,
					Replacement: "/full-path",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^ /full-path break",
			},
			msg: "full path",
		},
		{
			path: "/original",
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/prefix-path",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^/original([^?]*)? /prefix-path$1?$args? break",
			},
			msg: "prefix path no trailing slashes",
		},
		{
			path: "/original",
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^/original(?:/([^?]*))? /$1?$args? break",
			},
			msg: "prefix path empty string",
		},
		{
			path: "/original",
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^/original(?:/([^?]*))? /$1?$args? break",
			},
			msg: "prefix path /",
		},
		{
			path: "/original",
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/trailing/",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^/original(?:/([^?]*))? /trailing/$1?$args? break",
			},
			msg: "prefix path replacement with trailing /",
		},
		{
			path: "/original/",
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/prefix-path",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^/original/([^?]*)? /prefix-path/$1?$args? break",
			},
			msg: "prefix path original with trailing /",
		},
		{
			path: "/original/",
			filter: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{
					Type:        dataplane.ReplacePrefixMatch,
					Replacement: "/trailing/",
				},
			},
			expected: &rewriteConfig{
				InternalRewrite: "^ $request_uri",
				MainRewrite:     "^/original/([^?]*)? /trailing/$1?$args? break",
			},
			msg: "prefix path both with trailing slashes",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createRewritesValForRewriteFilter(test.filter, test.path)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestCreateRouteMatch(t *testing.T) {
	t.Parallel()
	testPath := "/internal_loc"

	testMethodMatch := helpers.GetPointer("PUT")
	testHeaderMatches := []dataplane.HTTPHeaderMatch{
		{
			Name:  "header-1",
			Value: "val-1",
		},
		{
			Name:  "header-2",
			Value: "val-2",
		},
		{
			Name:  "header-3",
			Value: "val-3",
		},
	}

	testDuplicateHeaders := make([]dataplane.HTTPHeaderMatch, 0, 4)
	duplicateHeaderMatch := dataplane.HTTPHeaderMatch{
		Name:  "HEADER-2", // header names are case-insensitive
		Value: "val-2",
	}
	testDuplicateHeaders = append(testDuplicateHeaders, testHeaderMatches...)
	testDuplicateHeaders = append(testDuplicateHeaders, duplicateHeaderMatch)

	testQueryParamMatches := []dataplane.HTTPQueryParamMatch{
		{
			Name:  "arg1",
			Value: "val1",
		},
		{
			Name:  "arg2",
			Value: "val2=another-val",
		},
		{
			Name:  "arg3",
			Value: "==val3",
		},
	}

	expectedHeaders := []string{"header-1:val-1", "header-2:val-2", "header-3:val-3"}
	expectedArgs := []string{"arg1=val1", "arg2=val2=another-val", "arg3===val3"}

	tests := []struct {
		match    dataplane.Match
		msg      string
		expected routeMatch
	}{
		{
			match: dataplane.Match{},
			expected: routeMatch{
				Any:          true,
				RedirectPath: testPath,
			},
			msg: "path only match",
		},
		{
			match: dataplane.Match{
				Method: testMethodMatch, // A path match with a method should not set the Any field to true
			},
			expected: routeMatch{
				Method:       "PUT",
				RedirectPath: testPath,
			},
			msg: "method only match",
		},
		{
			match: dataplane.Match{
				Headers: testHeaderMatches,
			},
			expected: routeMatch{
				RedirectPath: testPath,
				Headers:      expectedHeaders,
			},
			msg: "headers only match",
		},
		{
			match: dataplane.Match{
				QueryParams: testQueryParamMatches,
			},
			expected: routeMatch{
				QueryParams:  expectedArgs,
				RedirectPath: testPath,
			},
			msg: "query params only match",
		},
		{
			match: dataplane.Match{
				Method:      testMethodMatch,
				QueryParams: testQueryParamMatches,
			},
			expected: routeMatch{
				Method:       "PUT",
				QueryParams:  expectedArgs,
				RedirectPath: testPath,
			},
			msg: "method and query params match",
		},
		{
			match: dataplane.Match{
				Method:  testMethodMatch,
				Headers: testHeaderMatches,
			},
			expected: routeMatch{
				Method:       "PUT",
				Headers:      expectedHeaders,
				RedirectPath: testPath,
			},
			msg: "method and headers match",
		},
		{
			match: dataplane.Match{
				QueryParams: testQueryParamMatches,
				Headers:     testHeaderMatches,
			},
			expected: routeMatch{
				QueryParams:  expectedArgs,
				Headers:      expectedHeaders,
				RedirectPath: testPath,
			},
			msg: "query params and headers match",
		},
		{
			match: dataplane.Match{
				Headers:     testHeaderMatches,
				QueryParams: testQueryParamMatches,
				Method:      testMethodMatch,
			},
			expected: routeMatch{
				Method:       "PUT",
				Headers:      expectedHeaders,
				QueryParams:  expectedArgs,
				RedirectPath: testPath,
			},
			msg: "method, headers, and query params match",
		},
		{
			match: dataplane.Match{
				Headers: testDuplicateHeaders,
			},
			expected: routeMatch{
				Headers:      expectedHeaders,
				RedirectPath: testPath,
			},
			msg: "duplicate header names",
		},
	}
	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createRouteMatch(tc.match, testPath)
			g.Expect(helpers.Diff(result, tc.expected)).To(BeEmpty())
		})
	}
}

func TestCreateQueryParamKeyValString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	expected := "key=value"

	result := createQueryParamKeyValString(
		dataplane.HTTPQueryParamMatch{
			Name:  "key",
			Value: "value",
		},
	)

	g.Expect(result).To(Equal(expected))

	expected = "KeY=vaLUe=="

	result = createQueryParamKeyValString(
		dataplane.HTTPQueryParamMatch{
			Name:  "KeY",
			Value: "vaLUe==",
		},
	)

	g.Expect(result).To(Equal(expected))
}

func TestCreateHeaderKeyValString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	expected := "kEy:vALUe"

	result := createHeaderKeyValString(
		dataplane.HTTPHeaderMatch{
			Name:  "kEy",
			Value: "vALUe",
		},
	)

	g.Expect(result).To(Equal(expected))
}

func TestIsPathOnlyMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg      string
		match    dataplane.Match
		expected bool
	}{
		{
			match:    dataplane.Match{},
			expected: true,
			msg:      "path only match",
		},
		{
			match: dataplane.Match{
				Method: helpers.GetPointer("GET"),
			},
			expected: false,
			msg:      "method defined in match",
		},
		{
			match: dataplane.Match{
				Headers: []dataplane.HTTPHeaderMatch{
					{
						Name:  "header",
						Value: "val",
					},
				},
			},
			expected: false,
			msg:      "headers defined in match",
		},
		{
			match: dataplane.Match{
				QueryParams: []dataplane.HTTPQueryParamMatch{
					{
						Name:  "arg",
						Value: "val",
					},
				},
			},
			expected: false,
			msg:      "query params defined in match",
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := isPathOnlyMatch(tc.match)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestCreateProxyPass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rewrite  *dataplane.HTTPURLRewriteFilter
		expected string
		grp      dataplane.BackendGroup
		GRPC     bool
	}{
		{
			expected: "http://10.0.0.1:80$request_uri",
			grp: dataplane.BackendGroup{
				Backends: []dataplane.Backend{
					{
						UpstreamName: "10.0.0.1:80",
						Valid:        true,
						Weight:       1,
					},
				},
			},
		},
		{
			expected: "http://$group_ns1__bg_rule0$request_uri",
			grp: dataplane.BackendGroup{
				Source: types.NamespacedName{Namespace: "ns1", Name: "bg"},
				Backends: []dataplane.Backend{
					{
						UpstreamName: "my-variable",
						Valid:        true,
						Weight:       1,
					},
					{
						UpstreamName: "my-variable2",
						Valid:        true,
						Weight:       1,
					},
				},
			},
		},
		{
			expected: "http://10.0.0.1:80",
			rewrite: &dataplane.HTTPURLRewriteFilter{
				Path: &dataplane.HTTPPathModifier{},
			},
			grp: dataplane.BackendGroup{
				Backends: []dataplane.Backend{
					{
						UpstreamName: "10.0.0.1:80",
						Valid:        true,
						Weight:       1,
					},
				},
			},
		},
		{
			expected: "grpc://10.0.0.1:80",
			grp: dataplane.BackendGroup{
				Backends: []dataplane.Backend{
					{
						UpstreamName: "10.0.0.1:80",
						Valid:        true,
						Weight:       1,
					},
				},
			},
			GRPC: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := createProxyPass(tc.grp, tc.rewrite, generateProtocolString(nil, tc.GRPC), tc.GRPC)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestCreateMatchLocation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	expectedNoGRPC := http.Location{
		Path: "/path",
		Type: http.InternalLocationType,
	}

	grpc := false
	result := createMatchLocation("/path", grpc)
	g.Expect(result).To(Equal(expectedNoGRPC))

	expectedWithGRPC := http.Location{
		Path:     "/path",
		Type:     http.InternalLocationType,
		Rewrites: []string{"^ $request_uri break"},
	}

	grpc = true
	result = createMatchLocation("/path", grpc)
	g.Expect(result).To(Equal(expectedWithGRPC))
}

func TestGenerateProxySetHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filters         *dataplane.HTTPFilters
		msg             string
		expectedHeaders []http.Header
		baseHeaders     []http.Header
	}{
		{
			msg: "header filter",
			filters: &dataplane.HTTPFilters{
				RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
					Add: []dataplane.HTTPHeader{
						{
							Name:  "Authorization",
							Value: "my-auth",
						},
					},
					Set: []dataplane.HTTPHeader{
						{
							Name:  "Accept-Encoding",
							Value: "gzip",
						},
					},
					Remove: []string{"my-header"},
				},
			},
			expectedHeaders: append([]http.Header{
				{
					Name:  "Authorization",
					Value: "${authorization_header_var}my-auth",
				},
				{
					Name:  "Accept-Encoding",
					Value: "gzip",
				},
				{
					Name:  "my-header",
					Value: "",
				},
			}, httpBaseHeaders...),
			baseHeaders: httpBaseHeaders,
		},
		{
			msg: "with url rewrite hostname",
			filters: &dataplane.HTTPFilters{
				RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
					Add: []dataplane.HTTPHeader{
						{
							Name:  "Authorization",
							Value: "my-auth",
						},
					},
				},
				RequestURLRewrite: &dataplane.HTTPURLRewriteFilter{
					Hostname: helpers.GetPointer("rewrite-hostname"),
				},
			},
			expectedHeaders: []http.Header{
				{
					Name:  "Authorization",
					Value: "${authorization_header_var}my-auth",
				},
				{
					Name:  "Host",
					Value: "rewrite-hostname",
				},
				{
					Name:  "X-Forwarded-For",
					Value: "$proxy_add_x_forwarded_for",
				},
				{
					Name:  "X-Real-IP",
					Value: "$remote_addr",
				},
				{
					Name:  "X-Forwarded-Proto",
					Value: "$scheme",
				},
				{
					Name:  "X-Forwarded-Host",
					Value: "$host",
				},
				{
					Name:  "X-Forwarded-Port",
					Value: "$server_port",
				},
				{
					Name:  "Upgrade",
					Value: "$http_upgrade",
				},
				{
					Name:  "Connection",
					Value: "$connection_upgrade",
				},
			},
			baseHeaders: createBaseProxySetHeaders(httpUpgradeHeader, httpConnectionHeader),
		},
		{
			msg: "header filter with gRPC",
			filters: &dataplane.HTTPFilters{
				RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
					Add: []dataplane.HTTPHeader{
						{
							Name:  "Authorization",
							Value: "my-auth",
						},
					},
					Set: []dataplane.HTTPHeader{
						{
							Name:  "Accept-Encoding",
							Value: "gzip",
						},
					},
					Remove: []string{"my-header"},
				},
			},
			expectedHeaders: append([]http.Header{
				{
					Name:  "Authorization",
					Value: "${authorization_header_var}my-auth",
				},
				{
					Name:  "Accept-Encoding",
					Value: "gzip",
				},
				{
					Name:  "my-header",
					Value: "",
				},
			}, grpcBaseHeaders...),
			baseHeaders: grpcBaseHeaders,
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			headers := generateProxySetHeaders(tc.filters, tc.baseHeaders)
			g.Expect(headers).To(Equal(tc.expectedHeaders))
		})
	}
}

func TestCreateBaseProxySetHeaders(t *testing.T) {
	t.Parallel()

	expBaseHeaders := []http.Header{
		{
			Name:  "Host",
			Value: "$gw_api_compliant_host",
		},
		{
			Name:  "X-Forwarded-For",
			Value: "$proxy_add_x_forwarded_for",
		},
		{
			Name:  "X-Real-IP",
			Value: "$remote_addr",
		},
		{
			Name:  "X-Forwarded-Proto",
			Value: "$scheme",
		},
		{
			Name:  "X-Forwarded-Host",
			Value: "$host",
		},
		{
			Name:  "X-Forwarded-Port",
			Value: "$server_port",
		},
	}

	tests := []struct {
		msg               string
		additionalHeaders []http.Header
		expBaseHeaders    []http.Header
	}{
		{
			msg:               "no additional headers",
			additionalHeaders: []http.Header{},
			expBaseHeaders:    expBaseHeaders,
		},
		{
			msg: "single additional headers",
			additionalHeaders: []http.Header{
				grpcAuthorityHeader,
			},
			expBaseHeaders: append(expBaseHeaders, grpcAuthorityHeader),
		},
		{
			msg: "multiple additional headers",
			additionalHeaders: []http.Header{
				httpConnectionHeader,
				httpUpgradeHeader,
			},
			expBaseHeaders: append(expBaseHeaders, httpConnectionHeader, httpUpgradeHeader),
		},
		{
			msg: "unset connection header and upgrade header",
			additionalHeaders: []http.Header{
				unsetHTTPConnectionHeader,
				httpUpgradeHeader,
			},
			expBaseHeaders: append(expBaseHeaders, unsetHTTPConnectionHeader, httpUpgradeHeader),
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createBaseProxySetHeaders(test.additionalHeaders...)
			g.Expect(result).To(Equal(test.expBaseHeaders))
		})
	}
}

func TestGetConnectionHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg                 string
		upstreams           []http.Upstream
		expConnectionHeader http.Header
		backends            []dataplane.Backend
	}{
		{
			msg: "no upstreams with keepAlive enabled",
			upstreams: []http.Upstream{
				{
					Name: "upstream1",
				},
				{
					Name: "upstream2",
				},
				{
					Name: "upstream3",
				},
			},
			backends: []dataplane.Backend{
				{
					UpstreamName: "upstream1",
				},
				{
					UpstreamName: "upstream2",
				},
				{
					UpstreamName: "upstream3",
				},
			},
			expConnectionHeader: httpConnectionHeader,
		},
		{
			msg: "upstream with keepAlive enabled",
			upstreams: []http.Upstream{
				{
					Name: "upstream",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 1,
					},
				},
			},
			backends: []dataplane.Backend{
				{
					UpstreamName: "upstream",
				},
			},
			expConnectionHeader: unsetHTTPConnectionHeader,
		},
		{
			msg: "multiple upstreams with keepAlive enabled",
			upstreams: []http.Upstream{
				{
					Name: "upstream1",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 1,
					},
				},
				{
					Name: "upstream2",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 2,
						Requests:    1,
					},
				},
				{
					Name: "upstream3",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 3,
						Time:        "5s",
					},
				},
			},
			backends: []dataplane.Backend{
				{
					UpstreamName: "upstream1",
				},
				{
					UpstreamName: "upstream2",
				},
				{
					UpstreamName: "upstream3",
				},
			},
			expConnectionHeader: unsetHTTPConnectionHeader,
		},
		{
			msg:                 "mix of upstreams with keepAlive enabled and disabled",
			expConnectionHeader: unsetHTTPConnectionHeader,
			upstreams: []http.Upstream{
				{
					Name: "upstream1",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 1,
					},
				},
				{
					Name: "upstream2",
				},
			},
			backends: []dataplane.Backend{
				{
					UpstreamName: "upstream1",
				},
				{
					UpstreamName: "upstream2",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			keepAliveCheck := newKeepAliveChecker(tc.upstreams)

			connectionHeader := getConnectionHeader(keepAliveCheck, tc.backends)
			g.Expect(connectionHeader).To(Equal(tc.expConnectionHeader))
		})
	}
}

func TestConvertBackendTLSFromGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expected *http.ProxySSLVerify
		msg      string
		grp      []dataplane.Backend
	}{
		{
			msg: "tls enabled, one backend",
			grp: []dataplane.Backend{
				{
					UpstreamName: "my-upstream",
					Valid:        true,
					Weight:       1,
					VerifyTLS: &dataplane.VerifyTLS{
						CertBundleID: "default-my-cert",
						Hostname:     "my-hostname",
					},
				},
			},
			expected: &http.ProxySSLVerify{
				TrustedCertificate: "/etc/nginx/secrets/default-my-cert.crt",
				Name:               "my-hostname",
			},
		},
		{
			msg: "tls disabled",
			grp: []dataplane.Backend{
				{
					UpstreamName: "my-upstream",
					Valid:        true,
					Weight:       1,
					VerifyTLS:    nil,
				},
			},
			expected: nil,
		},
		{
			msg: "tls enabled, multiple backends",
			grp: []dataplane.Backend{
				{
					UpstreamName: "my-upstream",
					Valid:        true,
					Weight:       1,
					VerifyTLS: &dataplane.VerifyTLS{
						CertBundleID: "default-my-cert",
						Hostname:     "my-hostname",
					},
				},
				{
					UpstreamName: "my-upstream",
					Valid:        true,
					Weight:       2,
				},
			},
			expected: &http.ProxySSLVerify{
				TrustedCertificate: "/etc/nginx/secrets/default-my-cert.crt",
				Name:               "my-hostname",
			},
		},
		{
			msg: "tls enabled, system certs enabled",
			grp: []dataplane.Backend{
				{
					UpstreamName: "my-upstream",
					Valid:        true,
					Weight:       1,
					VerifyTLS: &dataplane.VerifyTLS{
						Hostname:   "my-hostname",
						RootCAPath: "/etc/ssl/certs/ca-certificates.crt",
					},
				},
			},
			expected: &http.ProxySSLVerify{
				TrustedCertificate: "/etc/ssl/certs/ca-certificates.crt",
				Name:               "my-hostname",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createProxyTLSFromBackends(tc.grp)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestGenerateResponseHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filters         *dataplane.HTTPFilters
		msg             string
		expectedHeaders http.ResponseHeaders
	}{
		{
			msg: "no filter set",
			filters: &dataplane.HTTPFilters{
				RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{},
			},
			expectedHeaders: http.ResponseHeaders{},
		},
		{
			msg: "set filters correctly",
			filters: &dataplane.HTTPFilters{
				ResponseHeaderModifiers: &dataplane.HTTPHeaderFilter{
					Add: []dataplane.HTTPHeader{
						{
							Name:  "Accept-Encoding",
							Value: "gzip",
						},
						{
							Name:  "Authorization",
							Value: "my-auth",
						},
					},
					Set: []dataplane.HTTPHeader{
						{
							Name:  "Accept-Encoding",
							Value: "my-new-overwritten-value",
						},
					},
					Remove: []string{"Transfer-Encoding"},
				},
			},
			expectedHeaders: http.ResponseHeaders{
				Add: []http.Header{
					{
						Name:  "Accept-Encoding",
						Value: "gzip",
					},
					{
						Name:  "Authorization",
						Value: "my-auth",
					},
				},
				Set: []http.Header{
					{
						Name:  "Accept-Encoding",
						Value: "my-new-overwritten-value",
					},
				},
				Remove: []string{"Transfer-Encoding"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			headers := generateResponseHeaders(tc.filters)
			g.Expect(headers).To(Equal(tc.expectedHeaders))
		})
	}
}

func TestGetIPFamily(t *testing.T) {
	t.Parallel()
	test := []struct {
		msg            string
		baseHTTPConfig dataplane.BaseHTTPConfig
		expected       shared.IPFamily
	}{
		{
			msg:            "ipv4",
			baseHTTPConfig: dataplane.BaseHTTPConfig{IPFamily: dataplane.IPv4},
			expected:       shared.IPFamily{IPv4: true, IPv6: false},
		},
		{
			msg:            "ipv6",
			baseHTTPConfig: dataplane.BaseHTTPConfig{IPFamily: dataplane.IPv6},
			expected:       shared.IPFamily{IPv4: false, IPv6: true},
		},
		{
			msg:            "dual",
			baseHTTPConfig: dataplane.BaseHTTPConfig{IPFamily: dataplane.Dual},
			expected:       shared.IPFamily{IPv4: true, IPv6: true},
		},
	}

	for _, tc := range test {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := getIPFamily(tc.baseHTTPConfig)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
