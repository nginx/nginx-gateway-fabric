//! NGINX configuration directives for the guardrails module.

use std::ffi::{c_char, c_void};
use std::ptr;

use ngx::core;
use ngx::ffi::{
    NGX_CONF_FLAG, NGX_CONF_TAKE1, NGX_HTTP_LOC_CONF, NGX_HTTP_LOC_CONF_OFFSET, NGX_LOG_EMERG,
    ngx_command_t, ngx_conf_t, ngx_str_t, ngx_uint_t,
};
use ngx::{ngx_conf_log_error, ngx_string};

use crate::config::ModuleConfig;

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
    ngx_http_guardrails_set_internal_uri,
    "guardrails_internal_uri",
    |conf: &mut ModuleConfig, val: &str| {
        conf.internal_uri = Some(val.to_string());
    }
);

// NGINX directives table
pub(crate) static mut NGX_HTTP_GUARDRAILS_COMMANDS: [ngx_command_t; 6] = [
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
        name: ngx_string!("guardrails_internal_uri"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_guardrails_set_internal_uri),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t::empty(),
];
