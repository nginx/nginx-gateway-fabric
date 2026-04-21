//! inference: NGINX module for Gateway API Inference Extension
//!
//! This module implements the Endpoint Picker Protocol (EPP) using Envoy's ext_proc
//! bidirectional gRPC streaming protocol. It provides:
//!
//! - Request phase: Forward headers and body to EPP, receive endpoint selection
//! - Response phase: After upstream responds, send served endpoint back to EPP
//!   as a fire-and-forget notification (no waiting for EPP's reply)

use std::ffi::{c_char, c_void};
use std::ptr;

use ngx::core::{self, Pool, Status};
use ngx::ffi::{
    NGX_CONF_TAKE1, NGX_HTTP_LOC_CONF, NGX_HTTP_LOC_CONF_OFFSET, NGX_HTTP_MODULE, NGX_LOG_DEBUG,
    NGX_LOG_EMERG, NGX_LOG_ERR, NGX_LOG_NOTICE, NGX_LOG_WARN, ngx_array_push, ngx_command_t,
    ngx_conf_t, ngx_http_add_variable, ngx_http_handler_pt, ngx_http_module_t,
    ngx_http_output_header_filter_pt, ngx_http_phases_NGX_HTTP_ACCESS_PHASE, ngx_http_request_t,
    ngx_http_top_header_filter, ngx_int_t, ngx_module_t, ngx_str_t, ngx_uint_t,
};
use ngx::http::{self, HttpModule, HttpModuleLocationConf, Request};
use ngx::{http_request_handler, http_variable_get, ngx_conf_log_error, ngx_log_error, ngx_string};

/// Stored next header filter in the filter chain.
/// Set during postconfiguration, used by our header filter.
static mut NGX_HTTP_NEXT_HEADER_FILTER: ngx_http_output_header_filter_pt = None;

mod config;
mod grpc;
mod net;
mod protos;
mod stream;

use config::ModuleConfig;

struct Module;

impl http::HttpModule for Module {
    fn module() -> &'static ngx_module_t {
        unsafe { &*ptr::addr_of!(ngx_http_inference_module) }
    }

    unsafe extern "C" fn preconfiguration(cf: *mut ngx_conf_t) -> ngx_int_t {
        // Register $inference_endpoint variable
        // NGX_HTTP_VAR_NOCACHEABLE (2) ensures the variable is re-evaluated
        // on each use, since we set it dynamically in the access phase.
        const NGX_HTTP_VAR_NOCACHEABLE: ngx_uint_t = 2;

        let cf_ref = unsafe { &mut *cf };
        let name = unsafe { &mut ngx_str_t::from_str(cf_ref.pool, "inference_endpoint") as *mut _ };
        let v = unsafe { ngx_http_add_variable(cf, name, NGX_HTTP_VAR_NOCACHEABLE) };
        if v.is_null() {
            return Status::NGX_ERROR.into();
        }
        unsafe {
            (*v).get_handler = Some(inference_endpoint_var_get);
            (*v).data = 0;
        }
        Status::NGX_OK.into()
    }

    unsafe extern "C" fn postconfiguration(cf: *mut ngx_conf_t) -> ngx_int_t {
        // Access main conf via raw FFI to get mutable access for handler registration
        unsafe {
            let cf_ref = &*cf;
            let http_conf = cf_ref.ctx as *mut ngx::ffi::ngx_http_conf_ctx_t;
            if http_conf.is_null() {
                return Status::NGX_ERROR.into();
            }
            let main_conf_ptrs = (*http_conf).main_conf;
            if main_conf_ptrs.is_null() {
                return Status::NGX_ERROR.into();
            }

            let core_module = &raw const ngx::ffi::ngx_http_core_module;
            let cmcf = *main_conf_ptrs.add((*core_module).ctx_index)
                as *mut ngx::ffi::ngx_http_core_main_conf_t;
            if cmcf.is_null() {
                return Status::NGX_ERROR.into();
            }

            // Register Access phase handler
            let h = ngx_array_push(
                &raw mut (*cmcf).phases[ngx_http_phases_NGX_HTTP_ACCESS_PHASE as usize].handlers,
            ) as *mut ngx_http_handler_pt;
            if h.is_null() {
                return Status::NGX_ERROR.into();
            }
            *h = Some(inference_access_handler);

            // Register response header filter (insert at top of chain)
            NGX_HTTP_NEXT_HEADER_FILTER = ngx_http_top_header_filter;
            ngx_http_top_header_filter = Some(inference_header_filter);
        }
        Status::NGX_OK.into()
    }
}

unsafe impl HttpModuleLocationConf for Module {
    type LocationConf = ModuleConfig;
}

/// Generate an NGINX configuration directive handler.
///
/// Each handler parses a single string argument from the directive and applies
/// it to the `ModuleConfig` using the provided expression.
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
    ngx_http_inference_set_enable,
    "inference_epp",
    |conf: &mut ModuleConfig, val: &str| {
        conf.enable = val.eq_ignore_ascii_case("on");
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_endpoint,
    "inference_epp_endpoint",
    |conf: &mut ModuleConfig, val: &str| {
        conf.epp_endpoint = Some(val.to_string());
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_failopen,
    "inference_failopen",
    |conf: &mut ModuleConfig, val: &str| {
        conf.failopen = Some(val.to_string());
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_tls,
    "inference_epp_tls",
    |conf: &mut ModuleConfig, val: &str| {
        conf.use_tls = val == "on";
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_tls_skip_verify,
    "inference_epp_tls_skip_verify",
    |conf: &mut ModuleConfig, val: &str| {
        conf.tls_skip_verify = val == "on";
    }
);

// NGINX directives table
static mut NGX_HTTP_INFERENCE_COMMANDS: [ngx_command_t; 6] = [
    ngx_command_t {
        name: ngx_string!("inference_epp"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_enable),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_epp_endpoint"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_endpoint),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_failopen"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_failopen),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_epp_tls"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_tls),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_epp_tls_skip_verify"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_tls_skip_verify),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t::empty(),
];

static NGX_HTTP_INFERENCE_MODULE_CTX: ngx_http_module_t = ngx_http_module_t {
    preconfiguration: Some(Module::preconfiguration),
    postconfiguration: Some(Module::postconfiguration),
    create_main_conf: None,
    init_main_conf: None,
    create_srv_conf: None,
    merge_srv_conf: None,
    create_loc_conf: Some(Module::create_loc_conf),
    merge_loc_conf: None,
};

// Export ngx_modules table for dynamic module loading
ngx::ngx_modules!(ngx_http_inference_module);

#[used]
#[allow(non_upper_case_globals)]
#[unsafe(no_mangle)]
pub static mut ngx_http_inference_module: ngx_module_t = ngx_module_t {
    ctx: ptr::addr_of!(NGX_HTTP_INFERENCE_MODULE_CTX) as _,
    commands: unsafe { &NGX_HTTP_INFERENCE_COMMANDS[0] as *const _ as *mut _ },
    type_: NGX_HTTP_MODULE as _,
    ..ngx_module_t::default()
};

// Variable handler for $inference_endpoint
http_variable_get!(
    inference_endpoint_var_get,
    |request: &mut http::Request, v: *mut ngx::ffi::ngx_variable_value_t, _data: usize| {
        unsafe {
            if v.is_null() {
                return Status::NGX_ERROR;
            }

            let conf = match Module::location_conf(request) {
                Some(c) => c,
                None => {
                    (*v).set_not_found(1);
                    (*v).set_len(0);
                    (*v).data = ptr::null_mut();
                    return Status::NGX_OK;
                }
            };

            // Get endpoint from request context if set by EPP
            if let Some(endpoint) = request
                .get_module_ctx::<stream::RequestCtx>(Module::module())
                .and_then(|ctx| ctx.selected_endpoint.as_ref())
            {
                let pool = request.pool();
                return set_variable_from_bytes(v, &pool, endpoint.as_bytes());
            }

            // Fall back to failopen upstream if configured
            if let Some(ref upstream) = conf.failopen {
                let pool = request.pool();
                return set_variable_from_bytes(v, &pool, upstream.as_bytes());
            }

            (*v).set_not_found(1);
            (*v).set_len(0);
            (*v).data = ptr::null_mut();
        }
        Status::NGX_OK
    }
);

/// Helper function to allocate and set variable value from bytes
#[inline]
unsafe fn set_variable_from_bytes(
    v: *mut ngx::ffi::ngx_variable_value_t,
    pool: &Pool,
    bytes: &[u8],
) -> Status {
    unsafe {
        if bytes.is_empty() {
            (*v).set_not_found(1);
            (*v).set_len(0);
            (*v).data = ptr::null_mut();
            return Status::NGX_OK;
        }

        if bytes.len() > u32::MAX as usize {
            (*v).set_not_found(1);
            (*v).set_len(0);
            (*v).data = ptr::null_mut();
            return Status::NGX_ERROR;
        }

        let data_ptr = pool.alloc_unaligned(bytes.len());
        if data_ptr.is_null() {
            (*v).set_not_found(1);
            (*v).set_len(0);
            (*v).data = ptr::null_mut();
            return Status::NGX_ERROR;
        }

        ptr::copy_nonoverlapping(bytes.as_ptr(), data_ptr as *mut u8, bytes.len());

        (*v).set_len(bytes.len() as u32);
        (*v).set_valid(1);
        (*v).set_no_cacheable(1);
        (*v).set_escape(0);
        (*v).set_not_found(0);
        (*v).data = data_ptr as *mut u8;

        Status::NGX_OK
    }
}

/// Get raw mutable pointer to module context from request.
///
/// This is needed because Rust 2024 forbids casting `&T` to `&mut T`, even
/// through raw pointers. We get the raw pointer directly from NGINX.
fn get_module_ctx_mut(
    request: &http::Request,
    module: &ngx::ffi::ngx_module_t,
) -> *mut stream::RequestCtx {
    // SAFETY: accessing request internals directly to get raw pointer
    // This avoids the &T to &mut T cast issue
    unsafe {
        let r = request.as_ref();
        let ctx_ptr = *r.ctx.add(module.ctx_index);
        ctx_ptr as *mut stream::RequestCtx
    }
}

// Access phase handler
//
// This handler implements the async EPP processing pattern:
// 1. First call: no context exists → start async EPP, return NGX_AGAIN
// 2. Re-entry while waiting: done == false → return NGX_AGAIN
// 3. Re-entry after completion: done == true → finalize result, return NGX_DECLINED
http_request_handler!(inference_access_handler, |request: &mut http::Request| {
    let conf = match Module::location_conf(request) {
        Some(c) => c,
        None => {
            return http::HTTPStatus::INTERNAL_SERVER_ERROR.into();
        }
    };

    if !conf.enable {
        return Status::NGX_DECLINED;
    }

    // Check if EPP endpoint is configured
    let endpoint = match &conf.epp_endpoint {
        Some(e) if !e.is_empty() => e.clone(),
        _ => {
            return Status::NGX_DECLINED;
        }
    };

    let failopen_enabled = conf.failopen.is_some();

    // Get raw pointer to context (may be null on first call)
    let ctx_ptr = get_module_ctx_mut(request, Module::module());

    if ctx_ptr.is_null() {
        // First call — allocate context and start body reading.
        // We need the body to send to the EPP so it can parse the model name.
        ngx_log_error!(
            NGX_LOG_DEBUG,
            request.log(),
            "inference: starting body read for EPP request to {}",
            endpoint
        );

        // Allocate context early so re-entry can find it
        let ctx = request.pool().allocate(stream::RequestCtx::default());
        if ctx.is_null() {
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "inference: failed to allocate request context"
            );
            return http::HTTPStatus::INTERNAL_SERVER_ERROR.into();
        }
        request.set_module_ctx(ctx.cast(), Module::module());

        let ctx_mut = unsafe { &mut *ctx };
        ctx_mut.body_reading_started = true;

        match stream::start_body_reading(request) {
            Ok(true) => {
                // Body already available (e.g. empty body or pre-buffered)
                ctx_mut.body_ready = true;
                // Fall through to start EPP below
            }
            Ok(false) => {
                // Body reading started asynchronously — callback will re-enter us
                return Status::NGX_AGAIN;
            }
            Err(e) => {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "inference: body read error: {:?}",
                    e
                );
                return http::HTTPStatus::INTERNAL_SERVER_ERROR.into();
            }
        }
    }

    // Re-fetch context pointer (may have been allocated above or in a previous call)
    let ctx_ptr = get_module_ctx_mut(request, Module::module());
    let ctx = unsafe { &mut *ctx_ptr };

    // Phase 2: Body is ready, start EPP if not already started
    if ctx.body_reading_started && !ctx.body_ready {
        // Re-entry from body read callback — body is now available
        ctx.body_ready = true;
    }

    if ctx.body_ready && !ctx.task_spawned && !ctx.done {
        // Body ready, EPP not started yet — start it now
        ngx_log_error!(
            NGX_LOG_DEBUG,
            request.log(),
            "inference: body ready, starting EPP stream to {}",
            endpoint
        );

        // Initialize SSL context if TLS is enabled
        // TODO: move SSL init to postconfiguration for reuse across requests
        let ssl = if conf.use_tls {
            let mut ssl_ctx = net::ssl::NgxSsl::default();
            if let Err(e) = ssl_ctx.init() {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "inference: SSL init failed: {}",
                    e
                );
                return if failopen_enabled {
                    Status::NGX_DECLINED
                } else {
                    http::HTTPStatus::INTERNAL_SERVER_ERROR.into()
                };
            }
            ssl_ctx.set_verify(conf.tls_skip_verify);
            Some(ssl_ctx)
        } else {
            None
        };

        match stream::start_epp_stream(request, ctx, &endpoint, conf.use_tls, ssl) {
            Ok(()) => return Status::NGX_AGAIN,
            Err(stream::EppError::FailOpen(ref detail)) => {
                ngx_log_error!(
                    NGX_LOG_WARN,
                    request.log(),
                    "inference: EPP connection failed, failing open: {}",
                    detail
                );
                return Status::NGX_DECLINED;
            }
            Err(e) => {
                ngx_log_error!(
                    NGX_LOG_ERR,
                    request.log(),
                    "inference: EPP start error: {}, failopen={}",
                    e,
                    failopen_enabled
                );
                return if failopen_enabled {
                    Status::NGX_DECLINED
                } else {
                    http::HTTPStatus::BAD_GATEWAY.into()
                };
            }
        }
    }

    // Phase 3: EPP in progress or complete
    if !ctx.done {
        // Still waiting for EPP
        return Status::NGX_AGAIN;
    }

    // EPP completed — finalize the result on the NGINX thread
    match stream::finalize_epp_result(ctx, failopen_enabled) {
        Ok(()) => {
            ngx_log_error!(
                NGX_LOG_NOTICE,
                request.log(),
                "inference: selected endpoint={}",
                ctx.selected_endpoint.as_deref().unwrap_or("<none>")
            );
            Status::NGX_DECLINED
        }
        Err(stream::EppError::FailOpen(ref detail)) => {
            ngx_log_error!(
                NGX_LOG_WARN,
                request.log(),
                "inference: EPP failed, failing open: {}",
                detail
            );
            Status::NGX_DECLINED
        }
        Err(e) => {
            ngx_log_error!(
                NGX_LOG_ERR,
                request.log(),
                "inference: EPP error: {}, failopen={}",
                e,
                failopen_enabled
            );
            if failopen_enabled {
                Status::NGX_DECLINED
            } else {
                http::HTTPStatus::BAD_GATEWAY.into()
            }
        }
    }
});

// Response header filter
//
// Fires when the upstream responds. Sends a fire-and-forget notification
// to EPP with the served endpoint, then passes headers through immediately.
//
// If the request didn't go through EPP, or if EPP failed (no stream),
// falls through to the next filter normally.
unsafe extern "C" fn inference_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
    let request = unsafe { &mut *r.cast::<Request>() };

    // Only process main requests, not subrequests
    if !request.is_main() {
        return call_next_header_filter(r);
    }

    // Check that EPP is enabled for this location; the config value is not used further.
    match Module::location_conf(request) {
        Some(c) if c.enable => {}
        _ => return call_next_header_filter(r),
    }

    let ctx_ptr = get_module_ctx_mut(request, Module::module());
    if ctx_ptr.is_null() {
        // No context — request didn't go through EPP
        return call_next_header_filter(r);
    }

    let ctx = unsafe { &mut *ctx_ptr };

    // If EPP failed or no stream is available, skip response phase
    if ctx.epp_stream.is_none() {
        return call_next_header_filter(r);
    }

    // Send fire-and-forget notification to EPP with the served endpoint
    if !ctx.response_phase_spawned {
        ngx_log_error!(
            NGX_LOG_DEBUG,
            request.log(),
            "inference: upstream responded, sending served endpoint to EPP"
        );

        if let Err(e) = stream::send_epp_response_notification(ctx) {
            ngx_log_error!(
                NGX_LOG_WARN,
                request.log(),
                "inference: EPP response notification failed: {}",
                e
            );
        }
    }

    // Always pass headers through immediately
    call_next_header_filter(r)
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn call_next_header_filter_with_none() {
        // When no next filter is set (None), should return NGX_OK
        unsafe {
            NGX_HTTP_NEXT_HEADER_FILTER = None;
        }
        let result = call_next_header_filter(std::ptr::null_mut());
        assert_eq!(result, Status::NGX_OK.into());
    }
}
