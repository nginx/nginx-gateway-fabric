//! NGINX Guardrails Streaming Filter Module

use std::ptr;

use ngx::core::Status;
use ngx::ffi::{
    NGX_HTTP_MODULE, NGX_LOG_ERR, ngx_chain_t, ngx_conf_t, ngx_http_conf_ctx_t,
    ngx_http_core_main_conf_t, ngx_http_core_module, ngx_http_handler_pt, ngx_http_module_t,
    ngx_http_phases_NGX_HTTP_ACCESS_PHASE, ngx_http_request_t, ngx_http_top_body_filter, ngx_int_t,
    ngx_module_t,
};
use ngx::http::{self, HttpModule, HttpModuleLocationConf};
use ngx::ngx_conf_log_error;

/// Request body filter function pointer type — not exposed by the ngx crate.
#[allow(non_camel_case_types)]
pub(crate) type ngx_http_request_body_filter_pt =
    Option<unsafe extern "C" fn(r: *mut ngx_http_request_t, in_: *mut ngx_chain_t) -> ngx_int_t>;

/// Header filter function pointer type (mirrors `ngx_http_output_header_filter_pt`).
#[allow(non_camel_case_types)]
pub(crate) type ngx_http_output_header_filter_pt =
    Option<unsafe extern "C" fn(r: *mut ngx_http_request_t) -> ngx_int_t>;

unsafe extern "C" {
    // The ngx crate only exposes the response body filter chain; this fills the gap.
    static mut ngx_http_top_request_body_filter: ngx_http_request_body_filter_pt;
    /// Top of the NGINX header filter chain — not exposed by the ngx crate.
    static mut ngx_http_top_header_filter: ngx_http_output_header_filter_pt;
}

mod config;
mod ctx;
mod decision;
mod directives;
mod error;
mod request_path;
mod response_path;
mod stream;
mod subrequest_client;
mod sync_ptr;

use config::ModuleConfig;
use request_path::{guardrails_access_handler, guardrails_request_body_filter};
use response_path::{guardrails_header_filter, guardrails_response_body_filter};

pub(crate) struct Module;

impl http::HttpModule for Module {
    fn module() -> &'static ngx_module_t {
        unsafe { &*ptr::addr_of!(ngx_http_guardrails_module) }
    }

    unsafe extern "C" fn postconfiguration(cf: *mut ngx_conf_t) -> ngx_int_t {
        unsafe {
            // Register header filter (two-state: suppress on first pass, commit on second).
            // Must be registered before body filters so it sits at the top of the header chain.
            ctx::NGX_HTTP_NEXT_HEADER_FILTER = ngx_http_top_header_filter;
            ngx_http_top_header_filter = Some(guardrails_header_filter);

            // Register request body filter (pass-through only). Request inspection
            // now happens in the ACCESS phase handler below so it can be performed
            // asynchronously (non-blocking) via an NGINX subrequest.
            ctx::NGX_HTTP_NEXT_REQUEST_BODY_FILTER = ngx_http_top_request_body_filter;
            ngx_http_top_request_body_filter = Some(guardrails_request_body_filter);

            // Register response body filter for response inspection
            ctx::NGX_HTTP_NEXT_BODY_FILTER = ngx_http_top_body_filter;
            ngx_http_top_body_filter = Some(guardrails_response_body_filter);

            // Register the ACCESS-phase handler used for non-blocking request
            // inspection. Access-phase handlers may return NGX_AGAIN/NGX_DONE and
            // be resumed once async work completes — unlike body filters, which
            // cannot suspend.
            // Inline of the C macro `ngx_http_conf_get_module_main_conf(cf, mod)`
            // = `((ngx_http_conf_ctx_t *) cf->ctx)->main_conf[mod.ctx_index]`.
            let http_ctx = (*cf).ctx as *mut ngx_http_conf_ctx_t;
            if http_ctx.is_null() {
                ngx_conf_log_error!(NGX_LOG_ERR, cf, "guardrails: null http conf ctx");
                return Status::NGX_ERROR.into();
            }
            let core_ctx_index = ngx_http_core_module.ctx_index;
            let cmcf = *(*http_ctx).main_conf.add(core_ctx_index) as *mut ngx_http_core_main_conf_t;
            if cmcf.is_null() {
                ngx_conf_log_error!(NGX_LOG_ERR, cf, "guardrails: could not get core main conf");
                return Status::NGX_ERROR.into();
            }
            let handlers =
                &mut (*cmcf).phases[ngx_http_phases_NGX_HTTP_ACCESS_PHASE as usize].handlers;
            let h = ngx::ffi::ngx_array_push(handlers) as *mut ngx_http_handler_pt;
            if h.is_null() {
                ngx_conf_log_error!(
                    NGX_LOG_ERR,
                    cf,
                    "guardrails: could not push access-phase handler"
                );
                return Status::NGX_ERROR.into();
            }
            *h = Some(guardrails_access_handler);

            Status::NGX_OK.into()
        }
    }
}

unsafe impl HttpModuleLocationConf for Module {
    type LocationConf = ModuleConfig;
}

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
    commands: unsafe { &directives::NGX_HTTP_GUARDRAILS_COMMANDS[0] as *const _ as *mut _ },
    type_: NGX_HTTP_MODULE as _,
    ..ngx_module_t::default()
};
