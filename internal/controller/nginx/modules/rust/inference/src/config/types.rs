//! Module configuration types and validation.
//!
//! `LocationConfig` is the per-location configuration struct for the inference
//! module. It implements `Default` for use with `HttpModuleLocationConf`.
//! Validation runs at config-load time (`nginx -t`) during location merge,
//! so request-time code can assume the config is valid.

use std::ffi::CString;
use std::fmt;
use std::sync::Arc;
use std::time::Duration;

use crate::codec::endpoint::split_host_port;

/// Default EPP timeout (5 seconds).
pub(crate) const DEFAULT_TIMEOUT: Duration = Duration::from_secs(5);

/// Controls how the module handles the EPP response tier.
#[derive(Debug, Default, Clone, Copy, PartialEq, Eq)]
pub(crate) enum ResponsePhase {
    /// Notify EPP without gating the client response or applying mutations.
    /// EPP replies may still be consumed to complete the protocol exchange.
    #[default]
    Notify,
    /// Wait for the EPP's `HeadersResponse` and apply response-header mutations.
    Headers,
}

// ---------------------------------------------------------------------------
// SslContext — shared, pre-built SSL_CTX wrapper
// ---------------------------------------------------------------------------

/// Inner type that owns a raw `SSL_CTX*` and associated config-time TLS
/// parameters. Freed on drop.
struct SslCtxInner {
    ctx: *mut openssl_sys::SSL_CTX,
    /// Pre-converted SNI hostname for the TLS handshake.
    /// Stored as `CString` so per-request code avoids repeated allocation.
    name: Option<CString>,
}

impl Drop for SslCtxInner {
    fn drop(&mut self) {
        if !self.ctx.is_null() {
            unsafe { openssl_sys::SSL_CTX_free(self.ctx) };
        }
    }
}

// SSL_CTX is internally reference-counted and thread-safe.
unsafe impl Send for SslCtxInner {}
unsafe impl Sync for SslCtxInner {}

/// Shared TLS context created at config-load time.
///
/// Wraps an `SSL_CTX` and SNI name in an `Arc` so they can be cheaply cloned
/// into per-request async tasks. Everything is freed when the last
/// `SslContext` is dropped (i.e. on nginx reload when the old config
/// generation is discarded).
#[derive(Clone)]
pub(crate) struct SslContext(Arc<SslCtxInner>);

impl SslContext {
    /// Create a new `SslContext` from TLS configuration parameters.
    ///
    /// Called once per location at config-load time; the resulting `SSL_CTX`
    /// is reused for every EPP connection from this location.
    pub fn new(
        verify: bool,
        trusted_cert: Option<&str>,
        ssl_name: Option<&str>,
    ) -> Result<Self, String> {
        // Convert strings before allocating the SSL_CTX so early returns
        // cannot leak it.
        let ca = trusted_cert
            .map(|p| CString::new(p).map_err(|_| "ssl_trusted_certificate contains NUL byte"))
            .transpose()?;
        let name = ssl_name
            .map(|n| CString::new(n).map_err(|_| "ssl_name contains NUL byte"))
            .transpose()?;

        let ctx = unsafe { openssl_sys::SSL_CTX_new(openssl_sys::TLS_client_method()) };
        if ctx.is_null() {
            return Err("SSL_CTX_new failed".into());
        }
        // From here on, `inner` owns the ctx and frees it on any error path.
        let inner = SslCtxInner { ctx, name };

        // Require TLS 1.2 or newer independently of system OpenSSL defaults
        // and certificate verification mode. Leave the maximum unrestricted.
        if unsafe { openssl_sys::SSL_CTX_set_min_proto_version(ctx, openssl_sys::TLS1_2_VERSION) }
            != 1
        {
            return Err("SSL_CTX_set_min_proto_version(TLS1.2) failed".into());
        }

        let mode = if verify {
            openssl_sys::SSL_VERIFY_PEER
        } else {
            openssl_sys::SSL_VERIFY_NONE
        };
        unsafe { openssl_sys::SSL_CTX_set_verify(ctx, mode, None) };

        if verify && let Some(ca) = &ca {
            let rc = unsafe {
                openssl_sys::SSL_CTX_load_verify_locations(ctx, ca.as_ptr(), core::ptr::null())
            };
            if rc != 1 {
                return Err(format!(
                    "SSL_CTX_load_verify_locations(\"{}\") failed",
                    ca.to_string_lossy()
                ));
            }
        }

        Ok(Self(Arc::new(inner)))
    }

    /// Return a temporary `ngx_ssl_t` referencing the inner `SSL_CTX`.
    ///
    /// The caller must ensure this `SslContext` (or a clone) outlives any
    /// `ngx_ssl_create_connection` calls made with the returned value.
    /// `ngx_ssl_create_connection` increments the SSL_CTX refcount internally,
    /// so the per-connection SSL object is safe even after this borrow ends.
    pub fn as_ngx_ssl(&self) -> nginx_sys::ngx_ssl_t {
        let mut ssl: nginx_sys::ngx_ssl_t = unsafe { core::mem::zeroed() };
        ssl.ctx = self.0.ctx.cast();
        ssl
    }

    /// The SNI hostname for the TLS handshake, if configured.
    pub fn name(&self) -> Option<&std::ffi::CStr> {
        self.0.name.as_deref()
    }
}

impl fmt::Debug for SslContext {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("SslContext")
            .field("ctx", &self.0.ctx)
            .field("name", &self.0.name)
            .finish()
    }
}

/// Per-location configuration for the inference module.
///
/// This struct is the nginx `LocationConf` — one instance per `location {}` block.
/// Fields are populated by directive handlers in `directives.rs`. Scalar
/// settings are `Option` so that "not set" can be distinguished from an
/// explicit value when inheriting from the enclosing block; use the accessor
/// methods to read the effective value.
#[derive(Debug, Clone, Default)]
pub(crate) struct LocationConfig {
    /// `inference_epp on|off`.
    pub enabled: Option<bool>,

    /// EPP gRPC address (`host:port`), set by `inference_epp_endpoint`.
    pub endpoint: Option<String>,

    /// Total timeout for the EPP request phase (`inference_epp_timeout`).
    pub timeout: Option<Duration>,

    /// Verify the EPP certificate (`inference_epp_ssl_verify on|off`).
    /// Requires `ssl_trusted_certificate` to be set.
    pub ssl_verify: Option<bool>,

    /// PEM CA bundle for EPP certificate verification
    /// (`inference_epp_ssl_trusted_certificate`).
    pub ssl_trusted_certificate: Option<String>,

    /// Hostname for SNI and certificate SAN check (`inference_epp_ssl_name`).
    pub ssl_name: Option<String>,

    /// Upstream name for fail-open (`inference_failopen`).
    /// When set, any EPP failure resolves to this upstream instead of
    /// returning an error status.
    pub failopen: Option<String>,

    /// Response processing phase (`inference_epp_response_phase`).
    pub response_phase: Option<ResponsePhase>,

    /// Pre-built TLS context, created at config-load time.
    ///
    /// `Some` when the module is enabled and `ssl()` is true. Shared (via
    /// `Arc`) across all requests to this location.
    pub ssl_ctx: Option<SslContext>,
}

impl LocationConfig {
    /// Whether the module is active for this location.
    pub fn enabled(&self) -> bool {
        self.enabled.unwrap_or(false)
    }

    /// The configured EPP endpoint, or `""` when unset.
    pub fn endpoint(&self) -> &str {
        self.endpoint.as_deref().unwrap_or_default()
    }

    /// The effective EPP timeout.
    pub fn timeout(&self) -> Duration {
        self.timeout.unwrap_or(DEFAULT_TIMEOUT)
    }

    /// Whether the EPP certificate is verified.
    pub fn ssl_verify(&self) -> bool {
        self.ssl_verify.unwrap_or(false)
    }

    /// The effective response phase.
    pub fn response_phase(&self) -> ResponsePhase {
        self.response_phase.unwrap_or_default()
    }

    /// Returns `true` when TLS should be used for the EPP connection.
    ///
    /// TLS is implied by `ssl_trusted_certificate` or `ssl_name`.
    pub fn ssl(&self) -> bool {
        self.ssl_trusted_certificate.is_some() || self.ssl_name.is_some()
    }

    /// Name used for SNI and certificate host verification: the configured
    /// `ssl_name`, else the endpoint host when it is a DNS name (not an IP).
    pub fn tls_server_name(&self) -> Option<&str> {
        if let Some(name) = self.ssl_name.as_deref() {
            return Some(name);
        }
        let (host, _) = split_host_port(self.endpoint()).ok()?;
        (!host.is_empty() && host.parse::<std::net::IpAddr>().is_err()).then_some(host)
    }

    /// Create the shared TLS context from the current SSL parameters.
    ///
    /// Must be called after `validate()` succeeds.
    pub fn init_ssl(&mut self) -> Result<(), ConfigError> {
        if !self.ssl() {
            self.ssl_ctx = None;
            return Ok(());
        }
        let ctx = SslContext::new(
            self.ssl_verify(),
            self.ssl_trusted_certificate.as_deref(),
            self.tls_server_name(),
        )
        .map_err(ConfigError)?;
        self.ssl_ctx = Some(ctx);
        Ok(())
    }

    /// Validate cross-field constraints at config-load time.
    ///
    /// Disabled locations are not validated; directives may be present but
    /// inactive (e.g. set in a parent block for inheritance).
    pub fn validate(&self) -> Result<(), ConfigError> {
        if !self.enabled() {
            return Ok(());
        }

        let endpoint = self.endpoint();
        if endpoint.is_empty() {
            return Err(ConfigError(
                "inference_epp_endpoint must be set when inference_epp is on".into(),
            ));
        }
        if endpoint.contains("://") {
            return Err(ConfigError(format!(
                "inference_epp_endpoint must be host:port without a scheme, got '{endpoint}'"
            )));
        }
        split_host_port(endpoint)
            .map_err(|e| ConfigError(format!("inference_epp_endpoint: {e}")))?;

        if self.timeout().is_zero() {
            return Err(ConfigError("inference_epp_timeout must be > 0".into()));
        }

        if self.ssl_verify() && self.ssl_trusted_certificate.is_none() {
            return Err(ConfigError(
                "inference_epp_ssl_verify requires inference_epp_ssl_trusted_certificate".into(),
            ));
        }

        Ok(())
    }

    /// Inherit unset fields from `prev`, validate, and build the TLS context.
    ///
    /// Returns a descriptive error for the `merge_loc_conf` callback to log.
    pub fn merge_from(&mut self, prev: &Self) -> Result<(), ConfigError> {
        fn inherit<T: Clone>(field: &mut Option<T>, prev: &Option<T>) {
            if field.is_none() {
                field.clone_from(prev);
            }
        }

        // Reuse the parent's TLS context when no TLS parameter is overridden.
        // The endpoint supplies the default server name, so it must match too.
        let tls_inherited = self.endpoint.is_none()
            && self.ssl_verify.is_none()
            && self.ssl_trusted_certificate.is_none()
            && self.ssl_name.is_none();

        inherit(&mut self.enabled, &prev.enabled);
        inherit(&mut self.endpoint, &prev.endpoint);
        inherit(&mut self.timeout, &prev.timeout);
        inherit(&mut self.ssl_verify, &prev.ssl_verify);
        inherit(
            &mut self.ssl_trusted_certificate,
            &prev.ssl_trusted_certificate,
        );
        inherit(&mut self.ssl_name, &prev.ssl_name);
        inherit(&mut self.failopen, &prev.failopen);
        inherit(&mut self.response_phase, &prev.response_phase);

        if !self.enabled() {
            self.ssl_ctx = None;
            return Ok(());
        }

        self.validate()?;

        if tls_inherited && prev.ssl_ctx.is_some() {
            self.ssl_ctx.clone_from(&prev.ssl_ctx);
            return Ok(());
        }
        self.init_ssl()
    }
}

/// A configuration validation error.
#[derive(Debug)]
pub(crate) struct ConfigError(pub String);

impl fmt::Display for ConfigError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for ConfigError {}

#[cfg(test)]
mod tests {
    use super::*;

    fn valid_config() -> LocationConfig {
        LocationConfig {
            enabled: Some(true),
            endpoint: Some("epp.ns.svc.cluster.local:9002".into()),
            ..LocationConfig::default()
        }
    }

    // --- Basic validation ---

    #[test]
    fn valid() {
        valid_config().validate().unwrap();
    }

    #[test]
    fn valid_ipv6() {
        let mut cfg = valid_config();
        cfg.endpoint = Some("[::1]:9002".into());
        cfg.validate().unwrap();
    }

    #[test]
    fn disabled_skips_validation() {
        let cfg = LocationConfig::default();
        // endpoint is empty, but module is disabled — should pass.
        cfg.validate().unwrap();
    }

    #[test]
    fn empty_endpoint_when_enabled() {
        let mut cfg = valid_config();
        cfg.endpoint = None;
        assert!(cfg.validate().is_err());
    }

    #[test]
    fn scheme_http() {
        let mut cfg = valid_config();
        cfg.endpoint = Some("http://epp:9002".into());
        let err = cfg.validate().unwrap_err();
        assert!(err.to_string().contains("scheme"));
    }

    #[test]
    fn scheme_https() {
        let mut cfg = valid_config();
        cfg.endpoint = Some("https://epp:9002".into());
        assert!(cfg.validate().is_err());
    }

    #[test]
    fn no_port() {
        let mut cfg = valid_config();
        cfg.endpoint = Some("epp.ns.svc.cluster.local".into());
        assert!(cfg.validate().is_err());
    }

    #[test]
    fn ipv6_no_port() {
        let mut cfg = valid_config();
        cfg.endpoint = Some("[::1]".into());
        assert!(cfg.validate().is_err());
    }

    #[test]
    fn zero_timeout() {
        let mut cfg = valid_config();
        cfg.timeout = Some(Duration::ZERO);
        assert!(cfg.validate().is_err());
    }

    #[test]
    fn default_timeout_is_5s() {
        let cfg = LocationConfig::default();
        assert_eq!(cfg.timeout(), DEFAULT_TIMEOUT);
    }

    // --- SSL cross-field validation ---

    #[test]
    fn ssl_context_requires_tls12_with_or_without_verification() {
        for verify in [false, true] {
            let ctx = SslContext::new(verify, None, None).unwrap();
            assert_eq!(
                unsafe { openssl_sys::SSL_CTX_get_min_proto_version(ctx.0.ctx) },
                openssl_sys::TLS1_2_VERSION,
            );
            assert_eq!(
                unsafe { openssl_sys::SSL_CTX_get_max_proto_version(ctx.0.ctx) },
                0,
            );
        }
    }

    #[test]
    fn ssl_verify_requires_trusted_cert() {
        let mut cfg = valid_config();
        cfg.ssl_verify = Some(true);
        // ssl_trusted_certificate is None
        let err = cfg.validate().unwrap_err();
        assert!(err.to_string().contains("trusted_certificate"));
    }

    #[test]
    fn ssl_derived_from_sub_directives() {
        let cfg = valid_config();
        assert!(
            !cfg.ssl(),
            "ssl() should be false with no SSL sub-directives"
        );

        let mut cfg2 = valid_config();
        cfg2.ssl_verify = Some(true);
        assert!(
            !cfg2.ssl(),
            "ssl() should be false with only ssl_verify (no cert or name)"
        );

        let mut cfg3 = valid_config();
        cfg3.ssl_trusted_certificate = Some("/path/to/ca.crt".into());
        assert!(
            cfg3.ssl(),
            "ssl() should be true when ssl_trusted_certificate is set"
        );

        let mut cfg4 = valid_config();
        cfg4.ssl_name = Some("epp.example.com".into());
        assert!(cfg4.ssl(), "ssl() should be true when ssl_name is set");
    }

    #[test]
    fn valid_ssl_with_verify() {
        let mut cfg = valid_config();
        cfg.ssl_verify = Some(true);
        cfg.ssl_trusted_certificate = Some("/path/to/ca.crt".into());
        cfg.ssl_name = Some("epp.example.com".into());
        cfg.validate().unwrap();
    }

    // --- Failopen ---

    #[test]
    fn valid_with_failopen() {
        let mut cfg = valid_config();
        cfg.failopen = Some("pool_upstream".into());
        cfg.validate().unwrap();
    }

    #[test]
    fn unbracketed_ipv6_rejected() {
        let mut cfg = valid_config();
        cfg.endpoint = Some("::1:9002".into());
        assert!(cfg.validate().is_err());
    }

    // --- Merge ---

    #[test]
    fn merge_inherits_unset_fields() {
        let parent = LocationConfig {
            timeout: Some(Duration::from_secs(1)),
            response_phase: Some(ResponsePhase::Headers),
            failopen: Some("fallback".into()),
            ..valid_config()
        };
        let mut child = LocationConfig::default();
        child.merge_from(&parent).unwrap();
        assert!(child.enabled());
        assert_eq!(child.endpoint(), "epp.ns.svc.cluster.local:9002");
        assert_eq!(child.timeout(), Duration::from_secs(1));
        assert_eq!(child.response_phase(), ResponsePhase::Headers);
        assert_eq!(child.failopen.as_deref(), Some("fallback"));
    }

    #[test]
    fn merge_explicit_default_overrides_parent() {
        let parent = LocationConfig {
            timeout: Some(Duration::from_secs(1)),
            response_phase: Some(ResponsePhase::Headers),
            ..valid_config()
        };
        let mut child = LocationConfig {
            enabled: Some(false),
            timeout: Some(DEFAULT_TIMEOUT),
            response_phase: Some(ResponsePhase::Notify),
            ..LocationConfig::default()
        };
        child.merge_from(&parent).unwrap();
        assert!(!child.enabled());
        assert_eq!(child.timeout(), DEFAULT_TIMEOUT);
        assert_eq!(child.response_phase(), ResponsePhase::Notify);
    }

    #[test]
    fn merge_reports_validation_error() {
        let mut child = LocationConfig {
            enabled: Some(true),
            ..LocationConfig::default()
        };
        let err = child.merge_from(&LocationConfig::default()).unwrap_err();
        assert!(err.to_string().contains("inference_epp_endpoint"));
    }

    #[test]
    fn merge_disabled_skips_validation() {
        let mut child = LocationConfig {
            endpoint: Some("bad".into()),
            ..LocationConfig::default()
        };
        child.merge_from(&LocationConfig::default()).unwrap();
    }

    // --- TLS server name ---

    #[test]
    fn tls_server_name_defaults_to_dns_endpoint_host() {
        let mut cfg = valid_config();
        assert_eq!(cfg.tls_server_name(), Some("epp.ns.svc.cluster.local"));
        cfg.ssl_name = Some("override.example".into());
        assert_eq!(cfg.tls_server_name(), Some("override.example"));
        for ip in ["10.0.0.1:9002", "[::1]:9002"] {
            cfg.ssl_name = None;
            cfg.endpoint = Some(ip.into());
            assert_eq!(cfg.tls_server_name(), None, "{ip}");
        }
    }

    #[test]
    fn ssl_context_name_follows_inherited_endpoint_override() {
        // Verification off: the CA path is not loaded.
        let mut parent = valid_config();
        parent.ssl_trusted_certificate = Some("/unused/ca.pem".into());
        parent.merge_from(&LocationConfig::default()).unwrap();
        let parent_ctx = parent.ssl_ctx.as_ref().unwrap();
        assert_eq!(
            parent_ctx.name().unwrap().to_str().unwrap(),
            "epp.ns.svc.cluster.local"
        );

        let mut child = LocationConfig {
            endpoint: Some("other.example:9002".into()),
            ..LocationConfig::default()
        };
        child.merge_from(&parent).unwrap();
        let child_ctx = child.ssl_ctx.as_ref().unwrap();
        assert!(!Arc::ptr_eq(&child_ctx.0, &parent_ctx.0));
        assert_eq!(child_ctx.name().unwrap().to_str().unwrap(), "other.example");
    }
}
