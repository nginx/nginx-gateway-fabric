//! NGINX Inference (Endpoint Picker) Module
//!
//! Implements the Gateway API Inference Extension Endpoint Picker Protocol
//! natively in nginx. The module registers as a PRECONTENT-phase handler that
//! calls the EPP via gRPC, applies header mutations, and sets the
//! `$inference_endpoint` variable for upstream routing.

use std::ffi::{c_char, c_void};
use std::ptr;

use ngx::core::Status;
use ngx::core::{NGX_CONF_ERROR, NGX_CONF_OK};
use ngx::ffi::{
    NGX_HTTP_MODULE, NGX_LOG_EMERG, NGX_LOG_ERR, ngx_conf_t, ngx_http_handler_pt,
    ngx_http_module_t, ngx_http_phases_NGX_HTTP_PRECONTENT_PHASE, ngx_int_t, ngx_module_t,
};
use ngx::http::{HttpModule, HttpModuleLocationConf, HttpModuleMainConf, NgxHttpCoreModule};
use ngx::ngx_conf_log_error;

mod codec;
mod config;
mod error;
mod handler;
mod net;
mod stream;
use config::types::LocationConfig;

pub(crate) struct Module;

impl HttpModule for Module {
    fn module() -> &'static ngx_module_t {
        unsafe { &*ptr::addr_of!(ngx_http_inference_module) }
    }

    /// Register the `$inference_endpoint` variable.
    unsafe extern "C" fn preconfiguration(cf: *mut ngx_conf_t) -> ngx_int_t {
        unsafe { handler::register_variable(cf) }
    }

    /// Register the PRECONTENT-phase handler and header/body filters at config-load time.
    unsafe extern "C" fn postconfiguration(cf: *mut ngx_conf_t) -> ngx_int_t {
        unsafe {
            // Register the header and body filters for the response phase.
            handler::register_header_filter();
            handler::register_body_filter();

            let Some(cmcf) = NgxHttpCoreModule::main_conf_mut(&*cf) else {
                ngx_conf_log_error!(NGX_LOG_ERR, cf, "inference: could not get core main conf");
                return Status::NGX_ERROR.into();
            };
            let handlers =
                &mut cmcf.phases[ngx_http_phases_NGX_HTTP_PRECONTENT_PHASE as usize].handlers;
            let h = ngx::ffi::ngx_array_push(handlers) as *mut ngx_http_handler_pt;
            if h.is_null() {
                ngx_conf_log_error!(
                    NGX_LOG_ERR,
                    cf,
                    "inference: could not push precontent-phase handler"
                );
                return Status::NGX_ERROR.into();
            }
            *h = Some(handler::inference_precontent_handler);

            Status::NGX_OK.into()
        }
    }
}

unsafe impl HttpModuleLocationConf for Module {
    type LocationConf = LocationConfig;
}

/// Merge a location configuration with its parent, logging any validation
/// error so `nginx -t` reports the actual cause.
unsafe extern "C" fn merge_loc_conf(
    cf: *mut ngx_conf_t,
    prev: *mut c_void,
    conf: *mut c_void,
) -> *mut c_char {
    let prev = unsafe { &*(prev as *const LocationConfig) };
    let conf = unsafe { &mut *(conf as *mut LocationConfig) };
    match conf.merge_from(prev) {
        Ok(()) => NGX_CONF_OK,
        Err(e) => {
            ngx_conf_log_error!(NGX_LOG_EMERG, cf, "{}", e);
            NGX_CONF_ERROR
        }
    }
}

static NGX_HTTP_INFERENCE_MODULE_CTX: ngx_http_module_t = ngx_http_module_t {
    preconfiguration: Some(Module::preconfiguration),
    postconfiguration: Some(Module::postconfiguration),
    create_main_conf: None,
    init_main_conf: None,
    create_srv_conf: None,
    merge_srv_conf: None,
    create_loc_conf: Some(Module::create_loc_conf),
    merge_loc_conf: Some(merge_loc_conf),
};

// Export ngx_modules table for dynamic module loading
ngx::ngx_modules!(ngx_http_inference_module);

#[used]
#[allow(non_upper_case_globals)]
#[unsafe(no_mangle)]
pub static mut ngx_http_inference_module: ngx_module_t = ngx_module_t {
    ctx: ptr::addr_of!(NGX_HTTP_INFERENCE_MODULE_CTX) as _,
    commands: unsafe { &config::directives::NGX_HTTP_INFERENCE_COMMANDS[0] as *const _ as *mut _ },
    type_: NGX_HTTP_MODULE as _,
    ..ngx_module_t::default()
};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn module_is_http_module_with_ctx() {
        let m = Module::module();
        assert_eq!(m.type_, NGX_HTTP_MODULE as usize);
        assert_eq!(
            m.ctx as *const ngx_http_module_t,
            ptr::addr_of!(NGX_HTTP_INFERENCE_MODULE_CTX)
        );
        assert!(!m.commands.is_null());
    }
}
