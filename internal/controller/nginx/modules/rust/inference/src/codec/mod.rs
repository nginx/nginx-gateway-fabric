//! Codec modules for the Endpoint Picker Protocol.
//!
//! Encoding/decoding of ext_proc gRPC messages, endpoint parsing,
//! header mutations, and notify-tier response building.

pub(crate) mod endpoint;
pub(crate) mod grpc;
pub(crate) mod mutations;
pub(crate) mod notify;
pub(crate) mod request;
pub(crate) mod response;

/// Dynamic-metadata namespace carrying the selected and served endpoints.
pub(crate) const ENVOY_LB_NS: &str = "envoy.lb";

#[cfg(test)]
pub(crate) mod test_util {
    use envoy_types::pb::envoy::service::ext_proc::v3::ProcessingRequest;
    use prost::Message;

    use super::grpc;

    /// Decode a single framed `ProcessingRequest`, asserting the frame is
    /// exactly the canonical encoding of the decoded message.
    pub(crate) fn decode_request(frame: &[u8]) -> ProcessingRequest {
        let (payload, rest) = grpc::decode(frame).unwrap();
        assert!(rest.is_empty());
        let req = ProcessingRequest::decode(payload).unwrap();
        assert_eq!(frame, grpc::encode(&req.encode_to_vec()));
        req
    }
}
