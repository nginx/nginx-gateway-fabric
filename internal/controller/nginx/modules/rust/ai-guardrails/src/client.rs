//! Guardrails API client — synchronous blocking implementation.
//!
//! Using `reqwest::blocking` avoids any Tokio `block_on` re-entrancy issues
//! when called from NGINX's single-threaded event loop.

use once_cell::sync::Lazy;
use serde::{Deserialize, Serialize};
use std::time::Duration;

/// Shared blocking HTTP client. A single instance is reused across all
/// checkpoint calls so the connection pool is not exhausted.
static HTTP_CLIENT: Lazy<reqwest::blocking::Client> = Lazy::new(|| {
    reqwest::blocking::Client::builder()
        .pool_max_idle_per_host(10)
        .pool_idle_timeout(Duration::from_secs(90))
        .tcp_keepalive(Duration::from_secs(60))
        .build()
        .expect("Failed to create blocking HTTP client")
});

#[derive(Debug)]
pub enum GuardrailsError {
    RequestFailed(String),
    InvalidResponse(String),
    Timeout,
}

impl std::fmt::Display for GuardrailsError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::RequestFailed(e) => write!(f, "Request failed: {}", e),
            Self::InvalidResponse(e) => write!(f, "Invalid response: {}", e),
            Self::Timeout => write!(f, "Request timeout"),
        }
    }
}

impl std::error::Error for GuardrailsError {}

/// Path appended to the configured API base URL to reach the scan endpoint.
const SCANS_PATH: &str = "/backend/v1/scans";

/// Join the configured API base URL with the scan endpoint path, trimming any
/// trailing slash on the base URL so the result never contains a double slash.
fn build_endpoint(api_url: &str) -> String {
    format!("{}{}", api_url.trim_end_matches('/'), SCANS_PATH)
}

#[derive(Serialize)]
struct GuardrailsRequest<'a> {
    input: &'a str,
    #[serde(rename = "configOverrides")]
    config_overrides: serde_json::Value,
    #[serde(rename = "forceEnabled")]
    force_enabled: Vec<String>,
    disabled: Vec<String>,
    verbose: bool,
}

#[derive(Deserialize)]
struct GuardrailsResponse {
    result: GuardrailsResult,
}

#[derive(Deserialize)]
struct GuardrailsResult {
    outcome: String,
    #[serde(default)]
    details: Option<serde_json::Value>,
}

/// Synchronously inspect content using the Guardrails API.
///
/// Returns `Ok(true)` when cleared, `Ok(false)` when flagged/blocked.
/// Blocks the calling thread until the API responds or the timeout elapses.
pub fn inspect_content(
    content: &str,
    api_url: &str,
    api_token: Option<&str>,
    timeout_ms: u64,
) -> Result<bool, GuardrailsError> {
    let endpoint = build_endpoint(api_url);
    let request_body = GuardrailsRequest {
        input: content,
        config_overrides: serde_json::json!({}),
        force_enabled: vec![],
        disabled: vec![],
        verbose: false,
    };

    // Log outgoing request details with content preview
    let content_preview = if content.len() > 200 {
        format!("{}... (truncated)", &content[..200])
    } else {
        content.to_string()
    };

    eprintln!("[guardrails] Sending API request:");
    eprintln!("  URL: {}", endpoint);
    eprintln!("  Content length: {} chars", content.len());
    eprintln!("  Content preview: {}", content_preview);

    // Log the full JSON request body
    if let Ok(json_str) = serde_json::to_string_pretty(&request_body) {
        eprintln!("  Request JSON:\n{}", json_str);
    }
    let _ = std::io::Write::flush(&mut std::io::stderr());

    let mut req = HTTP_CLIENT
        .post(endpoint)
        .timeout(Duration::from_millis(timeout_ms))
        .header("Content-Type", "application/json")
        .header("User-Agent", "nginx-guardrails-filter/0.1.0")
        .json(&request_body);

    if let Some(token) = api_token {
        req = req.header("Authorization", format!("Bearer {}", token));
    }

    let response = req.send().map_err(|e| {
        if e.is_timeout() {
            eprintln!("[guardrails] API request timeout after {}ms", timeout_ms);
            GuardrailsError::Timeout
        } else {
            eprintln!("[guardrails] API request failed: {}", e);
            GuardrailsError::RequestFailed(e.to_string())
        }
    })?;

    let status = response.status();
    eprintln!("[guardrails] API response: status={}", status);
    let _ = std::io::Write::flush(&mut std::io::stderr());

    if !status.is_success() {
        return Err(GuardrailsError::InvalidResponse(format!(
            "Status: {}",
            status
        )));
    }

    let guardrails_response: GuardrailsResponse = response
        .json()
        .map_err(|e| GuardrailsError::InvalidResponse(e.to_string()))?;

    let cleared = guardrails_response.result.outcome == "cleared";

    // Log the response details
    eprintln!("  Outcome: {}", guardrails_response.result.outcome);
    eprintln!("  Cleared: {}", cleared);
    if let Some(ref details) = guardrails_response.result.details {
        eprintln!("  Details: {}", details);
    }
    let _ = std::io::Write::flush(&mut std::io::stderr());

    Ok(cleared)
}

#[cfg(test)]
mod tests {
    use super::*;
    use httpmock::prelude::*;

    const TIMEOUT_MS: u64 = 5000;

    #[test]
    fn test_build_endpoint_joins_path() {
        assert_eq!(
            build_endpoint("http://host:8080"),
            "http://host:8080/backend/v1/scans"
        );
    }

    #[test]
    fn test_build_endpoint_trims_trailing_slash() {
        assert_eq!(
            build_endpoint("http://host:8080/"),
            "http://host:8080/backend/v1/scans"
        );
    }

    #[test]
    fn test_cleared_outcome_returns_true() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST).path(SCANS_PATH);
            then.status(200)
                .header("content-type", "application/json")
                .json_body(serde_json::json!({"result": {"outcome": "cleared"}}));
        });

        let result = inspect_content("hello", &server.base_url(), None, TIMEOUT_MS);

        mock.assert();
        assert!(result.unwrap());
    }

    #[test]
    fn test_flagged_outcome_returns_false() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST).path(SCANS_PATH);
            then.status(200)
                .header("content-type", "application/json")
                .json_body(serde_json::json!({"result": {"outcome": "flagged"}}));
        });

        let result = inspect_content("hello", &server.base_url(), None, TIMEOUT_MS);

        mock.assert();
        assert!(!result.unwrap());
    }

    #[test]
    fn test_auth_token_sets_bearer_header() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST)
                .path(SCANS_PATH)
                .header("Authorization", "Bearer secret");
            then.status(200)
                .json_body(serde_json::json!({"result": {"outcome": "cleared"}}));
        });

        let result = inspect_content("hello", &server.base_url(), Some("secret"), TIMEOUT_MS);

        mock.assert();
        assert!(result.is_ok());
    }

    #[test]
    fn test_no_auth_token_omits_authorization_header() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST).path(SCANS_PATH).matches(|req| {
                req.headers
                    .as_ref()
                    .map(|h| {
                        !h.iter()
                            .any(|(k, _)| k.eq_ignore_ascii_case("authorization"))
                    })
                    .unwrap_or(true)
            });
            then.status(200)
                .json_body(serde_json::json!({"result": {"outcome": "cleared"}}));
        });

        let result = inspect_content("hello", &server.base_url(), None, TIMEOUT_MS);

        mock.assert();
        assert!(result.is_ok());
    }

    #[test]
    fn test_request_body_shape() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST)
                .path(SCANS_PATH)
                .json_body(serde_json::json!({
                    "input": "hello",
                    "configOverrides": {},
                    "forceEnabled": [],
                    "disabled": [],
                    "verbose": false,
                }));
            then.status(200)
                .json_body(serde_json::json!({"result": {"outcome": "cleared"}}));
        });

        let result = inspect_content("hello", &server.base_url(), None, TIMEOUT_MS);

        mock.assert();
        assert!(result.is_ok());
    }

    #[test]
    fn test_non_success_status_is_invalid_response() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST).path(SCANS_PATH);
            then.status(500);
        });

        let result = inspect_content("hello", &server.base_url(), None, TIMEOUT_MS);

        mock.assert();
        assert!(matches!(result, Err(GuardrailsError::InvalidResponse(_))));
    }

    #[test]
    fn test_malformed_body_is_invalid_response() {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.method(POST).path(SCANS_PATH);
            then.status(200)
                .header("content-type", "application/json")
                .body("not json");
        });

        let result = inspect_content("hello", &server.base_url(), None, TIMEOUT_MS);

        mock.assert();
        assert!(matches!(result, Err(GuardrailsError::InvalidResponse(_))));
    }
}
