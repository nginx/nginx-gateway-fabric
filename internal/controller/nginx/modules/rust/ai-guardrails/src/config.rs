//! Configuration module for guardrails filter directives

/// Module configuration for location context
#[derive(Default)]
pub struct ModuleConfig {
    /// Enable/disable the guardrails filter
    pub enabled: bool,

    /// Guardrails API token (set either inline via guardrails_api_token or from a
    /// file via guardrails_api_token_file; the file variant is preferred for secret safety).
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
}
