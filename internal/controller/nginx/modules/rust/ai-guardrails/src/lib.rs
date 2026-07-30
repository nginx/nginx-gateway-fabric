//! NGINX Guardrails Streaming Filter Module

use std::borrow::Cow;
use std::ffi::{c_char, c_void};
use std::ptr;

use ngx::core::{self, Status};
use ngx::ffi::{
    NGX_CONF_FLAG, NGX_CONF_TAKE1, NGX_HTTP_FORBIDDEN, NGX_HTTP_LOC_CONF, NGX_HTTP_LOC_CONF_OFFSET,
    NGX_HTTP_MODULE, NGX_LOG_EMERG, NGX_LOG_ERR, NGX_LOG_INFO, NGX_LOG_WARN, ngx_chain_t,
    ngx_command_t, ngx_conf_t, ngx_http_conf_ctx_t, ngx_http_core_main_conf_t,
    ngx_http_core_module, ngx_http_finalize_request, ngx_http_handler_pt, ngx_http_module_t,
    ngx_http_output_body_filter_pt, ngx_http_phases_NGX_HTTP_ACCESS_PHASE, ngx_http_request_t,
    ngx_http_top_body_filter, ngx_int_t, ngx_module_t, ngx_post_event, ngx_posted_events,
    ngx_str_t, ngx_uint_t,
};
use ngx::http::{self, HttpModule, HttpModuleLocationConf, Request};
use ngx::{ngx_conf_log_error, ngx_log_error, ngx_string};

use subrequest_client::{ScanDirection, inspect_content_async};

/// Request body filter function pointer type — not exposed by the ngx crate.
#[allow(non_camel_case_types)]
type ngx_http_request_body_filter_pt =
    Option<unsafe extern "C" fn(r: *mut ngx_http_request_t, in_: *mut ngx_chain_t) -> ngx_int_t>;

/// Header filter function pointer type (mirrors `ngx_http_output_header_filter_pt`).
#[allow(non_camel_case_types)]
type ngx_http_output_header_filter_pt =
    Option<unsafe extern "C" fn(r: *mut ngx_http_request_t) -> ngx_int_t>;

unsafe extern "C" {
    // The ngx crate only exposes the response body filter chain; this fills the gap.
    static mut ngx_http_top_request_body_filter: ngx_http_request_body_filter_pt;
    /// Top of the NGINX header filter chain — not exposed by the ngx crate.
    static mut ngx_http_top_header_filter: ngx_http_output_header_filter_pt;
}

/// Stored next body filter in the chain (for responses)
static mut NGX_HTTP_NEXT_BODY_FILTER: ngx_http_output_body_filter_pt = None;

/// Stored next request body filter in the chain (for requests)
static mut NGX_HTTP_NEXT_REQUEST_BODY_FILTER: ngx_http_request_body_filter_pt = None;

/// Stored next header filter in the chain
static mut NGX_HTTP_NEXT_HEADER_FILTER: ngx_http_output_header_filter_pt = None;

mod config;
mod error;
mod stream;
mod subrequest_client;

use config::ModuleConfig;
use stream::{ResponseInspect, ResponseVerdict, StreamContext};

struct Module;

impl http::HttpModule for Module {
    fn module() -> &'static ngx_module_t {
        unsafe { &*ptr::addr_of!(ngx_http_guardrails_module) }
    }

    unsafe extern "C" fn postconfiguration(cf: *mut ngx_conf_t) -> ngx_int_t {
        unsafe {
            // Log module initialization
            eprintln!("[guardrails] Rust module postconfiguration: registering filters");

            // Register header filter (two-state: suppress on first pass, commit on second).
            // Must be registered before body filters so it sits at the top of the header chain.
            NGX_HTTP_NEXT_HEADER_FILTER = ngx_http_top_header_filter;
            ngx_http_top_header_filter = Some(guardrails_header_filter);
            eprintln!("[guardrails] Registered header filter");

            // Register request body filter (pass-through only). Request inspection
            // now happens in the ACCESS phase handler below so it can be performed
            // asynchronously (non-blocking) via an NGINX subrequest.
            NGX_HTTP_NEXT_REQUEST_BODY_FILTER = ngx_http_top_request_body_filter;
            ngx_http_top_request_body_filter = Some(guardrails_request_body_filter);
            eprintln!("[guardrails] Registered request body filter (pass-through)");

            // Register response body filter for response inspection
            NGX_HTTP_NEXT_BODY_FILTER = ngx_http_top_body_filter;
            ngx_http_top_body_filter = Some(guardrails_response_body_filter);
            eprintln!("[guardrails] Registered response body filter");

            // Register the ACCESS-phase handler used for non-blocking request
            // inspection. Access-phase handlers may return NGX_AGAIN/NGX_DONE and
            // be resumed once async work completes — unlike body filters, which
            // cannot suspend.
            // Inline of the C macro `ngx_http_conf_get_module_main_conf(cf, mod)`
            // = `((ngx_http_conf_ctx_t *) cf->ctx)->main_conf[mod.ctx_index]`.
            let http_ctx = (*cf).ctx as *mut ngx_http_conf_ctx_t;
            if http_ctx.is_null() {
                eprintln!("[guardrails] ERROR: null http conf ctx");
                return Status::NGX_ERROR.into();
            }
            let core_ctx_index = ngx_http_core_module.ctx_index;
            let cmcf = *(*http_ctx).main_conf.add(core_ctx_index) as *mut ngx_http_core_main_conf_t;
            if cmcf.is_null() {
                eprintln!("[guardrails] ERROR: could not get core main conf");
                return Status::NGX_ERROR.into();
            }
            let handlers =
                &mut (*cmcf).phases[ngx_http_phases_NGX_HTTP_ACCESS_PHASE as usize].handlers;
            let h = ngx::ffi::ngx_array_push(handlers) as *mut ngx_http_handler_pt;
            if h.is_null() {
                eprintln!("[guardrails] ERROR: could not push access-phase handler");
                return Status::NGX_ERROR.into();
            }
            *h = Some(guardrails_access_handler);
            eprintln!("[guardrails] Registered access-phase handler");

            eprintln!("[guardrails] Rust module loaded successfully");
            Status::NGX_OK.into()
        }
    }
}

unsafe impl HttpModuleLocationConf for Module {
    type LocationConf = ModuleConfig;
}

/// Generate NGINX configuration directive handler
macro_rules! ngx_conf_handler {
    ($name:ident, $directive:literal, $apply:expr_2021) => {
        extern "C" fn $name(
            cf: *mut ngx_conf_t,
            _cmd: *mut ngx_command_t,
            conf: *mut c_void,
        ) -> *mut c_char {
            unsafe {
                if cf.is_null() || conf.is_null() {
                    return core::NGX_CONF_ERROR;
                }
                let cf_ref = &mut *cf;
                let conf = &mut *(conf as *mut ModuleConfig);
                let args: &[ngx_str_t] = (*cf_ref.args).as_slice();
                if args.len() < 2 {
                    ngx_conf_log_error!(
                        NGX_LOG_EMERG,
                        cf,
                        concat!("`", $directive, "` missing argument")
                    );
                    return core::NGX_CONF_ERROR;
                }
                let val = match args[1].to_str() {
                    Ok(s) => s,
                    Err(_) => {
                        ngx_conf_log_error!(
                            NGX_LOG_EMERG,
                            cf,
                            concat!("`", $directive, "` argument not utf-8")
                        );
                        return core::NGX_CONF_ERROR;
                    }
                };
                #[allow(clippy::redundant_closure_call)]
                ($apply)(conf, val);
            }
            core::NGX_CONF_OK
        }
    };
}

ngx_conf_handler!(
    ngx_http_guardrails_set_enable,
    "guardrails_filter",
    |conf: &mut ModuleConfig, val: &str| {
        conf.enabled = val.eq_ignore_ascii_case("on");
    }
);

ngx_conf_handler!(
    ngx_http_guardrails_set_api_url,
    "guardrails_api_url",
    |conf: &mut ModuleConfig, val: &str| {
        conf.api_url = Some(val.to_string());
    }
);

ngx_conf_handler!(
    ngx_http_guardrails_set_api_token,
    "guardrails_api_token",
    |conf: &mut ModuleConfig, val: &str| {
        conf.api_token = Some(val.to_string());
    }
);

/// Handler for `guardrails_api_token_file <path>`.
/// Reads the token from the given file at NGINX config-load time, strips whitespace, and stores it
/// in `ModuleConfig.api_token` exactly as if `guardrails_api_token` had been used.
extern "C" fn ngx_http_guardrails_set_api_token_file(
    cf: *mut ngx_conf_t,
    _cmd: *mut ngx_command_t,
    conf: *mut c_void,
) -> *mut c_char {
    unsafe {
        if cf.is_null() || conf.is_null() {
            return core::NGX_CONF_ERROR;
        }
        let cf_ref = &mut *cf;
        let conf = &mut *(conf as *mut ModuleConfig);
        let args: &[ngx_str_t] = (*cf_ref.args).as_slice();
        if args.len() < 2 {
            ngx_conf_log_error!(
                NGX_LOG_EMERG,
                cf,
                "`guardrails_api_token_file` missing argument"
            );
            return core::NGX_CONF_ERROR;
        }
        let path = match args[1].to_str() {
            Ok(s) => s,
            Err(_) => {
                ngx_conf_log_error!(
                    NGX_LOG_EMERG,
                    cf,
                    "`guardrails_api_token_file` path not valid UTF-8"
                );
                return core::NGX_CONF_ERROR;
            }
        };
        // Record the file path so we can surface it in config dumps / debug.
        conf.api_token_file = Some(path.to_string());
        // Read and trim the token at config load time.
        match std::fs::read_to_string(path) {
            Ok(contents) => {
                conf.api_token = Some(contents.trim().to_string());
            }
            Err(e) => {
                ngx_conf_log_error!(
                    NGX_LOG_EMERG,
                    cf,
                    "guardrails_api_token_file: failed to read \"{}\": {}",
                    path,
                    e
                );
                return core::NGX_CONF_ERROR;
            }
        }
    }
    core::NGX_CONF_OK
}

ngx_conf_handler!(
    ngx_http_guardrails_set_timeout,
    "guardrails_timeout_ms",
    |conf: &mut ModuleConfig, val: &str| {
        if let Ok(ms) = val.parse::<u64>() {
            conf.timeout_ms = ms;
        }
    }
);

ngx_conf_handler!(
    ngx_http_guardrails_set_inspect_mode,
    "guardrails_inspect_mode",
    |conf: &mut ModuleConfig, val: &str| {
        conf.inspect_mode = val.to_string();
    }
);

ngx_conf_handler!(
    ngx_http_guardrails_set_max_response_bytes,
    "guardrails_max_response_bytes",
    |conf: &mut ModuleConfig, val: &str| {
        if let Ok(n) = val.parse::<usize>() {
            conf.max_response_bytes = n;
        }
    }
);

ngx_conf_handler!(
    ngx_http_guardrails_set_internal_uri,
    "guardrails_internal_uri",
    |conf: &mut ModuleConfig, val: &str| {
        conf.internal_uri = Some(val.to_string());
    }
);

// NGINX directives table
static mut NGX_HTTP_GUARDRAILS_COMMANDS: [ngx_command_t; 9] = [
    ngx_command_t {
        name: ngx_string!("guardrails_filter"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_FLAG) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_enable),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_api_url"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_api_url),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_api_token"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_api_token),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_api_token_file"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_api_token_file),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_timeout_ms"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_timeout),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_inspect_mode"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_inspect_mode),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_max_response_bytes"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_max_response_bytes),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("guardrails_internal_uri"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_internal_uri),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t::empty(),
];

static NGX_HTTP_GUARDRAILS_MODULE_CTX: ngx_http_module_t = ngx_http_module_t {
    preconfiguration: None,
    postconfiguration: Some(Module::postconfiguration),
    create_main_conf: None,
    init_main_conf: None,
    create_srv_conf: None,
    merge_srv_conf: None,
    create_loc_conf: Some(Module::create_loc_conf),
    merge_loc_conf: None,
};

// Export ngx_modules table for dynamic module loading
ngx::ngx_modules!(ngx_http_guardrails_module);

#[used]
#[allow(non_upper_case_globals)]
#[unsafe(no_mangle)]
pub static mut ngx_http_guardrails_module: ngx_module_t = ngx_module_t {
    ctx: ptr::addr_of!(NGX_HTTP_GUARDRAILS_MODULE_CTX) as _,
    commands: unsafe { &NGX_HTTP_GUARDRAILS_COMMANDS[0] as *const _ as *mut _ },
    type_: NGX_HTTP_MODULE as _,
    ..ngx_module_t::default()
};

/// Get raw mutable pointer to module context
fn get_module_ctx_mut(
    request: &http::Request,
    module: &ngx::ffi::ngx_module_t,
) -> *mut StreamContext {
    unsafe {
        let r = request.as_ref();
        let ctx_ptr = *r.ctx.add(module.ctx_index);
        ctx_ptr as *mut StreamContext
    }
}

/// Cleanup handler that drops the boxed `StreamContext` at request teardown.
/// Dropping the box drops `inspect_task` (an in-flight `Task`), cancelling it.
unsafe extern "C" fn stream_ctx_cleanup(data: *mut c_void) {
    if !data.is_null() {
        drop(unsafe { Box::from_raw(data as *mut StreamContext) });
    }
}

/// Allocate a fresh `StreamContext` on the heap, register a pool cleanup so its
/// `Drop` runs at request teardown, and stash it in this module's ctx slot.
///
/// A raw `pool().allocate` is deliberately NOT used: `StreamContext` owns an
/// `Option<Task>` whose `Drop` must run to cancel any in-flight inspection task,
/// and pool allocations do not run destructors. Mirrors `get_request_inspect_state`.
///
/// Returns null on allocation failure (the caller should fail open / pass through).
unsafe fn alloc_stream_ctx(r: *mut ngx_http_request_t) -> *mut StreamContext {
    unsafe {
        let raw = Box::into_raw(Box::new(StreamContext::default()));

        let cln = ngx::ffi::ngx_http_cleanup_add(r, 0);
        if cln.is_null() {
            drop(Box::from_raw(raw));
            return ptr::null_mut();
        }
        (*cln).handler = Some(stream_ctx_cleanup);
        (*cln).data = raw as *mut c_void;

        let idx = Module::module().ctx_index;
        *(*r).ctx.add(idx) = raw as *mut c_void;
        raw
    }
}

/// Returns true if the upstream response has `Content-Type: text/event-stream`.
unsafe fn is_sse_response(r: *mut ngx_http_request_t) -> bool {
    unsafe {
        let ct = (*r).headers_out.content_type;
        if ct.len > 0 && !ct.data.is_null() {
            let ct_slice = std::slice::from_raw_parts(ct.data, ct.len);
            ct_slice.windows(17).any(|w| w == b"text/event-stream")
        } else {
            false
        }
    }
}

/// Typed request body for chat/completion API formats.
/// Uses borrowed `&str` to avoid allocations when only a reference to the
/// original body bytes is needed.
#[derive(serde::Deserialize)]
struct RequestBody<'a> {
    prompt: Option<&'a str>,
    messages: Option<Vec<RequestMessage<'a>>>,
}

#[derive(serde::Deserialize)]
struct RequestMessage<'a> {
    content: Option<&'a str>,
}

/// Extract the text content to inspect from a raw JSON request body.
/// Returns `None` when there is nothing meaningful to inspect.
fn extract_inspection_content(body_data: &[u8]) -> Option<String> {
    let body_str = std::str::from_utf8(body_data).ok()?;

    let content: Cow<'_, str> = match serde_json::from_str::<RequestBody<'_>>(body_str) {
        Ok(body) => {
            if let Some(prompt) = body.prompt.filter(|p| !p.is_empty()) {
                Cow::Borrowed(prompt)
            } else if let Some(messages) = body.messages {
                let extracted: String = messages
                    .iter()
                    .filter_map(|m| m.content)
                    .collect::<Vec<_>>()
                    .join("\n");
                if !extracted.is_empty() {
                    Cow::Owned(extracted)
                } else {
                    Cow::Borrowed(body_str)
                }
            } else {
                Cow::Borrowed(body_str)
            }
        }
        Err(_) => Cow::Borrowed(body_str),
    };

    if content.is_empty() {
        None
    } else {
        Some(content.into_owned())
    }
}

/// Verdict of an asynchronous request inspection.
#[derive(Clone, Copy, PartialEq, Eq)]
enum InspectVerdict {
    Pending,
    Allow,
    Block,
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
unsafe extern "C" fn request_inspect_state_cleanup(data: *mut c_void) {
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
        (*cln).data = raw as *mut c_void;

        *slot = raw as *mut c_void;
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
unsafe extern "C" fn guardrails_access_handler(r: *mut ngx_http_request_t) -> ngx_int_t {
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
                eprintln!("[guardrails] No internal URI configured (fail-closed)");
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
            eprintln!("[guardrails] Failed to allocate request inspect state (fail-closed)");
            return send_403_and_finalize(r, None);
        }
        let state = &mut *state_ptr;

        // Second (or later) invocation: verdict may be ready.
        match state.verdict {
            InspectVerdict::Allow => {
                eprintln!("[guardrails] Request content CLEARED (async)");
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
                return Status::NGX_OK.into();
            }
            InspectVerdict::Block => {
                eprintln!("[guardrails] Request content BLOCKED (async)");
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "guardrails: request content BLOCKED by policy"
                );
                return send_403_and_finalize(r, state.block_message.as_deref());
            }
            InspectVerdict::Pending => {}
        }

        // Already started and still pending: yield with NGX_AGAIN so the access
        // checker re-invokes this handler when phases resume.
        if state.started {
            return Status::NGX_AGAIN.into();
        }
        state.started = true;

        // Stash the token + uri on the state so the read handler can use them
        // without re-borrowing conf (which may be freed across the async gap).
        state.params = Some(InspectParams {
            internal_uri,
            api_token: conf.api_token.clone(),
        });

        // Trigger reading of the client request body. This does `r->count++`;
        // when the body is fully read, `guardrails_body_read_handler` fires.
        // Returning NGX_DONE yields without advancing the phase cursor.
        let rc = ngx::ffi::ngx_http_read_client_request_body(r, Some(guardrails_body_read_handler));
        if rc >= ngx::ffi::NGX_HTTP_SPECIAL_RESPONSE as ngx_int_t {
            // Error reading body.
            eprintln!("[guardrails] read_client_request_body error: {}", rc);
            return rc;
        }
        Status::NGX_DONE.into()
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
        eprintln!("[guardrails] Request body read complete; spawning inspection");

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
                eprintln!("[guardrails] No inspectable content; allowing");
                resume_phases(r, InspectVerdict::Allow, None);
                return;
            }
        };

        // Spawn the async inspection. The task is stored on the state to keep it
        // alive (dropping a Task cancels it); on completion it records the
        // verdict and resumes the phase engine.
        let r_send = SendPtr(r);
        let state_send = SendPtr(state_ptr);
        let task = ngx::async_::spawn(async move {
            let r = r_send.0;
            let state_ptr = state_send.0;
            let (verdict, message) = match inspect_content_async(
                r,
                &params.internal_uri,
                &content,
                params.api_token.as_deref(),
                ScanDirection::Request,
            )
            .await
            {
                Ok(v) if v.cleared => (InspectVerdict::Allow, None),
                Ok(v) => (InspectVerdict::Block, v.message),
                Err(e) => {
                    eprintln!("[guardrails] Async inspection error (fail-closed): {:?}", e);
                    (InspectVerdict::Block, None)
                }
            };
            let _ = state_ptr; // state pointer used only via resume_phases below
            resume_phases(r, verdict, message);
        });

        state.task = Some(task);
    }
}

/// Wrapper to move a raw pointer into a `'static` async task. Sound only under
/// single-threaded NGINX worker embedding.
struct SendPtr<T>(*mut T);
unsafe impl<T> Send for SendPtr<T> {}
unsafe impl<T> Sync for SendPtr<T> {}

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
unsafe extern "C" fn guardrails_request_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    call_next_request_body_filter(r, in_chain)
}

/// Call the next request body filter in the chain
#[inline]
fn call_next_request_body_filter(r: *mut ngx_http_request_t, chain: *mut ngx_chain_t) -> ngx_int_t {
    unsafe {
        match NGX_HTTP_NEXT_REQUEST_BODY_FILTER {
            Some(filter) => filter(r, chain),
            None => Status::NGX_OK.into(),
        }
    }
}

/// Call the next header filter in the chain.
#[inline]
fn call_next_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
    unsafe {
        match NGX_HTTP_NEXT_HEADER_FILTER {
            Some(filter) => filter(r),
            None => Status::NGX_OK.into(),
        }
    }
}

/// Two-state header filter.
///
/// **First pass** (upstream response headers arrive):
///   - SSE responses pass through immediately — they cannot be fully buffered.
///   - All other responses are suppressed: we return `NGX_OK` without calling the next
///     filter, so `r->header_sent` stays `0` and nothing is written to the socket.
///     `ctx.headers_suppressed` is set to `true`.
///
/// When the body filter is ready to commit headers (either the original 200 or a
/// replacement 403), it calls `call_next_header_filter(r)` **directly** — bypassing
/// this function and going straight to the rest of the chain.  This is the same pattern
/// used by `ngx_http_image_filter_module`.  There is no second pass through this function.
unsafe extern "C" fn guardrails_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
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

        // Don't suppress error responses — these originate from our own 403 injection
        // (send_403_and_finalize) and must reach the client unmodified.
        if (*r).headers_out.status >= 400 {
            return call_next_header_filter(r);
        }

        // SSE: always pass through — streaming responses cannot be fully buffered.
        if is_sse_response(r) {
            eprintln!("[guardrails] Header filter: SSE detected, passing through");
            return call_next_header_filter(r);
        }

        // Get or allocate per-request context.
        let ctx_ptr = get_module_ctx_mut(request, Module::module());
        let ctx = if ctx_ptr.is_null() {
            let new_ctx = alloc_stream_ctx(r);
            if new_ctx.is_null() {
                eprintln!("[guardrails] Header filter: ctx alloc failed, passing through");
                return call_next_header_filter(r);
            }
            &mut *new_ctx
        } else {
            &mut *ctx_ptr
        };

        // Suppress upstream headers; the body filter will commit them after inspection.
        eprintln!(
            "[guardrails] Header filter: suppressing upstream headers (status={})",
            (*r).headers_out.status
        );
        ctx.headers_suppressed = true;
        Status::NGX_OK.into()
    }
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
    eprintln!("[guardrails] Finalizing request with 403 Forbidden (JSON)");

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

/// Response body filter handler - called for each response chunk
unsafe extern "C" fn guardrails_response_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    unsafe {
        if r.is_null() {
            return Status::NGX_ERROR.into();
        }

        let request = &mut *r.cast::<Request>();

        // Log that filter was called
        eprintln!("[guardrails] Body filter called for request");

        // Only process main requests
        if !request.is_main() {
            eprintln!("[guardrails] Skipping subrequest, passing through");
            return call_next_response_body_filter(r, in_chain);
        }

        // Skip inspection for error responses (like 403 from blocked requests)
        let status = (*r).headers_out.status;
        if status >= 400 {
            eprintln!(
                "[guardrails] Skipping error response (status {}), passing through",
                status
            );
            return call_next_response_body_filter(r, in_chain);
        }

        eprintln!("[guardrails] Processing main request");

        // Get module configuration
        let conf = match Module::location_conf(request) {
            Some(c) => {
                eprintln!(
                    "[guardrails] Found location config: enabled={}, inspect_mode={}",
                    c.enabled, c.inspect_mode
                );
                c
            }
            None => {
                eprintln!("[guardrails] No location config found");
                ngx_log_error!(
                    NGX_LOG_INFO,
                    request.log(),
                    "guardrails: no location config found, passing through"
                );
                return call_next_response_body_filter(r, in_chain);
            }
        };

        // Skip if not enabled or not inspecting responses
        if !conf.inspect_responses() {
            eprintln!(
                "[guardrails] Response inspection disabled (enabled={}, mode={})",
                conf.enabled, conf.inspect_mode
            );
            ngx_log_error!(
                NGX_LOG_INFO,
                request.log(),
                "guardrails: response inspection disabled (enabled={}, mode={}), passing through",
                conf.enabled,
                conf.inspect_mode
            );
            return call_next_response_body_filter(r, in_chain);
        }

        eprintln!("[guardrails] Will inspect response");

        // Get or create context
        let ctx_ptr = get_module_ctx_mut(request, Module::module());

        let ctx = if ctx_ptr.is_null() {
            eprintln!("[guardrails] Allocating new context (first chunk)");

            // First chunk - allocate context (heap-boxed + pool cleanup so the
            // StreamContext's Drop runs at teardown, cancelling any Task).
            let new_ctx = alloc_stream_ctx(r);
            if new_ctx.is_null() {
                eprintln!("[guardrails] ERROR: Failed to allocate context!");
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "guardrails: failed to allocate context"
                );
                return call_next_response_body_filter(r, in_chain);
            }
            &mut *new_ctx
        } else {
            eprintln!("[guardrails] Using existing context");
            &mut *ctx_ptr
        };

        // If an async inspection is already in flight, hold: buffer any late
        // arrivals (there should be none after last_buf) and return without
        // forwarding. The async completion callback drives output from here on.
        if ctx.inspect_state == ResponseInspect::Pending {
            eprintln!("[guardrails] Body filter re-entered while inspection pending; holding");
            return Status::NGX_OK.into();
        }

        // If already blocked, send termination and stop
        if ctx.blocked {
            ngx_log_error!(
                NGX_LOG_WARN,
                request.log(),
                "guardrails: stream blocked, sending termination"
            );

            return send_termination(r, request, ctx.block_message.as_deref());
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

        eprintln!(
            "[guardrails] Chain processed: last_buf={}, pending_chunks={}, accumulated={}, buffered_bytes={}",
            last_buf,
            ctx.pending_chunks.len(),
            ctx.accumulated_text.len(),
            ctx.total_buffered_bytes
        );

        // --- Check buffer size limit -------------------------------------------
        if conf.max_response_bytes > 0 && ctx.total_buffered_bytes > conf.max_response_bytes {
            ngx_log_error!(
                NGX_LOG_WARN,
                request.log(),
                "guardrails: response buffer limit ({} bytes) exceeded, blocking stream",
                conf.max_response_bytes
            );
            ctx.blocked = true;
            ctx.clear_pending_chunks();
            return if ctx.headers_suppressed {
                send_blocked_response(r, request, ctx)
            } else {
                send_termination(r, request, ctx.block_message.as_deref())
            };
        }

        // --- Decide whether to inspect now or keep buffering -------------------
        // Flush any bytes still in line_buffer that were never terminated by a
        // newline.  This handles non-streaming responses (e.g. /v1/completions)
        // that arrive as a single JSON blob without a trailing newline.
        if last_buf {
            ctx.try_drain_remaining();
        }

        let do_inspect = ctx.should_inspect_final(last_buf);

        if !do_inspect {
            // At end-of-stream we must still release the buffered response even
            // when there is nothing to inspect (e.g. a `/v1/models` JSON body
            // that yields no LLM-extractable text, so `accumulated_text` is
            // empty). Otherwise the suppressed headers are never committed and
            // the buffered bytes are stranded, hanging the client. Mid-stream we
            // keep buffering as before.
            if last_buf || ctx.stream_done {
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

            // Not enough objects yet — keep buffering, return nothing to client.
            return Status::NGX_OK.into();
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
                return if ctx.headers_suppressed {
                    send_blocked_response(r, request, ctx)
                } else {
                    send_termination(r, request, ctx.block_message.as_deref())
                };
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

        let r_send = SendPtr(r);
        let task = ngx::async_::spawn(async move {
            let r = r_send.0;
            let (verdict, message) = match inspect_content_async(
                r,
                &internal_uri,
                &content,
                api_token.as_deref(),
                ScanDirection::Response,
            )
            .await
            {
                Ok(v) if v.cleared => (ResponseVerdict::Allow, None),
                Ok(v) => (ResponseVerdict::Block, v.message),
                Err(e) => {
                    eprintln!(
                        "[guardrails] Async response inspection error (fail-closed): {:?}",
                        e
                    );
                    (ResponseVerdict::Block, None)
                }
            };
            resume_output(r, verdict, message);
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
/// This runs from the spawned task's continuation. Per the ngx async scheduler
/// (`ngx::async_::spawn`), a task woken while parked is polled **inline**, and the
/// wake originates from our subrequest completion callback
/// (`PostSubrequest::handler`) — which NGINX invokes from **inside the
/// subrequest's `ngx_http_finalize_request`** (`request.c`), before the
/// subrequest's own `r->main->count--`. Driving the parent's output filter chain
/// and calling `ngx_http_finalize_request(parent, ...)` from that nested stack is
/// unsafe. So instead we only record the verdict here, arm
/// `guardrails_resume_write_handler` as the request's write-event handler, and
/// post the connection write event. The handler then runs on the next clean
/// event-loop iteration, after the subrequest has fully finalized. This mirrors
/// the deferral `start_subrequest` already does after `ngx_http_subrequest`
/// (`subrequest_client.rs`).
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
            eprintln!("[guardrails] resume_output: null ctx (fail-closed finalize)");
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
            eprintln!("[guardrails] resume_output: null connection/write (direct finalize)");
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
/// buffer, so the main request is still holding its normal in-flight `r->count`
/// reference. This handler MUST call `ngx_http_finalize_request` **exactly once**
/// to release it. The `ResponseInspect` state machine guards this: it acts only
/// on `Done(verdict)` and transitions to `Resumed`, so a second (spurious) write
/// event is a no-op.
///
/// The finalize **code matters**. The `send_*` helpers queue the body buffer
/// (with `last_buf`/`flush`) and return `NGX_ERROR` as a legacy sentinel meaning
/// "body queued; the caller must not finalize" — that contract was for the old
/// synchronous body filter. Here we deliberately **ignore** that sentinel and
/// finalize with `NGX_OK`. Finalizing with `NGX_ERROR` routes NGINX to
/// `ngx_http_terminate_request` (`request.c`), which tears the connection down
/// **before** the queued body is written — producing an empty `403 0` body and a
/// client hang. Finalizing with `NGX_OK` instead reaches the `r->buffered` flush
/// path, so the JSON/SSE body is written before the connection closes.
unsafe extern "C" fn guardrails_resume_write_handler(r: *mut ngx_http_request_t) {
    unsafe {
        let request = &mut *r.cast::<Request>();

        let ctx_ptr = get_module_ctx_mut(request, Module::module());
        if ctx_ptr.is_null() {
            eprintln!("[guardrails] resume_write: null ctx (fail-closed finalize)");
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
                if ctx.headers_suppressed {
                    // Non-SSE: 403 with JSON error body. Queues the body and
                    // returns the legacy NGX_ERROR sentinel (ignored below).
                    let _ = send_blocked_response(r, request, ctx);
                } else {
                    // SSE: 200 already flushed; inject an SSE termination frame
                    // carrying the backend's configurable block message.
                    let _ = send_termination(r, request, ctx.block_message.as_deref());
                }
                // Body queued. Finalize NGX_OK (NOT the helpers' NGX_ERROR) so the
                // queued body is flushed before the connection closes.
                ngx_http_finalize_request(r, Status::NGX_OK.into());
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
) -> ngx_int_t {
    unsafe {
        let is_sse = is_sse_response(r);
        let term_body: Vec<u8> = if is_sse {
            stream::termination_message(message)
        } else {
            stream::non_streaming_error_body(message)
        };
        let term_msg: &[u8] = term_body.as_slice();
        let pool = request.pool();
        let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), term_msg.len());
        if buf.is_null() {
            return Status::NGX_ERROR.into();
        }
        ptr::copy_nonoverlapping(term_msg.as_ptr(), (*buf).pos, term_msg.len());
        (*buf).last = (*buf).pos.add(term_msg.len());
        (*buf).set_last_buf(1);
        (*buf).set_flush(1);
        let out = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
        if out.is_null() {
            return Status::NGX_ERROR.into();
        }
        (*out).buf = buf;
        (*out).next = ptr::null_mut();
        // Forward the termination buffer to the client, then return NGX_ERROR so
        // NGINX closes the connection.  This is critical for non-streaming
        // responses that carry a Content-Length header: without closing the
        // connection, curl waits for the remaining promised bytes and hangs.
        // Do NOT call ngx_http_finalize_request here — calling it from inside the
        // body filter chain is unsafe and causes double-finalization on keep-alive
        // connections (finalize with NGX_OK keeps the connection open).
        call_next_response_body_filter(r, out);
        Status::NGX_ERROR.into()
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
///   3. Queue the JSON error body and return `NGX_ERROR`.
///
/// Return-value contract: the `NGX_ERROR` here is a **legacy sentinel** meaning
/// "body queued; the caller must not finalize". It fit the old synchronous body
/// filter, where returning `NGX_ERROR` up the filter chain let NGINX flush the
/// body then tear down. The async caller (`guardrails_resume_write_handler`)
/// **ignores** this return and finalizes with `NGX_OK` instead — finalizing with
/// `NGX_ERROR` would terminate the request before the queued body is written
/// (empty `403 0` + client hang). Do not "simplify" the caller to finalize with
/// this return value.
unsafe fn send_blocked_response(
    r: *mut ngx_http_request_t,
    request: &http::Request,
    ctx: &mut StreamContext,
) -> ngx_int_t {
    unsafe {
        // Non-SSE output-side block body (`type: api_error`), carrying the
        // backend's configurable message when present (else the hardcoded
        // fallback inside `non_streaming_error_body`).
        let json_body = stream::non_streaming_error_body(ctx.block_message.as_deref());
        let json_body = json_body.as_slice();

        eprintln!(
            "[guardrails] send_blocked_response: committing 403 via direct next-header-filter call"
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
            return Status::NGX_ERROR.into();
        }
        if request.header_only() {
            return Status::NGX_OK.into();
        }

        // Write the JSON error body.
        let pool = request.pool();
        let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), json_body.len());
        if buf.is_null() {
            return Status::NGX_ERROR.into();
        }
        ptr::copy_nonoverlapping(json_body.as_ptr(), (*buf).pos, json_body.len());
        (*buf).last = (*buf).pos.add(json_body.len());
        (*buf).set_last_buf(1);
        (*buf).set_flush(1);
        (*buf).set_memory(1);

        let out = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
        if out.is_null() {
            return Status::NGX_ERROR.into();
        }
        (*out).buf = buf;
        (*out).next = ptr::null_mut();

        call_next_response_body_filter(r, out);
        Status::NGX_ERROR.into()
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
        eprintln!(
            "[guardrails] Sending {} chunks to client (mark_last={})",
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

/// Call the next response body filter in the chain.
#[inline]
fn call_next_response_body_filter(
    r: *mut ngx_http_request_t,
    chain: *mut ngx_chain_t,
) -> ngx_int_t {
    unsafe {
        match NGX_HTTP_NEXT_BODY_FILTER {
            Some(filter) => filter(r, chain),
            None => Status::NGX_OK.into(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_call_next_response_body_filter_with_none() {
        unsafe {
            NGX_HTTP_NEXT_BODY_FILTER = None;
        }
        let result = call_next_response_body_filter(ptr::null_mut(), ptr::null_mut());
        assert_eq!(result, ngx_int_t::from(Status::NGX_OK));
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
}
