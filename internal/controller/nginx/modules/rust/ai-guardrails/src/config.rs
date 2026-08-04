//! Configuration module for guardrails filter directives

/// Module configuration for location context
#[derive(Default)]
pub struct ModuleConfig {
    /// Enable/disable the guardrails filter
    pub enabled: bool,

    /// Guardrails API token, read from the file named by `guardrails_api_token_file`.
    pub api_token: Option<String>,

    /// Path of the file that contains the bearer token (set by guardrails_api_token_file).
    pub api_token_file: Option<String>,

    /// Internal NGINX location URI that proxies to the guardrails backend.
    /// Set by `guardrails_internal_uri`. When present, request inspection is
    /// performed via a non-blocking NGINX subrequest to this location (which
    /// `proxy_pass`es to the guardrails API), instead of a blocking HTTP call.
    ///
    /// Timeouts against the guardrails backend are governed by that internal
    /// location's `proxy_connect_timeout` / `proxy_read_timeout` /
    /// `proxy_send_timeout` (the subrequest inherits them), derived by the
    /// control plane from the `PayloadProcessor` `Timeout`. The module itself
    /// holds no timeout configuration.
    pub internal_uri: Option<String>,
}

/// Error produced while parsing a `guardrails_api_token_file`.
#[derive(Debug, PartialEq, Eq)]
pub enum TokenFileError {
    /// The file was empty or contained only whitespace. This would otherwise
    /// yield an `Authorization: Bearer ` header (empty credential) and cause
    /// every inspection to fail closed, so it is treated as a config error.
    Empty,
}

impl TokenFileError {
    /// Human-readable description for config-load error logging.
    pub fn as_str(&self) -> &'static str {
        match self {
            TokenFileError::Empty => "is empty or whitespace-only",
        }
    }
}

/// Parse the contents of a token file into a non-empty bearer token.
///
/// Surrounding whitespace is trimmed. An empty or whitespace-only file is a
/// configuration error rather than "no auth": only an absent
/// `guardrails_api_token_file` directive means no `Authorization` header.
pub fn parse_token_file_contents(contents: &str) -> Result<String, TokenFileError> {
    let trimmed = contents.trim();
    if trimmed.is_empty() {
        return Err(TokenFileError::Empty);
    }
    Ok(trimmed.to_string())
}

/// Maximum bytes to buffer from a response before failing closed.
///
/// The control plane does not expose this as a directive, so it is a fixed
/// module constant. A response exceeding this cap is blocked rather than
/// buffered unbounded (which could exhaust worker memory).
pub const MAX_RESPONSE_BYTES: usize = 10 * 1024 * 1024;

impl ModuleConfig {
    /// Should we inspect requests?
    ///
    /// Inspection mode is not exposed as a directive; when the filter is
    /// enabled both directions are inspected ("both").
    pub fn inspect_requests(&self) -> bool {
        self.enabled
    }

    /// Should we inspect responses?
    ///
    /// See [`Self::inspect_requests`]: enabling the filter inspects both
    /// directions.
    pub fn inspect_responses(&self) -> bool {
        self.enabled
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_values() {
        let conf = ModuleConfig::default();
        assert!(!conf.enabled);
        assert!(conf.api_token.is_none());
        assert!(conf.api_token_file.is_none());
        assert!(conf.internal_uri.is_none());
    }

    /// Helper to build a config with a given enabled flag.
    fn conf(enabled: bool) -> ModuleConfig {
        ModuleConfig {
            enabled,
            ..ModuleConfig::default()
        }
    }

    #[test]
    fn test_disabled_never_inspects() {
        let c = conf(false);
        assert!(!c.inspect_requests());
        assert!(!c.inspect_responses());
    }

    #[test]
    fn test_enabled_inspects_both_directions() {
        let c = conf(true);
        assert!(c.inspect_requests());
        assert!(c.inspect_responses());
    }

    #[test]
    fn test_parse_token_file_empty_is_error() {
        assert_eq!(parse_token_file_contents(""), Err(TokenFileError::Empty));
    }

    #[test]
    fn test_parse_token_file_whitespace_only_is_error() {
        assert_eq!(
            parse_token_file_contents("   \n\t "),
            Err(TokenFileError::Empty)
        );
    }

    #[test]
    fn test_parse_token_file_trims_surrounding_whitespace() {
        assert_eq!(
            parse_token_file_contents("  sk-abc123\n"),
            Ok("sk-abc123".to_string())
        );
    }

    #[test]
    fn test_parse_token_file_plain_token() {
        assert_eq!(
            parse_token_file_contents("sk-abc123"),
            Ok("sk-abc123".to_string())
        );
    }
}
