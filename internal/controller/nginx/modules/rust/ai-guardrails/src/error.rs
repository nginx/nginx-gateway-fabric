//! Shared error type and constants for the guardrails inspection paths.
//!
//! Both the request path and the response path inspect content via a
//! non-blocking NGINX subrequest (see `subrequest_client.rs`). This module holds
//! the pieces they share so neither path depends on the other.

/// Error returned by the guardrails inspection paths. The caller's fail-closed
/// policy treats any `Err` as "block".
#[derive(Debug)]
pub enum GuardrailsError {
    RequestFailed(String),
    InvalidResponse(String),
}

impl std::fmt::Display for GuardrailsError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::RequestFailed(e) => write!(f, "Request failed: {}", e),
            Self::InvalidResponse(e) => write!(f, "Invalid response: {}", e),
        }
    }
}

impl std::error::Error for GuardrailsError {}

/// User-Agent sent on every guardrails inspection request. Shared by both
/// inspection directions (each issues an NGINX subrequest into the internal
/// guardrails location) so the two paths cannot drift.
///
/// The version suffix is taken from the crate version (`CARGO_PKG_VERSION`) at
/// compile time, so it stays in sync with `Cargo.toml` automatically instead of
/// being hardcoded.
///
/// Some guardrails backends front their API with an edge/WAF (e.g. CloudFront)
/// that rejects requests lacking a User-Agent header with `403 Forbidden`. NGINX's
/// proxy module does not synthesize a default User-Agent for a subrequest, so it
/// must be set explicitly; otherwise inspection fails closed on every request.
pub(crate) const GUARDRAILS_USER_AGENT: &str =
    concat!("nginx-guardrails-filter/", env!("CARGO_PKG_VERSION"));
