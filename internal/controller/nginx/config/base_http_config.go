package config

import (
	"fmt"
	"net"
	"sort"
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var baseHTTPTemplate = gotemplate.Must(gotemplate.New("baseHttp").Parse(baseHTTPTemplateText))

type AccessLog struct {
	Format     string // User's format string
	Escape     string // Escape setting for variables (default, json, none)
	Path       string // Where to write logs (/dev/stdout)
	FormatName string // Internal format name (ngf_user_defined_log_format)
	Disable    bool   // User's disable flag
}

// oidcConfiguration holds the OIDC config.
type oidcConfiguration struct {
	Name                   string
	Issuer                 string
	ClientID               string
	ClientSecret           string
	TrustedCertificatePath string
	CRLPath                string
	RedirectURI            string
	ConfigURL              string
	PKCE                   string
	ExtraAuthArgs          string
	CookieName             string
	Timeout                string
	LogoutURI              string
	PostLogoutURI          string
	FrontChannelLogoutURI  string
	TokenHint              string
}

// ClaimSet represents a single auth_jwt_claim_set directive with its NGINX variable name and claim path components.
type ClaimSet struct {
	// Variable is the NGINX variable name (e.g., "$jwt_claim_sub").
	Variable string
	// Claims are the claim path components (e.g., ["realm_access", "roles"]).
	Claims []string
}

type httpConfig struct {
	DNSResolver             *dataplane.DNSResolverConfig
	AccessLog               *AccessLog
	Compression             *dataplane.CompressionSettings
	GatewaySecretID         dataplane.SSLKeyPairID
	NginxReadinessProbePath string
	ServerTokens            string
	WAFCookieSeed           string
	ClaimSets               []ClaimSet
	Includes                []shared.Include
	OIDCProviders           []*oidcConfiguration
	NginxReadinessProbePort int32
	IPFamily                shared.IPFamily
	HTTP2                   bool
	WAF                     bool
}

func newExecuteBaseHTTPConfigFunc(generator policies.Generator) executeFunc {
	return func(conf dataplane.Configuration) []executeResult {
		return executeBaseHTTPConfig(conf, generator)
	}
}

func executeBaseHTTPConfig(conf dataplane.Configuration, generator policies.Generator) []executeResult {
	includes := createIncludesFromSnippets(conf.BaseHTTPConfig.Snippets)

	policyIncludes := createIncludesFromPolicyGenerateResult(generator.GenerateForHTTP(conf.BaseHTTPConfig.Policies))
	includes = append(includes, policyIncludes...)

	authZIncludes := createIncludesFromAuthZConfigs(conf.BaseHTTPConfig.AuthZConfigs)
	includes = append(includes, authZIncludes...)

	claimSets := collectAuthZClaimSets(conf.BaseHTTPConfig.AuthZConfigs)

	hc := httpConfig{
		HTTP2:                   conf.BaseHTTPConfig.HTTP2,
		Includes:                includes,
		NginxReadinessProbePort: conf.BaseHTTPConfig.NginxReadinessProbePort,
		NginxReadinessProbePath: conf.BaseHTTPConfig.NginxReadinessProbePath,
		IPFamily:                getIPFamily(conf.BaseHTTPConfig),
		DNSResolver:             buildDNSResolver(conf.BaseHTTPConfig.DNSResolver),
		AccessLog:               buildAccessLog(conf.Logging.AccessLog),
		GatewaySecretID:         conf.BaseHTTPConfig.GatewaySecretID,
		ServerTokens:            conf.BaseHTTPConfig.ServerTokens,
		OIDCProviders:           buildOIDCProviders(conf.OIDCProviders),
		Compression:             conf.BaseHTTPConfig.Compression,
		WAF:                     conf.WAF.Enabled,
		WAFCookieSeed:           conf.WAF.CookieSeed,
		ClaimSets:               claimSets,
	}

	results := make([]executeResult, 0, len(includes)+1)
	results = append(results, executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(baseHTTPTemplate, hc),
	})
	results = append(results, createIncludeExecuteResults(includes)...)

	return results
}

func buildDNSResolver(dnsResolver *dataplane.DNSResolverConfig) *dataplane.DNSResolverConfig {
	if dnsResolver == nil {
		return nil
	}

	fixed := &dataplane.DNSResolverConfig{
		Timeout:     dnsResolver.Timeout,
		Valid:       dnsResolver.Valid,
		DisableIPv6: dnsResolver.DisableIPv6,
	}

	for _, address := range dnsResolver.Addresses {
		ip := net.ParseIP(address)
		if ip == nil {
			// Not an IP; validateDNSResolver guarantees this is a DNS-1123 hostname.
			// nginx's resolver directive supports hostnames directly.
			fixed.Addresses = append(fixed.Addresses, address)
			continue
		}

		if ip.To4() == nil {
			// nginx expects IPv6 DNS resolvers to be passed with brackets
			fixed.Addresses = append(fixed.Addresses, fmt.Sprintf("[%s]", address))
		} else {
			fixed.Addresses = append(fixed.Addresses, address)
		}
	}

	return fixed
}

// buildOIDCProviders converts a slice of dataplane OIDCProviders to oidcConfiguration pointers.
func buildOIDCProviders(providers []dataplane.OIDCProvider) []*oidcConfiguration {
	if len(providers) == 0 {
		return nil
	}
	result := make([]*oidcConfiguration, 0, len(providers))
	for _, provider := range providers {
		if provider.Name == "" {
			continue
		}
		result = append(result, buildOIDCConfiguration(provider))
	}
	return result
}

// boolToNginxFlag converts a boolean pointer to Nginx acceptable values.
func boolToNginxFlag(v *bool) string {
	if v == nil {
		return ""
	}
	if *v {
		return "on"
	}
	return "off"
}

// buildOIDCConfiguration builds the OIDC configuration for a provider.
func buildOIDCConfiguration(provider dataplane.OIDCProvider) *oidcConfiguration {
	oidc := &oidcConfiguration{
		Name:          provider.Name,
		Issuer:        provider.Issuer,
		ClientID:      provider.ClientID,
		ClientSecret:  provider.ClientSecret,
		RedirectURI:   provider.RedirectURI,
		ExtraAuthArgs: provider.ExtraAuthArgs,
		PKCE:          boolToNginxFlag(provider.PKCE),
		TokenHint:     boolToNginxFlag(provider.TokenHint),
	}
	if provider.CACertBundleID != "" {
		oidc.TrustedCertificatePath = generateCertBundleFileName(provider.CACertBundleID)
	}
	if provider.CRLBundleID != "" {
		oidc.CRLPath = generateCRLBundleFileName(provider.CRLBundleID)
	}
	if provider.ConfigURL != nil {
		oidc.ConfigURL = *provider.ConfigURL
	}
	if provider.CookieName != nil {
		oidc.CookieName = *provider.CookieName
	}
	if provider.Timeout != nil {
		oidc.Timeout = *provider.Timeout
	}
	if provider.LogoutURI != nil {
		oidc.LogoutURI = *provider.LogoutURI
	}
	if provider.PostLogoutURI != nil {
		oidc.PostLogoutURI = *provider.PostLogoutURI
	}
	if provider.FrontChannelLogoutURI != nil {
		oidc.FrontChannelLogoutURI = *provider.FrontChannelLogoutURI
	}
	return oidc
}

func buildAccessLog(accessLogConfig *dataplane.AccessLog) *AccessLog {
	if accessLogConfig != nil {
		accessLog := &AccessLog{
			Path:       dataplane.DefaultAccessLogPath,
			FormatName: dataplane.DefaultLogFormatName,
		}
		if accessLogConfig.Format != "" {
			accessLog.Format = accessLogConfig.Format
		}
		if accessLogConfig.Escape != "" {
			accessLog.Escape = accessLogConfig.Escape
		}
		accessLog.Disable = accessLogConfig.Disable

		return accessLog
	}
	return nil
}

// collectAuthZClaimSets collects all ClaimSets from AuthZConfigs
// and returns a slice sorted by variable name for a deterministic output.
func collectAuthZClaimSets(authZConfigs []*dataplane.AuthZConfig) []ClaimSet {
	if len(authZConfigs) == 0 {
		return nil
	}

	seen := make(map[string][]string)

	for _, cfg := range authZConfigs {
		if cfg == nil {
			continue
		}
		for k, v := range cfg.AuthClaimSets {
			if _, exists := seen[k]; !exists {
				seen[k] = v
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	result := make([]ClaimSet, 0, len(seen))
	for variable, claims := range seen {
		result = append(result, ClaimSet{Variable: variable, Claims: claims})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Variable < result[j].Variable
	})

	return result
}
