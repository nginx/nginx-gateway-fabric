package config

import (
	"fmt"
	"strings"
)

// NGINX Variable names cannot have hyphens.
// This function converts a hyphenated string to an underscored string.
func convertStringToSafeVariableName(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

// generateAddHeaderMapVariableName Generate the variable name for a proxy add header map.
// We have increased the proxy_headers_hash_bucket_size and variables_hash_bucket_size to 512; and
// proxy_headers_hash_max_size and variables_hash_max_size to 1024 to support the longest header name as allowed
// by the schema (256 characters). This ensures NGINX will not fail to reload.
// FIXME(ciarams87): Investigate if any there are any performance related concerns with changing these directives.
// https://github.com/nginx/nginx-gateway-fabric/issues/772
func generateAddHeaderMapVariableName(name string) string {
	return strings.ToLower(convertStringToSafeVariableName(name)) + "_header_var"
}

// generateCORSAllowedOriginVariableName generates the variable name
// for the CORS allowed origin map for a given path and match rule index.
func generateCORSAllowedOriginVariableName(pathRuleIndex, matchRuleIndex int) string {
	return fmt.Sprintf("$cors_allowed_origin_path%d_match%d", pathRuleIndex, matchRuleIndex)
}

// generateCORSAllowCredentialsVariableName generates the variable name
// for the CORS allow credentials map for a given path and match rule index.
func generateCORSAllowCredentialsVariableName(pathRuleIndex, matchRuleIndex int) string {
	return fmt.Sprintf("$cors_allow_credentials_path%d_match%d", pathRuleIndex, matchRuleIndex)
}
