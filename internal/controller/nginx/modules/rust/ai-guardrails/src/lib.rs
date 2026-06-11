//! NGINX Guardrails Streaming Filter Module

use std::borrow::Cow;
use std::ffi::{c_char, c_void};
use std::ptr;

use ngx::core::{self, Status};
use ngx::ffi::{
    ngx_chain_t, ngx_command_t, ngx_conf_t, ngx_http_finalize_request, ngx_http_module_t,
    ngx_http_output_body_filter_pt, ngx_http_request_t, ngx_http_top_body_filter, ngx_int_t,
    ngx_module_t, ngx_str_t, ngx_uint_t, NGX_CONF_FLAG, NGX_CONF_TAKE1, NGX_HTTP_FORBIDDEN,
    NGX_HTTP_LOC_CONF, NGX_HTTP_LOC_CONF_OFFSET, NGX_HTTP_MODULE, NGX_LOG_EMERG, NGX_LOG_ERR,
    NGX_LOG_INFO, NGX_LOG_WARN,
};
use ngx::http::{self, HttpModule, HttpModuleLocationConf, Request};
use ngx::{ngx_conf_log_error, ngx_log_error, ngx_string};

/// Request body filter function pointer type — not exposed by the ngx crate.
#[allow(non_camel_case_types)]
type ngx_http_request_body_filter_pt =
    Option<unsafe extern "C" fn(r: *mut ngx_http_request_t, in_: *mut ngx_chain_t) -> ngx_int_t>;

/// Header filter function pointer type (mirrors `ngx_http_output_header_filter_pt`).
#[allow(non_camel_case_types)]
type ngx_http_output_header_filter_pt =
    Option<unsafe extern "C" fn(r: *mut ngx_http_request_t) -> ngx_int_t>;

extern "C" {
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

mod client;
mod config;
mod stream;

use config::ModuleConfig;
use stream::StreamContext;

struct Module;

impl http::HttpModule for Module {
    fn module() -> &'static ngx_module_t {
        unsafe { &*ptr::addr_of!(ngx_http_guardrails_module) }
    }

    unsafe extern "C" fn postconfiguration(_cf: *mut ngx_conf_t) -> ngx_int_t {
        // Log module initialization
        eprintln!("[guardrails] Rust module postconfiguration: registering filters");

        // Register header filter (two-state: suppress on first pass, commit on second).
        // Must be registered before body filters so it sits at the top of the header chain.
        NGX_HTTP_NEXT_HEADER_FILTER = ngx_http_top_header_filter;
        ngx_http_top_header_filter = Some(guardrails_header_filter);
        eprintln!("[guardrails] Registered header filter");

        // Register request body filter for request inspection
        NGX_HTTP_NEXT_REQUEST_BODY_FILTER = ngx_http_top_request_body_filter;
        ngx_http_top_request_body_filter = Some(guardrails_request_body_filter);
        eprintln!("[guardrails] Registered request body filter");

        // Register response body filter for response inspection
        NGX_HTTP_NEXT_BODY_FILTER = ngx_http_top_body_filter;
        ngx_http_top_body_filter = Some(guardrails_response_body_filter);
        eprintln!("[guardrails] Registered response body filter");

        eprintln!("[guardrails] Rust module loaded successfully");
        Status::NGX_OK.into()
    }
}

unsafe impl HttpModuleLocationConf for Module {
    type LocationConf = ModuleConfig;
}

/// Generate NGINX configuration directive handler
macro_rules! ngx_conf_handler {
    ($name:ident, $directive:literal, $apply:expr) => {
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

// NGINX directives table
static mut NGX_HTTP_GUARDRAILS_COMMANDS: [ngx_command_t; 8] = [
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

/// Returns true if the upstream response has `Content-Type: text/event-stream`.
unsafe fn is_sse_response(r: *mut ngx_http_request_t) -> bool {
    let ct = (*r).headers_out.content_type;
    if ct.len > 0 && !ct.data.is_null() {
        let ct_slice = std::slice::from_raw_parts(ct.data, ct.len);
        ct_slice.windows(17).any(|w| w == b"text/event-stream")
    } else {
        false
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

/// Request body filter handler - called for each request body chunk
unsafe extern "C" fn guardrails_request_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    if r.is_null() {
        return Status::NGX_ERROR.into();
    }

    let request = unsafe { &mut *r.cast::<Request>() };

    eprintln!("[guardrails] Request body filter called");

    // Only process main requests
    if !request.is_main() {
        eprintln!("[guardrails] Skipping subrequest");
        return call_next_request_body_filter(r, in_chain);
    }

    // Get module configuration
    let conf = match Module::location_conf(request) {
        Some(c) => c,
        None => {
            eprintln!("[guardrails] No location config");
            return call_next_request_body_filter(r, in_chain);
        }
    };

    // Skip if request inspection is not enabled
    if !conf.inspect_requests() {
        eprintln!("[guardrails] Request inspection disabled");
        return call_next_request_body_filter(r, in_chain);
    }

    // Collect all request body data from the chain
    let mut body_data = Vec::new();
    let mut chain = in_chain;

    while !chain.is_null() {
        let buf = unsafe { (*chain).buf };
        if !buf.is_null() {
            let buffer = unsafe { &*buf };
            if !buffer.pos.is_null() && !buffer.last.is_null() {
                let len = unsafe { buffer.last.offset_from(buffer.pos) as usize };
                let data = unsafe { std::slice::from_raw_parts(buffer.pos, len) };
                body_data.extend_from_slice(data);
            }
        }
        chain = unsafe { (*chain).next };
    }

    if body_data.is_empty() {
        eprintln!("[guardrails] No request body data");
        return call_next_request_body_filter(r, in_chain);
    }

    eprintln!(
        "[guardrails] Read {} bytes from request body",
        body_data.len()
    );

    // Parse JSON and extract text content for LLM chat/completion requests.
    // Using a typed struct with borrowed &str fields avoids allocating a full
    // serde_json::Value tree; the Cow is Borrowed in the common single-field
    // case and only becomes Owned when multiple messages are concatenated.
    let body_str = match std::str::from_utf8(&body_data) {
        Ok(s) => s,
        Err(_) => {
            eprintln!("[guardrails] Request body is not UTF-8");
            return call_next_request_body_filter(r, in_chain);
        }
    };

    let content_to_inspect: Cow<'_, str> =
        match serde_json::from_str::<RequestBody<'_>>(body_str) {
            Ok(body) => {
                if let Some(prompt) = body.prompt.filter(|p| !p.is_empty()) {
                    // /v1/completions — borrow directly, no allocation.
                    eprintln!("[guardrails] Extracted {} chars from prompt field", prompt.len());
                    Cow::Borrowed(prompt)
                } else if let Some(messages) = body.messages {
                    // /v1/chat/completions — join content fields.
                    let extracted: String = messages
                        .iter()
                        .filter_map(|m| m.content)
                        .collect::<Vec<_>>()
                        .join("\n");
                    if !extracted.is_empty() {
                        eprintln!("[guardrails] Extracted {} chars from messages", extracted.len());
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

    if content_to_inspect.is_empty() {
        return call_next_request_body_filter(r, in_chain);
    }

    // Call Guardrails API
    let api_url = match &conf.api_url {
        Some(url) => url,
        None => {
            eprintln!("[guardrails] No API URL configured");
            return call_next_request_body_filter(r, in_chain);
        }
    };

    let api_token = conf.api_token.as_deref();

    eprintln!(
        "[guardrails] Inspecting request content ({} chars)",
        content_to_inspect.len()
    );

    // Perform synchronous inspection
    let inspection_result =
        client::inspect_content(&content_to_inspect, api_url, api_token, conf.timeout_ms);

    match inspection_result {
        Ok(allowed) => {
            if allowed {
                eprintln!("[guardrails] Request content CLEARED");
                ngx_log_error!(
                    NGX_LOG_INFO,
                    request.log(),
                    "guardrails: request content cleared by policy"
                );
                call_next_request_body_filter(r, in_chain)
            } else {
                eprintln!("[guardrails] Request content BLOCKED");
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "guardrails: request content BLOCKED by policy"
                );
                unsafe { send_403_and_finalize(r) }
            }
        }
        Err(e) => {
            eprintln!(
                "[guardrails] Request inspection error (fail-closed): {:?}",
                e
            );
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: request inspection error (fail-closed): {:?}",
                e
            );
            unsafe { send_403_and_finalize(r) }
        }
    }
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
    if r.is_null() {
        return call_next_header_filter(r);
    }

    let request = unsafe { &mut *r.cast::<Request>() };

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
        let new_ctx = request.pool().allocate(StreamContext::default());
        if new_ctx.is_null() {
            eprintln!("[guardrails] Header filter: pool alloc failed, passing through");
            return call_next_header_filter(r);
        }
        request.set_module_ctx(new_ctx.cast(), Module::module());
        unsafe { &mut *new_ctx }
    } else {
        unsafe { &mut *ctx_ptr }
    };

    // Suppress upstream headers; the body filter will commit them after inspection.
    eprintln!(
        "[guardrails] Header filter: suppressing upstream headers (status={})",
        (*r).headers_out.status
    );
    ctx.headers_suppressed = true;
    Status::NGX_OK.into()
}

/// Send a 403 Forbidden response with a JSON error body.
/// Used by the request body filter (called before any upstream headers are sent).
unsafe fn send_403_and_finalize(r: *mut ngx_http_request_t) -> ngx_int_t {
    eprintln!("[guardrails] Finalizing request with 403 Forbidden (JSON)");

    static JSON_BODY: &[u8] = b"{\"error\":{\"message\":\"Request blocked by guardrails policy.\",\"type\":\"invalid_request_error\",\"param\":null,\"code\":\"content_policy_violation\"}}";

    let request = unsafe { &mut *r.cast::<http::Request>() };

    request.set_status(http::HTTPStatus(NGX_HTTP_FORBIDDEN as ngx_uint_t));
    request.set_content_length_n(JSON_BODY.len());
    if request
        .add_header_out("Content-Type", "application/json")
        .is_none()
    {
        // Fall back to letting NGINX generate the error page
        unsafe { ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t) };
        return Status::NGX_ERROR.into();
    }

    let send_rc = request.send_header();
    if send_rc == Status::NGX_ERROR || request.header_only() {
        unsafe { ngx_http_finalize_request(r, send_rc.into()) };
        return Status::NGX_ERROR.into();
    }

    // Build a single-buffer chain for the JSON body
    let pool = request.pool();
    unsafe {
        let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), JSON_BODY.len());
        if buf.is_null() {
            ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t);
            return Status::NGX_ERROR.into();
        }
        ptr::copy_nonoverlapping(JSON_BODY.as_ptr(), (*buf).pos, JSON_BODY.len());
        (*buf).last = (*buf).pos.add(JSON_BODY.len());
        (*buf).set_last_buf(1);
        (*buf).set_memory(1);

        let out = ngx::ffi::ngx_alloc_chain_link(pool.as_ptr());
        if out.is_null() {
            ngx_http_finalize_request(r, NGX_HTTP_FORBIDDEN as ngx_int_t);
            return Status::NGX_ERROR.into();
        }
        (*out).buf = buf;
        (*out).next = ptr::null_mut();

        let filter_rc = request.output_filter(&mut *out);
        ngx_http_finalize_request(r, filter_rc.into());
    }

    Status::NGX_ERROR.into()
}

/// Response body filter handler - called for each response chunk
unsafe extern "C" fn guardrails_response_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    if r.is_null() {
        return Status::NGX_ERROR.into();
    }

    let request = unsafe { &mut *r.cast::<Request>() };

    // Log that filter was called
    eprintln!("[guardrails] Body filter called for request");

    // Only process main requests
    if !request.is_main() {
        eprintln!("[guardrails] Skipping subrequest, passing through");
        return call_next_response_body_filter(r, in_chain);
    }

    // Skip inspection for error responses (like 403 from blocked requests)
    let status = unsafe { (*r).headers_out.status };
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

        // First chunk - allocate context
        let new_ctx = request.pool().allocate(StreamContext::default());
        if new_ctx.is_null() {
            eprintln!("[guardrails] ERROR: Failed to allocate context!");
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: failed to allocate context"
            );
            return call_next_response_body_filter(r, in_chain);
        }
        request.set_module_ctx(new_ctx.cast(), Module::module());
        unsafe { &mut *new_ctx }
    } else {
        eprintln!("[guardrails] Using existing context");
        unsafe { &mut *ctx_ptr }
    };

    // If already blocked, send termination and stop
    if ctx.blocked {
        ngx_log_error!(
            NGX_LOG_WARN,
            request.log(),
            "guardrails: stream blocked, sending termination"
        );

        return send_termination(r, request);
    }

    // --- Ingest all buffers from the upstream chain -------------------------
    let mut chain = in_chain;
    let mut last_buf = false;

    while !chain.is_null() {
        let buf = unsafe { (*chain).buf };
        if !buf.is_null() {
            let buffer = unsafe { &*buf };

            if buffer.last_buf() != 0 || buffer.last_in_chain() != 0 {
                last_buf = true;
            }

            if !buffer.pos.is_null() && !buffer.last.is_null() {
                let len = unsafe { buffer.last.offset_from(buffer.pos) as usize };
                let data = unsafe { std::slice::from_raw_parts(buffer.pos, len) };
                // process_chunk: adds raw bytes to pending_chunks AND parses
                // complete JSON lines for text extraction / object counting.
                ctx.process_chunk(data);
                // Advance pos to last to mark this upstream buffer as consumed.
                // Without this NGINX thinks the buffer is still in use and stops
                // reading from upstream once its ~4KB proxy_buffer_size fills up.
                unsafe {
                    (*buf).pos = (*buf).last;
                }
            }
        }
        chain = unsafe { (*chain).next };
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
            send_termination(r, request)
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
        // Not enough objects yet — keep buffering, return nothing to client.
        return Status::NGX_OK.into();
    }

    // --- Run synchronous Guardrails inspection -----------------------------
    ngx_log_error!(
        NGX_LOG_INFO,
        request.log(),
        "guardrails: inspecting full stream, accumulated={}",
        ctx.accumulated_text.len()
    );

    let inspection_result = stream::inspect_checkpoint(ctx, conf);

    match inspection_result {
        Ok(true) => {
            // Cleared — release buffered chunks to the client.
            ngx_log_error!(NGX_LOG_INFO, request.log(), "guardrails: content cleared");

            // If we suppressed the upstream headers, commit them now before sending body.
            // Call directly into the rest of the header chain — same pattern as image_filter.
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

            send_chunks(r, request, &chunks_to_send, last_buf || ctx.stream_done)
        }
        Ok(false) => {
            // Blocked — discard buffer and send error response.
            ngx_log_error!(NGX_LOG_WARN, request.log(), "guardrails: content BLOCKED");
            ctx.blocked = true;
            ctx.clear_pending_chunks();
            if ctx.headers_suppressed {
                send_blocked_response(r, request, ctx)
            } else {
                send_termination(r, request)
            }
        }
        Err(e) => {
            // Fail-closed: block on any inspection error (consistent with failureMode: FailClosed).
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "guardrails: inspection error (fail-closed): {:?}",
                e
            );
            ctx.blocked = true;
            ctx.clear_pending_chunks();
            if ctx.headers_suppressed {
                send_blocked_response(r, request, ctx)
            } else {
                send_termination(r, request)
            }
        }
    }
}

/// Write the appropriate error body into an NGINX buffer and forward it to the next filter.
/// Uses SSE format (`data: {...}`) for event-stream responses and plain JSON for all others.
/// Because the response body filter cannot change the HTTP status after headers are committed,
/// this always returns a 200 response — the error is communicated via the body content.
/// The only path that can return a true 403 is the request body filter (`send_403_and_finalize`).
unsafe fn send_termination(r: *mut ngx_http_request_t, request: &http::Request) -> ngx_int_t {
    let is_sse = is_sse_response(r);
    let term_msg: &[u8] = if is_sse {
        stream::termination_message()
    } else {
        stream::non_streaming_error_body()
    };
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

/// Commit a 403 response via the standard NGINX body-filter header commit pattern.
///
/// Called from the response body filter after inspection blocks a non-SSE response.
/// At this point `r->header_sent == 0` because `guardrails_header_filter` suppressed
/// the upstream headers on the first pass.
///
/// Steps:
///   1. Overwrite `headers_out` with 403 status + correct `Content-Length`.
///   2. Call `call_next_header_filter(r)` **directly** — this skips our own header
///      filter (which has already done its job) and goes straight to the rest of the
///      chain, ending at `ngx_http_header_filter` which writes "403 Forbidden" to wire.
///      This is the same pattern used by `ngx_http_image_filter_module`.
///   3. Write the JSON error body and return `NGX_ERROR` to close the connection.
unsafe fn send_blocked_response(
    r: *mut ngx_http_request_t,
    request: &http::Request,
    _ctx: &mut StreamContext,
) -> ngx_int_t {
    static JSON_BODY: &[u8] = b"{\"error\":{\"message\":\"Response blocked by guardrails policy.\",\"type\":\"invalid_request_error\",\"param\":null,\"code\":\"content_policy_violation\"}}";

    eprintln!("[guardrails] send_blocked_response: committing 403 via direct next-header-filter call");

    (*r).headers_out.status = NGX_HTTP_FORBIDDEN as ngx_uint_t;
    (*r).headers_out.content_length_n = JSON_BODY.len() as i64;
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
    let buf = ngx::ffi::ngx_create_temp_buf(pool.as_ptr(), JSON_BODY.len());
    if buf.is_null() {
        return Status::NGX_ERROR.into();
    }
    ptr::copy_nonoverlapping(JSON_BODY.as_ptr(), (*buf).pos, JSON_BODY.len());
    (*buf).last = (*buf).pos.add(JSON_BODY.len());
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
}
