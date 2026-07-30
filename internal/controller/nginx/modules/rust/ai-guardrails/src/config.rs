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

    /// Inspect mode: "request", "response", "both", "off"
    pub inspect_mode: String,

    /// Maximum bytes to buffer from the response before failing open.
    /// 0 means unlimited. Default: 10 MB.
    pub max_response_bytes: usize,
}

impl Default for ModuleConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            api_url: None,
            api_token: None,
            api_token_file: None,
            internal_uri: None,
            timeout_ms: 5000,
            inspect_mode: "both".to_string(),
            max_response_bytes: 10 * 1024 * 1024,
        }
    }
}

impl ModuleConfig {
    /// Should we inspect requests?
    pub fn inspect_requests(&self) -> bool {
        self.enabled && (self.inspect_mode == "request" || self.inspect_mode == "both")
    }

    /// Should we inspect responses?
    pub fn inspect_responses(&self) -> bool {
        self.enabled && (self.inspect_mode == "response" || self.inspect_mode == "both")
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
        assert_eq!(conf.inspect_mode, "both");
        assert_eq!(conf.max_response_bytes, 10 * 1024 * 1024);
    }

    /// Helper to build a config with a given enabled flag and inspect mode.
    fn conf(enabled: bool, mode: &str) -> ModuleConfig {
        ModuleConfig {
            enabled,
            inspect_mode: mode.to_string(),
            ..ModuleConfig::default()
        }
    }

    #[test]
    fn test_inspect_matrix_disabled_never_inspects() {
        // When disabled, neither direction is inspected regardless of mode.
        for mode in ["request", "response", "both", "off"] {
            let c = conf(false, mode);
            assert!(!c.inspect_requests(), "requests, mode={mode}");
            assert!(!c.inspect_responses(), "responses, mode={mode}");
        }
    }

    #[test]
    fn test_inspect_matrix_enabled() {
        // (mode, expect_requests, expect_responses)
        let cases = [
            ("request", true, false),
            ("response", false, true),
            ("both", true, true),
            ("off", false, false),
            ("unknown", false, false),
        ];
        for (mode, want_req, want_resp) in cases {
            let c = conf(true, mode);
            assert_eq!(c.inspect_requests(), want_req, "requests, mode={mode}");
            assert_eq!(c.inspect_responses(), want_resp, "responses, mode={mode}");
        }
    }
}
