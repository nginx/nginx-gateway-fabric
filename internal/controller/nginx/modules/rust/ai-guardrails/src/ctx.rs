//! Shared FFI-seam statics, per-request context helpers, and next-filter wrappers.

use std::ffi::c_void;
use std::ptr;

use ngx::core::Status;
use ngx::ffi::{ngx_chain_t, ngx_http_output_body_filter_pt, ngx_http_request_t, ngx_int_t};
use ngx::http::{self, HttpModule};

use crate::Module;
use crate::request_path::is_unsupported_encoding;
use crate::stream::StreamContext;
use crate::{ngx_http_output_header_filter_pt, ngx_http_request_body_filter_pt};

/// Stored next body filter in the chain (for responses)
pub(crate) static mut NGX_HTTP_NEXT_BODY_FILTER: ngx_http_output_body_filter_pt = None;

/// Stored next request body filter in the chain (for requests)
pub(crate) static mut NGX_HTTP_NEXT_REQUEST_BODY_FILTER: ngx_http_request_body_filter_pt = None;

/// Stored next header filter in the chain
pub(crate) static mut NGX_HTTP_NEXT_HEADER_FILTER: ngx_http_output_header_filter_pt = None;

/// Get raw mutable pointer to module context
pub(crate) fn get_module_ctx_mut(
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
pub(crate) unsafe fn alloc_stream_ctx(r: *mut ngx_http_request_t) -> *mut StreamContext {
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

/// Whether the upstream response carries a `Content-Encoding` the module cannot
/// inspect (anything other than `identity`).
///
/// A content-encoded (`gzip`/`br`/`deflate`/`zstd`/…) response body is transformed
/// on the wire; buffering and scanning its bytes as text inspects garbage while the
/// original compressed bytes are what gets released to the client — a guardrail
/// bypass. The response path fails closed on this, mirroring the request path's
/// `request_has_unsupported_encoding`.
///
/// Both the dedicated `headers_out.content_encoding` field (set by the proxy/gzip
/// modules) and any `Content-Encoding` entry in the generic `headers_out.headers`
/// list are checked, so detection does not depend on which one upstream populated.
/// The token semantics are shared with the request path via [`is_unsupported_encoding`].
pub(crate) unsafe fn response_has_unsupported_encoding(r: *mut ngx_http_request_t) -> bool {
    unsafe {
        // Dedicated field first: the proxy/gzip modules populate this for a
        // content-encoded response.
        let ce = (*r).headers_out.content_encoding;
        if !ce.is_null() && (*ce).value.len > 0 && !(*ce).value.data.is_null() {
            let value = std::slice::from_raw_parts((*ce).value.data, (*ce).value.len);
            if is_unsupported_encoding(value) {
                return true;
            }
        }

        // Fall back to scanning the generic headers list in case the dedicated
        // field was not wired for this response.
        let list = &(*r).headers_out.headers;
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

/// Returns true if the upstream response has `Content-Type: text/event-stream`.
pub(crate) unsafe fn is_sse_response(r: *mut ngx_http_request_t) -> bool {
    unsafe {
        let ct = (*r).headers_out.content_type;
        if ct.len > 0 && !ct.data.is_null() {
            let ct_slice = std::slice::from_raw_parts(ct.data, ct.len);
            is_sse_content_type(ct_slice)
        } else {
            false
        }
    }
}

/// Returns `true` when a Content-Type header value is `text/event-stream`.
///
/// The media type is matched case-insensitively (per RFC 9110 media types are
/// case-insensitive) on the portion before any `;` parameters, with surrounding
/// whitespace trimmed — so `Text/Event-Stream; charset=utf-8` matches.
fn is_sse_content_type(ct: &[u8]) -> bool {
    let media_type = ct.split(|&b| b == b';').next().unwrap_or(ct);
    let media_type = trim_ascii_whitespace(media_type);
    media_type.eq_ignore_ascii_case(b"text/event-stream")
}

/// Trim leading/trailing ASCII whitespace from a byte slice.
fn trim_ascii_whitespace(mut s: &[u8]) -> &[u8] {
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

/// Call the next request body filter in the chain
#[inline]
pub(crate) fn call_next_request_body_filter(
    r: *mut ngx_http_request_t,
    chain: *mut ngx_chain_t,
) -> ngx_int_t {
    unsafe {
        match NGX_HTTP_NEXT_REQUEST_BODY_FILTER {
            Some(filter) => filter(r, chain),
            None => Status::NGX_OK.into(),
        }
    }
}

/// Call the next header filter in the chain.
#[inline]
pub(crate) fn call_next_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
    unsafe {
        match NGX_HTTP_NEXT_HEADER_FILTER {
            Some(filter) => filter(r),
            None => Status::NGX_OK.into(),
        }
    }
}

/// Call the next response body filter in the chain.
#[inline]
pub(crate) fn call_next_response_body_filter(
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
    fn test_is_sse_content_type_variants() {
        assert!(is_sse_content_type(b"text/event-stream"));
        // Case-insensitive media type.
        assert!(is_sse_content_type(b"Text/Event-Stream"));
        // Parameters after `;` are ignored.
        assert!(is_sse_content_type(b"text/event-stream; charset=utf-8"));
        // Surrounding whitespace trimmed.
        assert!(is_sse_content_type(b"  text/event-stream  "));
        assert!(is_sse_content_type(b"TEXT/EVENT-STREAM;charset=UTF-8"));
        // Non-SSE types.
        assert!(!is_sse_content_type(b"application/json"));
        assert!(!is_sse_content_type(b"text/plain"));
        // Must not match on substring in a larger token.
        assert!(!is_sse_content_type(b"application/text/event-stream"));
        assert!(!is_sse_content_type(b""));
    }
}
