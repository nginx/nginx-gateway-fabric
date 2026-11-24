package config

import (
	"fmt"
	"strings"
)

// safeVariableNameReplacer escapes characters that are valid in Kubernetes names but
// terminate an NGINX variable name.  The mapping is injective: hyphens and dots are
// replaced with distinct uppercase-letter sequences (_H and _D respectively) that
// cannot appear in the lowercase Kubernetes-name input, so names like "a-b" and "a.b"
// never collide.
var safeVariableNameReplacer = strings.NewReplacer("-", "_H", ".", "_D")

// convertStringToSafeVariableName converts hyphens and dots to distinct
// underscore-prefixed tokens, since NGINX variable names cannot contain
// hyphens or dots.
func convertStringToSafeVariableName(s string) string {
	return safeVariableNameReplacer.Replace(s)
}

// httpHeaderVariableName converts an HTTP header name to the NGINX $http_<name>
// convention: lowercase with hyphens replaced by underscores.  This must match
// NGINX's own mapping so that $http_<result> resolves correctly.
func httpHeaderVariableName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "-", "_"))
}

// generateAddHeaderMapVariableName generates the variable name for a proxy add header map.
// We have increased the proxy_headers_hash_bucket_size to 512 and variables_hash_bucket_size to 1024; and
// proxy_headers_hash_max_size and variables_hash_max_size to 1024 to support the longest variable names.
// A worst-case inference split variable (63-char namespace + 253-char route name, all hyphens
// expanded to "_H") reaches ~515 characters, so the bucket size must be at least 1024.
// FIXME(ciarams87): Investigate if any there are any performance related concerns with changing these directives.
// https://github.com/nginx/nginx-gateway-fabric/issues/772
func generateAddHeaderMapVariableName(name string) string {
	return httpHeaderVariableName(name) + "_header_var"
}

func generateCORSAllowedOriginVariableName(serverID string, pathRuleIndex, matchRuleIndex int) string {
	return fmt.Sprintf("$cors_allowed_origin_server%s_path%d_match%d", serverID, pathRuleIndex, matchRuleIndex)
}

func generateCORSAllowCredentialsVariableName(serverID string, pathRuleIndex, matchRuleIndex int) string {
	return fmt.Sprintf("$cors_allow_credentials_server%s_path%d_match%d", serverID, pathRuleIndex, matchRuleIndex)
}

// preRoutingVarPrefix is the prefix for all NGINX variables backing pre-routing
// (listener-level ExternalAuth) filters.
const preRoutingVarPrefix = "$ngf_pf"

// Pre-routing variables use positional indices (f<i> filter, h<j> header) rather than header
// names: header names collide under the lowercase/hyphen-to-underscore NGINX variable mapping
// (e.g. "X-A-B" vs "X_A_B"), while (filterIdx, headerIdx) pairs from
// preRoutingResponseHeaderNames are unique by construction.

// generatePreRoutingFilterVarName generates the per-filter variable that captures auth response
// header headerIdx for filter filterIdx on the given server.
func generatePreRoutingFilterVarName(serverID string, filterIdx, headerIdx int) string {
	return fmt.Sprintf("%s_%s_f%d_h%d", preRoutingVarPrefix, serverID, filterIdx, headerIdx)
}

// generatePreRoutingMergeVarName generates the intermediate map variable that merges the
// per-filter values of header headerIdx up to (and including) filter upToIdx.
func generatePreRoutingMergeVarName(serverID string, headerIdx, upToIdx int) string {
	return fmt.Sprintf("%s_%s_h%d_m%d", preRoutingVarPrefix, serverID, headerIdx, upToIdx)
}

// generatePreRoutingMergedVarName generates the final merged variable for header headerIdx on
// the given server. It must only be read after the last filter has run.
func generatePreRoutingMergedVarName(serverID string, headerIdx int) string {
	return fmt.Sprintf("%s_%s_h%d", preRoutingVarPrefix, serverID, headerIdx)
}

// generatePreRoutingAuthPath generates the internal auth_request location path for a filter.
func generatePreRoutingAuthPath(serverID string, filterIdx int) string {
	return fmt.Sprintf("/_ngf_pf_%s_auth%d", serverID, filterIdx)
}

// generatePreRoutingChainName generates the named location that runs filter filterIdx of the
// pre-routing chain for the chain identified by chainID. Chain locations are per chain because
// their next hop is chain-specific: one chain per path rule, plus a catch-all chain.
func generatePreRoutingChainName(serverID, chainID string, filterIdx int) string {
	return fmt.Sprintf("@ngf_pf_%s_f%d_pr%s", serverID, filterIdx, chainID)
}

// generatePreRoutingMatchName generates the named location that ends the pre-routing chain for a
// path rule and hands off to the njs route matcher.
func generatePreRoutingMatchName(serverID, chainID string) string {
	return fmt.Sprintf("@ngf_pf_%s_match_pr%s", serverID, chainID)
}

// catchAllChainID identifies the pre-routing chain that runs for requests whose path matches no
// route. It cannot collide with a path-rule chain, which uses the rule index.
const catchAllChainID = "default"

// generatePreRoutingNotFoundName generates the named location that ends the catch-all chain with a
// 404. It must be a separate location because NGINX evaluates return before auth_request.
func generatePreRoutingNotFoundName(serverID string) string {
	return fmt.Sprintf("@ngf_pf_%s_notfound", serverID)
}
