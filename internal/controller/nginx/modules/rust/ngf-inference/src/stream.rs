//! Request processing and EPP stream state management.
//!
//! This module handles the two-phase EPP processing flow using NGINX's async
//! event model. A single bidirectional gRPC stream is kept alive across both
//! phases:
//!
//! **Phase 1 — Request (access handler):**
//! 1. Extract headers and body from incoming request
//! 2. Spawn async task on nginx event loop (via ngx::async_::spawn)
//! 3. Task opens gRPC stream and sends RequestHeaders + RequestBody
//! 4. Task reads endpoint selection response
//! 5. Task stores endpoint + live gRPC stream in RequestCtx, signals done
//! 6. Handler re-enters, reads result, returns NGX_DECLINED
//! 7. NGINX proxy_passes to the selected endpoint
//!
//! **Phase 2 — Response (header filter, fire-and-forget):**
//! 8. Upstream responds → header filter fires
//! 9. Header filter sends ResponseHeaders (served endpoint) on the open stream
//! 10. gRPC stream is closed; headers and body pass through to client immediately

use std::cell::Cell;
use std::ptr::NonNull;
use std::rc::Rc;

use ngx::async_::Task;
use ngx::core::Status;
use ngx::ffi::{
    ngx_buf_t, ngx_chain_t, ngx_connection_t, ngx_event_t, ngx_http_read_client_request_body,
    ngx_http_request_t, ngx_post_event, ngx_posted_events, ngx_posted_next_events,
};
use ngx::http::{HttpModuleLocationConf, NgxHttpCoreModule, Request};

use crate::config::EPP_TIMEOUT_MS;
use crate::grpc;
use crate::grpc::{EppStream, ResolverParams};
use crate::net::ssl::NgxSsl;

/// Errors that can occur during EPP processing.
///
/// Variant payloads are read via the `Debug` impl (logged with `{:?}`).
#[derive(Debug)]
pub enum EppError {
    /// EPP failed but failopen is enabled.
    FailOpen(String),
    /// EPP returned an error.
    ProcessingFailed(String),
    /// Internal error.
    Internal(String),
}

impl std::fmt::Display for EppError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::FailOpen(msg) => write!(f, "EPP failed (failopen): {msg}"),
            Self::ProcessingFailed(msg) => write!(f, "EPP processing failed: {msg}"),
            Self::Internal(msg) => write!(f, "internal error: {msg}"),
        }
    }
}

/// Per-request context stored in the NGINX request pool.
///
/// Tracks the state of the two-phase async EPP processing for a single request.
///
/// **Phase 1 (request):** The access handler spawns an async task that connects
/// to EPP, sends RequestHeaders+Body, and reads the endpoint selection. The live
/// gRPC stream is stored in [`epp_stream`](Self::epp_stream) for phase 2.
///
/// **Phase 2 (response, fire-and-forget):** The header filter sends the
/// ResponseHeaders (served endpoint) on the still-open stream and closes it.
/// The client response passes through immediately without waiting for EPP.
#[repr(C)]
pub struct RequestCtx {
    // --- Phase 1 (request) state ---
    /// Set to true when the request-phase async task completes (endpoint resolved).
    pub done: bool,
    /// Set to true by the async task if the gRPC call failed.
    pub epp_failed: bool,
    /// Error detail string (set by async task on failure).
    pub error_detail: Option<String>,
    /// NGINX event for polling the request-phase async task.
    pub event: ngx_event_t,
    /// Whether the request-phase async task has been spawned.
    pub task_spawned: bool,
    /// The selected endpoint from EPP (set in phase 1).
    pub selected_endpoint: Option<String>,
    /// Flag indicating body reading has been started.
    pub body_reading_started: bool,
    /// Flag indicating body reading is complete.
    pub body_ready: bool,

    // --- Lifetime management ---
    /// Handle to the spawned async EPP task.
    ///
    /// Dropping this cancels the task (prevents further polling of the
    /// future), which is critical if the request is finalized early
    /// (client disconnect, timeout). Without this, the detached task
    /// could dereference `ctx_ptr` after the request pool is freed.
    pub task: Option<Task<()>>,
    /// Cancellation flag shared with the async closure.
    ///
    /// Set to `true` in [`Drop`] so that even if a final poll of the
    /// future sneaks in, it will skip writing to the freed `ctx_ptr`.
    /// Uses `Rc<Cell<bool>>` because nginx is single-threaded.
    pub cancelled: Rc<Cell<bool>>,

    // --- Phase 2 (response) state ---
    /// Live gRPC stream from phase 1, consumed by phase 2.
    pub epp_stream: Option<EppStream>,
    /// Whether the response phase notification has been sent.
    pub response_phase_spawned: bool,
}

impl Default for RequestCtx {
    fn default() -> Self {
        Self {
            done: false,
            epp_failed: false,
            error_detail: None,
            event: unsafe { core::mem::zeroed() },
            task_spawned: false,
            selected_endpoint: None,
            body_reading_started: false,
            body_ready: false,

            task: None,
            cancelled: Rc::new(Cell::new(false)),

            epp_stream: None,
            response_phase_spawned: false,
        }
    }
}

impl Drop for RequestCtx {
    fn drop(&mut self) {
        // Signal cancellation so any in-flight poll of the async closure
        // will skip writing to the (soon-to-be-freed) ctx_ptr.
        self.cancelled.set(true);

        // Drop the task handle, which cancels the future (prevents further
        // polling). This is the primary safety mechanism — the cancellation
        // flag above is defense-in-depth.
        self.task.take();

        // Remove from NGINX posted event queues if still queued
        if self.event.posted() != 0 {
            unsafe { ngx::ffi::ngx_delete_posted_event(&raw mut self.event) };
        }
    }
}

/// NGINX event callback: polls whether the async EPP task has completed.
///
/// This is called by the NGINX event loop on each iteration. If the async
/// task has finished (done == true), it posts to the connection's write event
/// which triggers the access handler to be re-entered. Otherwise, it
/// re-schedules itself for the next event loop iteration.
unsafe extern "C" fn check_epp_done(event: *mut ngx_event_t) {
    let ctx = ngx::ngx_container_of!(event, RequestCtx, event);
    let c: *mut ngx_connection_t = unsafe { (*event).data.cast() };

    if unsafe { (*ctx).done } {
        // EPP complete — trigger access handler re-entry via connection write event
        unsafe { ngx_post_event((*c).write, &raw mut ngx_posted_events) };
    } else {
        // Not ready yet — re-schedule on next event loop iteration
        unsafe { ngx_post_event(event, &raw mut ngx_posted_next_events) };
    }
}

/// NGINX callback invoked when request body reading is complete.
///
/// Posts the connection's write event to trigger the access handler
/// to re-enter, at which point the body will be available.
///
/// `ngx_http_read_client_request_body()` increments `r->main->count` before
/// starting the async read.  We must decrement it here so the reference count
/// is balanced when the request eventually finishes. Without this, the count
/// never reaches zero, `ngx_http_free_request()` is never called, and the
/// access-log phase is skipped entirely.
unsafe extern "C" fn body_read_callback(r: *mut ngx_http_request_t) {
    // Balance the count++ from ngx_http_read_client_request_body().
    // `count` is a bitfield, so we use the generated accessor methods.
    let main = unsafe { (*r).main };
    let current = unsafe { (*main).count() };
    unsafe { (*main).set_count(current - 1) };

    let c: *mut ngx_connection_t = unsafe { (*r).connection };
    unsafe { ngx_post_event((*c).write, &raw mut ngx_posted_events) };
}

/// Start reading the client request body.
///
/// Returns Ok(true) if body is already available (no reading needed),
/// Ok(false) if body reading was started asynchronously (return NGX_AGAIN),
/// or Err if body reading failed.
pub fn start_body_reading(request: &mut Request) -> Result<bool, EppError> {
    let raw: *mut ngx_http_request_t = request.into();

    // Request buffering of the body into a single buffer (bitfield setter)
    unsafe { (*raw).set_request_body_in_single_buf(1) };

    let rc = unsafe { ngx_http_read_client_request_body(raw, Some(body_read_callback)) };

    if rc == Status::NGX_OK.into() {
        // Body is already available (buffered from previous read or empty)
        Ok(true)
    } else if rc == Status::NGX_AGAIN.into() {
        // Body reading started asynchronously
        Ok(false)
    } else {
        Err(EppError::Internal(format!(
            "ngx_http_read_client_request_body failed with rc={}",
            rc
        )))
    }
}

/// Start the EPP request phase (phase 1) as an async task on the nginx event loop.
///
/// This expects the RequestCtx to already be allocated and set on the request.
/// It sets up event polling, extracts headers/body, and spawns the async task.
///
/// Phase 1 connects to EPP, sends RequestHeaders+Body, and reads the endpoint
/// selection. The gRPC stream is kept alive and stored in the context for
/// phase 2 (response phase).
///
/// The caller should return `NGX_AGAIN` after this succeeds.
pub fn start_epp_stream(
    request: &mut Request,
    ctx: &mut RequestCtx,
    epp_endpoint: &str,
    use_tls: bool,
    ssl: Option<NgxSsl>,
) -> Result<(), EppError> {
    // Extract headers on the NGINX thread
    let headers = extract_request_headers(request);

    // Set up NGINX event for polling the async task
    ctx.event.handler = Some(check_epp_done);
    ctx.event.data = request.connection().cast();
    ctx.event.log = unsafe { (*request.connection()).log };

    // Post the polling event
    unsafe { ngx_post_event(&raw mut ctx.event, &raw mut ngx_posted_next_events) };

    // Extract request body — should be available now after body reading phase
    let body = extract_request_body(request);

    // Get log for the async task
    let log = unsafe { NonNull::new_unchecked((*request.connection()).log) };

    let endpoint = epp_endpoint.to_string();

    // Extract resolver from the location configuration. When a `resolver`
    // directive is configured, the async resolver avoids blocking the nginx
    // event loop during DNS lookups. Without it, we fall back to getaddrinfo(3).
    let resolver_params = extract_resolver_params(request);

    // Get a raw pointer to the context so the async task can write results.
    // SAFETY: The context is allocated from the request pool and lives for
    // the entire request lifetime. The async task runs on the same nginx
    // worker thread (single-threaded), so there are no data races.
    let ctx_ptr = ctx as *mut RequestCtx;

    // Clone the cancellation flag into the async closure. If the request is
    // torn down (client disconnect, timeout) while the task is pending,
    // RequestCtx::drop sets this to true, and the closure skips writing to
    // the freed ctx_ptr. This is defense-in-depth — the primary mechanism
    // is dropping the Task handle (which prevents further polling).
    let cancelled = Rc::clone(&ctx.cancelled);

    // Increment r->main->count to prevent nginx from finalizing the request
    // while the async task is in flight. The task decrements it on completion.
    let raw: *mut ngx_http_request_t = request.into();
    let main = unsafe { (*raw).main };
    let current = unsafe { (*main).count() };
    unsafe { (*main).set_count(current + 1) };

    // Spawn async task on the nginx event loop
    // ssl is moved (owned) into the async block so it lives for 'static
    // resolver_params is Copy — safe to move into the async block
    let task = ngx::async_::spawn(async move {
        let result = grpc::epp_request_phase(
            log,
            &endpoint,
            EPP_TIMEOUT_MS,
            headers,
            body,
            use_tls,
            ssl,
            resolver_params,
        )
        .await;

        // Check cancellation before writing to ctx_ptr. If the request was
        // torn down, the pool (and RequestCtx) may already be freed.
        if cancelled.get() {
            // Balance the count++ even on cancellation.
            let current = unsafe { (*main).count() };
            unsafe { (*main).set_count(current - 1) };
            return;
        }

        match result {
            Ok(result) => {
                // SAFETY: single-threaded, ctx_ptr valid (not cancelled)
                let ctx = unsafe { &mut *ctx_ptr };
                ctx.selected_endpoint = result.selected_endpoint;
                ctx.epp_stream = result.stream;
            }
            Err(e) => {
                let ctx = unsafe { &mut *ctx_ptr };
                ctx.error_detail = Some(e);
                ctx.epp_failed = true;
            }
        }
        let ctx = unsafe { &mut *ctx_ptr };
        ctx.done = true;

        // Balance the count++ from start_epp_stream. This allows nginx to
        // finalize the request once the access handler returns NGX_DECLINED.
        let current = unsafe { (*main).count() };
        unsafe { (*main).set_count(current - 1) };
    });

    ctx.task = Some(task);
    ctx.task_spawned = true;

    Ok(())
}

/// Extract nginx resolver parameters from the request's location configuration.
///
/// Returns a [`ResolverParams`] with the resolver pointer and timeout if a
/// `resolver` directive is configured for this location, otherwise `resolver`
/// is `None` and the caller should fall back to blocking DNS.
fn extract_resolver_params(request: &Request) -> ResolverParams {
    let clcf = NgxHttpCoreModule::location_conf(request);
    match clcf {
        Some(loc_conf) => ResolverParams {
            resolver: valid_resolver_ptr(loc_conf.resolver),
            timeout: loc_conf.resolver_timeout,
        },
        None => ResolverParams {
            resolver: None,
            timeout: 0,
        },
    }
}

/// Return a [`NonNull`] only when `ptr` is a usable resolver with nameservers.
///
/// nginx always creates a resolver during `merge_loc_conf`, but when no
/// `resolver` directive is present, it creates a "dummy" resolver via
/// `ngx_resolver_create(cf, NULL, 0)` — a valid `ngx_resolver_t*` with
/// `connections.nelts == 0` (zero nameservers).
///
/// Attempting to use such a resolver causes nginx's `ngx_resolve_start` to
/// return `NGX_NO_RESOLVER` (`(void*)-1`), and the ngx-rust `Resolver` wrapper
/// doesn't detect this sentinel, leading to a crash when dereferenced.
///
/// This function returns `None` for:
/// - null pointers (shouldn't happen after merge, but defensive)
/// - `NGX_CONF_UNSET_PTR` sentinel (`(void*)-1`) from unmerged configs
/// - resolvers with zero nameservers (dummy resolver)
fn valid_resolver_ptr(
    ptr: *mut nginx_sys::ngx_resolver_t,
) -> Option<NonNull<nginx_sys::ngx_resolver_t>> {
    // NGX_CONF_UNSET_PTR is `(void *) -1`, i.e. usize::MAX.
    if ptr.is_null() || ptr as usize == usize::MAX {
        return None;
    }

    // SAFETY: We've verified ptr is not null and not the unset sentinel.
    // The resolver struct is owned by nginx and valid for the request lifetime.
    let resolver = unsafe { &*ptr };

    // A dummy resolver has connections.nelts == 0 (no nameservers configured).
    // Using it would cause ngx_resolve_start to return NGX_NO_RESOLVER (-1),
    // which the ngx-rust Resolver wrapper doesn't handle, leading to a crash.
    if resolver.connections.nelts == 0 {
        return None;
    }

    NonNull::new(ptr)
}

/// Send the EPP response-phase notification (fire-and-forget).
///
/// Called by the header filter when the upstream responds. This sends
/// the ResponseHeaders message (with the served endpoint) on the still-open
/// gRPC stream and closes it. The client response is not delayed — headers
/// and body pass through to the client immediately.
///
/// This is non-blocking: `epp_response_phase` sends the gRPC frame on the
/// h2 SendStream and spawns a background task to drive the h2 connection
/// until the data is flushed to the wire.
pub fn send_epp_response_notification(ctx: &mut RequestCtx) -> Result<(), EppError> {
    let stream = ctx.epp_stream.take().ok_or_else(|| {
        EppError::Internal("no EPP stream available for response phase".to_string())
    })?;

    let served_ep = ctx.selected_endpoint.clone().unwrap_or_default();

    // Fire-and-forget: send the served endpoint and close the stream.
    // The actual network write happens asynchronously via a background task
    // that drives the h2 connection until the data is flushed.
    let result = grpc::epp_response_phase(stream, &served_ep);

    ctx.response_phase_spawned = true;

    result.map_err(EppError::Internal)
}

/// Finalize EPP request-phase processing after the async task has completed.
///
/// Called on the NGINX thread when the access handler is re-entered and
/// `done` is true. Returns the appropriate status based on success/failure
/// and failopen configuration.
pub fn finalize_epp_result(ctx: &mut RequestCtx, failopen: bool) -> Result<(), EppError> {
    if ctx.selected_endpoint.is_some() {
        return Ok(());
    }

    // No endpoint — check if it was a failure
    if ctx.epp_failed {
        let detail = ctx
            .error_detail
            .take()
            .unwrap_or_else(|| "EPP gRPC call failed".to_string());
        if failopen {
            return Err(EppError::FailOpen(detail));
        }
        return Err(EppError::ProcessingFailed(detail));
    }

    // EPP succeeded but returned no endpoint (unusual)
    Ok(())
}

/// Extract HTTP headers from an NGINX request.
fn extract_request_headers(request: &Request) -> Vec<(String, String)> {
    let mut headers = Vec::new();

    // Iterate all headers_in via the safe iterator API
    for (key, value) in request.headers_in_iterator() {
        let key_str = key.to_string();
        let val_str = value.to_string();
        // Use lowercase keys per HTTP/2 convention
        headers.push((key_str.to_lowercase(), val_str));
    }

    headers
}

/// Extract request body from an NGINX request.
///
/// The request body must have been read already (via `ngx_http_read_client_request_body`
/// or implicitly by upstream buffering). This function reads the buffered body data
/// from the chain of buffers.
///
/// Returns `None` if no body is available or the body hasn't been read yet.
fn extract_request_body(request: &Request) -> Option<Vec<u8>> {
    // Safety: access raw request structure to read the request_body
    let raw: *const ngx_http_request_t = request.into();

    let request_body = unsafe { (*raw).request_body };
    if request_body.is_null() {
        return None;
    }

    let bufs: *mut ngx_chain_t = unsafe { (*request_body).bufs };
    if bufs.is_null() {
        return None;
    }

    // Collect body data from the buffer chain
    let mut body = Vec::new();
    let mut chain = bufs;

    while !chain.is_null() {
        let buf: *mut ngx_buf_t = unsafe { (*chain).buf };
        if !buf.is_null() {
            let buf = unsafe { &*buf };
            let pos = buf.pos;
            let last = buf.last;

            if !pos.is_null() && !last.is_null() && last > pos {
                let len = unsafe { last.offset_from(pos) } as usize;
                let data = unsafe { std::slice::from_raw_parts(pos, len) };
                body.extend_from_slice(data);
            }
        }
        chain = unsafe { (*chain).next };
    }

    if body.is_empty() { None } else { Some(body) }
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- EppError Display ---

    #[test]
    fn epp_error_display_failopen() {
        let err = EppError::FailOpen("connection refused".to_string());
        assert_eq!(
            format!("{err}"),
            "EPP failed (failopen): connection refused"
        );
    }

    #[test]
    fn epp_error_display_processing_failed() {
        let err = EppError::ProcessingFailed("connection reset".to_string());
        assert_eq!(format!("{err}"), "EPP processing failed: connection reset");
    }

    #[test]
    fn epp_error_display_internal() {
        let err = EppError::Internal("alloc failed".to_string());
        assert_eq!(format!("{err}"), "internal error: alloc failed");
    }

    // --- finalize_epp_result ---

    fn make_ctx() -> RequestCtx {
        RequestCtx::default()
    }

    #[test]
    fn finalize_epp_result_success() {
        let mut ctx = make_ctx();
        ctx.selected_endpoint = Some("10.0.0.1:8080".to_string());
        ctx.done = true;

        let result = finalize_epp_result(&mut ctx, false);
        assert!(result.is_ok());
        assert_eq!(ctx.selected_endpoint.as_deref(), Some("10.0.0.1:8080"));
    }

    #[test]
    fn finalize_epp_result_failure_no_failopen() {
        let mut ctx = make_ctx();
        ctx.error_detail = Some("gRPC connection failed: connection refused".to_string());
        ctx.epp_failed = true;
        ctx.done = true;

        let result = finalize_epp_result(&mut ctx, false);
        match result {
            Err(EppError::ProcessingFailed(msg)) => {
                assert_eq!(msg, "gRPC connection failed: connection refused");
            }
            other => panic!("expected ProcessingFailed, got {:?}", other),
        }
    }

    #[test]
    fn finalize_epp_result_failure_no_failopen_generic() {
        // When no error detail is stored, falls back to generic message
        let mut ctx = make_ctx();
        ctx.epp_failed = true;
        ctx.done = true;

        let result = finalize_epp_result(&mut ctx, false);
        match result {
            Err(EppError::ProcessingFailed(msg)) => {
                assert_eq!(msg, "EPP gRPC call failed");
            }
            other => panic!("expected ProcessingFailed, got {:?}", other),
        }
    }

    #[test]
    fn finalize_epp_result_failure_with_failopen() {
        let mut ctx = make_ctx();
        ctx.epp_failed = true;
        ctx.error_detail = Some("resolver timeout".to_string());
        ctx.done = true;

        let result = finalize_epp_result(&mut ctx, true);
        match result {
            Err(EppError::FailOpen(msg)) => {
                assert_eq!(msg, "resolver timeout");
            }
            other => panic!("expected FailOpen, got {:?}", other),
        }
    }

    #[test]
    fn finalize_epp_result_no_endpoint_no_failure() {
        let mut ctx = make_ctx();
        ctx.done = true;

        let result = finalize_epp_result(&mut ctx, false);
        assert!(result.is_ok());
        assert!(ctx.selected_endpoint.is_none());
    }

    // --- RequestCtx default ---

    #[test]
    fn request_ctx_default_state() {
        let ctx = RequestCtx::default();
        assert!(!ctx.done);
        assert!(!ctx.epp_failed);
        assert!(ctx.error_detail.is_none());
        assert!(!ctx.task_spawned);
        assert!(ctx.selected_endpoint.is_none());
        assert!(!ctx.body_reading_started);
        assert!(!ctx.body_ready);
        assert!(ctx.task.is_none());
        assert!(!ctx.cancelled.get());
        assert!(!ctx.response_phase_spawned);
        assert!(ctx.epp_stream.is_none());
    }

    #[test]
    fn request_ctx_drop_sets_cancelled() {
        let cancelled_flag;
        {
            let ctx = RequestCtx::default();
            cancelled_flag = Rc::clone(&ctx.cancelled);
            assert!(!cancelled_flag.get());
            // ctx is dropped here
        }
        assert!(cancelled_flag.get());
    }

    // --- RequestCtx mutation and lifecycle ---

    #[test]
    fn request_ctx_set_endpoint() {
        let mut ctx = make_ctx();
        ctx.selected_endpoint = Some("10.0.0.1:8080".to_string());
        ctx.done = true;

        assert_eq!(ctx.selected_endpoint.as_deref(), Some("10.0.0.1:8080"));
        assert!(ctx.done);
    }

    #[test]
    fn request_ctx_error_detail_taken_once() {
        let mut ctx = make_ctx();
        ctx.error_detail = Some("connection refused".to_string());
        ctx.epp_failed = true;

        // First take gets the detail
        let detail = ctx.error_detail.take();
        assert_eq!(detail.as_deref(), Some("connection refused"));

        // Second take returns None
        assert!(ctx.error_detail.is_none());
    }

    // --- EppError Debug ---

    #[test]
    fn epp_error_debug_format() {
        let err = EppError::ProcessingFailed("test".to_string());
        let debug = format!("{err:?}");
        assert!(debug.contains("ProcessingFailed"));
        assert!(debug.contains("test"));
    }

    #[test]
    fn epp_error_failopen_debug() {
        let err = EppError::FailOpen("timeout".to_string());
        let debug = format!("{err:?}");
        assert!(debug.contains("FailOpen"));
        assert!(debug.contains("timeout"));
    }

    // --- finalize_epp_result edge cases ---

    #[test]
    fn finalize_epp_result_error_detail_consumed() {
        let mut ctx = make_ctx();
        ctx.epp_failed = true;
        ctx.error_detail = Some("timeout".to_string());
        ctx.done = true;

        let _ = finalize_epp_result(&mut ctx, false);
        // error_detail should have been taken
        assert!(ctx.error_detail.is_none());
    }

    #[test]
    fn finalize_epp_result_endpoint_takes_precedence_over_failure() {
        // If endpoint is set but epp_failed is also true, endpoint wins
        let mut ctx = make_ctx();
        ctx.selected_endpoint = Some("10.0.0.1:8080".to_string());
        ctx.epp_failed = true;
        ctx.done = true;

        let result = finalize_epp_result(&mut ctx, false);
        assert!(result.is_ok());
    }

    #[test]
    fn epp_error_internal_debug() {
        let err = EppError::Internal("segfault".to_string());
        let debug = format!("{err:?}");
        assert!(debug.contains("Internal"));
        assert!(debug.contains("segfault"));
    }

    #[test]
    fn request_ctx_response_phase_default_false() {
        let ctx = RequestCtx::default();
        assert!(!ctx.response_phase_spawned);
    }

    #[test]
    fn request_ctx_body_flags_independent() {
        let mut ctx = make_ctx();
        ctx.body_reading_started = true;
        assert!(!ctx.body_ready);
        ctx.body_ready = true;
        assert!(ctx.body_reading_started);
    }

    #[test]
    fn valid_resolver_ptr_null_returns_none() {
        let ptr: *mut nginx_sys::ngx_resolver_t = std::ptr::null_mut();
        assert!(super::valid_resolver_ptr(ptr).is_none());
    }

    #[test]
    fn valid_resolver_ptr_unset_sentinel_returns_none() {
        // NGX_CONF_UNSET_PTR = (void *) -1 = usize::MAX
        let ptr = usize::MAX as *mut nginx_sys::ngx_resolver_t;
        assert!(super::valid_resolver_ptr(ptr).is_none());
    }

    #[test]
    fn valid_resolver_ptr_dummy_resolver_returns_none() {
        // Simulate nginx's dummy resolver: valid pointer but connections.nelts == 0.
        // ngx_pcalloc zeroes the struct, so MaybeUninit::zeroed is equivalent.
        let mut resolver: std::mem::MaybeUninit<nginx_sys::ngx_resolver_t> =
            std::mem::MaybeUninit::zeroed();
        let ptr = resolver.as_mut_ptr();
        assert!(super::valid_resolver_ptr(ptr).is_none());
    }

    #[test]
    fn valid_resolver_ptr_with_nameservers_returns_some() {
        // Simulate a resolver with at least one nameserver configured.
        let mut resolver: std::mem::MaybeUninit<nginx_sys::ngx_resolver_t> =
            std::mem::MaybeUninit::zeroed();
        // SAFETY: we only set connections.nelts on the zeroed struct; no other
        // field is read before we call valid_resolver_ptr, which only reads
        // connections.nelts.
        unsafe {
            let ptr = resolver.as_mut_ptr();
            (*ptr).connections.nelts = 1;
        }
        let ptr = resolver.as_mut_ptr();
        assert!(super::valid_resolver_ptr(ptr).is_some());
    }
}
