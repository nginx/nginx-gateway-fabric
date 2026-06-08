//! Configuration module for guardrails filter directives

/// Module configuration for location context
pub struct ModuleConfig {
    /// Enable/disable the guardrails filter
    pub enabled: bool,

    /// Guardrails API base URL (the path /backend/v1/scans is appended automatically)
    pub api_url: Option<String>,

    /// Guardrails API token (set either inline via guardrails_api_token or from a
    /// file via guardrails_api_token_file; the file variant is preferred for secret safety).
    pub api_token: Option<String>,

    /// Path of the file that contains the bearer token (set by guardrails_api_token_file).
    pub api_token_file: Option<String>,

    /// Request timeout in milliseconds
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
