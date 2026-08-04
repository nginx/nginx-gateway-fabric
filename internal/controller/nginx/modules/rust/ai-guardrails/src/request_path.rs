//! Request-path (input) inspection: ACCESS-phase handler, async subrequest
//! spawning, and the 403 block response.

use std::borrow::Cow;
use std::ptr;

use ngx::core::Status;
use ngx::ffi::{
    NGX_HTTP_FORBIDDEN, NGX_LOG_DEBUG_HTTP, NGX_LOG_ERR, NGX_LOG_INFO, NGX_LOG_WARN, ngx_chain_t,
    ngx_http_finalize_request, ngx_http_request_t, ngx_int_t, ngx_uint_t,
};
use ngx::http::{self, HttpModule, HttpModuleLocationConf, Request};
use ngx::ngx_log_error;

use crate::Module;
use crate::ctx::call_next_request_body_filter;
use crate::decision::{
    AccessAction, RequestVerdict, decide_access_action, verdict_from_inspection,
};
use crate::stream;
use crate::subrequest_client::{ScanDirection, inspect_content_async};
use crate::sync_ptr::AssertSendSync;

/// Typed request body for chat/completion API formats.
///
/// Uses owned `String` fields (not borrowed `&str`): serde cannot zero-copy
/// borrow a JSON string that contains escapes (`\n`, `\"`, `\uXXXX`), and a
/// borrowed deserialize would *fail* on such inputs — silently falling back to
/// inspecting the raw JSON envelope instead of the prompt text. Owning the
/// strings makes prompt extraction reliable for real (escaped) prompts.
///
/// `messages[].content` accepts both the plain-string shape and the OpenAI
/// multimodal array shape (`[{"type":"text","text":"…"}, {"type":"image_url",…}]`);
/// see [`MessageContent`]. Only text parts are inspected — image/audio parts are
/// ignored so we scan the prompt text rather than base64 blobs.
#[derive(serde::Deserialize)]
struct RequestBody {
    prompt: Option<String>,
    messages: Option<Vec<RequestMessage>>,
}

#[derive(serde::Deserialize)]
struct RequestMessage {
    content: Option<MessageContent>,
}

/// OpenAI chat `content`: either a plain string or an array of typed parts
/// (multimodal). Untagged so serde tries the string form first, then the array
/// form; a `content` that is neither still fails deserialize and lands on the
/// raw-envelope fallback in [`extract_inspection_content`].
#[derive(serde::Deserialize)]
#[serde(untagged)]
enum MessageContent {
    Text(String),
    Parts(Vec<ContentPart>),
}

/// One element of an array-shaped `content`. Non-text parts (e.g. `image_url`)
/// have `text: None` and are ignored during extraction.
#[derive(serde::Deserialize)]
struct ContentPart {
    #[serde(rename = "type")]
    kind: String,
    text: Option<String>,
}

impl MessageContent {
    /// Flatten to the inspectable text: the string itself for the string form,
    /// or the newline-joined non-empty `text` fields of `type == "text"` parts
    /// for the array form (non-text parts ignored).
    fn into_text(self) -> String {
        match self {
            MessageContent::Text(s) => s,
            MessageContent::Parts(parts) => parts
                .into_iter()
                .filter(|p| p.kind == "text")
                .filter_map(|p| p.text)
                .filter(|t| !t.is_empty())
                .collect::<Vec<_>>()
                .join("\n"),
        }
    }
}

/// Extract the text content to inspect from a raw JSON request body.
///
/// Preference order: a non-empty `prompt`, else the newline-joined non-empty
/// `messages[].content` (string or multimodal-array shape; text parts only),
/// else the raw body string (so unknown shapes are still inspected). Returns
/// `None` when there is nothing meaningful to inspect.
fn extract_inspection_content(body_data: &[u8]) -> Option<String> {
    let body_str = std::str::from_utf8(body_data).ok()?;

    let content: Cow<'_, str> = match serde_json::from_str::<RequestBody>(body_str) {
        Ok(body) => {
            if let Some(prompt) = body.prompt.filter(|p| !p.is_empty()) {
                Cow::Owned(prompt)
            } else if let Some(messages) = body.messages {
                let extracted: String = messages
                    .into_iter()
                    .filter_map(|m| m.content)
                    .map(MessageContent::into_text)
                    .filter(|c| !c.is_empty())
                    .collect::<Vec<_>>()
                    .join("\n");
                if !extracted.is_empty() {
                    Cow::Owned(extracted)
                } else {
                    // Nothing inspectable (e.g. an image-only multimodal
                    // message): fall back to the raw envelope so nothing slips
                    // through unscanned.
                    Cow::Borrowed(body_str)
                }
            } else {
                Cow::Borrowed(body_str)
            }
        }
        // Genuinely unknown shape (not a known prompt/messages body): inspect
        // the raw envelope so nothing bypasses scanning.
        Err(_) => Cow::Borrowed(body_str),
    };

    if content.is_empty() {
        None
    } else {
        Some(content.into_owned())
    }
}

/// Verdict of an asynchronous request inspection.
///
/// This is the FFI-side storage enum held on `RequestInspectState`. The pure
/// access-handler state machine lives in [`crate::decision`]; this maps into its
/// [`RequestVerdict`] so the handler can defer the decision to
/// [`decide_access_action`].
#[derive(Clone, Copy, PartialEq, Eq)]
enum InspectVerdict {
    Pending,
    Allow,
    Block,
}

impl InspectVerdict {
    /// Project onto the pure decision enum used by [`decide_access_action`].
    fn as_decision(self) -> RequestVerdict {
        match self {
            InspectVerdict::Pending => RequestVerdict::Pending,
            InspectVerdict::Allow => RequestVerdict::Allow,
            InspectVerdict::Block => RequestVerdict::Block,
        }
    }
}

/// Per-request state for the asynchronous ACCESS-phase inspection.
///
/// A boxed instance is created when the access handler first fires, its pointer
/// stashed on the request via a pool cleanup handler so it is dropped when the
/// request ends (which cancels the spawned task if still running). The verdict
/// is written by the spawned task's completion and read on the access handler's
/// second invocation.
struct RequestInspectState {
    /// Async task performing the subrequest inspection. Kept alive here so it is
    /// not cancelled by being dropped (dropping an `async-task` `Task` cancels
    /// it); it is dropped with the state at request cleanup.
    task: Option<ngx::async_::Task<()>>,
    verdict: InspectVerdict,
    /// Configurable block message from the guardrails backend for this request,
    /// recorded by the async completion alongside a `Block` verdict. `None` when
    /// the backend supplied no usable message (fall back to hardcoded copy).
    block_message: Option<String>,
    /// True once the body read + spawn has been kicked off (guards re-entry).
    started: bool,
    /// Parameters captured for the async inspection, taken by the body-read
    /// handler when it spawns the task.
    params: Option<InspectParams>,
}

impl RequestInspectState {
    fn new() -> Self {
        Self {
            task: None,
            verdict: InspectVerdict::Pending,
            block_message: None,
            started: false,
            params: None,
        }
    }
}

/// Cleanup handler that drops the boxed `RequestInspectState` at request
/// teardown. Dropping the box drops any in-flight `Task`, cancelling it.
unsafe extern "C" fn request_inspect_state_cleanup(data: *mut std::ffi::c_void) {
    if !data.is_null() {
        // Reconstitute and drop the box.
        drop(unsafe { Box::from_raw(data as *mut RequestInspectState) });
    }
}

/// Retrieve the per-request inspection state, allocating it (and registering a
/// pool cleanup to free it) on first access.
///
/// The state pointer is stashed in this module's ctx slot. That slot is shared
/// with the response-path `StreamContext`; the access handler clears it
/// (`clear_request_inspect_ctx`) before allowing the request to proceed, so the
/// response body filter always allocates a fresh `StreamContext`. The boxed
/// state itself is owned by the pool cleanup handler, so clearing the slot does
/// not free it — it lives until request teardown, keeping the spawned `Task`
/// and verdict valid for the duration of inspection.
///
/// Returns null only on allocation failure.
unsafe fn get_request_inspect_state(r: *mut ngx_http_request_t) -> *mut RequestInspectState {
    unsafe {
        let idx = Module::module().ctx_index;
        let slot = (*r).ctx.add(idx);
        let existing = *slot;
        if !existing.is_null() {
            return existing as *mut RequestInspectState;
        }

        // Allocate the state and register a cleanup to drop it at teardown.
        let boxed = Box::new(RequestInspectState::new());
        let raw = Box::into_raw(boxed);

        let cln = ngx::ffi::ngx_http_cleanup_add(r, 0);
        if cln.is_null() {
            // Reclaim the box to avoid a leak, then signal failure.
            drop(Box::from_raw(raw));
            return ptr::null_mut();
        }
        (*cln).handler = Some(request_inspect_state_cleanup);
        (*cln).data = raw as *mut std::ffi::c_void;

        *slot = raw as *mut std::ffi::c_void;
        raw
    }
}

/// Clear this module's ctx slot so the response body filter allocates a fresh
/// `StreamContext`. Does not free the boxed `RequestInspectState`; that is owned
/// by the registered pool cleanup handler.
unsafe fn clear_request_inspect_ctx(r: *mut ngx_http_request_t) {
    unsafe {
        let idx = Module::module().ctx_index;
        *(*r).ctx.add(idx) = ptr::null_mut();
    }
}

/// Access-phase handler for non-blocking request inspection.
///
/// Modelled on `ngx_http_auth_request_module` (async wait via `NGX_AGAIN`) plus
/// `ngx_http_mirror_module` (read body then resume phases). While inspection is
/// pending the handler returns `NGX_AGAIN`, which makes the access-phase checker
/// yield without advancing `r->phase_handler`, so this same handler is
/// re-invoked when phases are resumed.
///
/// Lifecycle:
///   1. First call: if inspection is enabled, allocate `RequestInspectState`,
///      trigger `ngx_http_read_client_request_body` (which does `r->count++`)
///      and return `NGX_DONE` to yield.
///   2. The body-read handler extracts the prompt and spawns an async subrequest
///      task; when it completes it records the verdict then resumes the phase
///      engine (`r->write_event_handler = ngx_http_core_run_phases`; call it).
///   3. Re-invocation: read the verdict — `Allow` → `NGX_OK` (access granted),
///      `Block`/error → send 403 (fail-closed), `Pending` → `NGX_AGAIN` (wait).
pub(crate) unsafe extern "C" fn guardrails_access_handler(r: *mut ngx_http_request_t) -> ngx_int_t {
    unsafe {
        if r.is_null() {
            return Status::NGX_ERROR.into();
        }

        let request = &mut *r.cast::<Request>();

        // Only main requests; subrequests (including our own guardrails
        // subrequest) must pass straight through.
        if !request.is_main() {
            return Status::NGX_DECLINED.into();
        }

        let conf = match Module::location_conf(request) {
            Some(c) => c,
            None => return Status::NGX_DECLINED.into(),
        };

        if !conf.inspect_requests() {
            return Status::NGX_DECLINED.into();
        }

        // The internal guardrails location must be configured; if not, fail
        // closed (do not silently allow unfiltered content).
        let internal_uri = match &conf.internal_uri {
            Some(u) => u.clone(),
            None => {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: guardrails_internal_uri not configured (fail-closed)"
                );
                return send_403_and_finalize(r, None);
            }
        };

        // Retrieve or create the per-request inspection state.
        let state_ptr = get_request_inspect_state(r);
        if state_ptr.is_null() {
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: failed to allocate request inspect state (fail-closed)"
            );
            return send_403_and_finalize(r, None);
        }
        let state = &mut *state_ptr;

        // Dispatch on the pure access-handler state machine (see
        // `decision::decide_access_action`): a resolved verdict (Allow/Block)
        // wins over `started`; only while Pending does `started` distinguish
        // "wait for the in-flight task" from "start a new inspection".
        match decide_access_action(state.verdict.as_decision(), state.started) {
            AccessAction::GrantAccess => {
                ngx_log_error!(
                    NGX_LOG_INFO,
                    request.log(),
                    "guardrails: request content cleared by policy"
                );
                // Clear the module ctx slot so the response body filter's
                // `get_module_ctx_mut` sees null and allocates a fresh
                // `StreamContext` (the slot is shared between the request-inspect
                // state and the response StreamContext). The boxed state is freed
                // by the registered pool cleanup handler at request teardown.
                clear_request_inspect_ctx(r);
                // Access granted: advance past the access phase.
                Status::NGX_OK.into()
            }
            AccessAction::Block => {
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "guardrails: request content BLOCKED by policy"
                );
                send_403_and_finalize(r, state.block_message.as_deref())
            }
            AccessAction::Wait => {
                // Already started and still pending: yield with NGX_AGAIN so the
                // access checker re-invokes this handler when phases resume.
                Status::NGX_AGAIN.into()
            }
            AccessAction::StartInspection => {
                state.started = true;

                // Stash the token + uri on the state so the read handler can use
                // them without re-borrowing conf (which may be freed across the
                // async gap).
                state.params = Some(InspectParams {
                    internal_uri,
                    api_token: conf.api_token.clone(),
                });

                // Trigger reading of the client request body. This does
                // `r->count++`; when the body is fully read,
                // `guardrails_body_read_handler` fires. Returning NGX_DONE yields
                // without advancing the phase cursor.
                let rc = ngx::ffi::ngx_http_read_client_request_body(
                    r,
                    Some(guardrails_body_read_handler),
                );
                if rc >= ngx::ffi::NGX_HTTP_SPECIAL_RESPONSE as ngx_int_t {
                    // Error reading body.
                    ngx_log_error!(
                        NGX_LOG_ERR,
                        request.log(),
                        "guardrails: ngx_http_read_client_request_body failed: {}",
                        rc
                    );
                    return rc;
                }
                Status::NGX_DONE.into()
            }
        }
    }
}

/// Parameters captured for the async inspection, stored alongside the state so
/// they survive across the body-read and task boundaries.
struct InspectParams {
    internal_uri: String,
    api_token: Option<String>,
}

/// Request body read completion handler. Extracts the prompt and spawns the
/// async inspection subrequest.
unsafe extern "C" fn guardrails_body_read_handler(r: *mut ngx_http_request_t) {
    unsafe {
        ngx_log_error!(
            NGX_LOG_DEBUG_HTTP,
            (*(*r).connection).log,
            "guardrails: request body read complete; spawning inspection"
        );

        let state_ptr = get_request_inspect_state(r);
        if state_ptr.is_null() {
            resume_phases(r, InspectVerdict::Block, None);
            return;
        }
        let state = &mut *state_ptr;

        let params = match state.params.take() {
            Some(p) => p,
            None => {
                resume_phases(r, InspectVerdict::Block, None);
                return;
            }
        };

        // Gather the buffered request body.
        let content = collect_request_body(r).and_then(|bytes| extract_inspection_content(&bytes));

        let content = match content {
            Some(c) => c,
            None => {
                // Nothing to inspect — allow and resume immediately.
                ngx_log_error!(
                    NGX_LOG_DEBUG_HTTP,
                    (*(*r).connection).log,
                    "guardrails: no inspectable request content; allowing"
                );
                resume_phases(r, InspectVerdict::Allow, None);
                return;
            }
        };

        // Spawn the async inspection. The task is stored on the state to keep it
        // alive (dropping a Task cancels it); on completion it records the
        // verdict and resumes the phase engine.
        let r_send = AssertSendSync(r);
        let state_send = AssertSendSync(state_ptr);
        let task = ngx::async_::spawn(async move {
            let r = r_send.0;
            let state_ptr = state_send.0;
            // Reduce the inspection result to an Option<Verdict> (None == error,
            // logged here for its side effect), then defer the allow/block policy
            // to the shared, unit-tested `verdict_from_inspection`.
            let outcome = match inspect_content_async(
                r,
                &params.internal_uri,
                &content,
                params.api_token.as_deref(),
                ScanDirection::Request,
            )
            .await
            {
                Ok(v) => Some(v),
                Err(e) => {
                    ngx_log_error!(
                        NGX_LOG_ERR,
                        (*(*r).connection).log,
                        "guardrails: async request inspection error (fail-closed): {:?}",
                        e
                    );
                    None
                }
            };
            let decision = verdict_from_inspection(outcome);
            let verdict = if decision.allow {
                InspectVerdict::Allow
            } else {
                InspectVerdict::Block
            };
            let _ = state_ptr; // state pointer used only via resume_phases below
            resume_phases(r, verdict, decision.message);
        });

        state.task = Some(task);
    }
}

/// Record the verdict on the request-inspection state and resume the HTTP phase
/// engine so the access handler re-runs and acts on the verdict.
///
/// This mirrors `ngx_http_mirror_module`'s body-completion resume: set
/// `r->write_event_handler = ngx_http_core_run_phases` and call it. The async
/// task runs from the ngx async scheduler's posted-event context (already inside
/// the worker event loop), so calling `ngx_http_core_run_phases` directly here
/// is safe.
unsafe fn resume_phases(
    r: *mut ngx_http_request_t,
    verdict: InspectVerdict,
    message: Option<String>,
) {
    unsafe {
        let state_ptr = get_request_inspect_state(r);
        if !state_ptr.is_null() {
            (*state_ptr).verdict = verdict;
            (*state_ptr).block_message = message;
        }
        (*r).write_event_handler = Some(ngx::ffi::ngx_http_core_run_phases);
        ngx::ffi::ngx_http_core_run_phases(r);
    }
}

/// Collect the fully-buffered request body into a contiguous buffer.
unsafe fn collect_request_body(r: *mut ngx_http_request_t) -> Option<Vec<u8>> {
    unsafe {
        let rb = (*r).request_body;
        if rb.is_null() {
            return None;
        }
        let mut chain = (*rb).bufs;
        if chain.is_null() {
            return None;
        }
        let mut data = Vec::new();
        while !chain.is_null() {
            let buf = (*chain).buf;
            if !buf.is_null() {
                let b = &*buf;
                if !b.pos.is_null() && !b.last.is_null() {
                    let len = b.last.offset_from(b.pos) as usize;
                    let slice = std::slice::from_raw_parts(b.pos, len);
                    data.extend_from_slice(slice);
                }
            }
            chain = (*chain).next;
        }
        if data.is_empty() { None } else { Some(data) }
    }
}

/// Request body filter handler — pass-through only.
///
/// Request inspection has moved to the ACCESS-phase handler
/// (`guardrails_access_handler`) so it can run asynchronously without blocking
/// the worker. This filter now simply forwards buffers to the next filter.
pub(crate) unsafe extern "C" fn guardrails_request_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    call_next_request_body_filter(r, in_chain)
}

/// Default request-block message used when the guardrails backend supplies none.
const DEFAULT_REQUEST_BLOCK_MESSAGE: &str = "Request blocked by guardrails policy.";

/// Build the request-side 403 error JSON body.
///
/// Uses `type: "invalid_request_error"` (the client's request was bad) — distinct
/// from the output-side `api_error` used by the response-path helpers. `message`
/// is the backend's configurable block text; when present it is appended to the
/// default as `"<default> Message: <m>"` (see [`stream::compose_block_message`]),
/// otherwise the plain [`DEFAULT_REQUEST_BLOCK_MESSAGE`] is used. The composed
/// message is JSON-escaped via `serde_json`, so backend-supplied text cannot
/// break out of the JSON envelope.
fn request_block_body(message: Option<&str>) -> Vec<u8> {
    let msg = stream::compose_block_message(DEFAULT_REQUEST_BLOCK_MESSAGE, message);
    serde_json::json!({
        "error": {
            "message": msg,
            "type": "invalid_request_error",
            "param": null,
            "code": "content_policy_violation",
        }
    })
    .to_string()
    .into_bytes()
}

/// Send a 403 Forbidden response with a JSON error body, then finalize the request.
///
/// Called from the ACCESS-phase handler (`guardrails_access_handler`) when request
/// inspection blocks the prompt.
///
/// # Access-phase finalize/return contract (critical)
///
/// This function is invoked as (part of) an access-phase handler's return path, which
/// runs under `ngx_http_core_access_phase`. That checker calls
/// `ngx_http_finalize_request(r, rc)` for any handler return code that is not
/// `NGX_OK` / `NGX_AGAIN` / `NGX_DONE`. Therefore this function must:
///
///   1. call `ngx_http_finalize_request` **exactly once** itself, and
///   2. return **`NGX_DONE`** so the phase checker does NOT finalize a second time.
///
/// Returning `NGX_ERROR` here (the obvious-looking choice) is a bug: it makes the phase
/// checker finalize again, and the resulting double-finalize tears the connection down
/// before the queued body buffer is flushed — the client then receives `403` with an
/// **empty body** (`403 0` in the access log). This mirrors the same hazard documented
/// on the response-path helpers (`send_termination`, `send_blocked_response`), which
/// deliberately avoid calling finalize from inside the body-filter chain.
///
/// Every early-exit branch below finalizes once and returns `NGX_DONE` for the same reason.
///
/// `message` is the guardrails backend's configurable block text; when `None`
/// the [`DEFAULT_REQUEST_BLOCK_MESSAGE`] fallback is used. The request-side error
/// `type` is `invalid_request_error` (a bad client request), distinct from the
/// output-side `api_error` used by the response-path helpers.
unsafe fn send_403_and_finalize(r: *mut ngx_http_request_t, message: Option<&str>) -> ngx_int_t {
    ngx_log_error!(
        NGX_LOG_DEBUG_HTTP,
        unsafe { (*(*r).connection).log },
        "guardrails: finalizing request with 403 Forbidden (JSON)"
    );

    let json_body = request_block_body(message);
    let json_body = json_body.as_slice();

    let request = unsafe { &mut *r.cast::<http::Request>() };

    request.set_status(http::HTTPStatus(NGX_HTTP_FORBIDDEN as ngx_uint_t));
    request.set_content_length_n(json_body.len());
    if request
        .add_header_out("Content-Type", "application/json")
        .is_none()
    {
        // Header alloc failed: finalize with 403 (NGINX generates its default error
        // page) and yield with NGX_DONE so the phase checker does not finalize again.
        unsafe { ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t) };
        return Status::NGX_DONE.into();
    }

    let send_rc = request.send_header();
    if send_rc == Status::NGX_ERROR || request.header_only() {
        unsafe { ngx_http_finalize_request(r, send_rc.into()) };
        return Status::NGX_DONE.into();
    }

    // Build a single-buffer chain for the JSON body
    let pool = request.pool();
    unsafe {
        let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), json_body.len());
        if buf.is_null() {
            ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t);
            return Status::NGX_DONE.into();
        }
        ptr::copy_nonoverlapping(json_body.as_ptr(), (*buf).pos, json_body.len());
        (*buf).last = (*buf).pos.add(json_body.len());
        (*buf).set_last_buf(1);
        (*buf).set_memory(1);

        let out = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
        if out.is_null() {
            ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t);
            return Status::NGX_DONE.into();
        }
        (*out).buf = buf;
        (*out).next = ptr::null_mut();

        let filter_rc = request.output_filter(&mut *out);
        ngx_http_finalize_request(r, filter_rc.into());
    }

    // Body queued and finalized exactly once above. Return NGX_DONE so the access-phase
    // checker yields without a second finalize (which would drop the body).
    Status::NGX_DONE.into()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_request_block_body_none_uses_default() {
        // No backend message -> plain default, no "Message:" suffix; type stays
        // invalid_request_error (request-side block).
        let body = request_block_body(None);
        let value: serde_json::Value =
            serde_json::from_slice(&body).expect("request block body must be valid JSON");
        assert_eq!(value["error"]["message"], DEFAULT_REQUEST_BLOCK_MESSAGE);
        assert!(
            !value["error"]["message"]
                .as_str()
                .unwrap()
                .contains("Message:"),
            "default request block body must not contain a Message: suffix"
        );
        assert_eq!(value["error"]["type"], "invalid_request_error");
        assert_eq!(value["error"]["code"], "content_policy_violation");
    }

    #[test]
    fn test_request_block_body_some_appends_message() {
        // Backend message is appended as "<default> Message: <m>", escaped safely,
        // type stays invalid_request_error.
        let msg = r#"blocked by "IP" guardrail"#;
        let body = request_block_body(Some(msg));
        let value: serde_json::Value =
            serde_json::from_slice(&body).expect("request block body must be valid JSON");
        assert_eq!(
            value["error"]["message"],
            format!("{DEFAULT_REQUEST_BLOCK_MESSAGE} Message: {msg}")
        );
        assert_eq!(value["error"]["type"], "invalid_request_error");
        assert_eq!(value["error"]["code"], "content_policy_violation");
    }

    #[test]
    fn test_extract_content_prompt_with_escapes() {
        // Prompt containing JSON escapes (\n, \") must be extracted and
        // unescaped — the previous borrowed-deserialize path failed on these and
        // silently inspected the raw JSON envelope instead.
        let body = br#"{"prompt": "line one\nline \"two\""}"#;
        let got = extract_inspection_content(body).expect("prompt must be extracted");
        assert_eq!(got, "line one\nline \"two\"");
    }

    #[test]
    fn test_extract_content_messages_with_escapes() {
        // Chat-style messages: non-empty contents are newline-joined, escapes
        // decoded, empty contents skipped.
        let body = br#"{"messages": [
            {"content": "hello \\ world"},
            {"content": ""},
            {"content": "second\nline"}
        ]}"#;
        let got = extract_inspection_content(body).expect("messages must be extracted");
        assert_eq!(got, "hello \\ world\nsecond\nline");
    }

    #[test]
    fn test_extract_content_prompt_preferred_over_messages() {
        let body = br#"{"prompt": "P", "messages": [{"content": "M"}]}"#;
        assert_eq!(extract_inspection_content(body).as_deref(), Some("P"));
    }

    #[test]
    fn test_extract_content_array_text_parts() {
        // Multimodal array-shaped content with only text parts.
        let body = br#"{"messages": [{"content": [{"type": "text", "text": "hello"}]}]}"#;
        assert_eq!(extract_inspection_content(body).as_deref(), Some("hello"));
    }

    #[test]
    fn test_extract_content_array_ignores_image_parts() {
        // Regression for the array-shaped fallback: a text + image_url message
        // must extract only the text, NOT fall back to scanning the raw JSON
        // envelope (which would include the base64 image blob).
        let body = br#"{"messages": [{"content": [
            {"type": "text", "text": "describe this"},
            {"type": "image_url", "image_url": {"url": "data:image/png;base64,AAAABBBBCCCC"}}
        ]}]}"#;
        assert_eq!(
            extract_inspection_content(body).as_deref(),
            Some("describe this")
        );
    }

    #[test]
    fn test_extract_content_array_image_only_falls_back_to_raw() {
        // No text parts -> nothing inspectable from messages -> raw envelope.
        let body = br#"{"messages": [{"content": [
            {"type": "image_url", "image_url": {"url": "data:image/png;base64,AAAA"}}
        ]}]}"#;
        let got = extract_inspection_content(body).expect("raw envelope inspected");
        assert!(got.contains("image_url"));
    }

    #[test]
    fn test_extract_content_mixed_string_and_array_messages() {
        // A string-content message and an array-content message are joined.
        let body = br#"{"messages": [
            {"content": "first"},
            {"content": [{"type": "text", "text": "second"}, {"type": "text", "text": "third"}]}
        ]}"#;
        assert_eq!(
            extract_inspection_content(body).as_deref(),
            Some("first\nsecond\nthird")
        );
    }

    #[test]
    fn test_extract_content_unknown_shape_falls_back_to_raw() {
        // No prompt/messages -> inspect the raw JSON so nothing slips through.
        let body = br#"{"input": "unknown shape"}"#;
        assert_eq!(
            extract_inspection_content(body).as_deref(),
            Some(r#"{"input": "unknown shape"}"#)
        );
    }

    #[test]
    fn test_extract_content_non_json_falls_back_to_raw() {
        let body = b"just plain text";
        assert_eq!(
            extract_inspection_content(body).as_deref(),
            Some("just plain text")
        );
    }

    #[test]
    fn test_extract_content_empty_prompt_falls_back_to_raw() {
        // An empty prompt is not meaningful -> raw body is inspected instead.
        let body = br#"{"prompt": ""}"#;
        assert_eq!(
            extract_inspection_content(body).as_deref(),
            Some(r#"{"prompt": ""}"#)
        );
    }

    #[test]
    fn test_extract_content_invalid_utf8_returns_none() {
        assert!(extract_inspection_content(&[0xff, 0xfe]).is_none());
    }
}
