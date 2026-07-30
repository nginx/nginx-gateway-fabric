//! Configuration module for guardrails filter directives

/// Module configuration for location context
pub struct ModuleConfig {
    /// Enable/disable the guardrails filter
    pub enabled: bool,

    /// Guardrails API base URL, set by `guardrails_api_url`.
    ///
    /// Retained only so the directive parses; both inspection directions now
    /// reach the backend via the internal subrequest location (`internal_uri`),
    /// not via a direct call from the module. The concrete backend URL lives in
    /// the internal location's `proxy_pass`. Kept for config/debug visibility.
    pub api_url: Option<String>,

    /// Guardrails API token (set either inline via guardrails_api_token or from a
    /// file via guardrails_api_token_file; the file variant is preferred for secret safety).
    pub api_token: Option<String>,

    /// Path of the file that contains the bearer token (set by guardrails_api_token_file).
    pub api_token_file: Option<String>,

    /// Internal NGINX location URI that proxies to the guardrails backend.
    /// Set by `guardrails_internal_uri`. When present, request inspection is
    /// performed via a non-blocking NGINX subrequest to this location (which
    /// `proxy_pass`es to the guardrails API), instead of a blocking HTTP call.
    pub internal_uri: Option<String>,

    /// Request timeout in milliseconds, set by `guardrails_timeout_ms`.
    ///
    /// Retained only so the directive parses. Timeouts against the guardrails
    /// backend are now governed by the internal location's `proxy_*_timeout`
    /// settings (the subrequest inherits them), not by the module.
    pub timeout_ms: u64,
}

/// Maximum bytes to buffer from a response before failing closed.
///
/// The control plane does not expose this as a directive, so it is a fixed
/// module constant. A response exceeding this cap is blocked rather than
/// buffered unbounded (which could exhaust worker memory).
pub const MAX_RESPONSE_BYTES: usize = 10 * 1024 * 1024;

impl Default for ModuleConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            api_url: None,
            api_token: None,
            api_token_file: None,
            internal_uri: None,
            timeout_ms: 5000,
        }
    }
}

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
        assert!(conf.api_url.is_none());
        assert!(conf.api_token.is_none());
        assert!(conf.api_token_file.is_none());
        assert!(conf.internal_uri.is_none());
        assert_eq!(conf.timeout_ms, 5000);
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
}
