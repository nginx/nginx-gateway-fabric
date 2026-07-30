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
//! Both call [`inspect_content_async`] with the internal location URI and the
//! text to scan.
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

/// Wrapper asserting `Send + Sync` for values that are only ever touched on the
/// single NGINX worker thread. Used to move raw request pointers across the
/// `.await` boundary of a spawned task.
struct AssertSendSync<T>(T);
// Safety: single-threaded embedding — the NGINX worker never touches these
// values from more than one thread.
unsafe impl<T> Send for AssertSendSync<T> {}
unsafe impl<T> Sync for AssertSendSync<T> {}

/// Guardrails scan request body (mirrors the blocking client's schema).
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

/// Serialize the guardrails scan request body for the given content.
fn build_request_body(content: &str) -> Result<Vec<u8>, GuardrailsError> {
    let request_body = GuardrailsRequest {
        input: content,
        config_overrides: serde_json::json!({}),
        force_enabled: vec![],
        disabled: vec![],
        verbose: false,
    };
    serde_json::to_vec(&request_body).map_err(|e| GuardrailsError::RequestFailed(e.to_string()))
}

/// Parse the guardrails scan response, returning `true` when cleared.
fn parse_outcome(body: &[u8]) -> Result<bool, GuardrailsError> {
    let resp: GuardrailsResponse = serde_json::from_slice(body)
        .map_err(|e| GuardrailsError::InvalidResponse(e.to_string()))?;
    let cleared = resp.result.outcome == "cleared";
    eprintln!("[guardrails] Subrequest outcome: {}", resp.result.outcome);
    if let Some(ref details) = resp.result.details {
        eprintln!("  Details: {}", details);
    }
    Ok(cleared)
}

/// Inspect `content` by issuing a non-blocking subrequest to the internal
/// guardrails location `internal_uri`.
///
/// Returns `Ok(true)` when cleared, `Ok(false)` when flagged/blocked. The
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
) -> Result<bool, GuardrailsError> {
    let body = build_request_body(content)?;

    eprintln!(
        "[guardrails] Issuing guardrails subrequest to {} ({} bytes body)",
        internal_uri,
        body.len()
    );

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
        if let Some(token) = api_token {
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
) -> Result<bool, GuardrailsError> {
    let status = unsafe { subrequest.as_ref().headers_out.status };
    eprintln!("[guardrails] Subrequest response status={}", status);
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
        let body = build_request_body("hello").unwrap();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(
            v,
            serde_json::json!({
                "input": "hello",
                "configOverrides": {},
                "forceEnabled": [],
                "disabled": [],
                "verbose": false,
            })
        );
    }

    #[test]
    fn test_parse_outcome_cleared() {
        assert!(parse_outcome(br#"{"result":{"outcome":"cleared"}}"#).unwrap());
    }

    #[test]
    fn test_parse_outcome_flagged() {
        assert!(!parse_outcome(br#"{"result":{"outcome":"flagged"}}"#).unwrap());
    }

    #[test]
    fn test_parse_outcome_malformed() {
        assert!(matches!(
            parse_outcome(b"not json"),
            Err(GuardrailsError::InvalidResponse(_))
        ));
    }
}
