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
