//! Response-path (output) inspection: two-state header filter, buffering body
//! filter, async subrequest spawning, and the deferred output commit.
//!
//! Only genuine `2xx`-with-body responses are inspected. `1xx`, `204`, `304`,
//! `3xx` (redirects / flow-control), and `>= 400` (errors, including our own
//! injected 403) all pass straight through — they carry no inspectable LLM
//! payload, and buffering a no-body status would suppress headers that are never
//! committed (hanging the client). The status gate lives in
//! [`crate::decision::should_inspect_status`] so the header and body filters
//! stay in lockstep.
//!
//! Header-only responses (HEAD requests) also pass straight through. Their
//! no-body-ness comes from the request method, not the status code, so the
//! status gate cannot catch them; both filters additionally check
//! `request.header_only()`. Suppressing a header-only `2xx` would strand the
//! headers (the body filter that commits them is never invoked), hanging the
//! client.

use std::ptr;

use ngx::core::Status;
use ngx::ffi::{
    NGX_HTTP_FORBIDDEN, NGX_LOG_DEBUG_HTTP, NGX_LOG_ERR, NGX_LOG_INFO, NGX_LOG_WARN, ngx_chain_t,
    ngx_http_finalize_request, ngx_http_request_t, ngx_int_t, ngx_post_event, ngx_posted_events,
    ngx_uint_t,
};
use ngx::http::{self, HttpModule, HttpModuleLocationConf, Request};
use ngx::ngx_log_error;

use crate::Module;
use crate::config::MAX_RESPONSE_BYTES;
use crate::ctx::{
    alloc_stream_ctx, call_next_header_filter, call_next_response_body_filter, get_module_ctx_mut,
    is_sse_response,
};
use crate::decision::{
    BlockCommitKind, ResponseAction, ResponseFilterInputs, block_commit_kind,
    decide_response_action, should_inspect_status,
};
use crate::stream::{
    ResponseInspect, ResponseVerdict, StreamContext, non_streaming_error_body, termination_message,
};
use crate::subrequest_client::{ScanDirection, run_inspection};
use crate::sync_ptr::AssertSendSync;

/// Two-state header filter.
///
/// **First pass** (upstream response headers arrive):
///   - Only `2xx`-with-body responses are candidates; anything the status gate
///     ([`should_inspect_status`]) rejects (`1xx`, `204`, `304`, `3xx`, `>= 400`)
///     passes through unmodified.
///   - Header-only responses (HEAD) pass through unmodified: they have no body
///     filter to commit suppressed headers, so buffering them hangs the client.
///   - SSE responses pass through immediately — they cannot be fully buffered.
///   - All other (buffer-eligible `2xx`) responses are suppressed: we return `NGX_OK`
///     without calling the next filter, so `r->header_sent` stays `0` and nothing is
///     written to the socket. `ctx.headers_suppressed` is set to `true`.
///
/// When the body filter is ready to commit headers (either the original 200 or a
/// replacement 403), it calls `call_next_header_filter(r)` **directly** — bypassing
/// this function and going straight to the rest of the chain.  This is the same pattern
/// used by `ngx_http_image_filter_module`.  There is no second pass through this function.
pub(crate) unsafe extern "C" fn guardrails_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
    unsafe {
        if r.is_null() {
            return call_next_header_filter(r);
        }

        let request = &mut *r.cast::<Request>();

        // Always pass through for subrequests / internal redirects.
        if !request.is_main() {
            return call_next_header_filter(r);
        }

        // Pass through if response inspection is not configured for this location.
        let conf = match Module::location_conf(request) {
            Some(c) => c,
            None => return call_next_header_filter(r),
        };
        if !conf.inspect_responses() {
            return call_next_header_filter(r);
        }

        // Only buffer/suppress genuine 2xx-with-body responses. 1xx, 204, 304, 3xx
        // and >= 400 (including our own 403 injection from send_403_and_finalize)
        // carry no inspectable body and must reach the client unmodified; buffering
        // a no-body status would strand the suppressed headers (client hang).
        if !should_inspect_status((*r).headers_out.status) {
            return call_next_header_filter(r);
        }

        // Header-only responses (HEAD requests) carry no response body, so the
        // body filter that commits these suppressed headers is never invoked.
        // Suppressing here would strand the headers and hang the client. Pass
        // through before any ctx allocation. Kept in lockstep with the body
        // filter's identical guard.
        if request.header_only() {
            return call_next_header_filter(r);
        }

        // SSE: always pass through — streaming responses cannot be fully buffered.
        if is_sse_response(r) {
            ngx_log_error!(
                NGX_LOG_DEBUG_HTTP,
                request.log(),
                "guardrails: header filter: SSE detected, passing through"
            );
            return call_next_header_filter(r);
        }

        // Get or allocate per-request context.
        let ctx_ptr = get_module_ctx_mut(request, Module::module());
        let ctx = if ctx_ptr.is_null() {
            let new_ctx = alloc_stream_ctx(r);
            if new_ctx.is_null() {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: header filter: ctx alloc failed, passing through"
                );
                return call_next_header_filter(r);
            }
            &mut *new_ctx
        } else {
            &mut *ctx_ptr
        };

        // Suppress upstream headers; the body filter will commit them after inspection.
        ngx_log_error!(
            NGX_LOG_DEBUG_HTTP,
            request.log(),
            "guardrails: header filter: suppressing upstream headers (status={})",
            (*r).headers_out.status
        );
        ctx.headers_suppressed = true;
        Status::NGX_OK.into()
    }
}

/// Response body filter handler - called for each response chunk
pub(crate) unsafe extern "C" fn guardrails_response_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    unsafe {
        if r.is_null() {
            return Status::NGX_ERROR.into();
        }

        let request = &mut *r.cast::<Request>();

        // Only process main requests
        if !request.is_main() {
            return call_next_response_body_filter(r, in_chain);
        }

        // Only inspect genuine 2xx-with-body responses. Must match the header
        // filter's gate exactly (see should_inspect_status and the header_only
        // guard below): if the two disagree, suppressed headers never get
        // committed. Skips 1xx, 204, 304, 3xx and >= 400 (e.g. the 403 from a
        // blocked request).
        if !should_inspect_status((*r).headers_out.status) {
            return call_next_response_body_filter(r, in_chain);
        }

        // Mirror the header filter's header-only pass-through (see there). NGINX
        // normally will not invoke the body filter for a header-only (HEAD)
        // request; this keeps the two gates textually consistent and is defensive.
        if request.header_only() {
            return call_next_response_body_filter(r, in_chain);
        }

        // Get module configuration
        let conf = match Module::location_conf(request) {
            Some(c) => c,
            None => {
                ngx_log_error!(
                    NGX_LOG_DEBUG_HTTP,
                    request.log(),
                    "guardrails: no location config found, passing through"
                );
                return call_next_response_body_filter(r, in_chain);
            }
        };

        // Skip if not enabled or not inspecting responses
        if !conf.inspect_responses() {
            ngx_log_error!(
                NGX_LOG_DEBUG_HTTP,
                request.log(),
                "guardrails: response inspection disabled (enabled={}), passing through",
                conf.enabled
            );
            return call_next_response_body_filter(r, in_chain);
        }

        // Get or create context
        let ctx_ptr = get_module_ctx_mut(request, Module::module());

        let ctx = if ctx_ptr.is_null() {
            // First chunk - allocate context (heap-boxed + pool cleanup so the
            // StreamContext's Drop runs at teardown, cancelling any Task).
            let new_ctx = alloc_stream_ctx(r);
            if new_ctx.is_null() {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: failed to allocate context"
                );
                return call_next_response_body_filter(r, in_chain);
            }
            &mut *new_ctx
        } else {
            &mut *ctx_ptr
        };

        // --- Pre-ingest short-circuits ----------------------------------------
        // `inspection_pending` and `already_blocked` are decided *before* the
        // upstream chain is ingested (a held/blocked stream must not accumulate
        // further). `decide_response_action` owns the precedence; the remaining
        // (post-ingest) inputs are false here and are re-evaluated after ingest.
        let pre = decide_response_action(ResponseFilterInputs {
            inspection_pending: ctx.inspect_state == ResponseInspect::Pending,
            already_blocked: ctx.blocked,
            over_limit: false,
            should_inspect: false,
            last_buf: false,
            stream_done: false,
        });
        match pre {
            ResponseAction::HoldForPending => {
                // An async inspection is already in flight: buffer any late
                // arrivals (there should be none after last_buf) and return
                // without forwarding. The async completion drives output.
                ngx_log_error!(
                    NGX_LOG_DEBUG_HTTP,
                    request.log(),
                    "guardrails: body filter re-entered while inspection pending; holding"
                );
                return Status::NGX_OK.into();
            }
            ResponseAction::EmitBlock => {
                // Already blocked on a prior chain: send termination and stop.
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "guardrails: stream blocked, sending termination"
                );
                return send_termination(r, request, ctx.block_message.as_deref()).into_filter_rc();
            }
            // No pre-ingest short-circuit: fall through to ingest the chain and
            // re-decide with the post-ingest inputs below.
            _ => {}
        }

        // --- Ingest all buffers from the upstream chain -------------------------
        let mut chain = in_chain;
        let mut last_buf = false;

        while !chain.is_null() {
            let buf = (*chain).buf;
            if !buf.is_null() {
                let buffer = &*buf;

                if buffer.last_buf() != 0 || buffer.last_in_chain() != 0 {
                    last_buf = true;
                }

                if !buffer.pos.is_null() && !buffer.last.is_null() {
                    let len = buffer.last.offset_from(buffer.pos) as usize;
                    let data = std::slice::from_raw_parts(buffer.pos, len);
                    // process_chunk: adds raw bytes to pending_chunks AND parses
                    // complete JSON lines for text extraction / object counting.
                    ctx.process_chunk(data);
                    // Advance pos to last to mark this upstream buffer as consumed.
                    // Without this NGINX thinks the buffer is still in use and stops
                    // reading from upstream once its ~4KB proxy_buffer_size fills up.
                    (*buf).pos = (*buf).last;
                }
            }
            chain = (*chain).next;
        }

        ngx_log_error!(
            NGX_LOG_DEBUG_HTTP,
            request.log(),
            "guardrails: chain processed: last_buf={}, pending_chunks={}, accumulated={}, buffered_bytes={}",
            last_buf,
            ctx.pending_chunks.len(),
            ctx.accumulated_text.len(),
            ctx.total_buffered_bytes
        );

        // --- Post-ingest decision ---------------------------------------------
        // Flush any bytes still in line_buffer that were never terminated by a
        // newline BEFORE evaluating the checkpoint. This handles non-streaming
        // responses (e.g. /v1/completions) that arrive as a single JSON blob
        // without a trailing newline.
        if last_buf {
            ctx.try_drain_remaining();
        }

        let over_limit = ctx.total_buffered_bytes > MAX_RESPONSE_BYTES;
        let action = decide_response_action(ResponseFilterInputs {
            // Already resolved to "no short-circuit" in the pre-ingest pass.
            inspection_pending: false,
            already_blocked: false,
            over_limit,
            should_inspect: ctx.should_inspect_final(last_buf),
            last_buf,
            stream_done: ctx.stream_done,
        });

        match action {
            ResponseAction::BlockOverLimit => {
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "guardrails: response buffer limit ({} bytes) exceeded, blocking stream",
                    MAX_RESPONSE_BYTES
                );
                ctx.blocked = true;
                ctx.clear_pending_chunks();
                return commit_block(r, request, ctx).into_filter_rc();
            }
            ResponseAction::FlushBuffered => {
                // At end-of-stream we must still release the buffered response
                // even when there is nothing to inspect (e.g. a `/v1/models` JSON
                // body that yields no LLM-extractable text, so `accumulated_text`
                // is empty). Otherwise the suppressed headers are never committed
                // and the buffered bytes are stranded, hanging the client.
                // Commit the upstream headers we previously suppressed.
                if ctx.headers_suppressed {
                    let hdr_rc = call_next_header_filter(r);
                    if hdr_rc == ngx_int_t::from(Status::NGX_ERROR) {
                        return Status::NGX_ERROR.into();
                    }
                }

                let chunks_to_send = ctx.take_pending_chunks();
                if chunks_to_send.is_empty() {
                    return Status::NGX_OK.into();
                }

                return send_chunks(r, request, &chunks_to_send, true);
            }
            ResponseAction::KeepBuffering => {
                // Mid-stream and below the inspection threshold — keep buffering,
                // return nothing to the client.
                return Status::NGX_OK.into();
            }
            // Falls through to the async suspend/spawn below.
            ResponseAction::SpawnInspection => {}
            // HoldForPending / EmitBlock are resolved in the pre-ingest pass and
            // cannot reach here; fail loud if that invariant is ever violated.
            ResponseAction::HoldForPending | ResponseAction::EmitBlock => {
                unreachable!("pre-ingest short-circuit actions resolved earlier")
            }
        }

        // --- Suspend output and inspect asynchronously -------------------------
        //
        // The response is fully buffered (headers suppressed for non-SSE; every
        // chunk held in `ctx.pending_chunks`). Rather than block the worker on a
        // synchronous guardrails call, return NGX_OK without forwarding the final
        // buffer and spawn an async subrequest (the subrequest keeps the request
        // alive). When it completes, `resume_output` records the verdict and posts
        // a write event; `guardrails_resume_write_handler` then commits headers,
        // flushes the buffered body (or sends the block/termination body), and
        // finalizes the request exactly once — off the subrequest-finalize stack.
        //
        // The spawned `run_inspection(...).await` (below) has no Rust-side
        // timeout; it is bounded only by the internal location's `proxy_*_timeout`
        // (see `config.rs`) or by request teardown (which cancels the task). Until
        // it resolves, the body filter forwards nothing and holds every buffered
        // chunk.
        ngx_log_error!(
            NGX_LOG_INFO,
            request.log(),
            "guardrails: inspecting full stream (async), accumulated={}",
            ctx.accumulated_text.len()
        );

        // The internal guardrails location must be configured; if not, fail
        // closed (do not silently release unfiltered content).
        let internal_uri = match &conf.internal_uri {
            Some(u) => u.clone(),
            None => {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: guardrails_internal_uri not configured (fail-closed)"
                );
                ctx.blocked = true;
                ctx.clear_pending_chunks();
                return commit_block(r, request, ctx).into_filter_rc();
            }
        };

        // Capture owned parameters for the async task — `conf` (and the borrowed
        // `ctx` fields) must not be held across the await boundary.
        let content = ctx.accumulated_text.clone();
        let api_token = conf.api_token.clone();

        // The request is held open across the async gap by the subrequest itself:
        // `ngx_http_subrequest` does `r->main->count++`, and that reference is
        // released when the subrequest finalizes (which is what wakes our task).
        // `guardrails_resume_write_handler` then issues the single
        // `ngx_http_finalize_request` that releases the main request's own
        // outstanding reference. We therefore do NOT manipulate `r->count` here.
        ctx.inspect_state = ResponseInspect::Pending;

        let r_send = AssertSendSync(r);
        let task = ngx::async_::spawn(async move {
            let r = r_send.0;
            // Await + fail-closed error log + verdict mapping are shared with the
            // request path via `run_inspection`.
            let decision = run_inspection(
                r,
                &internal_uri,
                &content,
                api_token.as_deref(),
                ScanDirection::Response,
            )
            .await;
            let verdict = if decision.allow {
                ResponseVerdict::Allow
            } else {
                ResponseVerdict::Block
            };
            resume_output(r, verdict, decision.message);
        });

        // Store the task on the ctx so it is not cancelled by being dropped.
        ctx.inspect_task = Some(task);

        Status::NGX_OK.into()
    }
}

/// Record the async inspection verdict and **defer** the actual output commit to
/// a posted write event.
///
/// # Why defer (critical)
///
/// This runs from the spawned task's continuation, which is polled **inline** by
/// the wake from `PostSubrequest::handler` — invoked by NGINX from **inside the
/// subrequest's `ngx_http_finalize_request`**, before its own `r->main->count--`.
/// Driving the parent's output chain / finalizing the parent from that nested
/// stack is unsafe. So we only record the verdict, arm
/// `guardrails_resume_write_handler`, and post the connection write event; it then
/// runs on the next clean event-loop iteration, after the subrequest has fully
/// finalized. Mirrors the deferral `start_subrequest` does after
/// `ngx_http_subrequest` (`subrequest_client.rs`).
unsafe fn resume_output(
    r: *mut ngx_http_request_t,
    verdict: ResponseVerdict,
    message: Option<String>,
) {
    unsafe {
        let request = &mut *r.cast::<Request>();

        // Fetch the response context; if it is somehow gone, finalize defensively.
        let ctx_ptr = get_module_ctx_mut(request, Module::module());
        if ctx_ptr.is_null() {
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: resume_output: null ctx (fail-closed finalize)"
            );
            ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t);
            return;
        }
        let ctx = &mut *ctx_ptr;

        ctx.block_message = message;
        ctx.inspect_state = ResponseInspect::Done(verdict);
        // Drop the task handle: it has completed (we are running from it) and
        // keeping it would leave a self-referential handle on the ctx.
        ctx.inspect_task = None;

        // Arm the resume handler and post the write event so NGINX drives it on
        // the next event-loop iteration (NOT from inside the subrequest finalize).
        let conn = (*r).connection;
        if conn.is_null() || (*conn).write.is_null() {
            // No connection to post to — fall back to a direct finalize. Without a
            // write event we cannot flush a body anyway, so release the request.
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: resume_output: null connection/write (direct finalize)"
            );
            ctx.inspect_state = ResponseInspect::Resumed;
            ngx_http_finalize_request(r, Status::NGX_OK.into());
            return;
        }
        (*r).write_event_handler = Some(guardrails_resume_write_handler);
        ngx_post_event((*conn).write, ptr::addr_of_mut!(ngx_posted_events));
    }
}

/// Write-event handler that performs the deferred output commit + single finalize
/// for the response path. Armed by `resume_output` and driven by a posted write
/// event, so it runs cleanly in the worker event loop (not nested inside the
/// subrequest's finalize).
///
/// # Finalize contract (critical)
///
/// The suspended body filter returned `NGX_OK` without forwarding the final
/// buffer, so the main request still holds its in-flight `r->count` reference.
/// This handler MUST `ngx_http_finalize_request` **exactly once** to release it;
/// the `ResponseInspect` state machine guards this (acts only on `Done`, then
/// moves to `Resumed`, so a spurious second write event is a no-op).
///
/// The finalize **code matters**: always `NGX_OK`. The `send_*` helpers return an
/// `NGX_ERROR` sentinel meaning "body queued, don't finalize" (a contract for the
/// old sync filter) which we ignore here. Finalizing with `NGX_ERROR` routes to
/// `ngx_http_terminate_request`, tearing the connection down **before** the queued
/// body is written (empty `403 0` + client hang); `NGX_OK` reaches the
/// `r->buffered` flush path so the JSON/SSE body is written first.
unsafe extern "C" fn guardrails_resume_write_handler(r: *mut ngx_http_request_t) {
    unsafe {
        let request = &mut *r.cast::<Request>();

        let ctx_ptr = get_module_ctx_mut(request, Module::module());
        if ctx_ptr.is_null() {
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: resume_write: null ctx (fail-closed finalize)"
            );
            ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t);
            return;
        }
        let ctx = &mut *ctx_ptr;

        // Only act on a fresh verdict. Anything else (Idle/Pending/Resumed) means
        // this write event is spurious or a duplicate — do nothing so we finalize
        // exactly once.
        let verdict = match ctx.inspect_state {
            ResponseInspect::Done(v) => v,
            _ => return,
        };
        // Mark consumed up front so a re-entrant write event cannot double-run.
        ctx.inspect_state = ResponseInspect::Resumed;

        match verdict {
            ResponseVerdict::Allow => {
                ngx_log_error!(
                    NGX_LOG_INFO,
                    request.log(),
                    "guardrails: content cleared (async)"
                );

                // Commit the previously-suppressed upstream headers (non-SSE).
                if ctx.headers_suppressed {
                    let hdr_rc = call_next_header_filter(r);
                    if hdr_rc == ngx_int_t::from(Status::NGX_ERROR) {
                        // Genuine header-filter failure: terminate.
                        ngx_http_finalize_request(r, Status::NGX_ERROR.into());
                        return;
                    }
                }

                let chunks_to_send = ctx.take_pending_chunks();
                if !chunks_to_send.is_empty() {
                    // mark_last: the stream is complete at this checkpoint.
                    send_chunks(r, request, &chunks_to_send, true);
                }
                // Body queued (or nothing to send). Finalize NGX_OK so NGINX
                // flushes any buffered output, then completes the request.
                ngx_http_finalize_request(r, Status::NGX_OK.into());
            }
            ResponseVerdict::Block => {
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "guardrails: content BLOCKED (async)"
                );
                ctx.blocked = true;
                ctx.clear_pending_chunks();
                // Non-SSE => 403 + JSON error body; SSE (200 already flushed) =>
                // an SSE termination frame carrying the backend's block message.
                let commit = commit_block(r, request, ctx);
                // The helper's BodyCommit is intentionally not turned into a
                // finalize code here: whatever the outcome (Queued / HeaderOnly /
                // Failed), this handler owns the single finalize and always uses
                // NGX_OK so any queued body is flushed before the connection closes
                // (finalizing with NGX_ERROR would terminate before the write).
                let _: BodyCommit = commit;
                ngx_http_finalize_request(r, Status::NGX_OK.into());
            }
        }
    }
}

/// Outcome of committing a block/termination body from a body-filter helper
/// (`send_termination` / `send_blocked_response`).
///
/// This replaces the previous overloaded `NGX_ERROR` sentinel, which conflated
/// "body queued successfully, caller must not finalize" with "a step failed".
/// Callers translate this into the correct action for their context:
///
///  * When **returned up the body-filter chain** (synchronous path), map it back
///    to the legacy `ngx_int_t` via [`BodyCommit::into_filter_rc`]: both `Queued`
///    and `Failed` become `NGX_ERROR` (NGINX flushes any queued body, then closes
///    — the behavior the Content-Length hang-avoidance relies on), and
///    `HeaderOnly` becomes `NGX_OK`.
///  * When invoked from the **async resume handler**, the handler issues its own
///    single `ngx_http_finalize_request(r, NGX_OK)` regardless of variant (so the
///    queued body is flushed off the subrequest-finalize stack); the enum simply
///    documents intent instead of an ignored `let _ =` on a magic code.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum BodyCommit {
    /// A complete body buffer was queued to the next filter. NGINX must flush it
    /// and then close the connection.
    Queued,
    /// The request was header-only (no body to send); headers were committed and
    /// nothing was queued.
    HeaderOnly,
    /// A step failed (buffer/chain allocation or a downstream header-filter
    /// error). The request should be terminated.
    Failed,
}

impl BodyCommit {
    /// Map to the legacy body-filter return code, preserving the exact
    /// NGINX-visible behavior of the previous `ngx_int_t` sentinel.
    fn into_filter_rc(self) -> ngx_int_t {
        match self {
            // Body queued: return NGX_ERROR so NGINX flushes then closes.
            // Failed: also NGX_ERROR so NGINX terminates. Both matched the old code.
            BodyCommit::Queued | BodyCommit::Failed => Status::NGX_ERROR.into(),
            BodyCommit::HeaderOnly => Status::NGX_OK.into(),
        }
    }
}

/// Emit the block/termination body for a response-path block, choosing between
/// the non-SSE 403 (`send_blocked_response`) and the SSE termination frame
/// (`send_termination`) via the shared, unit-tested [`block_commit_kind`]
/// predicate (headers-suppressed => non-SSE).
///
/// This replaces the `if ctx.headers_suppressed { .. } else { .. }` that was
/// duplicated at every response block site (over-limit, missing internal_uri,
/// async `Block`), keeping the SSE-vs-buffered distinction in one place.
unsafe fn commit_block(
    r: *mut ngx_http_request_t,
    request: &http::Request,
    ctx: &mut StreamContext,
) -> BodyCommit {
    unsafe {
        match block_commit_kind(ctx.headers_suppressed) {
            BlockCommitKind::BlockedResponse => send_blocked_response(r, request, ctx),
            BlockCommitKind::Termination => {
                send_termination(r, request, ctx.block_message.as_deref())
            }
        }
    }
}

/// Write the appropriate error body into an NGINX buffer and forward it to the next filter.
/// Uses SSE format (`data: {...}`) for event-stream responses and plain JSON for all others.
///
/// HTTP status vs. error `type` matrix across all three block paths:
///
/// | Path                            | Function                 | HTTP status | error.type            |
/// |---------------------------------|--------------------------|-------------|-----------------------|
/// | Request blocked (input)         | `send_403_and_finalize`  | 403         | `invalid_request_error` |
/// | Response blocked, non-SSE       | `send_blocked_response`  | 403         | `api_error`           |
/// | Response blocked, SSE stream    | `send_termination`       | 200 (*)     | `api_error`           |
///
/// (*) For SSE responses the upstream headers were already flushed to the client with a
/// 200 status (SSE cannot be buffered, so the header filter lets it through immediately).
/// Once headers are committed the status can no longer be changed, so the block is signaled
/// via an `api_error` body inside an SSE `data:` frame. A 200 response carrying an
/// `api_error` body is therefore an unavoidable consequence of SSE header timing, NOT a bug.
/// Request-side (`invalid_request_error`) vs output-side (`api_error`) typing is intentional:
/// only the input-block path represents a bad client request.
unsafe fn send_termination(
    r: *mut ngx_http_request_t,
    request: &http::Request,
    message: Option<&str>,
) -> BodyCommit {
    unsafe {
        let is_sse = is_sse_response(r);
        let term_body: Vec<u8> = if is_sse {
            termination_message(message)
        } else {
            non_streaming_error_body(message)
        };
        let term_msg: &[u8] = term_body.as_slice();
        let pool = request.pool();
        let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), term_msg.len());
        if buf.is_null() {
            return BodyCommit::Failed;
        }
        ptr::copy_nonoverlapping(term_msg.as_ptr(), (*buf).pos, term_msg.len());
        (*buf).last = (*buf).pos.add(term_msg.len());
        (*buf).set_last_buf(1);
        (*buf).set_flush(1);
        let out = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
        if out.is_null() {
            return BodyCommit::Failed;
        }
        (*out).buf = buf;
        (*out).next = ptr::null_mut();
        // Forward the termination buffer to the client. The caller returns
        // NGX_ERROR up the body-filter chain (via BodyCommit::into_filter_rc) so
        // NGINX closes the connection. This is critical for non-streaming
        // responses that carry a Content-Length header: without closing the
        // connection, curl waits for the remaining promised bytes and hangs.
        // Do NOT call ngx_http_finalize_request here — calling it from inside the
        // body filter chain is unsafe and causes double-finalization on keep-alive
        // connections (finalize with NGX_OK keeps the connection open).
        call_next_response_body_filter(r, out);
        BodyCommit::Queued
    }
}

/// Commit a 403 response via the standard NGINX body-filter header commit pattern.
///
/// Called from the response body filter after inspection blocks a non-SSE response.
/// At this point `r->header_sent == 0` because `guardrails_header_filter` suppressed
/// the upstream headers on the first pass, so we can still set the status to 403.
///
/// Error `type` note: this is an *output-side* block (the client request was valid;
/// the model's response was blocked), so the body uses `type: "api_error"` — NOT
/// `invalid_request_error`, which is reserved for the request-side block in
/// `send_403_and_finalize`. The body is built by the shared
/// `stream::non_streaming_error_body(message)` helper, which injects the backend's
/// configurable block message (or the hardcoded fallback when `None`).
///
/// Steps:
///   1. Overwrite `headers_out` with 403 status + correct `Content-Length`.
///   2. Call `call_next_header_filter(r)` **directly** — this skips our own header
///      filter (which has already done its job) and goes straight to the rest of the
///      chain, ending at `ngx_http_header_filter` which writes "403 Forbidden" to wire.
///      This is the same pattern used by `ngx_http_image_filter_module`.
///   3. Queue the JSON error body and return [`BodyCommit::Queued`].
///
/// Return-value contract: on success this returns [`BodyCommit::Queued`], the
/// typed successor to the old overloaded `NGX_ERROR` sentinel ("body queued; the
/// caller must not finalize"). When returned up the synchronous body-filter chain
/// it maps back to `NGX_ERROR` (NGINX flushes the queued body then tears down).
/// The async caller (`guardrails_resume_write_handler`) instead finalizes with
/// `NGX_OK` itself — finalizing with `NGX_ERROR` would terminate the request
/// before the queued body is written (empty `403 0` + client hang). A header-only
/// request returns [`BodyCommit::HeaderOnly`]; an allocation/header-filter failure
/// returns [`BodyCommit::Failed`].
unsafe fn send_blocked_response(
    r: *mut ngx_http_request_t,
    request: &http::Request,
    ctx: &mut StreamContext,
) -> BodyCommit {
    unsafe {
        // Non-SSE output-side block body (`type: api_error`), carrying the
        // backend's configurable message when present (else the hardcoded
        // fallback inside `non_streaming_error_body`).
        let json_body = non_streaming_error_body(ctx.block_message.as_deref());
        let json_body = json_body.as_slice();

        ngx_log_error!(
            NGX_LOG_DEBUG_HTTP,
            request.log(),
            "guardrails: send_blocked_response: committing 403 via direct next-header-filter call"
        );

        (*r).headers_out.status = NGX_HTTP_FORBIDDEN as ngx_uint_t;
        (*r).headers_out.content_length_n = json_body.len() as i64;
        // Clear the pre-built status_line string that the proxy module set to "200 OK".
        // If status_line.len > 0, ngx_http_header_filter writes that string verbatim to the
        // socket regardless of headers_out.status.  Zeroing it forces NGINX to derive the
        // status line from the integer status code instead.
        (*r).headers_out.status_line.len = 0;
        (*r).headers_out.status_line.data = ptr::null_mut();

        // Call directly into the rest of the header filter chain — NOT through
        // ngx_http_send_header / ngx_http_top_header_filter, which would re-enter
        // our own guardrails_header_filter and cause double-processing.
        let hdr_rc = call_next_header_filter(r);
        if hdr_rc == ngx_int_t::from(Status::NGX_ERROR) {
            return BodyCommit::Failed;
        }
        if request.header_only() {
            return BodyCommit::HeaderOnly;
        }

        // Write the JSON error body.
        let pool = request.pool();
        let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), json_body.len());
        if buf.is_null() {
            return BodyCommit::Failed;
        }
        ptr::copy_nonoverlapping(json_body.as_ptr(), (*buf).pos, json_body.len());
        (*buf).last = (*buf).pos.add(json_body.len());
        (*buf).set_last_buf(1);
        (*buf).set_flush(1);
        (*buf).set_memory(1);

        let out = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
        if out.is_null() {
            return BodyCommit::Failed;
        }
        (*out).buf = buf;
        (*out).next = ptr::null_mut();

        call_next_response_body_filter(r, out);
        BodyCommit::Queued
    }
}

/// Build an ngx_chain_t from `chunks` and pass it to the next body filter.
///
/// The last buffer in the chain is marked with `last_buf` only when
/// `mark_last` is true (stream is complete).
unsafe fn send_chunks(
    r: *mut ngx_http_request_t,
    request: &http::Request,
    chunks: &[Vec<u8>],
    mark_last: bool,
) -> ngx_int_t {
    unsafe {
        ngx_log_error!(
            NGX_LOG_DEBUG_HTTP,
            request.log(),
            "guardrails: sending {} chunks to client (mark_last={})",
            chunks.len(),
            mark_last
        );

        let pool = request.pool();
        let mut first_link: *mut ngx_chain_t = ptr::null_mut();
        let mut prev_link: *mut ngx_chain_t = ptr::null_mut();
        let last_idx = chunks.len().saturating_sub(1);

        for (idx, chunk_data) in chunks.iter().enumerate() {
            let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), chunk_data.len());
            if buf.is_null() {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: ngx_create_temp_buf failed"
                );
                continue;
            }

            ptr::copy_nonoverlapping(chunk_data.as_ptr(), (*buf).pos, chunk_data.len());
            (*buf).last = (*buf).pos.add(chunk_data.len());

            // flush=1 tells NGINX to push this data to the client socket immediately;
            // without it the worker buffers the chain and nothing reaches the client.
            (*buf).set_flush(1);

            if idx == last_idx && mark_last {
                (*buf).set_last_buf(1);
            }

            let link = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
            if link.is_null() {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: ngx_alloc_chain_link failed"
                );
                continue;
            }

            (*link).buf = buf;
            (*link).next = ptr::null_mut();

            if first_link.is_null() {
                first_link = link;
            } else {
                (*prev_link).next = link;
            }
            prev_link = link;
        }

        if first_link.is_null() {
            return Status::NGX_OK.into();
        }

        call_next_response_body_filter(r, first_link)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_body_commit_filter_rc_mapping() {
        // Queued and Failed both map to NGX_ERROR (NGINX flushes any queued body
        // then closes / terminates) — preserving the legacy sentinel behavior.
        // HeaderOnly maps to NGX_OK.
        assert_eq!(
            BodyCommit::Queued.into_filter_rc(),
            ngx_int_t::from(Status::NGX_ERROR)
        );
        assert_eq!(
            BodyCommit::Failed.into_filter_rc(),
            ngx_int_t::from(Status::NGX_ERROR)
        );
        assert_eq!(
            BodyCommit::HeaderOnly.into_filter_rc(),
            ngx_int_t::from(Status::NGX_OK)
        );
    }
}
