//! Build the notify-tier response messages for the EPP.
//!
//! After the upstream responds, the module sends `ResponseHeaders` with
//! served-endpoint metadata and an empty `ResponseBody` with
//! `end_of_stream = true` on the same gRPC stream. This tells the EPP
//! which endpoint handled the request and releases in-flight accounting.

use std::collections::HashMap;

use envoy_types::pb::envoy::config::core::v3::{HeaderMap, HeaderValue};
use envoy_types::pb::envoy::service::ext_proc::v3::{
    HttpHeaders, ProcessingRequest, processing_request,
};
use envoy_types::pb::google::protobuf::{Struct, Value, value::Kind};

use crate::codec::{ENVOY_LB_NS, grpc};
use crate::error::ModuleError;

/// Metadata key for the served endpoint.
const SERVED_HEADER: &str = "x-gateway-destination-endpoint-served";

/// gRPC-framed `ProcessingRequest { response_body: HttpBody { end_of_stream: true } }`.
/// The message is fixed, so it is encoded once; a unit test checks it against prost.
pub(crate) const RESPONSE_BODY_EOS: &[u8] = &[0, 0, 0, 0, 4, 0x2a, 2, 0x10, 1];

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
    use envoy_types::pb::envoy::service::ext_proc::v3::HttpBody;

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
    fn response_body_eos() {
        let encoded = grpc::encode_message(&ProcessingRequest {
            request: Some(processing_request::Request::ResponseBody(HttpBody {
                end_of_stream: true,
                ..Default::default()
            })),
            ..Default::default()
        })
        .unwrap();
        assert_eq!(encoded, RESPONSE_BODY_EOS);
        let req = decode_request(RESPONSE_BODY_EOS);

        let rb = match req.request.unwrap() {
            processing_request::Request::ResponseBody(b) => b,
            other => panic!("expected ResponseBody, got {other:?}"),
        };
        assert!(rb.end_of_stream);
        assert!(rb.body.is_empty());
    }
}
