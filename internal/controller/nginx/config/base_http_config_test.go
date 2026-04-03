package config

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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
			unexpectedOutputs: []string{
				"escape=",
			},
		},
		{
			name:      "Log format with escape=json",
			accessLog: &dataplane.AccessLog{Format: logFormat, Escape: "json"},
			expectedOutputs: []string{
				fmt.Sprintf("log_format %s escape=json '%s'", dataplane.DefaultLogFormatName, logFormat),
				fmt.Sprintf("access_log %s %s", dataplane.DefaultAccessLogPath, dataplane.DefaultLogFormatName),
			},
		},
		{
			name:      "Log format with escape=default",
			accessLog: &dataplane.AccessLog{Format: logFormat, Escape: "default"},
			expectedOutputs: []string{
				fmt.Sprintf("log_format %s escape=default '%s'", dataplane.DefaultLogFormatName, logFormat),
				fmt.Sprintf("access_log %s %s", dataplane.DefaultAccessLogPath, dataplane.DefaultLogFormatName),
			},
		},
		{
			name:      "Log format with escape=none",
			accessLog: &dataplane.AccessLog{Format: logFormat, Escape: "none"},
			expectedOutputs: []string{
				fmt.Sprintf("log_format %s escape=none '%s'", dataplane.DefaultLogFormatName, logFormat),
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
			name:      "Empty format with escape set should not output escape",
			accessLog: &dataplane.AccessLog{Format: "", Escape: "json"},
			unexpectedOutputs: []string{
				"log_format",
				"escape=",
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

			res := executeBaseHTTPConfig(conf, policies.UnimplementedGenerator{})
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

			res := executeBaseHTTPConfig(test.conf, policies.UnimplementedGenerator{})
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

	res := executeBaseHTTPConfig(conf, policies.UnimplementedGenerator{})
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
			NginxReadinessProbePath: dataplane.DefaultNginxReadinessProbePath,
		},
	}

	customPortPathConfig := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: 9090,
			NginxReadinessProbePath: "/nginx-ready",
		},
	}

	customIPv4Config := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: dataplane.DefaultNginxReadinessProbePort,
			IPFamily:                dataplane.IPv4,
			NginxReadinessProbePath: dataplane.DefaultNginxReadinessProbePath,
		},
	}

	customIPv6Config := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			NginxReadinessProbePort: dataplane.DefaultNginxReadinessProbePort,
			IPFamily:                dataplane.IPv6,
			NginxReadinessProbePath: dataplane.DefaultNginxReadinessProbePath,
		},
	}

	tests := []struct {
		name             string
		expectedPath     string
		expectedListen   string
		expectedNoListen string
		conf             dataplane.Configuration
	}{
		{
			name:           "default nginx readiness probe port",
			conf:           defaultConfig,
			expectedPath:   "/readyz",
			expectedListen: "listen 8081;",
		},
		{
			name:           "default nginx readiness probe port on ipv6",
			conf:           defaultConfig,
			expectedPath:   "/readyz",
			expectedListen: "listen [::]:8081;",
		},
		{
			name:           "custom nginx readiness probe 9090",
			conf:           customPortPathConfig,
			expectedPath:   "/nginx-ready",
			expectedListen: "listen 9090;",
		},
		{
			name:           "custom nginx readiness probe 9090 on ipv6",
			conf:           customPortPathConfig,
			expectedPath:   "/nginx-ready",
			expectedListen: "listen [::]:9090;",
		},
		{
			name:             "custom ipv4 nginx readiness probe does not have ipv6 listen",
			conf:             customIPv4Config,
			expectedPath:     "/readyz",
			expectedListen:   "listen 8081;",
			expectedNoListen: "listen [::]:8081;",
		},
		{
			name:             "custom ipv6 nginx readiness probe does not have ipv4 listen",
			conf:             customIPv6Config,
			expectedPath:     "/readyz",
			expectedListen:   "listen [::]:8081;",
			expectedNoListen: "listen 8081;",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf, policies.UnimplementedGenerator{})
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
			g.Expect(httpConfig).To(ContainSubstring(fmt.Sprintf("location = %s {", test.expectedPath)))
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
			name: "DNS resolver with multiple IPv6 addresses",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses: []string{"2606:4700:4700::64", "2606:4700:4700::6400"},
					},
				},
			},
			expectedConfig: "resolver [2606:4700:4700::64] [2606:4700:4700::6400];",
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
			name: "DNS resolver with multiple IPv6 addresses and multiple IPv4 addresses",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					DNSResolver: &dataplane.DNSResolverConfig{
						Addresses: []string{"2606:4700:4700::64", "8.8.8.8", "2606:4700:4700::6400", "8.8.4.4"},
					},
				},
			},
			expectedConfig: "resolver [2606:4700:4700::64] 8.8.8.8 [2606:4700:4700::6400] 8.8.4.4;",
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

			res := executeBaseHTTPConfig(test.conf, policies.UnimplementedGenerator{})
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

			res := executeBaseHTTPConfig(test.conf, policies.UnimplementedGenerator{})
			g.Expect(res).To(HaveLen(1))

			httpConfig := string(res[0].data)

			if test.expectedConfig != "" {
				g.Expect(httpConfig).To(ContainSubstring(test.expectedConfig))
			}
		})
	}
}

func TestExecuteBaseHttp_ConnectionHeaderMaps(t *testing.T) {
	t.Parallel()

	expSubStringsWithCount := map[string]int{
		"map $http_upgrade $connection_upgrade":   1,
		"default upgrade;":                        2,
		"'' close;":                               1,
		"map $http_upgrade $connection_keepalive": 1,
		"'' '';": 1,
	}

	g := NewWithT(t)
	res := executeBaseHTTPConfig(dataplane.Configuration{}, policies.UnimplementedGenerator{})
	g.Expect(res).To(HaveLen(1))
	httpConfig := string(res[0].data)
	for subStr, count := range expSubStringsWithCount {
		g.Expect(strings.Count(httpConfig, subStr)).To(Equal(count))
	}
}

func TestExecuteBaseHttp_Policies(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	fakeGen := &policiesfakes.FakeGenerator{}
	fakeGen.GenerateForHTTPReturns(policies.GenerateResultFiles{
		{
			Name:    "policy1.conf",
			Content: []byte("policy1 content"),
		},
		{
			Name:    "policy2.conf",
			Content: []byte("policy2 content"),
		},
	})

	conf := dataplane.Configuration{
		BaseHTTPConfig: dataplane.BaseHTTPConfig{
			Policies: []policies.Policy{
				&policiesfakes.FakePolicy{},
				&policiesfakes.FakePolicy{},
			},
		},
	}

	res := executeBaseHTTPConfig(conf, fakeGen)
	g.Expect(res).To(HaveLen(3)) // 1 http.conf + 2 policy files

	sort.Slice(res, func(i, j int) bool {
		return res[i].dest < res[j].dest
	})

	/*
		Order of files:
		/etc/nginx/conf.d/http.conf
		/etc/nginx/includes/policy1.conf
		/etc/nginx/includes/policy2.conf
	*/

	httpRes := string(res[0].data)
	g.Expect(httpRes).To(ContainSubstring("map $http_host $gw_api_compliant_host {"))
	g.Expect(httpRes).To(ContainSubstring("include /etc/nginx/includes/policy1.conf;"))
	g.Expect(httpRes).To(ContainSubstring("include /etc/nginx/includes/policy2.conf;"))

	policy1Res := string(res[1].data)
	g.Expect(policy1Res).To(Equal("policy1 content"))

	policy2Res := string(res[2].data)
	g.Expect(policy2Res).To(Equal("policy2 content"))

	// Verify GenerateForHTTP was called with the correct policies
	g.Expect(fakeGen.GenerateForHTTPCallCount()).To(Equal(1))
	calledPolicies := fakeGen.GenerateForHTTPArgsForCall(0)
	g.Expect(calledPolicies).To(HaveLen(2))
}

//nolint:gosec // Test data with hardcoded values, no injection risk.
func TestExecuteBaseHttp_ServerTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		expServerTokens string
		conf            dataplane.Configuration
	}{
		{
			name: "custom server tokens",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{ServerTokens: "on"},
			},
			expServerTokens: "server_tokens on;",
		},
		{
			name: "empty string server tokens",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{ServerTokens: fmt.Sprintf(`"%s"`, "")},
			},
			expServerTokens: "server_tokens \"\";",
		},
		{
			name: "custom string server tokens",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{ServerTokens: fmt.Sprintf(`"%s"`, "custom-string")},
			},
			expServerTokens: "server_tokens \"custom-string\";",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf, &policiesfakes.FakeGenerator{})
			g.Expect(res).To(HaveLen(1))
			g.Expect(res[0].dest).To(Equal(httpConfigFile))
			g.Expect(string(res[0].data)).To(ContainSubstring(test.expServerTokens))
		})
	}
}

func TestExecuteBaseHttp_OIDCProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		expSubStrings []string
		expAbsent     []string
		conf          dataplane.Configuration
	}{
		{
			name: "no OIDC configuration, missing OIDC directives",
			conf: dataplane.Configuration{},
			expAbsent: []string{
				"oidc_provider",
				"client_secret",
			},
		},
		{
			name: "single OIDC provider without CA cert and with custom redirect URI",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:         "oidc_test_my-filter",
						Issuer:       "https://idp.example.com",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						RedirectURI:  "/custom_callback/path",
					},
				},
			},
			expSubStrings: []string{
				"oidc_provider oidc_test_my-filter {",
				"issuer https://idp.example.com;",
				"client_id my-client-id;",
				"client_secret my-client-secret;",
				"redirect_uri /custom_callback/path;",
			},
			expAbsent: []string{
				"ssl_trusted_certificate",
			},
		},
		{
			name: "single OIDC provider with CA cert",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:           "oidc_test_my-filter",
						Issuer:         "https://idp.example.com",
						ClientID:       "my-client-id",
						ClientSecret:   "my-client-secret",
						CACertBundleID: "oidc_ca_test_my-ca",
						RedirectURI:    "/oidc_callback_test_my-filter",
					},
				},
			},
			expSubStrings: []string{
				"oidc_provider oidc_test_my-filter {",
				"issuer https://idp.example.com;",
				"client_id my-client-id;",
				"client_secret my-client-secret;",
				"redirect_uri /oidc_callback_test_my-filter;",
				"ssl_trusted_certificate /etc/nginx/secrets/oidc_ca_test_my-ca.crt;",
			},
		},
		{
			name: "OIDC provider with empty name is skipped and generates no oidc_provider block",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:         "",
						Issuer:       "https://idp.example.com",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						RedirectURI:  "/oidc_callback",
					},
				},
			},
			expAbsent: []string{
				"oidc_provider",
				"client_secret",
			},
		},
		{
			name: "two OIDC providers each generates its own oidc_provider block",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:         "oidc_test_filter-one",
						Issuer:       "https://idp1.example.com",
						ClientID:     "client-id-1",
						ClientSecret: "client-secret-1",
						RedirectURI:  "/oidc_callback_test_filter-one",
					},
					{
						Name:           "oidc_test_filter-two",
						Issuer:         "https://idp2.example.com",
						ClientID:       "client-id-2",
						ClientSecret:   "client-secret-2",
						CACertBundleID: "oidc_ca_test_filter-two",
						RedirectURI:    "/oidc_callback_test_filter-two",
					},
				},
			},
			expSubStrings: []string{
				"oidc_provider oidc_test_filter-one {",
				"issuer https://idp1.example.com;",
				"client_id client-id-1;",
				"client_secret client-secret-1;",
				"redirect_uri /oidc_callback_test_filter-one;",
				"oidc_provider oidc_test_filter-two {",
				"issuer https://idp2.example.com;",
				"client_id client-id-2;",
				"client_secret client-secret-2;",
				"redirect_uri /oidc_callback_test_filter-two;",
				"ssl_trusted_certificate /etc/nginx/secrets/oidc_ca_test_filter-two.crt;",
			},
		},
		{
			name: "OIDC provider with all optional fields renders all corresponding directives",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:                  "oidc_test_full",
						Issuer:                "https://idp.example.com",
						ClientID:              "client-id",
						ClientSecret:          "client-secret",
						RedirectURI:           "/oidc_callback_test_full",
						CACertBundleID:        "oidc_ca_test_full",
						CRLBundleID:           "crl_bundle_test_crl-secret",
						ConfigURL:             helpers.GetPointer("https://idp.example.com/.well-known/openid-configuration"),
						PKCE:                  helpers.GetPointer(true),
						ExtraAuthArgs:         "audience=api&prompt=consent",
						CookieName:            helpers.GetPointer("MY_SESSION"),
						Timeout:               helpers.GetPointer("2h"),
						LogoutURI:             helpers.GetPointer("/logout"),
						PostLogoutURI:         helpers.GetPointer("/logged-out"),
						FrontChannelLogoutURI: helpers.GetPointer("/frontchannel-logout"),
						TokenHint:             helpers.GetPointer(true),
					},
				},
			},
			expSubStrings: []string{
				"ssl_trusted_certificate /etc/nginx/secrets/oidc_ca_test_full.crt;",
				"ssl_crl /etc/nginx/secrets/crl_bundle_test_crl-secret.pem;",
				"config_url https://idp.example.com/.well-known/openid-configuration;",
				"pkce on;",
				`extra_auth_args "audience=api&prompt=consent";`,
				"cookie_name MY_SESSION;",
				"session_timeout 2h;",
				"logout_uri /logout;",
				"post_logout_uri /logged-out;",
				"frontchannel_logout_uri /frontchannel-logout;",
				"logout_token_hint on;",
			},
		},
		{
			name: "OIDC provider with no scopes does not render scope directive",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:         "oidc_test_my-filter",
						Issuer:       "https://idp.example.com",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						RedirectURI:  "/oidc_callback_test_my-filter",
					},
				},
			},
			expAbsent: []string{"scope"},
		},
		{
			name: "OIDC provider with PKCE off and TokenHint off renders off directives",
			conf: dataplane.Configuration{
				OIDCProviders: []dataplane.OIDCProvider{
					{
						Name:         "oidc_test_off",
						Issuer:       "https://idp.example.com",
						ClientID:     "client-id",
						ClientSecret: "client-secret",
						RedirectURI:  "/oidc_callback",
						PKCE:         helpers.GetPointer(false),
						TokenHint:    helpers.GetPointer(false),
					},
				},
			},
			expSubStrings: []string{
				"pkce off;",
				"logout_token_hint off;",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf, &policiesfakes.FakeGenerator{})
			g.Expect(res).To(HaveLen(1))
			data := string(res[0].data)

			for _, sub := range test.expSubStrings {
				g.Expect(data).To(ContainSubstring(sub))
			}
			for _, absent := range test.expAbsent {
				g.Expect(data).NotTo(ContainSubstring(absent))
			}
		})
	}
}

func TestExecuteBaseHttp_Compression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		expSubStrings []string
		expAbsent     []string
		conf          dataplane.Configuration
	}{
		{
			name: "compression disabled",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{},
			},
			expAbsent: []string{"gzip on;"},
		},
		{
			name: "compression enabled with defaults",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					Compression: &dataplane.CompressionSettings{
						Level: 1,
						Types: []string{"text/css", "application/json"},
					},
				},
			},
			expSubStrings: []string{
				"gzip on;",
				"gzip_comp_level 1;",
				"gzip_min_length 0;",
				"gzip_types text/css application/json;",
			},
			expAbsent: []string{"gzip_vary", "gzip_proxied"},
		},
		{
			name: "compression enabled with all options",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					Compression: &dataplane.CompressionSettings{
						Level:     6,
						MinLength: 256,
						Types:     []string{"text/css", "application/json", "application/javascript"},
						Proxied:   []string{"any"},
						Vary:      true,
					},
				},
			},
			expSubStrings: []string{
				"gzip on;",
				"gzip_comp_level 6;",
				"gzip_min_length 256;",
				"gzip_types text/css application/json application/javascript;",
				"gzip_proxied any;",
				"gzip_vary on;",
			},
		},
		{
			name: "compression with multiple proxied values",
			conf: dataplane.Configuration{
				BaseHTTPConfig: dataplane.BaseHTTPConfig{
					Compression: &dataplane.CompressionSettings{
						Level:   3,
						Types:   []string{"text/plain"},
						Proxied: []string{"no-cache", "no-store", "expired"},
					},
				},
			},
			expSubStrings: []string{
				"gzip on;",
				"gzip_proxied no-cache no-store expired;",
			},
			expAbsent: []string{"gzip_vary"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			res := executeBaseHTTPConfig(test.conf, &policiesfakes.FakeGenerator{})
			g.Expect(res).To(HaveLen(1))
			data := string(res[0].data)

			for _, sub := range test.expSubStrings {
				g.Expect(data).To(ContainSubstring(sub))
			}
			for _, absent := range test.expAbsent {
				g.Expect(data).NotTo(ContainSubstring(absent))
			}
		})
	}
}
