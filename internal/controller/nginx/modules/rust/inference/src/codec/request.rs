//! Build ext_proc `ProcessingRequest` messages from HTTP request data.
//!
//! The module sends one or more messages per EPP call:
//! 1. `RequestHeaders` — pseudo-headers followed by the client headers.
//! 2. One or more `RequestBody` chunks (≤64 KB each), only the last carrying
//!    `end_of_stream = true`.

use crate::codec::grpc;
use crate::error::ModuleError;
use envoy_types::pb::envoy::config::core::v3::{HeaderMap, HeaderValue};
use envoy_types::pb::envoy::service::ext_proc::v3::{
    HttpBody, HttpHeaders, ProcessingRequest, processing_request,
};

/// A client header: lowercased name and raw value.
pub(crate) struct Header<'a> {
    pub name: &'a str,
    pub value: &'a [u8],
}

/// Pseudo-headers extracted from the nginx request.
pub(crate) struct PseudoHeaders<'a> {
    pub method: &'a [u8],
    pub path: &'a [u8],
    pub authority: &'a [u8],
    pub scheme: &'a [u8],
}

/// Build a gRPC-framed `RequestHeaders` message.
///
/// Pseudo-headers come first (`:method`, `:path`, `:authority`, `:scheme`),
/// followed by the regular client headers. Values are placed in `raw_value`,
/// which is what GAIE reads.
pub(crate) fn encode_request_headers(
    pseudo: &PseudoHeaders<'_>,
    headers: &[Header<'_>],
) -> Result<Vec<u8>, ModuleError> {
    let mut entries = Vec::with_capacity(4 + headers.len());

    // Pseudo-headers first, in conventional order.
    for (name, value) in [
        (":method", pseudo.method),
        (":path", pseudo.path),
        (":authority", pseudo.authority),
        (":scheme", pseudo.scheme),
    ] {
        entries.push(header_value(name, value));
    }

    for h in headers {
        entries.push(header_value(h.name, h.value));
    }

    let req = ProcessingRequest {
        request: Some(processing_request::Request::RequestHeaders(HttpHeaders {
            headers: Some(HeaderMap { headers: entries }),
            end_of_stream: false,
            ..Default::default()
        })),
        ..Default::default()
    };

    grpc::encode_message(&req)
}

/// Maximum body bytes per gRPC frame. EPP (llm-d-router) defaults to a 4 MB
/// `grpc.MaxRecvMsgSize`; we stay well under that with 64 KB per message.
const BODY_CHUNK_MAX: usize = 64 * 1024;

/// Validate a prospective collected body length before growing its buffer.
/// Only the platform allocation limit applies; nginx's configured request-size
/// policy remains responsible for body limits, and per-frame size is checked at
/// encode time.
pub(crate) fn check_body_len(len: usize) -> Result<(), ModuleError> {
    if len > isize::MAX as usize {
        return Err(ModuleError::GrpcFrame(
            "request body length exceeds allocation limit".into(),
        ));
    }
    Ok(())
}

/// Build a single gRPC-framed `RequestBody` chunk.
fn encode_request_body_chunk(body: &[u8], eos: bool) -> Result<Vec<u8>, ModuleError> {
    let req = ProcessingRequest {
        request: Some(processing_request::Request::RequestBody(HttpBody {
            body: body.to_vec(),
            end_of_stream: eos,
            ..Default::default()
        })),
        ..Default::default()
    };
    grpc::encode_message(&req)
}

/// Build gRPC-framed `RequestBody` messages, splitting at [`BODY_CHUNK_MAX`]
/// boundaries. Only the last chunk carries `end_of_stream = true`.
pub(crate) fn encode_request_body_frames(body: &[u8]) -> Result<Vec<Vec<u8>>, ModuleError> {
    if body.is_empty() {
        return Ok(vec![encode_request_body_chunk(b"", true)?]);
    }
    let chunks: Vec<&[u8]> = body.chunks(BODY_CHUNK_MAX).collect();
    let last_idx = chunks.len() - 1;
    chunks
        .into_iter()
        .enumerate()
        .map(|(i, chunk)| encode_request_body_chunk(chunk, i == last_idx))
        .collect()
}

/// Build a gRPC-framed `RequestBody` message with `end_of_stream = true`.
/// Convenience wrapper used by tests that send a single body frame.
#[cfg(test)]
pub(crate) fn encode_request_body(body: Vec<u8>) -> Result<Vec<u8>, ModuleError> {
    encode_request_body_chunk(&body, true)
}

fn header_value(key: &str, raw: &[u8]) -> HeaderValue {
    HeaderValue {
        key: key.to_owned(),
        raw_value: raw.to_vec(),
        ..Default::default()
    }
}

#[cfg(test)]
mod tests {
    use prost::{Message, length_delimiter_len};

    use super::*;
    use crate::codec::test_util::decode_request;

    /// Exact encoded length of a `ProcessingRequest::RequestBody` with the
    /// given body size and `end_of_stream = true`.
    fn body_message_len(len: usize) -> Result<usize, ModuleError> {
        let overflow = || ModuleError::GrpcFrame("request body wire length overflow".into());
        let bytes_len = if len == 0 {
            0
        } else {
            len.checked_add(1 + length_delimiter_len(len))
                .ok_or_else(overflow)?
        };
        let http_body_len = bytes_len.checked_add(2).ok_or_else(overflow)?;
        http_body_len
            .checked_add(1 + length_delimiter_len(http_body_len))
            .ok_or_else(overflow)
    }

    #[test]
    fn body_wire_lengths_match_prost_at_delimiter_boundaries() {
        for (len, expected) in [
            (0, 4),
            (1, 7),
            (123, 129),
            (124, 131),
            (127, 134),
            (128, 136),
            (16_378, 16_386),
            (16_379, 16_388),
            (16_383, 16_392),
            (16_384, 16_394),
        ] {
            let frame = encode_request_body(vec![0xff; len]).unwrap();
            let req = decode_request(&frame);
            assert_eq!(
                body_message_len(len).unwrap(),
                expected,
                "body length {len}"
            );
            assert_eq!(req.encoded_len(), expected, "body length {len}");
            assert_eq!(frame.len(), expected + 5);
        }
    }

    #[test]
    fn body_length_limit_is_allocation_only() {
        // check_body_len now only guards allocation (isize::MAX), not gRPC framing.
        assert!(check_body_len(0).is_ok());
        assert!(check_body_len(isize::MAX as usize).is_ok());
        assert!(check_body_len(isize::MAX as usize + 1).is_err());
        assert!(check_body_len(usize::MAX).is_err());
    }

    #[test]
    fn body_frames_preserve_wire_tags_and_eos() {
        assert_eq!(
            encode_request_body(vec![]).unwrap(),
            [0, 0, 0, 0, 4, 0x22, 2, 0x10, 1],
        );
        assert_eq!(
            encode_request_body(vec![0xff]).unwrap(),
            [0, 0, 0, 0, 7, 0x22, 5, 0x0a, 1, 0xff, 0x10, 1],
        );
    }

    #[test]
    fn body_frames_chunking_at_64kb_boundary() {
        // Body smaller than BODY_CHUNK_MAX → one frame with EOS.
        let frames = encode_request_body_frames(b"small").unwrap();
        assert_eq!(frames.len(), 1);
        let req = decode_request(&frames[0]);
        let rb = match req.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            other => panic!("expected RequestBody, got {other:?}"),
        };
        assert_eq!(rb.body, b"small");
        assert!(rb.end_of_stream);

        // Empty body → one frame with EOS.
        let frames = encode_request_body_frames(b"").unwrap();
        assert_eq!(frames.len(), 1);
        let req = decode_request(&frames[0]);
        let rb = match req.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            _ => panic!("expected RequestBody"),
        };
        assert!(rb.body.is_empty());
        assert!(rb.end_of_stream);

        // Exactly BODY_CHUNK_MAX → one frame.
        let big = vec![0xAB; BODY_CHUNK_MAX];
        let frames = encode_request_body_frames(&big).unwrap();
        assert_eq!(frames.len(), 1);
        let req = decode_request(&frames[0]);
        let rb = match req.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            _ => panic!("expected RequestBody"),
        };
        assert_eq!(rb.body.len(), BODY_CHUNK_MAX);
        assert!(rb.end_of_stream);

        // BODY_CHUNK_MAX + 1 → two frames, EOS only on last.
        let big = vec![0xCD; BODY_CHUNK_MAX + 1];
        let frames = encode_request_body_frames(&big).unwrap();
        assert_eq!(frames.len(), 2);
        let req0 = decode_request(&frames[0]);
        let rb0 = match req0.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            _ => panic!("expected RequestBody"),
        };
        assert_eq!(rb0.body.len(), BODY_CHUNK_MAX);
        assert!(!rb0.end_of_stream);
        let req1 = decode_request(&frames[1]);
        let rb1 = match req1.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            _ => panic!("expected RequestBody"),
        };
        assert_eq!(rb1.body.len(), 1);
        assert!(rb1.end_of_stream);

        // 3 * BODY_CHUNK_MAX → three frames.
        let big = vec![0xEF; BODY_CHUNK_MAX * 3];
        let frames = encode_request_body_frames(&big).unwrap();
        assert_eq!(frames.len(), 3);
        for (i, frame) in frames.iter().enumerate() {
            let req = decode_request(frame);
            let rb = match req.request.unwrap() {
                processing_request::Request::RequestBody(b) => b,
                _ => panic!("expected RequestBody"),
            };
            assert_eq!(rb.body.len(), BODY_CHUNK_MAX);
            assert_eq!(rb.end_of_stream, i == 2);
        }
    }

    #[test]
    fn request_headers_has_pseudos_first() {
        let pseudo = PseudoHeaders {
            method: b"POST",
            path: b"/v1/chat/completions",
            authority: b"example.com",
            scheme: b"https",
        };
        let hdrs = [
            Header {
                name: "content-type",
                value: b"application/json",
            },
            Header {
                name: "x-custom",
                value: b"val",
            },
        ];
        let frame = encode_request_headers(&pseudo, &hdrs).unwrap();
        let req = decode_request(&frame);

        let rh = match req.request.unwrap() {
            processing_request::Request::RequestHeaders(h) => h,
            other => panic!("expected RequestHeaders, got {other:?}"),
        };
        assert!(!rh.end_of_stream);

        let entries = &rh.headers.unwrap().headers;
        assert_eq!(entries.len(), 6);
        assert_eq!(entries[0].key, ":method");
        assert_eq!(entries[0].raw_value, b"POST");
        assert_eq!(entries[1].key, ":path");
        assert_eq!(entries[1].raw_value, b"/v1/chat/completions");
        assert_eq!(entries[2].key, ":authority");
        assert_eq!(entries[3].key, ":scheme");
        assert_eq!(entries[4].key, "content-type");
        assert_eq!(entries[5].key, "x-custom");
    }

    #[test]
    fn request_headers_empty_headers() {
        let pseudo = PseudoHeaders {
            method: b"GET",
            path: b"/",
            authority: b"localhost",
            scheme: b"http",
        };
        let frame = encode_request_headers(&pseudo, &[]).unwrap();
        let req = decode_request(&frame);

        let rh = match req.request.unwrap() {
            processing_request::Request::RequestHeaders(h) => h,
            _ => panic!("expected RequestHeaders"),
        };
        // Only pseudo-headers.
        assert_eq!(rh.headers.unwrap().headers.len(), 4);
    }

    #[test]
    fn request_headers_raw_value_is_bytes() {
        let pseudo = PseudoHeaders {
            method: b"GET",
            path: b"/",
            authority: b"host",
            scheme: b"http",
        };
        let hdrs = [Header {
            name: "binary",
            value: &[0xFF, 0x00, 0x80],
        }];
        let frame = encode_request_headers(&pseudo, &hdrs).unwrap();
        let req = decode_request(&frame);

        let rh = match req.request.unwrap() {
            processing_request::Request::RequestHeaders(h) => h,
            _ => panic!("expected RequestHeaders"),
        };
        let binary_hdr = &rh.headers.unwrap().headers[4];
        assert_eq!(binary_hdr.raw_value, &[0xFF, 0x00, 0x80]);
    }

    #[test]
    fn request_headers_preserve_duplicate_values_and_order() {
        let pseudo = PseudoHeaders {
            method: b"POST",
            path: b"/",
            authority: b"host",
            scheme: b"http",
        };
        let headers = [
            Header {
                name: "x-test",
                value: b"first",
            },
            Header {
                name: "x-test",
                value: b"second",
            },
        ];
        let req = decode_request(&encode_request_headers(&pseudo, &headers).unwrap());
        let Some(processing_request::Request::RequestHeaders(headers)) = req.request else {
            panic!("expected RequestHeaders");
        };
        let entries = headers.headers.unwrap().headers;
        assert_eq!(entries.len(), 6);
        assert_eq!(entries[4].key, "x-test");
        assert_eq!(entries[4].raw_value, b"first");
        assert_eq!(entries[5].key, "x-test");
        assert_eq!(entries[5].raw_value, b"second");
    }

    #[test]
    fn request_body_with_eos() {
        let body = b"hello body";
        let frame = encode_request_body(body.to_vec()).unwrap();
        let req = decode_request(&frame);

        let rb = match req.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            other => panic!("expected RequestBody, got {other:?}"),
        };
        assert_eq!(rb.body, body);
        assert!(rb.end_of_stream);
    }

    #[test]
    fn request_body_empty() {
        let frame = encode_request_body(Vec::new()).unwrap();
        let req = decode_request(&frame);

        let rb = match req.request.unwrap() {
            processing_request::Request::RequestBody(b) => b,
            _ => panic!("expected RequestBody"),
        };
        assert!(rb.body.is_empty());
        assert!(rb.end_of_stream);
    }
}
