//! Rust FFI library for NGINX EPP (Endpoint Picker) integration.
//!
//! This cdylib exposes a stable C ABI callable from a thin NGINX C module.
//! For the POC, we provide a blocking API that spins a Tokio runtime per call
//! and returns a destination endpoint as a C string on success.
//!
//! FFI contract:
//! - `rust_epp_get_endpoint` returns 0 on success, non-zero on error.
//! - On success, `*endpoint_out` is set to a newly-allocated C string that the caller must free via `rust_epp_free`.
//! - On error, `*error_out` is set to a newly-allocated error message string the caller must free via `rust_epp_free`.
//!
//! NOTE: The gRPC ext_proc client to EPP is stubbed initially in `epp_client.rs`.
//!       Replace the stub with a tonic-based client implementation.

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int, c_uchar};
use std::ptr;

mod epp_client;

fn cstr_to_str<'a>(p: *const c_char, name: &str) -> Result<&'a str, String> {
    if p.is_null() {
        return Err(format!("null pointer for {}", name));
    }
    unsafe { CStr::from_ptr(p) }
        .to_str()
        .map_err(|e| format!("invalid utf8 for {}: {}", name, e))
}

/// Allocate a C string from a Rust string for FFI return values.
fn to_c_string(s: &str) -> *mut c_char {
    CString::new(s).unwrap_or_else(|_| CString::new("").unwrap()).into_raw()
}

/// Set error_out with provided error message (allocates via CString) if error_out is non-null.
fn set_error(error_out: *mut *mut c_char, msg: &str) {
    if !error_out.is_null() {
        unsafe {
            *error_out = to_c_string(msg);
        }
    }
}

/// rust_epp_get_endpoint
///
/// Inputs:
/// - host: C string for EPP host (Service DNS name)
/// - port: C string for EPP port
/// - method: HTTP method of client request
/// - headers_json: JSON string of request headers (lowercased keys recommended)
/// - body_ptr/body_len: request body bytes (optional; pass null/0 if none)
///
/// Outputs:
/// - endpoint_out: on success, allocated C string "host:port" of destination endpoint
/// - error_out: on error, allocated error message string
///
/// Return:
/// - 0 on success, non-zero on error
#[no_mangle]
pub extern "C" fn rust_epp_get_endpoint(
    host: *const c_char,
    port: *const c_char,
    method: *const c_char,
    headers_json: *const c_char,
    body_ptr: *const c_uchar,
    body_len: usize,
    endpoint_out: *mut *mut c_char,
    error_out: *mut *mut c_char,
) -> c_int {
    // Initialize outputs to null for safety.
    if !endpoint_out.is_null() {
        unsafe { *endpoint_out = ptr::null_mut() }
    }
    if !error_out.is_null() {
        unsafe { *error_out = ptr::null_mut() }
    }

    // Convert inputs
    let host = match cstr_to_str(host, "host") {
        Ok(v) => v,
        Err(e) => {
            set_error(error_out, &e);
            return 1;
        }
    };
    let port = match cstr_to_str(port, "port") {
        Ok(v) => v,
        Err(e) => {
            set_error(error_out, &e);
            return 2;
        }
    };
    let method = match cstr_to_str(method, "method") {
        Ok(v) => v,
        Err(e) => {
            set_error(error_out, &e);
            return 3;
        }
    };
    let headers_json = match cstr_to_str(headers_json, "headers_json") {
        Ok(v) => v,
        Err(e) => {
            set_error(error_out, &e);
            return 4;
        }
    };
    // Body slice (may be empty)
    let body: &[u8] = if !body_ptr.is_null() && body_len > 0 {
        unsafe { std::slice::from_raw_parts(body_ptr, body_len) }
    } else {
        &[]
    };

    // Parse headers JSON (best-effort; if fail, use empty object)
    let headers_val: serde_json::Value = match serde_json::from_str(headers_json) {
        Ok(v) => v,
        Err(_) => serde_json::json!({}),
    };

    // Run the (stubbed) EPP client to get the destination endpoint.
    // For POC simplicity we create a runtime and block on it per call.
    let rt = match tokio::runtime::Runtime::new() {
        Ok(r) => r,
        Err(e) => {
            set_error(error_out, &format!("tokio runtime init failed: {}", e));
            return 10;
        }
    };

    let res = rt.block_on(epp_client::get_destination_endpoint(
        host, port, method, headers_val, body,
    ));

    match res {
        Ok(endpoint) => {
            if endpoint_out.is_null() {
                set_error(error_out, "endpoint_out is null");
                return 11;
            }
            unsafe {
                *endpoint_out = to_c_string(&endpoint);
            }
            0
        }
        Err(err) => {
            set_error(error_out, &err);
            12
        }
    }
}

/// Free a C string previously returned by this library (e.g., endpoint_out or error_out).
#[no_mangle]
pub extern "C" fn rust_epp_free(p: *mut c_char) {
    if !p.is_null() {
        unsafe {
            let _ = CString::from_raw(p);
        }
    }
}
