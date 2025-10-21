use serde_json::Value;

/// get_destination_endpoint
///
/// POC stub for EPP gRPC client logic. This will be replaced with a tonic-based
/// ext_proc client that connects to the Endpoint Picker and streams request headers/body.
///
/// Temporary behavior:
/// - If the `test-epp-endpoint-selection` header is present in `headers`,
///   parse it as a comma-separated list and return the first `ip:port`.
/// - Else, return an error indicating the real client is not yet implemented.
pub async fn get_destination_endpoint(
    host: &str,
    port: &str,
    method: &str,
    headers: Value,
    body: &[u8],
) -> Result<String, String> {
    // Try to read the special test header to simulate EPP response.
    if let Some(selection) = find_header(&headers, "test-epp-endpoint-selection") {
        // "10.0.0.1:8080,10.0.0.2:8080"
        let first = selection
            .split(',')
            .map(|s| s.trim())
            .filter(|s| !s.is_empty())
            .next()
            .ok_or_else(|| "empty test-epp-endpoint-selection header".to_string())?;
        return Ok(first.to_string());
    }

    // Optional future: Extract hints from body (e.g., model name) to pass to EPP.
    let _ = (method, body);

    Err(format!(
        "EPP client not implemented yet (tonic ext_proc). target={}:{}",
        host, port
    ))
}

/// Find a header (case-insensitive) from a JSON object of headers.
///
/// Expected input format (example):
/// {
///   "content-type": "application/json",
///   "test-epp-endpoint-selection": "10.0.0.1:8080,10.0.0.2:8080"
/// }
fn find_header(headers: &Value, key: &str) -> Option<String> {
    match headers {
        Value::Object(map) => {
            // Search case-insensitively
            let key_lower = key.to_ascii_lowercase();
            for (k, v) in map {
                if k.to_ascii_lowercase() == key_lower {
                    if let Some(s) = v.as_str() {
                        return Some(s.to_string());
                    }
                }
            }
            None
        }
        _ => None,
    }
}
