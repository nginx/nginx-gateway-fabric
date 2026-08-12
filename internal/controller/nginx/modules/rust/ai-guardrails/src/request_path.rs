//! Request-path (input) inspection: ACCESS-phase handler, async subrequest
//! spawning, and the 403 block response.

use std::borrow::Cow;
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
use crate::ctx::call_next_request_body_filter;
use crate::decision::{AccessAction, RequestVerdict, decide_access_action};
use crate::stream;
use crate::subrequest_client::{ScanDirection, run_inspection};
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

/// Outcome of extracting inspectable text from a raw request body.
///
/// Distinguishes "there is genuinely nothing to inspect" (safe to allow) from
/// "there were bytes but we could not decode them" (must fail closed). The
/// caller (`guardrails_body_read_handler`) treats these differently: `None`
/// allows, `Undecodable` blocks. Conflating the two (the previous
/// `Option<String>` returning `None` for non-UTF-8 bytes) let a non-UTF-8 /
/// content-encoded prompt bypass inspection.
enum InspectableContent {
    /// Decodable content that should be sent to the guardrails backend.
    Text(String),
    /// The body was empty or yielded no meaningful text — allow.
    None,
    /// The body was non-empty but not valid UTF-8 (e.g. a gzip/br/deflate
    /// compressed prompt). It cannot be inspected in memory, so the caller
    /// must fail closed and block.
    Undecodable,
}

/// Extract the text content to inspect from a raw JSON request body.
///
/// Preference order: a non-empty `prompt`, else the newline-joined non-empty
/// `messages[].content` (string or multimodal-array shape; text parts only),
/// else the raw body string (so unknown shapes are still inspected). Returns
/// [`InspectableContent::None`] when there is nothing meaningful to inspect, and
/// [`InspectableContent::Undecodable`] when the (non-empty) body is not valid
/// UTF-8 so it cannot be inspected in memory.
fn extract_inspection_content(body_data: &[u8]) -> InspectableContent {
    let body_str = match std::str::from_utf8(body_data) {
        Ok(s) => s,
        // Non-UTF-8 bytes: cannot be inspected in memory (e.g. a compressed
        // body). Signal the caller to fail closed rather than silently allow.
        Err(_) => return InspectableContent::Undecodable,
    };

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
        InspectableContent::None
    } else {
        InspectableContent::Text(content.into_owned())
    }
}

/// Whether a request `Content-Encoding` header value denotes an encoding the
/// module cannot inspect. The only inspectable ("no-op") encoding is `identity`;
/// any other token (`gzip`, `br`, `deflate`, `compress`, …) means the body is
/// transformed and cannot be scanned in memory, so the caller must fail closed.
///
/// The value is matched case-insensitively with surrounding whitespace trimmed.
/// A comma-separated list is treated as unsupported if **any** token is
/// non-empty and not `identity` (a single `identity`, or an empty value, is
/// supported).
pub(crate) fn is_unsupported_encoding(value: &[u8]) -> bool {
    value
        .split(|&b| b == b',')
        .map(trim_ascii_ws)
        .filter(|tok| !tok.is_empty())
        .any(|tok| !tok.eq_ignore_ascii_case(b"identity"))
}

/// Trim leading/trailing ASCII whitespace from a byte slice.
fn trim_ascii_ws(mut s: &[u8]) -> &[u8] {
    while let [first, rest @ ..] = s {
        if first.is_ascii_whitespace() {
            s = rest;
        } else {
            break;
        }
    }
    while let [rest @ .., last] = s {
        if last.is_ascii_whitespace() {
            s = rest;
        } else {
            break;
        }
    }
    s
}

/// Whether the request carries a `Content-Encoding` header that the module
/// cannot inspect (anything other than `identity`). Iterates `headers_in` (there
/// is no dedicated struct field for request `Content-Encoding`). Used to fail
/// closed on content-encoded prompts before deciding there is nothing to inspect.
unsafe fn request_has_unsupported_encoding(r: *mut ngx_http_request_t) -> bool {
    unsafe {
        let list = &(*r).headers_in.headers;
        let mut part = &list.part as *const ngx::ffi::ngx_list_part_t;
        while !part.is_null() {
            let elts = (*part).elts as *const ngx::ffi::ngx_table_elt_t;
            let n = (*part).nelts;
            for i in 0..n {
                let h = &*elts.add(i);
                if h.key.len == 0 || h.key.data.is_null() {
                    continue;
                }
                let key = std::slice::from_raw_parts(h.key.data, h.key.len);
                if !key.eq_ignore_ascii_case(b"content-encoding") {
                    continue;
                }
                if h.value.len == 0 || h.value.data.is_null() {
                    continue;
                }
                let value = std::slice::from_raw_parts(h.value.data, h.value.len);
                if is_unsupported_encoding(value) {
                    return true;
                }
            }
            part = (*part).next;
        }
        false
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
    /// True when the request body spilled to a temp file and could not be
    /// inspected in memory. Drives the distinct `request_too_large` 403 error
    /// body instead of the content-policy block body.
    too_large: bool,
    /// True when the request could not be inspected because it used an
    /// unsupported Content-Encoding or a non-UTF-8 body. Drives the distinct
    /// `unsupported_content_encoding` 403 error body.
    uninspectable: bool,
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
            too_large: false,
            uninspectable: false,
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
///      engine via a **posted write event** (see `resume_phases`) so the phase
///      engine re-runs on a clean event-loop iteration rather than inline from
///      inside the subrequest's finalize.
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
                return send_403_and_finalize(r, BlockKind::ContentPolicy(None));
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
            return send_403_and_finalize(r, BlockKind::ContentPolicy(None));
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
                if state.too_large {
                    ngx_log_error!(
                        NGX_LOG_WARN,
                        request.log(),
                        "guardrails: request BLOCKED — body too large to inspect (fail-closed)"
                    );
                    send_403_and_finalize(r, BlockKind::TooLarge)
                } else if state.uninspectable {
                    ngx_log_error!(
                        NGX_LOG_WARN,
                        request.log(),
                        "guardrails: request BLOCKED — unsupported content encoding / \
                         non-UTF-8 body (fail-closed)"
                    );
                    send_403_and_finalize(r, BlockKind::UnsupportedEncoding)
                } else {
                    ngx_log_error!(
                        NGX_LOG_WARN,
                        request.log(),
                        "guardrails: request content BLOCKED by policy"
                    );
                    send_403_and_finalize(
                        r,
                        BlockKind::ContentPolicy(state.block_message.as_deref()),
                    )
                }
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

        // A content-encoded request body (Content-Encoding other than identity)
        // is transformed on the wire and cannot be inspected in memory. Reject it
        // fail-closed before looking at the (compressed) bytes, so an encoded
        // prompt cannot slip through as "nothing to inspect".
        if request_has_unsupported_encoding(r) {
            ngx_log_error!(
                NGX_LOG_WARN,
                (*(*r).connection).log,
                "guardrails: request uses unsupported Content-Encoding and cannot be \
                 inspected; blocking (fail-closed)"
            );
            if !state_ptr.is_null() {
                (*state_ptr).uninspectable = true;
            }
            resume_phases(r, InspectVerdict::Block, None);
            return;
        }

        // Gather the buffered request body.
        let content = match collect_request_body(r) {
            CollectedBody::SpilledToDisk => {
                ngx_log_error!(
                    NGX_LOG_WARN,
                    (*(*r).connection).log,
                    "guardrails: request body spilled to disk (exceeds \
                     client_body_buffer_size) and cannot be inspected; blocking (fail-closed)"
                );
                if !state_ptr.is_null() {
                    (*state_ptr).too_large = true;
                }
                resume_phases(r, InspectVerdict::Block, None);
                return;
            }
            CollectedBody::Content(bytes) => extract_inspection_content(&bytes),
            CollectedBody::Empty => InspectableContent::None,
        };

        let content = match content {
            InspectableContent::Text(c) => c,
            InspectableContent::None => {
                // Nothing to inspect — allow and resume immediately.
                ngx_log_error!(
                    NGX_LOG_DEBUG_HTTP,
                    (*(*r).connection).log,
                    "guardrails: no inspectable request content; allowing"
                );
                resume_phases(r, InspectVerdict::Allow, None);
                return;
            }
            InspectableContent::Undecodable => {
                // Non-empty but non-UTF-8 body (e.g. a compressed prompt that
                // slipped past the Content-Encoding check). Cannot be inspected
                // in memory — fail closed.
                ngx_log_error!(
                    NGX_LOG_WARN,
                    (*(*r).connection).log,
                    "guardrails: request body is not valid UTF-8 and cannot be \
                     inspected; blocking (fail-closed)"
                );
                if !state_ptr.is_null() {
                    (*state_ptr).uninspectable = true;
                }
                resume_phases(r, InspectVerdict::Block, None);
                return;
            }
        };

        // Spawn the async inspection. The task is stored on the state to keep it
        // alive (dropping a Task cancels it); on completion it records the
        // verdict and resumes the phase engine.
        //
        // The `run_inspection(...).await` below has no Rust-side timeout; it is
        // bounded only by the internal location's `proxy_*_timeout` (see
        // `config.rs`) or by request teardown (which cancels this task). Until it
        // resolves, the access phase stays suspended (`NGX_AGAIN`).
        let r_send = AssertSendSync(r);
        let task = ngx::async_::spawn(async move {
            let r = r_send.0;
            // Await + fail-closed error log + verdict mapping are shared with the
            // response path via `run_inspection`.
            let decision = run_inspection(
                r,
                &params.internal_uri,
                &content,
                params.api_token.as_deref(),
                ScanDirection::Request,
            )
            .await;
            let verdict = if decision.allow {
                InspectVerdict::Allow
            } else {
                InspectVerdict::Block
            };
            // The verdict is recorded via resume_phases, which re-fetches the
            // per-request state from `r`; no captured state pointer is needed.
            resume_phases(r, verdict, decision.message);
        });

        state.task = Some(task);
    }
}

/// Record the async inspection verdict on the request-inspection state and
/// **defer** the phase-engine resume to a posted write event.
///
/// # Why defer (critical)
///
/// This runs from the spawned task's continuation, which is polled **inline** by
/// the wake from `PostSubrequest::handler` — invoked by NGINX from **inside the
/// subrequest's `ngx_http_finalize_request`**, before its own `r->main->count--`
/// and posted-request draining. Re-driving the parent's phase engine from that
/// nested stack is unsafe: a `Block` verdict re-runs the ACCESS phase and
/// finalizes the parent (`send_403_and_finalize`) while the child subrequest is
/// still unwinding its own finalize. So we only record the verdict, arm
/// `guardrails_resume_phases_handler`, and post the connection write event; NGINX
/// then runs it on the next clean event-loop iteration, after the subrequest has
/// fully finalized. Mirrors the response path's `resume_output` deferral and the
/// deferral `start_subrequest` does after `ngx_http_subrequest`.
///
/// The verdict slot itself guards against acting twice: the re-invoked access
/// handler reads the recorded `Allow`/`Block` and either grants access or
/// finalizes exactly once (a spurious/duplicate write event re-runs the phase
/// engine, which is idempotent under NGINX's `phase_handler` cursor).
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

        // Post the resume to a write event so the phase engine runs on the next
        // event-loop iteration, NOT nested inside the subrequest's finalize.
        let conn = (*r).connection;
        if conn.is_null() || (*conn).write.is_null() {
            // No connection to post to — fall back to driving the phase engine
            // directly.
            let request = &mut *r.cast::<Request>();
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: resume_phases: null connection/write (direct resume)"
            );
            (*r).write_event_handler = Some(ngx::ffi::ngx_http_core_run_phases);
            ngx::ffi::ngx_http_core_run_phases(r);
            return;
        }
        (*r).write_event_handler = Some(guardrails_resume_phases_handler);
        ngx_post_event((*conn).write, ptr::addr_of_mut!(ngx_posted_events));
    }
}

/// Write-event handler that drives the deferred phase-engine resume for the
/// request path. Armed by `resume_phases` and driven by a posted write event, so
/// it runs cleanly in the worker event loop (not nested inside the subrequest's
/// `ngx_http_finalize_request`).
///
/// It hands control back to the core phase engine: the re-invoked ACCESS-phase
/// handler (`guardrails_access_handler`) reads the recorded verdict and grants
/// access (`NGX_OK`) or blocks (`send_403_and_finalize`). Setting
/// `write_event_handler = ngx_http_core_run_phases` keeps subsequent write events
/// (e.g. while the 403 body flushes) flowing through the core engine.
unsafe extern "C" fn guardrails_resume_phases_handler(r: *mut ngx_http_request_t) {
    unsafe {
        (*r).write_event_handler = Some(ngx::ffi::ngx_http_core_run_phases);
        ngx::ffi::ngx_http_core_run_phases(r);
    }
}

/// Outcome of collecting the buffered request body.
///
/// Distinguishes three cases so the read handler can act correctly on each:
///   - `Content` — the body was fully available in memory and collected.
///   - `Empty` — there is genuinely no request body to inspect (allow).
///   - `SpilledToDisk` — NGINX buffered part/all of the body to a temp file
///     because it exceeded `client_body_buffer_size`. Those buffers are
///     file-backed (`in_file`) and contribute no bytes to `pos`/`last`, so the
///     in-memory content is incomplete and cannot be trusted. The module does
///     not read disk-backed bodies, so this must **fail closed** (block) rather
///     than allow an un-inspected prompt through.
enum CollectedBody {
    Content(Vec<u8>),
    Empty,
    SpilledToDisk,
}

/// Collect the in-memory request body into a contiguous buffer.
///
/// Only reads memory-resident buffers. If any buffer in the chain is file-backed
/// (NGINX spilled the body past `client_body_buffer_size` to a temp file), the
/// body cannot be inspected in memory and [`CollectedBody::SpilledToDisk`] is
/// returned so the caller fails closed. See the module README for how operators
/// can raise `client_body_buffer_size` to inspect larger prompts in memory.
unsafe fn collect_request_body(r: *mut ngx_http_request_t) -> CollectedBody {
    unsafe {
        let rb = (*r).request_body;
        if rb.is_null() {
            return CollectedBody::Empty;
        }
        let mut chain = (*rb).bufs;
        if chain.is_null() {
            return CollectedBody::Empty;
        }
        let mut data = Vec::new();
        while !chain.is_null() {
            let buf = (*chain).buf;
            if !buf.is_null() {
                let b = &*buf;
                // A file-backed buffer means the body spilled to a temp file.
                // We do not read disk-backed bodies; treat this as un-inspectable
                // and signal the caller to fail closed. Detect via the `in_file`
                // flag and, defensively, a non-empty file byte range.
                if b.in_file() != 0 || b.file_last > b.file_pos {
                    return CollectedBody::SpilledToDisk;
                }
                if !b.pos.is_null() && !b.last.is_null() {
                    let len = b.last.offset_from(b.pos) as usize;
                    let slice = std::slice::from_raw_parts(b.pos, len);
                    data.extend_from_slice(slice);
                }
            }
            chain = (*chain).next;
        }
        if data.is_empty() {
            CollectedBody::Empty
        } else {
            CollectedBody::Content(data)
        }
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

/// Message for the request-too-large (body spilled to disk) block.
const REQUEST_TOO_LARGE_MESSAGE: &str = "Request body is too large to be inspected by guardrails and was rejected. \
     Reduce the request size, or raise client_body_buffer_size on the gateway.";

/// Message for the unsupported-content-encoding / non-UTF-8 body block.
const UNSUPPORTED_ENCODING_MESSAGE: &str = "Request body could not be inspected by guardrails \
     (unsupported content encoding or non-UTF-8 body) and was rejected. \
     Send the request body uncompressed and as UTF-8.";

/// Why a request is being blocked with a 403, selecting the error body shape.
enum BlockKind<'a> {
    /// Content flagged by policy (or a fail-closed config/allocation error).
    /// Carries the optional backend-supplied block message.
    ContentPolicy(Option<&'a str>),
    /// The request body spilled to a temp file and could not be inspected, so it
    /// was rejected fail-closed. Emits a distinct `request_too_large` error type.
    TooLarge,
    /// The request used an unsupported Content-Encoding or a non-UTF-8 body and
    /// could not be inspected, so it was rejected fail-closed. Emits a distinct
    /// `unsupported_content_encoding` error type.
    UnsupportedEncoding,
}

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

/// Build the request-too-large 403 error JSON body.
///
/// Uses a distinct `type: "request_too_large"` / `code: "request_body_too_large"`
/// so clients (and operators) can tell a fail-closed "body spilled to disk and
/// could not be inspected" rejection apart from a content-policy block.
fn request_too_large_body() -> Vec<u8> {
    serde_json::json!({
        "error": {
            "message": REQUEST_TOO_LARGE_MESSAGE,
            "type": "request_too_large",
            "param": null,
            "code": "request_body_too_large",
        }
    })
    .to_string()
    .into_bytes()
}

/// Build the unsupported-content-encoding 403 error JSON body.
///
/// Uses a distinct `type: "invalid_request_error"` / `code:
/// "unsupported_content_encoding"` so clients (and operators) can tell a
/// fail-closed "body could not be decoded/inspected" rejection apart from a
/// content-policy block or a request-too-large rejection.
fn unsupported_encoding_body() -> Vec<u8> {
    serde_json::json!({
        "error": {
            "message": UNSUPPORTED_ENCODING_MESSAGE,
            "type": "invalid_request_error",
            "param": null,
            "code": "unsupported_content_encoding",
        }
    })
    .to_string()
    .into_bytes()
}

/// Build the 403 error JSON body for the given block reason.
fn block_body(kind: BlockKind<'_>) -> Vec<u8> {
    match kind {
        BlockKind::ContentPolicy(message) => request_block_body(message),
        BlockKind::TooLarge => request_too_large_body(),
        BlockKind::UnsupportedEncoding => unsupported_encoding_body(),
    }
}

/// Send a 403 Forbidden response with a JSON error body, then finalize the request.
///
/// Called from the ACCESS-phase handler (`guardrails_access_handler`) when request
/// inspection blocks the prompt.
///
/// # Access-phase finalize/return contract (critical)
///
/// `ngx_http_core_access_phase` finalizes for any return code other than
/// `NGX_OK` / `NGX_AGAIN` / `NGX_DONE`. So this function must (1) call
/// `ngx_http_finalize_request` **exactly once** itself and (2) return **`NGX_DONE`**
/// so the checker does not finalize again. Returning `NGX_ERROR` instead triggers a
/// double-finalize that tears the connection down before the queued body flushes —
/// the client gets `403` with an **empty body** (`403 0`). Every early-exit branch
/// below finalizes once and returns `NGX_DONE`.
///
/// `kind` selects the JSON error body: [`BlockKind::ContentPolicy`] emits the
/// content-policy block (`type: invalid_request_error`, carrying the backend's
/// optional block text), while [`BlockKind::TooLarge`] emits the distinct
/// `request_too_large` body. Both are request-side (not the output-side
/// `api_error` used by the response-path helpers).
unsafe fn send_403_and_finalize(r: *mut ngx_http_request_t, kind: BlockKind<'_>) -> ngx_int_t {
    ngx_log_error!(
        NGX_LOG_DEBUG_HTTP,
        unsafe { (*(*r).connection).log },
        "guardrails: finalizing request with 403 Forbidden (JSON)"
    );

    let json_body = block_body(kind);
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

    impl InspectableContent {
        /// Test helper: the extracted text if this is `Text`, else `None`.
        fn as_text(&self) -> Option<&str> {
            match self {
                InspectableContent::Text(s) => Some(s.as_str()),
                _ => None,
            }
        }

        /// Test helper: true if this is the genuinely-nothing-to-inspect case.
        fn is_none(&self) -> bool {
            matches!(self, InspectableContent::None)
        }

        /// Test helper: true if this is the non-UTF-8 fail-closed case.
        fn is_undecodable(&self) -> bool {
            matches!(self, InspectableContent::Undecodable)
        }
    }

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
    fn test_request_too_large_body_uses_distinct_error_type() {
        // A body spilled to disk is rejected fail-closed with a DISTINCT error
        // type/code so it is not confused with a content-policy block.
        let body = request_too_large_body();
        let value: serde_json::Value =
            serde_json::from_slice(&body).expect("too-large body must be valid JSON");
        assert_eq!(value["error"]["message"], REQUEST_TOO_LARGE_MESSAGE);
        assert_eq!(value["error"]["type"], "request_too_large");
        assert_eq!(value["error"]["code"], "request_body_too_large");
        // Must NOT reuse the content-policy typing.
        assert_ne!(value["error"]["type"], "invalid_request_error");
        assert_ne!(value["error"]["code"], "content_policy_violation");
    }

    #[test]
    fn test_block_body_dispatches_on_kind() {
        // ContentPolicy -> content-policy error shape.
        let policy: serde_json::Value =
            serde_json::from_slice(&block_body(BlockKind::ContentPolicy(None)))
                .expect("content-policy body must be valid JSON");
        assert_eq!(policy["error"]["type"], "invalid_request_error");

        // TooLarge -> request-too-large error shape.
        let too_large: serde_json::Value = serde_json::from_slice(&block_body(BlockKind::TooLarge))
            .expect("too-large body must be valid JSON");
        assert_eq!(too_large["error"]["type"], "request_too_large");
    }

    #[test]
    fn test_extract_content_prompt_with_escapes() {
        // Prompt containing JSON escapes (\n, \") must be extracted and
        // unescaped — the previous borrowed-deserialize path failed on these and
        // silently inspected the raw JSON envelope instead.
        let body = br#"{"prompt": "line one\nline \"two\""}"#;
        let got = extract_inspection_content(body);
        assert_eq!(got.as_text(), Some("line one\nline \"two\""));
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
        let got = extract_inspection_content(body);
        assert_eq!(got.as_text(), Some("hello \\ world\nsecond\nline"));
    }

    #[test]
    fn test_extract_content_prompt_preferred_over_messages() {
        let body = br#"{"prompt": "P", "messages": [{"content": "M"}]}"#;
        assert_eq!(extract_inspection_content(body).as_text(), Some("P"));
    }

    #[test]
    fn test_extract_content_array_text_parts() {
        // Multimodal array-shaped content with only text parts.
        let body = br#"{"messages": [{"content": [{"type": "text", "text": "hello"}]}]}"#;
        assert_eq!(extract_inspection_content(body).as_text(), Some("hello"));
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
            extract_inspection_content(body).as_text(),
            Some("describe this")
        );
    }

    #[test]
    fn test_extract_content_array_image_only_falls_back_to_raw() {
        // No text parts -> nothing inspectable from messages -> raw envelope.
        let body = br#"{"messages": [{"content": [
            {"type": "image_url", "image_url": {"url": "data:image/png;base64,AAAA"}}
        ]}]}"#;
        let got = extract_inspection_content(body);
        assert!(
            got.as_text()
                .expect("raw envelope inspected")
                .contains("image_url")
        );
    }

    #[test]
    fn test_extract_content_mixed_string_and_array_messages() {
        // A string-content message and an array-content message are joined.
        let body = br#"{"messages": [
            {"content": "first"},
            {"content": [{"type": "text", "text": "second"}, {"type": "text", "text": "third"}]}
        ]}"#;
        assert_eq!(
            extract_inspection_content(body).as_text(),
            Some("first\nsecond\nthird")
        );
    }

    #[test]
    fn test_extract_content_unknown_shape_falls_back_to_raw() {
        // No prompt/messages -> inspect the raw JSON so nothing slips through.
        let body = br#"{"input": "unknown shape"}"#;
        assert_eq!(
            extract_inspection_content(body).as_text(),
            Some(r#"{"input": "unknown shape"}"#)
        );
    }

    #[test]
    fn test_extract_content_non_json_falls_back_to_raw() {
        let body = b"just plain text";
        assert_eq!(
            extract_inspection_content(body).as_text(),
            Some("just plain text")
        );
    }

    #[test]
    fn test_extract_content_empty_prompt_falls_back_to_raw() {
        // An empty prompt is not meaningful -> raw body is inspected instead.
        let body = br#"{"prompt": ""}"#;
        assert_eq!(
            extract_inspection_content(body).as_text(),
            Some(r#"{"prompt": ""}"#)
        );
    }

    #[test]
    fn test_extract_content_invalid_utf8_is_undecodable() {
        // A non-empty, non-UTF-8 body (e.g. a compressed prompt) must be reported
        // as Undecodable so the caller fails closed — NOT None (which would allow
        // it through un-inspected).
        let got = extract_inspection_content(&[0xff, 0xfe]);
        assert!(got.is_undecodable());
        assert!(!got.is_none());
        assert_eq!(got.as_text(), None);
    }

    #[test]
    fn test_extract_content_empty_body_is_none() {
        // A genuinely empty body has nothing to inspect and is allowed.
        assert!(extract_inspection_content(b"").is_none());
    }

    #[test]
    fn test_extract_content_gzip_magic_bytes_is_undecodable() {
        // gzip stream magic + non-UTF-8 continuation: must fail closed.
        let gzip = [0x1f, 0x8b, 0x08, 0x00, 0xff, 0xfe, 0x00, 0x03];
        assert!(extract_inspection_content(&gzip).is_undecodable());
    }

    #[test]
    fn test_is_unsupported_encoding() {
        // identity (any case) and empty are supported (inspectable).
        assert!(!is_unsupported_encoding(b"identity"));
        assert!(!is_unsupported_encoding(b"Identity"));
        assert!(!is_unsupported_encoding(b"  IDENTITY  "));
        assert!(!is_unsupported_encoding(b""));
        assert!(!is_unsupported_encoding(b"   "));
        // Any real transfer encoding is unsupported.
        assert!(is_unsupported_encoding(b"gzip"));
        assert!(is_unsupported_encoding(b"GZIP"));
        assert!(is_unsupported_encoding(b"br"));
        assert!(is_unsupported_encoding(b"deflate"));
        assert!(is_unsupported_encoding(b"compress"));
        assert!(is_unsupported_encoding(b" gzip "));
        // A list containing any non-identity token is unsupported.
        assert!(is_unsupported_encoding(b"identity, gzip"));
        assert!(is_unsupported_encoding(b"gzip, identity"));
        // A list of only identity tokens stays supported.
        assert!(!is_unsupported_encoding(b"identity, identity"));
    }

    #[test]
    fn test_unsupported_encoding_body_uses_distinct_error_type() {
        // The encoding/UTF-8 fail-closed rejection has a DISTINCT type/code so it
        // is not confused with a content-policy block or a too-large rejection.
        let body = unsupported_encoding_body();
        let value: serde_json::Value =
            serde_json::from_slice(&body).expect("unsupported-encoding body must be valid JSON");
        assert_eq!(value["error"]["message"], UNSUPPORTED_ENCODING_MESSAGE);
        assert_eq!(value["error"]["type"], "invalid_request_error");
        assert_eq!(value["error"]["code"], "unsupported_content_encoding");
        // Must NOT reuse the content-policy or too-large code.
        assert_ne!(value["error"]["code"], "content_policy_violation");
        assert_ne!(value["error"]["code"], "request_body_too_large");
    }

    #[test]
    fn test_block_body_dispatches_unsupported_encoding() {
        let value: serde_json::Value =
            serde_json::from_slice(&block_body(BlockKind::UnsupportedEncoding))
                .expect("unsupported-encoding body must be valid JSON");
        assert_eq!(value["error"]["type"], "invalid_request_error");
        assert_eq!(value["error"]["code"], "unsupported_content_encoding");
    }
}
