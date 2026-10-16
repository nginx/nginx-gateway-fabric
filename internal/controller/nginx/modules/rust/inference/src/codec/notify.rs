//! Build the response-phase messages for the EPP.
//!
//! After the upstream responds, the module sends `ResponseHeaders` with
//! served-endpoint metadata, followed by one or more `ResponseBody`
//! messages containing the actual upstream body. The final body message
//! carries `end_of_stream = true`, signalling the EPP to release
//! in-flight accounting.

use std::collections::HashMap;

use envoy_types::pb::envoy::config::core::v3::{HeaderMap, HeaderValue};
use envoy_types::pb::envoy::service::ext_proc::v3::{
    HttpBody, HttpHeaders, ProcessingRequest, processing_request,
};
use envoy_types::pb::google::protobuf::{Struct, Value, value::Kind};

use crate::codec::{ENVOY_LB_NS, grpc};
use crate::error::ModuleError;

/// Metadata key for the served endpoint.
const SERVED_HEADER: &str = "x-gateway-destination-endpoint-served";

/// Maximum body bytes per response-phase gRPC frame. Matches the EPP
/// (llm-d-router) `BodyByteLimit` of 62 000 bytes.
const RESPONSE_BODY_CHUNK_MAX: usize = 62_000;

/// Build a gRPC-framed `ResponseHeaders` message carrying the served endpoint
/// in both the headers map and the `envoy.lb` dynamic metadata namespace.
pub(crate) fn encode_response_headers(served_endpoint: &str) -> Result<Vec<u8>, ModuleError> {
    let metadata = build_served_metadata(served_endpoint);

    let req = ProcessingRequest {
        request: Some(processing_request::Request::ResponseHeaders(HttpHeaders {
            headers: Some(HeaderMap {
                headers: vec![HeaderValue {
                    key: SERVED_HEADER.into(),
                    raw_value: served_endpoint.as_bytes().to_vec(),
                    ..Default::default()
                }],
            }),
            end_of_stream: false,
            ..Default::default()
        })),
        metadata_context: Some(metadata),
        ..Default::default()
    };

    grpc::encode_message(&req)
}

/// Build a single gRPC-framed `ResponseBody` chunk.
fn encode_response_body_chunk(body: &[u8], eos: bool) -> Result<Vec<u8>, ModuleError> {
    let req = ProcessingRequest {
        request: Some(processing_request::Request::ResponseBody(HttpBody {
            body: body.to_vec(),
            end_of_stream: eos,
            ..Default::default()
        })),
        ..Default::default()
    };
    grpc::encode_message(&req)
}

/// Build gRPC-framed `ResponseBody` messages for `body`, splitting at
/// [`RESPONSE_BODY_CHUNK_MAX`] boundaries. `eos` is set only on the very
/// last chunk.
pub(crate) fn encode_response_body_frames(
    body: &[u8],
    eos: bool,
) -> Result<Vec<Vec<u8>>, ModuleError> {
    if body.is_empty() {
        return Ok(vec![encode_response_body_chunk(b"", eos)?]);
    }
    let chunks: Vec<&[u8]> = body.chunks(RESPONSE_BODY_CHUNK_MAX).collect();
    let last_idx = chunks.len() - 1;
    chunks
        .into_iter()
        .enumerate()
        .map(|(i, chunk)| encode_response_body_chunk(chunk, eos && i == last_idx))
        .collect()
}

/// Build `envoy.lb` metadata with the served endpoint.
fn build_served_metadata(
    served_endpoint: &str,
) -> envoy_types::pb::envoy::config::core::v3::Metadata {
    let fields: HashMap<String, Value> = [(
        SERVED_HEADER.into(),
        Value {
            kind: Some(Kind::StringValue(served_endpoint.into())),
        },
    )]
    .into();

    envoy_types::pb::envoy::config::core::v3::Metadata {
        filter_metadata: [(ENVOY_LB_NS.into(), Struct { fields })].into(),
        ..Default::default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::codec::test_util::decode_request;

    #[test]
    fn response_headers_carries_served_endpoint() {
        let frame = encode_response_headers("10.0.0.1:8080").unwrap();
        let req = decode_request(&frame);

        let rh = match req.request.unwrap() {
            processing_request::Request::ResponseHeaders(h) => h,
            other => panic!("expected ResponseHeaders, got {other:?}"),
        };
        assert!(!rh.end_of_stream);

        let headers = rh.headers.unwrap().headers;
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0].key, SERVED_HEADER);
        assert_eq!(headers[0].raw_value, b"10.0.0.1:8080");
    }

    #[test]
    fn response_headers_has_metadata() {
        let frame = encode_response_headers("10.0.0.1:8080").unwrap();
        let req = decode_request(&frame);

        let meta = req.metadata_context.unwrap();
        let lb = meta.filter_metadata.get(ENVOY_LB_NS).unwrap();
        let ep = lb.fields.get(SERVED_HEADER).unwrap();
        if let Some(Kind::StringValue(v)) = &ep.kind {
            assert_eq!(v, "10.0.0.1:8080");
        } else {
            panic!("expected StringValue for served endpoint");
        }
    }

    #[test]
    fn response_body_empty_eos() {
        let frames = encode_response_body_frames(b"", true).unwrap();
        assert_eq!(frames.len(), 1);
        let req = decode_request(&frames[0]);

        let rb = match req.request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(rb.end_of_stream);
        assert!(rb.body.is_empty());
    }

    #[test]
    fn response_body_small_payload() {
        let body = b"hello world";
        let frames = encode_response_body_frames(body, true).unwrap();
        assert_eq!(frames.len(), 1);
        let req = decode_request(&frames[0]);

        let rb = match req.request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(rb.end_of_stream);
        assert_eq!(rb.body, body);
    }

    #[test]
    fn response_body_chunked_at_boundary() {
        // Body larger than RESPONSE_BODY_CHUNK_MAX → multiple frames.
        let big = vec![0xAB; RESPONSE_BODY_CHUNK_MAX + 1];
        let frames = encode_response_body_frames(&big, true).unwrap();
        assert_eq!(frames.len(), 2);

        let req0 = decode_request(&frames[0]);
        let rb0 = match req0.request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(!rb0.end_of_stream);
        assert_eq!(rb0.body.len(), RESPONSE_BODY_CHUNK_MAX);

        let req1 = decode_request(&frames[1]);
        let rb1 = match req1.request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(rb1.end_of_stream);
        assert_eq!(rb1.body.len(), 1);
    }

    #[test]
    fn response_body_non_eos_intermediate() {
        // Intermediate chunk: eos=false even on last sub-chunk.
        let body = b"data";
        let frames = encode_response_body_frames(body, false).unwrap();
        assert_eq!(frames.len(), 1);
        let req = decode_request(&frames[0]);

        let rb = match req.request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(!rb.end_of_stream);
        assert_eq!(rb.body, body);
    }

    /// A body of exactly 62 001 bytes must be split into two frames: one of
    /// 62 000 bytes and one of 1 byte. This tests against the literal EPP
    /// `BodyByteLimit` rather than deriving from the constant.
    #[test]
    fn response_body_splits_at_epp_body_byte_limit() {
        let body = vec![0x42; 62_001];
        let frames = encode_response_body_frames(&body, true).unwrap();
        assert_eq!(frames.len(), 2);

        let rb0 = match decode_request(&frames[0]).request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(!rb0.end_of_stream);
        assert_eq!(rb0.body.len(), 62_000);

        let rb1 = match decode_request(&frames[1]).request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(rb1.end_of_stream);
        assert_eq!(rb1.body.len(), 1);
    }
}
