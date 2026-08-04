//! Guardrails API client — non-blocking NGINX subrequest implementation.
//!
//! Instead of making a blocking outbound HTTP call from the worker (which
//! stalls the entire worker event loop for the duration of the round-trip),
//! this module issues an **NGINX subrequest** to an internal `location` that
//! `proxy_pass`es to the guardrails backend. The subrequest is driven by
//! NGINX's own event loop, so the worker remains free to process other
//! connections while the guardrails API is consulted.
//!
//! This is the **single** inspection client, shared by both directions:
//!   * the **request path** (access-phase handler) inspects the client prompt;
//!   * the **response path** (response-body filter) inspects the accumulated
//!     LLM output at end-of-stream.
//!
//! Both call [`inspect_content_async`] with the internal location URI, the
//! text to scan, and a [`ScanDirection`] (`Request` vs `Response`) that is sent
//! to the backend as `scanDirection` so it applies the guardrails configured
//! for that side of the LLM exchange.
//!
//! # Threading
//! NGINX workers are single-threaded; all of the `unsafe impl Send/Sync` below
//! are sound only under that single-threaded-embedding assumption, matching
//! `ngx::async_`'s own runtime invariant.

use core::ffi::c_void;
use core::mem::size_of;
use core::ptr::NonNull;

use futures::channel::oneshot;
use ngx::core::Status;
use ngx::ffi::{
    NGX_HTTP_SUBREQUEST_IN_MEMORY, NGX_OK, ngx_chain_t, ngx_http_post_subrequest_t,
    ngx_http_request_body_t, ngx_http_request_t, ngx_http_subrequest, ngx_int_t, ngx_list_init,
    ngx_palloc, ngx_pool_t, ngx_post_event, ngx_posted_events, ngx_str_t, ngx_table_elt_t,
};
use serde::{Deserialize, Serialize};

use crate::error::{GUARDRAILS_USER_AGENT, GuardrailsError};
use crate::sync_ptr::AssertSendSync;

/// Which side of the LLM exchange a scan applies to. Serializes to the exact
/// `scanDirection` values the Guardrails backend expects: the request path (the
/// user's prompt heading to the LLM) is `"request"`; the response path (the
/// model's output heading back to the client) is `"response"`.
#[derive(Clone, Copy)]
pub enum ScanDirection {
    Request,
    Response,
}

impl ScanDirection {
    fn as_str(self) -> &'static str {
        match self {
            ScanDirection::Request => "request",
            ScanDirection::Response => "response",
        }
    }
}

/// Guardrails scan request body (mirrors the blocking client's schema).
#[derive(Serialize)]
struct GuardrailsRequest<'a> {
    input: &'a str,
    #[serde(rename = "configOverrides")]
    config_overrides: serde_json::Value,
    #[serde(rename = "forceEnabled")]
    force_enabled: Vec<String>,
    disabled: Vec<String>,
    #[serde(rename = "scanDirection")]
    scan_direction: &'a str,
    verbose: bool,
}

/// Outcome of an inspection: whether the content cleared, plus an optional
/// human-facing block message sourced from the guardrails backend.
///
/// `message` is populated only on a block, and only when the backend supplied a
/// usable per-guardrail message (see [`extract_block_message`]). When it is
/// `None`, callers fall back to their own hardcoded block copy.
pub struct Verdict {
    /// `true` when the backend outcome was `cleared`.
    pub cleared: bool,
    /// Configurable block message from the backend (first failed guardrail's
    /// `message`), or `None` to use the caller fallback.
    pub message: Option<String>,
}

/// Top-level guardrails scan response. All fields beyond `result.outcome` are
/// lenient (`#[serde(default)]`) so a minimal `{"result":{"outcome":...}}`
/// body still deserializes and yields no message (caller fallback applies).
#[derive(Deserialize)]
struct GuardrailsResponse {
    result: GuardrailsResult,
}

#[derive(Deserialize)]
struct GuardrailsResult {
    outcome: String,
    /// Per-guardrail results. Present on a flagged scan even at `verbose:false`;
    /// used to find the first failed guardrail and its operator-configured
    /// `message`.
    #[serde(rename = "scannerResults", default)]
    scanner_results: Vec<ScannerResult>,
}

/// One guardrail's result within `result.scannerResults`.
#[derive(Deserialize)]
struct ScannerResult {
    /// `"failed"` or `"passed"` (`ScanOutcome`).
    #[serde(default)]
    outcome: String,
    /// The guardrail's operator-configured message, if any.
    #[serde(default)]
    message: Option<String>,
}

/// Serialize the guardrails scan request body for the given content.
///
/// `verbose` stays `false`. The operator-configured block text is delivered on a
/// flagged scan as `result.scannerResults[].message` even at `verbose:false`, and
/// [`extract_block_message`] sources it from there. `verbose:true` would ALSO add
/// a large top-level scanner-config block, but that inflates the response past the
/// default NGINX `subrequest_output_buffer_size` ("too big subrequest response" ->
/// empty body -> fail-closed block) and is not needed to obtain the message. So
/// keep `verbose:false` for the smaller response.
fn build_request_body(content: &str, direction: ScanDirection) -> Result<Vec<u8>, GuardrailsError> {
    let request_body = GuardrailsRequest {
        input: content,
        config_overrides: serde_json::json!({}),
        force_enabled: vec![],
        disabled: vec![],
        scan_direction: direction.as_str(),
        verbose: false,
    };
    serde_json::to_vec(&request_body).map_err(|e| GuardrailsError::RequestFailed(e.to_string()))
}

/// Extract the configurable block message from a parsed response.
///
/// Selects the **first failed** guardrail (`outcome == "failed"`) and returns
/// its operator-configured `message`. Empty/whitespace strings are treated as
/// absent. Returns `None` when no failed guardrail has a usable message,
/// signalling the caller to use its fallback.
fn extract_block_message(resp: &GuardrailsResponse) -> Option<String> {
    resp.result
        .scanner_results
        .iter()
        .find(|s| s.outcome == "failed")
        .and_then(|s| s.message.as_deref())
        .map(str::trim)
        .filter(|m| !m.is_empty())
        .map(str::to_owned)
}

/// Parse the guardrails scan response into a [`Verdict`].
///
/// `cleared` is `true` iff `result.outcome == "cleared"`; on any other outcome
/// the caller's fail-closed policy blocks. On a block, `message` carries the
/// backend's configurable text when available (see [`extract_block_message`]).
fn parse_outcome(body: &[u8]) -> Result<Verdict, GuardrailsError> {
    let resp: GuardrailsResponse = serde_json::from_slice(body)
        .map_err(|e| GuardrailsError::InvalidResponse(e.to_string()))?;
    let cleared = resp.result.outcome == "cleared";
    let message = if cleared {
        None
    } else {
        extract_block_message(&resp)
    };
    Ok(Verdict { cleared, message })
}

/// Inspect `content` by issuing a non-blocking subrequest to the internal
/// guardrails location `internal_uri`.
///
/// `direction` sets the scan's `scanDirection`: the request path passes
/// [`ScanDirection::Request`] (the prompt heading to the LLM) and the response
/// path passes [`ScanDirection::Response`] (the model output heading back to the
/// client), so the backend applies the guardrails configured for that side.
///
/// Returns a [`Verdict`] (`cleared` + optional backend block message). The
/// caller's fail-closed policy applies to the `Err` cases.
///
/// # Safety
/// `parent` must be a valid main-request `*mut ngx_http_request_t` that remains
/// alive for the duration of the returned future (the caller keeps `r->count`
/// elevated by returning `NGX_DONE` until the future resolves).
pub async unsafe fn inspect_content_async(
    parent: *mut ngx_http_request_t,
    internal_uri: &str,
    content: &str,
    api_token: Option<&str>,
    direction: ScanDirection,
) -> Result<Verdict, GuardrailsError> {
    let body = build_request_body(content, direction)?;

    let (post_subrequest, subrequest) =
        unsafe { start_subrequest(parent, internal_uri, &body, api_token) }?;

    // Keep the raw subrequest pointer across the await point. Sound only under
    // single-threaded embedding.
    let subrequest = AssertSendSync(subrequest);

    // Yield to the NGINX event loop until the subrequest completes. After this
    // await, `subrequest` has response status in `headers_out` and a complete
    // in-memory body in `out`.
    let rc = post_subrequest.finish().await?;
    if rc != Status::NGX_OK {
        return Err(GuardrailsError::RequestFailed(format!(
            "guardrails subrequest failed: rc={}",
            rc.0
        )));
    }

    unsafe { extract_outcome(subrequest.0) }
}

/// Allocate and issue the subrequest. Returns the pending completion handle and
/// the created subrequest pointer.
unsafe fn start_subrequest(
    parent: *mut ngx_http_request_t,
    internal_uri: &str,
    body: &[u8],
    api_token: Option<&str>,
) -> Result<(PostSubrequest, NonNull<ngx_http_request_t>), GuardrailsError> {
    let pool = unsafe { (*parent).pool };
    if pool.is_null() {
        return Err(GuardrailsError::RequestFailed("null request pool".into()));
    }

    let post_subrequest = unsafe { PostSubrequest::new(pool) }?;

    // Copy the internal URI into the request pool so it outlives this call.
    let mut subrequest_uri = unsafe { ngx_str_t::from_bytes(pool, internal_uri.as_bytes()) }
        .ok_or_else(|| GuardrailsError::RequestFailed("uri allocation failed".into()))?;

    let mut subrequest: *mut ngx_http_request_t = core::ptr::null_mut();
    let stat = unsafe {
        ngx_http_subrequest(
            parent,
            core::ptr::from_mut(&mut subrequest_uri),
            core::ptr::null_mut(),
            core::ptr::from_mut(&mut subrequest),
            post_subrequest.raw(),
            NGX_HTTP_SUBREQUEST_IN_MEMORY as usize,
        )
    };
    if stat != NGX_OK as ngx_int_t {
        return Err(GuardrailsError::RequestFailed(format!(
            "ngx_http_subrequest failed: {}",
            stat
        )));
    }
    let mut subrequest = NonNull::new(subrequest)
        .ok_or_else(|| GuardrailsError::RequestFailed("null subrequest".into()))?;

    // Set the subrequest method to POST.
    unsafe {
        subrequest.as_mut().method = ngx::ffi::NGX_HTTP_POST as ngx::ffi::ngx_uint_t;
        subrequest.as_mut().method_name = ngx_str_t::from_bytes(pool, b"POST")
            .ok_or_else(|| GuardrailsError::RequestFailed("method alloc failed".into()))?;
    }

    // Reset the subrequest's headers_in (it is initialized as a copy of the
    // parent's). Zero it, then restore the two fields NGINX leaves non-zero,
    // and re-init the headers list — matching ngx_http_alloc_request.
    unsafe {
        core::ptr::write_bytes(
            core::ptr::from_mut(&mut subrequest.as_mut().headers_in),
            0,
            1,
        );
        subrequest.as_mut().headers_in.content_length_n = -1;
        subrequest.as_mut().headers_in.keep_alive_n = -1;

        let stat = ngx_list_init(
            core::ptr::from_mut(&mut subrequest.as_mut().headers_in.headers),
            pool,
            20,
            size_of::<ngx_table_elt_t>(),
        );
        if stat != NGX_OK as ngx_int_t {
            return Err(GuardrailsError::RequestFailed(
                "headers_in list init failed".into(),
            ));
        }
    }

    // Add request headers.
    {
        let request = unsafe { ngx::http::Request::from_ngx_http_request(subrequest.as_ptr()) };
        let _ = request.add_header_in("Content-Type", "application/json");
        // Required: some guardrails backends sit behind an edge/WAF that rejects
        // requests without a User-Agent with 403. NGINX's proxy module does not add a
        // default User-Agent for a subrequest, so set it explicitly (matching client.rs).
        let _ = request.add_header_in("User-Agent", GUARDRAILS_USER_AGENT);
        // Only emit the header for a non-empty token: an empty credential would
        // produce `Authorization: Bearer ` and fail closed. An empty token file
        // is already rejected at config load, so this is defense-in-depth.
        if let Some(token) = api_token.filter(|t| !t.is_empty()) {
            let _ = request.add_header_in("Authorization", &format!("Bearer {}", token));
        }
        let _ = request.add_header_in("Content-Length", &format!("{}", body.len()));
    }

    // Attach the synthesized JSON body to the subrequest.
    unsafe {
        let request_body =
            ngx_palloc(pool, size_of::<ngx_http_request_body_t>()) as *mut ngx_http_request_body_t;
        if request_body.is_null() {
            return Err(GuardrailsError::RequestFailed(
                "request_body allocation failed".into(),
            ));
        }
        core::ptr::write_bytes(request_body, 0, 1);
        (*request_body).bufs = body_to_chain(pool, body)?;
        subrequest.as_mut().request_body = request_body;
        subrequest.as_mut().headers_in.content_length_n = body.len() as i64;
        subrequest.as_mut().headers_in.set_chunked(0);
    }

    // `ngx_http_subrequest` only queues the subrequest on the parent's
    // `posted_requests` list; it is drained by `ngx_http_run_posted_requests`
    // the next time nginx processes the parent request's events. Because we are
    // called from within a spawned async task (running in the async scheduler's
    // posted-event context, not inside `ngx_http_request_handler`), nothing will
    // drain that list unless we re-enter HTTP event processing. Post the
    // parent's write event so nginx runs the posted subrequest.
    unsafe {
        let conn = (*parent).connection;
        if !conn.is_null() && !(*conn).write.is_null() {
            ngx_post_event((*conn).write, core::ptr::addr_of_mut!(ngx_posted_events));
        }
    }

    Ok((post_subrequest, subrequest))
}

/// Build a single-buffer `ngx_chain_t` in the request pool holding `data`.
unsafe fn body_to_chain(
    pool: *mut ngx_pool_t,
    data: &[u8],
) -> Result<*mut ngx_chain_t, GuardrailsError> {
    unsafe {
        let buf = ngx::ffi::ngx_create_temp_buf(pool, data.len());
        if buf.is_null() {
            return Err(GuardrailsError::RequestFailed(
                "body buf alloc failed".into(),
            ));
        }
        core::ptr::copy_nonoverlapping(data.as_ptr(), (*buf).pos, data.len());
        (*buf).last = (*buf).pos.add(data.len());
        (*buf).set_temporary(1);
        (*buf).set_last_buf(1);
        (*buf).set_last_in_chain(1);

        let chain = ngx::ffi::ngx_alloc_chain_link(pool);
        if chain.is_null() {
            return Err(GuardrailsError::RequestFailed(
                "body chain alloc failed".into(),
            ));
        }
        (*chain).buf = buf;
        (*chain).next = core::ptr::null_mut();
        Ok(chain)
    }
}

/// Read the subrequest's captured response and decide the outcome.
unsafe fn extract_outcome(
    subrequest: NonNull<ngx_http_request_t>,
) -> Result<Verdict, GuardrailsError> {
    let status = unsafe { subrequest.as_ref().headers_out.status };
    if !(200..300).contains(&(status as u32)) {
        return Err(GuardrailsError::InvalidResponse(format!(
            "guardrails backend status: {}",
            status
        )));
    }

    // With NGX_HTTP_SUBREQUEST_IN_MEMORY, the response body is buffered in the
    // upstream's buffer; walk `out` (and fall back to the upstream buffer) to
    // reassemble it.
    let mut body = Vec::new();
    let mut chain = unsafe { subrequest.as_ref().out };
    while !chain.is_null() {
        let buf = unsafe { (*chain).buf };
        if !buf.is_null() {
            let b = unsafe { &*buf };
            if !b.pos.is_null() && !b.last.is_null() {
                let len = unsafe { b.last.offset_from(b.pos) } as usize;
                let slice = unsafe { core::slice::from_raw_parts(b.pos, len) };
                body.extend_from_slice(slice);
            }
        }
        chain = unsafe { (*chain).next };
    }

    // Fallback: in-memory subrequests place the body in upstream->buffer.
    if body.is_empty() {
        let upstream = unsafe { subrequest.as_ref().upstream };
        if !upstream.is_null() {
            let b = unsafe { &(*upstream).buffer };
            if !b.pos.is_null() && !b.last.is_null() {
                let len = unsafe { b.last.offset_from(b.pos) } as usize;
                if len > 0 {
                    let slice = unsafe { core::slice::from_raw_parts(b.pos, len) };
                    body.extend_from_slice(slice);
                }
            }
        }
    }

    if body.is_empty() {
        return Err(GuardrailsError::InvalidResponse(
            "empty guardrails response body".into(),
        ));
    }

    parse_outcome(&body)
}

/// Data boxed into the `ngx_http_post_subrequest_t`'s `data` member. Holds the
/// sender used to notify completion, plus a back-pointer to the
/// `post_subrequest` so `data` can be nulled exactly once (double-free guard).
struct PSData {
    sender: oneshot::Sender<Status>,
    post_subrequest: *mut ngx_http_post_subrequest_t,
}

/// Pending subrequest completion handle. Awaiting `finish()` resolves when the
/// C `handler` fires inside the subrequest's `ngx_http_finalize_request`.
struct PostSubrequest {
    post_subrequest: *mut ngx_http_post_subrequest_t,
    receiver: Option<oneshot::Receiver<Status>>,
}
// Safety: single-threaded embedding.
unsafe impl Send for PostSubrequest {}
unsafe impl Sync for PostSubrequest {}

impl PostSubrequest {
    /// Allocate the `ngx_http_post_subrequest_t` in the request pool and wire up
    /// the completion channel.
    unsafe fn new(pool: *mut ngx_pool_t) -> Result<Self, GuardrailsError> {
        let post_subrequest = unsafe { ngx_palloc(pool, size_of::<ngx_http_post_subrequest_t>()) }
            as *mut ngx_http_post_subrequest_t;
        if post_subrequest.is_null() {
            return Err(GuardrailsError::RequestFailed(
                "post_subrequest allocation failed".into(),
            ));
        }
        let (sender, receiver) = oneshot::channel();

        // The handler consumes this box at most once; `Drop` turns the handler
        // into a no-op if the future is dropped before completion.
        let data = Box::new(PSData {
            sender,
            post_subrequest,
        });
        unsafe {
            (*post_subrequest).handler = Some(Self::handler);
            (*post_subrequest).data = Box::into_raw(data) as *mut c_void;
        }
        Ok(Self {
            post_subrequest,
            receiver: Some(receiver),
        })
    }

    fn raw(&self) -> *mut ngx_http_post_subrequest_t {
        self.post_subrequest
    }

    /// Await subrequest completion.
    async fn finish(mut self) -> Result<Status, GuardrailsError> {
        match self.receiver.take() {
            Some(receiver) => receiver
                .await
                .map_err(|_| GuardrailsError::RequestFailed("subrequest channel canceled".into())),
            None => unreachable!("finish called once"),
        }
    }

    /// Post-subrequest callback, invoked from the subrequest's
    /// `ngx_http_finalize_request`.
    unsafe extern "C" fn handler(
        _r: *mut ngx_http_request_t,
        data: *mut c_void,
        rc: ngx_int_t,
    ) -> ngx_int_t {
        // `data` is nulled when the PostSubrequest is dropped; if so, no-op.
        if !data.is_null() {
            let psdata = *unsafe { Box::from_raw(data as *mut PSData) };
            // Null the data member so Drop does not double-free.
            unsafe { (*psdata.post_subrequest).data = core::ptr::null_mut() };
            // Wake the awaiting future. This runs the continuation in place on
            // the current stack (single-threaded), which is fine.
            let _ = psdata.sender.send(Status(rc));
        }
        NGX_OK as ngx_int_t
    }
}

impl Drop for PostSubrequest {
    fn drop(&mut self) {
        // If the handler has not yet consumed the boxed data, reclaim and drop
        // it, and null the pointer so a later handler run is a no-op.
        unsafe {
            if !(*self.post_subrequest).data.is_null() {
                let _ = Box::from_raw((*self.post_subrequest).data as *mut PSData);
                (*self.post_subrequest).data = core::ptr::null_mut();
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_build_request_body_shape() {
        let body = build_request_body("hello", ScanDirection::Request).unwrap();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(
            v,
            serde_json::json!({
                "input": "hello",
                "configOverrides": {},
                "forceEnabled": [],
                "disabled": [],
                "scanDirection": "request",
                "verbose": false,
            })
        );
    }

    #[test]
    fn test_build_request_body_response_direction() {
        let body = build_request_body("hello", ScanDirection::Response).unwrap();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(v["scanDirection"], serde_json::json!("response"));
    }

    #[test]
    fn test_parse_outcome_cleared() {
        let v = parse_outcome(br#"{"result":{"outcome":"cleared"}}"#).unwrap();
        assert!(v.cleared);
        assert!(v.message.is_none());
    }

    #[test]
    fn test_parse_outcome_flagged() {
        // Minimal flagged body with no scannerResults -> block, no message
        // (caller falls back to its hardcoded copy).
        let v = parse_outcome(br#"{"result":{"outcome":"flagged"}}"#).unwrap();
        assert!(!v.cleared);
        assert!(v.message.is_none());
    }

    #[test]
    fn test_parse_outcome_malformed() {
        assert!(matches!(
            parse_outcome(b"not json"),
            Err(GuardrailsError::InvalidResponse(_))
        ));
    }

    #[test]
    fn test_block_message_uses_scanner_message() {
        // The failed guardrail's own message is used.
        let body = br#"{
            "result": {
                "outcome": "flagged",
                "scannerResults": [
                    {"outcome": "failed", "message": "detector said no"}
                ]
            }
        }"#;
        let v = parse_outcome(body).unwrap();
        assert!(!v.cleared);
        assert_eq!(v.message.as_deref(), Some("detector said no"));
    }

    #[test]
    fn test_block_message_uses_first_failed_guardrail() {
        // The first failed guardrail's message is used; passed ones are skipped.
        let body = br#"{
            "result": {
                "outcome": "flagged",
                "scannerResults": [
                    {"outcome": "passed", "message": "ok"},
                    {"outcome": "failed", "message": "first fail"},
                    {"outcome": "failed", "message": "second fail"}
                ]
            }
        }"#;
        let v = parse_outcome(body).unwrap();
        assert_eq!(v.message.as_deref(), Some("first fail"));
    }

    #[test]
    fn test_block_message_empty_is_ignored() {
        // Whitespace-only messages are treated as absent -> fallback (None).
        let body = br#"{
            "result": {
                "outcome": "flagged",
                "scannerResults": [
                    {"outcome": "failed", "message": "   "}
                ]
            }
        }"#;
        let v = parse_outcome(body).unwrap();
        assert!(!v.cleared);
        assert!(v.message.is_none());
    }

    #[test]
    fn test_block_message_from_real_verbose_false_response() {
        // Regression fixture mirroring an observed live `verbose:false` response:
        // there is NO top-level `scanners` block (that is verbose-only), but the
        // failed scanner still carries the operator-configured `message`. The
        // trailing whitespace in the real message must be trimmed.
        let body = br#"{
            "id": "019fb483-2087-7078-b24c-fa57585a52c7",
            "result": {
                "scannerResults": [
                    {"scannerId": "01915be3-0e4e-70d5-aeac-74c59225988e", "outcome": "passed", "data": {"type": "regex", "matches": []}},
                    {"scannerId": "01915be3-0e4e-70da-b3d1-4177fd5f6136", "outcome": "failed", "message": "This message has been blocked by IP guardrail FLAG MESSAGE@!!!! ", "data": {"type": "regex", "matches": [[27, 37]]}},
                    {"scannerId": "019d9ada-eb9f-7072-9561-4cdda47bd98d", "outcome": "passed", "data": {"type": "keyword", "matches": {}}}
                ],
                "outcome": "flagged"
            },
            "redactedInput": "Here is a test IP address: 192.0.2.14"
        }"#;
        let v = parse_outcome(body).unwrap();
        assert!(!v.cleared);
        assert_eq!(
            v.message.as_deref(),
            Some("This message has been blocked by IP guardrail FLAG MESSAGE@!!!!")
        );
    }
}
