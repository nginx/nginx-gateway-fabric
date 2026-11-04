//! NGINX configuration directives for the inference module.
//!
//! Each directive maps to a handler that parses the argument and applies it to
//! the per-location `LocationConfig`. The `ngx_conf_handler!` macro handles
//! null-pointer checks, arity, and UTF-8 decoding.

use std::ffi::{c_char, c_void};
use std::ptr;
use std::time::Duration;

use ngx::core;
use ngx::ffi::{
    NGX_CONF_FLAG, NGX_CONF_TAKE1, NGX_HTTP_LOC_CONF, NGX_HTTP_LOC_CONF_OFFSET, NGX_LOG_EMERG,
    ngx_command_t, ngx_conf_t, ngx_str_t, ngx_uint_t,
};
use ngx::{ngx_conf_log_error, ngx_string};

use super::types::{LocationConfig, ResponsePhase};

/// Generate an NGINX configuration directive handler.
///
/// Handles null-pointer checks, single-argument arity, and UTF-8 decoding,
/// then calls `$apply(conf, val) -> Result<(), String>`. On error, the handler
/// logs the message at `NGX_LOG_EMERG` and returns `NGX_CONF_ERROR`.
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
                let conf = &mut *(conf as *mut LocationConfig);
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
                match ($apply)(conf, val) {
                    Ok(()) => {}
                    Err(msg) => {
                        let msg: String = msg;
                        ngx_conf_log_error!(NGX_LOG_EMERG, cf, "{}", msg);
                        return core::NGX_CONF_ERROR;
                    }
                }
            }
            core::NGX_CONF_OK
        }
    };
}

/// Parse an NGINX-style boolean flag: "on" → true, "off" → false,
/// anything else → None.
fn parse_flag(val: &str) -> Option<bool> {
    if val.eq_ignore_ascii_case("on") {
        Some(true)
    } else if val.eq_ignore_ascii_case("off") {
        Some(false)
    } else {
        None
    }
}

// --- Directive handlers ---

ngx_conf_handler!(
    ngx_http_inference_set_enable,
    "inference_epp",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.enabled = Some(
            parse_flag(val)
                .ok_or_else(|| format!("inference_epp must be 'on' or 'off', got '{val}'"))?,
        );
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_endpoint,
    "inference_epp_endpoint",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.endpoint = Some(val.to_string());
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_timeout,
    "inference_epp_timeout",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        // Same syntax as nginx's own msec directives (e.g. "1m 30s", "500ms").
        let mut s = ngx_str_t {
            len: val.len(),
            data: val.as_ptr().cast_mut(),
        };
        // SAFETY (macro-provided unsafe context): ngx_parse_time only reads
        // `s`, which borrows `val` for the duration of the call.
        let ms = ngx::ffi::ngx_parse_time(&mut s, 0);
        let ms = u64::try_from(ms)
            .map_err(|_| format!("inference_epp_timeout: invalid time value '{val}'"))?;
        conf.timeout = Some(Duration::from_millis(ms));
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_ssl_verify,
    "inference_epp_ssl_verify",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.ssl_verify = Some(parse_flag(val).ok_or_else(|| {
            format!("inference_epp_ssl_verify must be 'on' or 'off', got '{val}'")
        })?);
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_ssl_trusted_certificate,
    "inference_epp_ssl_trusted_certificate",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.ssl_trusted_certificate = Some(val.to_string());
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_ssl_name,
    "inference_epp_ssl_name",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.ssl_name = Some(val.to_string());
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_failopen,
    "inference_failopen",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.failopen = Some(val.to_string());
        Ok(())
    }
);

ngx_conf_handler!(
    ngx_http_inference_set_response_phase,
    "inference_epp_response_phase",
    |conf: &mut LocationConfig, val: &str| -> Result<(), String> {
        conf.response_phase = Some(match val {
            "notify" => ResponsePhase::Notify,
            "headers" => ResponsePhase::Headers,
            _ => {
                return Err(format!(
                    "inference_epp_response_phase must be 'notify' or 'headers', got '{val}'"
                ));
            }
        });
        Ok(())
    }
);

// --- Directives table ---

/// NGINX directives table for the inference module.
///
/// Array length = number of directives + 1 sentinel (`ngx_command_t::empty()`).
pub(crate) static mut NGX_HTTP_INFERENCE_COMMANDS: [ngx_command_t; 9] = [
    ngx_command_t {
        name: ngx_string!("inference_epp"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_FLAG) as ngx_uint_t,
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
        name: ngx_string!("inference_epp_timeout"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_timeout),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_epp_ssl_verify"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_FLAG) as ngx_uint_t,
        set: Some(ngx_http_inference_set_ssl_verify),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_epp_ssl_trusted_certificate"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_ssl_trusted_certificate),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t {
        name: ngx_string!("inference_epp_ssl_name"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_ssl_name),
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
        name: ngx_string!("inference_epp_response_phase"),
        type_: (NGX_HTTP_LOC_CONF | NGX_CONF_TAKE1) as ngx_uint_t,
        set: Some(ngx_http_inference_set_response_phase),
        conf: NGX_HTTP_LOC_CONF_OFFSET,
        offset: 0,
        post: ptr::null_mut(),
    },
    ngx_command_t::empty(),
];
