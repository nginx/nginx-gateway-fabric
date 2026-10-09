//! Error types for the inference module.
//!
//! Every error carries an HTTP status code so the caller can translate it
//! directly into an nginx response without a second match.

use std::fmt;

/// HTTP status codes returned on EPP failures.
pub(crate) const STATUS_INTERNAL: u16 = 500;
pub(crate) const STATUS_TIMEOUT: u16 = 504;

/// Errors produced during the EPP request/response codec pipeline.
#[derive(Debug)]
pub(crate) enum ModuleError {
    /// Malformed gRPC length-prefixed frame.
    GrpcFrame(String),
    /// Protobuf decode failure.
    ProtoDecode(String),
    /// EPP returned a response we cannot interpret.
    InvalidResponse(String),
    /// EPP did not provide an endpoint (missing header or metadata).
    NoEndpoint,
    /// EPP sent an `ImmediateResponse` with the given status code.
    /// The code comes from the EPP itself (typically 503 or 429).
    ImmediateResponse(u16),
    /// EPP request timed out.
    Timeout,
    /// Transport-level failure (connection refused, reset, etc.).
    Transport(String),
}

impl ModuleError {
    /// The HTTP status code to return from nginx's ACCESS phase for this error.
    pub(crate) fn status(&self) -> u16 {
        match self {
            // nginx generates a response for these ACCESS return codes. Other
            // 1xx/2xx values finalize without sending a terminal response.
            Self::ImmediateResponse(code @ (201 | 204 | 300..=599)) => *code,
            Self::Timeout => STATUS_TIMEOUT,
            _ => STATUS_INTERNAL,
        }
    }
}

impl fmt::Display for ModuleError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::GrpcFrame(detail) => write!(f, "gRPC frame error: {detail}"),
            Self::ProtoDecode(detail) => write!(f, "protobuf decode error: {detail}"),
            Self::InvalidResponse(detail) => write!(f, "invalid EPP response: {detail}"),
            Self::NoEndpoint => f.write_str("EPP did not return an endpoint"),
            Self::ImmediateResponse(code) => {
                write!(f, "EPP rejected the request (status {code})")
            }
            Self::Timeout => f.write_str("EPP request timed out"),
            Self::Transport(detail) => write!(f, "EPP transport error: {detail}"),
        }
    }
}

impl std::error::Error for ModuleError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn status_codes() {
        assert_eq!(ModuleError::GrpcFrame("x".into()).status(), STATUS_INTERNAL);
        assert_eq!(
            ModuleError::ProtoDecode("x".into()).status(),
            STATUS_INTERNAL
        );
        assert_eq!(
            ModuleError::InvalidResponse("x".into()).status(),
            STATUS_INTERNAL
        );
        assert_eq!(ModuleError::NoEndpoint.status(), STATUS_INTERNAL);
        assert_eq!(ModuleError::ImmediateResponse(503).status(), 503);
        assert_eq!(ModuleError::ImmediateResponse(429).status(), 429);
        assert_eq!(ModuleError::Timeout.status(), STATUS_TIMEOUT);
        assert_eq!(ModuleError::Transport("x".into()).status(), STATUS_INTERNAL);
    }

    #[test]
    fn immediate_response_status_is_safe_for_access_finalization() {
        for code in [0, 100, 103, 199, 200, 202, 206, 299, 600] {
            assert_eq!(
                ModuleError::ImmediateResponse(code).status(),
                STATUS_INTERNAL
            );
        }
        for code in [201, 204, 300, 429, 503, 599] {
            assert_eq!(ModuleError::ImmediateResponse(code).status(), code);
        }
    }

    #[test]
    fn display_messages() {
        assert!(
            ModuleError::GrpcFrame("bad len".into())
                .to_string()
                .contains("bad len")
        );
        assert!(
            ModuleError::ImmediateResponse(429)
                .to_string()
                .contains("429")
        );
        assert!(ModuleError::Timeout.to_string().contains("timed out"));
        assert!(
            ModuleError::NoEndpoint
                .to_string()
                .contains("did not return")
        );
    }
}
