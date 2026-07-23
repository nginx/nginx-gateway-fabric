//! Guardrails API client — synchronous blocking implementation.
//!
//! Uses `minreq`, a small blocking HTTP client, which avoids any async-runtime
//! re-entrancy issues when called from NGINX's single-threaded event loop.
//! `minreq` is deliberately chosen over `ureq`/`reqwest` because it does not
//! depend on the `url` crate, and therefore avoids the idna -> ICU4X
//! (Unicode-3.0) transitive dependency stack that the dependency-review policy
//! rejects. TLS is provided by rustls verified against the operating system's
//! native trust store (via `rustls-native-certs`, enabled by the
//! `https-rustls-probe` feature), so no bundled root certificate crate is
//! required — the runtime image must ship `ca-certificates`.

use serde::{Deserialize, Serialize};

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

/// Classify an I/O error as a timeout. minreq surfaces a request timeout as an
/// `io::Error` with kind `TimedOut` (or `WouldBlock` on some platforms), so we
/// treat either as a timeout to preserve the caller's fail-closed behaviour.
fn is_timeout(e: &std::io::Error) -> bool {
    matches!(
        e.kind(),
        std::io::ErrorKind::TimedOut | std::io::ErrorKind::WouldBlock
    )
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

    // minreq's timeout is expressed in whole seconds, whereas the public
    // `guardrails_timeout_ms` API is millisecond-granular. Round up to the
    // nearest second (never below 1s) so a configured timeout is never silently
    // truncated to zero. This is a deliberate, documented precision reduction.
    let timeout_secs = timeout_ms.div_ceil(1000).max(1);

    let mut req = minreq::post(&endpoint)
        .with_timeout(timeout_secs)
        .with_header("Content-Type", "application/json")
        .with_header("User-Agent", "nginx-guardrails-filter/0.1.0");

    if let Some(token) = api_token {
        req = req.with_header("Authorization", format!("Bearer {}", token));
    }

    let req = req
        .with_json(&request_body)
        .map_err(|e| GuardrailsError::RequestFailed(e.to_string()))?;

    // Unlike ureq, minreq does not treat a non-2xx status as an error; we check
    // the status code explicitly below. Transport-level failures (including
    // timeouts, which surface as an I/O error) are mapped so the caller's
    // fail-closed logic behaves identically to the previous implementation.
    let response = match req.send() {
        Ok(r) => r,
        Err(minreq::Error::IoError(e)) if is_timeout(&e) => {
            eprintln!("[guardrails] API request timeout after {}ms", timeout_ms);
            let _ = std::io::Write::flush(&mut std::io::stderr());
            return Err(GuardrailsError::Timeout);
        }
        Err(e) => {
            eprintln!("[guardrails] API request failed: {}", e);
            let _ = std::io::Write::flush(&mut std::io::stderr());
            return Err(GuardrailsError::RequestFailed(e.to_string()));
        }
    };

    let status = response.status_code;
    eprintln!("[guardrails] API response: status={}", status);
    let _ = std::io::Write::flush(&mut std::io::stderr());

    if !(200..300).contains(&status) {
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
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::sync::mpsc;
    use std::thread;

    const TIMEOUT_MS: u64 = 5000;

    /// A captured HTTP request, split into the raw head (request line + headers)
    /// and the decoded body.
    struct CapturedRequest {
        head: String,
        body: String,
    }

    impl CapturedRequest {
        /// Case-insensitive check for the presence of a header line.
        fn has_header_line(&self, needle: &str) -> bool {
            let needle = needle.to_ascii_lowercase();
            self.head
                .lines()
                .any(|l| l.to_ascii_lowercase().starts_with(&needle))
        }
    }

    /// A one-shot mock HTTP/1.1 server.
    ///
    /// Binds an ephemeral loopback port, accepts a single connection on a
    /// background thread, reads the full request (honouring `Content-Length`),
    /// replies with the canned response, and returns the captured request over
    /// a channel. This replaces the `httpmock` dev-dependency (which pulled in
    /// license-incompatible transitive crates) with the standard library only.
    fn mock_once(
        status_line: &'static str,
        content_type: &'static str,
        resp_body: &'static str,
    ) -> (String, mpsc::Receiver<CapturedRequest>) {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback");
        let port = listener.local_addr().unwrap().port();
        let (tx, rx) = mpsc::channel();

        thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("accept connection");

            // Read until we have the full header block, then the body based on
            // the Content-Length header.
            let mut raw = Vec::new();
            let mut chunk = [0u8; 1024];
            let header_end = loop {
                let n = stream.read(&mut chunk).expect("read request");
                if n == 0 {
                    break raw.len();
                }
                raw.extend_from_slice(&chunk[..n]);
                if let Some(pos) = find_subslice(&raw, b"\r\n\r\n") {
                    break pos + 4;
                }
            };

            let head = String::from_utf8_lossy(&raw[..header_end.min(raw.len())]).to_string();
            let content_length = parse_content_length(&head);

            // Read any remaining body bytes not already buffered.
            let mut body_bytes = raw[header_end.min(raw.len())..].to_vec();
            while body_bytes.len() < content_length {
                let n = stream.read(&mut chunk).expect("read body");
                if n == 0 {
                    break;
                }
                body_bytes.extend_from_slice(&chunk[..n]);
            }
            let body = String::from_utf8_lossy(&body_bytes).to_string();

            let response = format!(
                "{status_line}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{resp_body}",
                resp_body.len(),
            );
            let _ = stream.write_all(response.as_bytes());
            let _ = stream.flush();

            let _ = tx.send(CapturedRequest { head, body });
        });

        (format!("http://127.0.0.1:{port}"), rx)
    }

    fn find_subslice(haystack: &[u8], needle: &[u8]) -> Option<usize> {
        haystack.windows(needle.len()).position(|w| w == needle)
    }

    fn parse_content_length(head: &str) -> usize {
        head.lines()
            .find_map(|l| {
                let (k, v) = l.split_once(':')?;
                if k.trim().eq_ignore_ascii_case("content-length") {
                    v.trim().parse::<usize>().ok()
                } else {
                    None
                }
            })
            .unwrap_or(0)
    }

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
        let (base, rx) = mock_once(
            "HTTP/1.1 200 OK",
            "application/json",
            r#"{"result":{"outcome":"cleared"}}"#,
        );

        let result = inspect_content("hello", &base, None, TIMEOUT_MS);

        let req = rx.recv().unwrap();
        assert!(req.head.starts_with("POST /backend/v1/scans"));
        assert!(result.unwrap());
    }

    #[test]
    fn test_flagged_outcome_returns_false() {
        let (base, rx) = mock_once(
            "HTTP/1.1 200 OK",
            "application/json",
            r#"{"result":{"outcome":"flagged"}}"#,
        );

        let result = inspect_content("hello", &base, None, TIMEOUT_MS);

        rx.recv().unwrap();
        assert!(!result.unwrap());
    }

    #[test]
    fn test_auth_token_sets_bearer_header() {
        let (base, rx) = mock_once(
            "HTTP/1.1 200 OK",
            "application/json",
            r#"{"result":{"outcome":"cleared"}}"#,
        );

        let result = inspect_content("hello", &base, Some("secret"), TIMEOUT_MS);

        let req = rx.recv().unwrap();
        assert!(req.has_header_line("authorization: bearer secret"));
        assert!(result.is_ok());
    }

    #[test]
    fn test_no_auth_token_omits_authorization_header() {
        let (base, rx) = mock_once(
            "HTTP/1.1 200 OK",
            "application/json",
            r#"{"result":{"outcome":"cleared"}}"#,
        );

        let result = inspect_content("hello", &base, None, TIMEOUT_MS);

        let req = rx.recv().unwrap();
        assert!(!req.has_header_line("authorization:"));
        assert!(result.is_ok());
    }

    #[test]
    fn test_request_body_shape() {
        let (base, rx) = mock_once(
            "HTTP/1.1 200 OK",
            "application/json",
            r#"{"result":{"outcome":"cleared"}}"#,
        );

        let result = inspect_content("hello", &base, None, TIMEOUT_MS);

        let req = rx.recv().unwrap();
        let sent: serde_json::Value = serde_json::from_str(&req.body).expect("body is JSON");
        assert_eq!(
            sent,
            serde_json::json!({
                "input": "hello",
                "configOverrides": {},
                "forceEnabled": [],
                "disabled": [],
                "verbose": false,
            })
        );
        assert!(result.is_ok());
    }

    #[test]
    fn test_non_success_status_is_invalid_response() {
        let (base, rx) = mock_once("HTTP/1.1 500 Internal Server Error", "text/plain", "boom");

        let result = inspect_content("hello", &base, None, TIMEOUT_MS);

        rx.recv().unwrap();
        assert!(matches!(result, Err(GuardrailsError::InvalidResponse(_))));
    }

    #[test]
    fn test_malformed_body_is_invalid_response() {
        let (base, rx) = mock_once("HTTP/1.1 200 OK", "application/json", "not json");

        let result = inspect_content("hello", &base, None, TIMEOUT_MS);

        rx.recv().unwrap();
        assert!(matches!(result, Err(GuardrailsError::InvalidResponse(_))));
    }
}
