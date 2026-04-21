//! Module configuration stored in NGINX location context.
//!
//! This configuration is set by NGINX directives:
//! - `inference_epp on|off`
//! - `inference_epp_endpoint <host:port>`
//! - `inference_failopen <upstream_name>` — enables failopen; on EPP failure, `$inference_endpoint`
//!   resolves to this upstream so proxy_pass falls back to it instead of returning 502.
//! - `inference_epp_tls on|off` — enable TLS for EPP gRPC connection
//! - `inference_epp_tls_skip_verify on|off` — skip server certificate verification

/// Default timeout for EPP gRPC calls in milliseconds.
pub const EPP_TIMEOUT_MS: u64 = 5000;

/// Per-location configuration for the inference module.
#[derive(Debug, Clone, Default)]
#[repr(C)]
pub struct ModuleConfig {
    /// Whether EPP is enabled for this location.
    pub enable: bool,
    /// EPP gRPC endpoint (host:port).
    pub epp_endpoint: Option<String>,
    /// If set, enables failopen: on EPP failure, `$inference_endpoint` resolves to this
    /// upstream name so proxy_pass falls back to it. If None, EPP failure returns 502.
    pub failopen: Option<String>,
    /// Whether to use TLS when connecting to the EPP server.
    pub use_tls: bool,
    /// Whether to skip server certificate verification when using TLS.
    pub tls_skip_verify: bool,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_config() {
        let cfg = ModuleConfig::default();
        assert!(!cfg.enable);
        assert!(cfg.epp_endpoint.is_none());
        assert!(cfg.failopen.is_none());
        assert!(!cfg.use_tls);
        assert!(!cfg.tls_skip_verify);
    }

    #[test]
    fn epp_timeout_is_reasonable() {
        const {
            assert!(
                EPP_TIMEOUT_MS >= 1000,
                "EPP timeout should be at least 1 second"
            );
            assert!(
                EPP_TIMEOUT_MS <= 30000,
                "EPP timeout should be at most 30 seconds"
            );
        }
    }

    #[test]
    fn config_clone_preserves_values() {
        let cfg = ModuleConfig {
            enable: true,
            epp_endpoint: Some("epp.default:9002".to_string()),
            failopen: Some("upstream-fallback".to_string()),
            use_tls: true,
            tls_skip_verify: false,
        };
        let cloned = cfg.clone();
        assert_eq!(cloned.enable, cfg.enable);
        assert_eq!(cloned.epp_endpoint, cfg.epp_endpoint);
        assert_eq!(cloned.failopen, cfg.failopen);
        assert_eq!(cloned.use_tls, cfg.use_tls);
        assert_eq!(cloned.tls_skip_verify, cfg.tls_skip_verify);
    }

    #[test]
    fn config_debug_output() {
        let cfg = ModuleConfig::default();
        let debug = format!("{cfg:?}");
        assert!(debug.contains("ModuleConfig"));
        assert!(debug.contains("enable"));
    }
}
