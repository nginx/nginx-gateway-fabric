//! Decode ext_proc `ProcessingResponse` messages from the EPP.
//!
//! The EPP replies to `RequestHeaders` and one or more `RequestBody` chunks
//! with `ProcessingResponse` messages that contain header mutations and the
//! selected endpoint. When the EPP uses `FULL_DUPLEX_STREAMED` body mode,
//! it may split the body reply into multiple chunks; only the last carries
//! `end_of_stream = true` in its `StreamedBodyResponse`.

use envoy_types::pb::envoy::service::ext_proc::v3::{
    CommonResponse, HeaderMutation, ProcessingResponse, body_mutation, processing_response,
};
use prost::Message;

use crate::codec::ENVOY_LB_NS;
use crate::codec::endpoint::{self, Endpoint};
use crate::codec::mutations::{self, MutationSet};
use crate::error::ModuleError;

/// Header name where the EPP places the selected endpoint.
const ENDPOINT_HEADER: &str = "x-gateway-destination-endpoint";

/// Result of decoding the EPP's replies to the request-phase messages.
#[derive(Debug)]
pub(crate) struct EppResult {
    /// Ordered endpoint list. The first entry is the primary target.
    pub endpoints: Vec<Endpoint>,
    /// Collected header mutations to apply to the upstream request.
    pub mutations: MutationSet,
}

/// Incremental decoder for the EPP's replies to `RequestHeaders` and
/// `RequestBody`.
///
/// Feed each `ProcessingResponse` payload to [`Self::on_message`]; it returns
/// `true` once the `RequestBody` reply has been seen, which is the last message
/// the EPP sends before the response phase. [`Self::finish`] then extracts the
/// endpoint. This avoids waiting for end-of-stream, which never arrives while
/// the stream is kept open for the response phase.
#[derive(Default)]
pub(crate) struct RequestReplyDecoder {
    mutations: MutationSet,
    endpoint_raw: Option<Vec<u8>>,
    headers_seen: bool,
    body_seen: bool,
}

impl RequestReplyDecoder {
    /// Process one `ProcessingResponse` payload.
    ///
    /// Returns `Err(ImmediateResponse)` if the EPP rejected the request, and
    /// `Err(InvalidResponse)` for a reply that does not match the protocol order
    /// (`RequestHeaders`, then `RequestBody`).
    pub(crate) fn on_message(&mut self, payload: &[u8]) -> Result<bool, ModuleError> {
        let resp = ProcessingResponse::decode(payload)
            .map_err(|e| ModuleError::ProtoDecode(e.to_string()))?;

        let mut complete = false;
        match &resp.response {
            Some(processing_response::Response::ImmediateResponse(ir)) => {
                let code = clamp_status_code(ir.status.as_ref().map(|s| s.code));
                return Err(ModuleError::ImmediateResponse(code));
            }
            Some(processing_response::Response::RequestHeaders(hr)) => {
                if self.headers_seen || self.body_seen {
                    return Err(ModuleError::InvalidResponse(
                        "duplicate or out-of-order request headers response".into(),
                    ));
                }
                self.process_common(hr.response.as_ref())?;
                self.headers_seen = true;
            }
            Some(processing_response::Response::RequestBody(br)) => {
                if !self.headers_seen || self.body_seen {
                    return Err(ModuleError::InvalidResponse(
                        "duplicate or out-of-order request body response".into(),
                    ));
                }
                self.process_common(br.response.as_ref())?;
                // In FULL_DUPLEX_STREAMED mode the EPP may split the body
                // reply into multiple chunks. Only the last carries EOS.
                // Without a StreamedResponse body_mutation (legacy/buffered
                // mode), treat as a single complete reply.
                if is_streamed_body_eos(br.response.as_ref()) {
                    self.body_seen = true;
                    complete = true;
                }
            }
            _ => {
                return Err(ModuleError::InvalidResponse(
                    "unexpected response type during request phase".into(),
                ));
            }
        }

        // Metadata is the fallback, only used if no header mutation provided one.
        if self.endpoint_raw.is_none() {
            self.endpoint_raw = extract_endpoint_from_metadata(&resp);
        }
        Ok(complete)
    }

    /// Extract the endpoint list and collected mutations.
    pub(crate) fn finish(self) -> Result<EppResult, ModuleError> {
        let raw = self.endpoint_raw.ok_or(ModuleError::NoEndpoint)?;
        let raw_str = core::str::from_utf8(&raw)
            .map_err(|e| ModuleError::InvalidResponse(format!("endpoint is not UTF-8: {e}")))?;
        let endpoints = endpoint::parse_endpoint_list(raw_str)?;

        Ok(EppResult {
            endpoints,
            mutations: self.mutations,
        })
    }

    /// Merge a `CommonResponse`'s header mutations and pick up the endpoint
    /// header, if present.
    fn process_common(&mut self, cr: Option<&CommonResponse>) -> Result<(), ModuleError> {
        let Some(hm) = cr.and_then(|cr| cr.header_mutation.as_ref()) else {
            return Ok(());
        };
        self.mutations.merge(hm)?;
        if let Some(ep) = extract_endpoint_from_mutation(hm) {
            self.endpoint_raw = Some(ep);
        }
        Ok(())
    }
}

/// Check whether a `RequestBody` reply is the final chunk.
///
/// In `FULL_DUPLEX_STREAMED` body mode, `CommonResponse.body_mutation` carries
/// a `StreamedResponse` with an `end_of_stream` flag. Intermediate chunks have
/// it `false`; the last chunk has it `true`. Without a `StreamedResponse`
/// body_mutation (legacy/buffered mode), the reply is a single complete message.
fn is_streamed_body_eos(cr: Option<&CommonResponse>) -> bool {
    match cr
        .and_then(|cr| cr.body_mutation.as_ref())
        .and_then(|bm| bm.mutation.as_ref())
    {
        Some(body_mutation::Mutation::StreamedResponse(sr)) => sr.end_of_stream,
        _ => true, // No StreamedResponse → legacy single-chunk, treat as complete.
    }
}

/// Look for the endpoint header in a `HeaderMutation`'s `set_headers`.
fn extract_endpoint_from_mutation(hm: &HeaderMutation) -> Option<Vec<u8>> {
    hm.set_headers
        .iter()
        .filter_map(|hvo| hvo.header.as_ref())
        .find(|hv| hv.key.eq_ignore_ascii_case(ENDPOINT_HEADER))
        .map(|hv| mutations::header_value_bytes(hv).to_vec())
}

/// Decode one `ProcessingResponse` payload received during the response phase.
///
/// Returns `Ok(Some(mutations))` for the EPP's `ResponseHeaders` reply (the set
/// may be empty), `Ok(None)` for any other message, and
/// `Err(ImmediateResponse)` if the EPP rejected the response.
pub(crate) fn decode_response_headers_reply(
    payload: &[u8],
) -> Result<Option<MutationSet>, ModuleError> {
    let resp =
        ProcessingResponse::decode(payload).map_err(|e| ModuleError::ProtoDecode(e.to_string()))?;

    match resp.response {
        Some(processing_response::Response::ResponseHeaders(hr)) => {
            let mut mutations = MutationSet::default();
            if let Some(hm) = hr
                .response
                .as_ref()
                .and_then(|cr| cr.header_mutation.as_ref())
            {
                mutations.merge(hm)?;
            }
            Ok(Some(mutations))
        }
        Some(processing_response::Response::ImmediateResponse(ir)) => {
            let code = clamp_status_code(ir.status.map(|s| s.code));
            Err(ModuleError::ImmediateResponse(code))
        }
        _ => Ok(None),
    }
}

/// Map an `ImmediateResponse` status code to a valid HTTP status (100..=599),
/// defaulting to 500.
fn clamp_status_code(code: Option<i32>) -> u16 {
    code.and_then(|c| u16::try_from(c).ok())
        .filter(|c| (100..=599).contains(c))
        .unwrap_or(500)
}

/// Look for the endpoint in `dynamic_metadata` under the `envoy.lb` namespace.
fn extract_endpoint_from_metadata(resp: &ProcessingResponse) -> Option<Vec<u8>> {
    let dm = resp.dynamic_metadata.as_ref()?;
    let lb = dm.fields.get(ENVOY_LB_NS)?;
    let inner = lb.kind.as_ref()?;
    if let envoy_types::pb::google::protobuf::value::Kind::StructValue(s) = inner {
        let ep_val = s.fields.get(ENDPOINT_HEADER)?;
        if let Some(envoy_types::pb::google::protobuf::value::Kind::StringValue(s)) = &ep_val.kind {
            return Some(s.as_bytes().to_vec());
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use envoy_types::pb::envoy::config::core::v3::{
        HeaderValue, HeaderValueOption, header_value_option::HeaderAppendAction,
    };
    use envoy_types::pb::envoy::service::ext_proc::v3::{
        BodyMutation, BodyResponse, CommonResponse, HeaderMutation, HeadersResponse,
        ImmediateResponse, ProcessingResponse, StreamedBodyResponse, processing_response,
    };
    use envoy_types::pb::envoy::r#type::v3::HttpStatus;

    use super::*;
    use crate::codec::grpc;

    /// Decode every complete frame in `buf` and finish (end-of-stream semantics).
    fn decode_responses(buf: &[u8]) -> Result<EppResult, ModuleError> {
        let mut d = RequestReplyDecoder::default();
        let mut rest = buf;
        while let Some((payload, tail)) = grpc::decode_partial(rest)? {
            d.on_message(payload)?;
            rest = tail;
        }
        d.finish()
    }

    /// Decode frames until the `RequestBody` reply is seen.
    fn try_decode_responses(buf: &[u8]) -> Result<Option<EppResult>, ModuleError> {
        let mut d = RequestReplyDecoder::default();
        let mut rest = buf;
        while let Some((payload, tail)) = grpc::decode_partial(rest)? {
            if d.on_message(payload)? {
                return d.finish().map(Some);
            }
            rest = tail;
        }
        Ok(None)
    }

    fn headers_reply(buf: &[u8]) -> Result<Option<MutationSet>, ModuleError> {
        let (payload, _) = grpc::decode(buf)?;
        decode_response_headers_reply(payload)
    }

    fn endpoint_hvo(addr: &str) -> HeaderValueOption {
        HeaderValueOption {
            header: Some(HeaderValue {
                key: ENDPOINT_HEADER.into(),
                raw_value: addr.as_bytes().to_vec(),
                ..Default::default()
            }),
            append_action: HeaderAppendAction::OverwriteIfExistsOrAdd as i32,
            ..Default::default()
        }
    }

    fn extra_hvo(key: &str, val: &str) -> HeaderValueOption {
        HeaderValueOption {
            header: Some(HeaderValue {
                key: key.into(),
                raw_value: val.as_bytes().to_vec(),
                ..Default::default()
            }),
            append_action: HeaderAppendAction::OverwriteIfExistsOrAdd as i32,
            ..Default::default()
        }
    }

    fn frame_response(resp: &ProcessingResponse) -> Vec<u8> {
        grpc::encode(&resp.encode_to_vec())
    }

    fn headers_response_with(hvos: Vec<HeaderValueOption>) -> ProcessingResponse {
        ProcessingResponse {
            response: Some(processing_response::Response::RequestHeaders(
                HeadersResponse {
                    response: Some(CommonResponse {
                        header_mutation: Some(HeaderMutation {
                            set_headers: hvos,
                            remove_headers: vec![],
                        }),
                        ..Default::default()
                    }),
                },
            )),
            ..Default::default()
        }
    }

    fn body_response_with(hvos: Vec<HeaderValueOption>) -> ProcessingResponse {
        ProcessingResponse {
            response: Some(processing_response::Response::RequestBody(BodyResponse {
                response: Some(CommonResponse {
                    header_mutation: Some(HeaderMutation {
                        set_headers: hvos,
                        remove_headers: vec![],
                    }),
                    ..Default::default()
                }),
            })),
            ..Default::default()
        }
    }

    #[test]
    fn duplicate_request_responses_are_rejected_without_merging() {
        for body in [false, true] {
            let response = if body {
                body_response_with
            } else {
                headers_response_with
            };
            let mut decoder = RequestReplyDecoder::default();
            if body {
                decoder
                    .on_message(&headers_response_with(vec![]).encode_to_vec())
                    .unwrap();
            }
            assert_eq!(
                decoder
                    .on_message(&response(vec![endpoint_hvo("first:80")]).encode_to_vec())
                    .unwrap(),
                body,
            );
            assert!(matches!(
                decoder.on_message(&response(vec![endpoint_hvo("second:80")]).encode_to_vec()),
                Err(ModuleError::InvalidResponse(_))
            ));
            let result = decoder.finish().unwrap();
            assert_eq!(result.endpoints[0].address, "first:80");
            assert_eq!(result.mutations.sets.len(), 1);
        }
    }

    #[test]
    fn duplicate_empty_request_responses_are_rejected() {
        for common in [None, Some(CommonResponse::default())] {
            let headers = ProcessingResponse {
                response: Some(processing_response::Response::RequestHeaders(
                    HeadersResponse {
                        response: common.clone(),
                    },
                )),
                ..Default::default()
            }
            .encode_to_vec();
            let body = ProcessingResponse {
                response: Some(processing_response::Response::RequestBody(BodyResponse {
                    response: common.clone(),
                })),
                ..Default::default()
            }
            .encode_to_vec();

            let mut decoder = RequestReplyDecoder::default();
            decoder.on_message(&headers).unwrap();
            assert!(matches!(
                decoder.on_message(&headers),
                Err(ModuleError::InvalidResponse(_))
            ));

            let mut decoder = RequestReplyDecoder::default();
            decoder.on_message(&headers).unwrap();
            decoder.on_message(&body).unwrap();
            assert!(matches!(
                decoder.on_message(&body),
                Err(ModuleError::InvalidResponse(_))
            ));
        }
    }

    #[test]
    fn out_of_order_request_responses_are_rejected() {
        // Body before headers.
        let mut decoder = RequestReplyDecoder::default();
        assert!(matches!(
            decoder.on_message(&body_response_with(vec![endpoint_hvo("host:80")]).encode_to_vec()),
            Err(ModuleError::InvalidResponse(_))
        ));

        // Headers after body.
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(&headers_response_with(vec![endpoint_hvo("host:80")]).encode_to_vec())
            .unwrap();
        decoder
            .on_message(&body_response_with(vec![]).encode_to_vec())
            .unwrap();
        assert!(matches!(
            decoder.on_message(&headers_response_with(vec![]).encode_to_vec()),
            Err(ModuleError::InvalidResponse(_))
        ));
    }

    #[test]
    fn unexpected_request_phase_response_types_are_rejected() {
        for response in [
            None,
            Some(processing_response::Response::ResponseHeaders(
                HeadersResponse::default(),
            )),
            Some(processing_response::Response::ResponseBody(
                BodyResponse::default(),
            )),
        ] {
            let payload = ProcessingResponse {
                response,
                ..Default::default()
            }
            .encode_to_vec();
            let mut decoder = RequestReplyDecoder::default();
            assert!(matches!(
                decoder.on_message(&payload),
                Err(ModuleError::InvalidResponse(_))
            ));
        }
    }

    #[test]
    fn decoded_batches_preserve_later_removals() {
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(
                &headers_response_with(vec![endpoint_hvo("host:80"), extra_hvo("x-test", "first")])
                    .encode_to_vec(),
            )
            .unwrap();
        let mut body = body_response_with(vec![]);
        let Some(processing_response::Response::RequestBody(ref mut reply)) = body.response else {
            unreachable!();
        };
        reply
            .response
            .as_mut()
            .unwrap()
            .header_mutation
            .as_mut()
            .unwrap()
            .remove_headers
            .push("x-test".into());
        assert!(decoder.on_message(&body.encode_to_vec()).unwrap());
        let mut headers = vec![("x-test".into(), b"old".to_vec())];
        crate::codec::mutations::apply(&decoder.finish().unwrap().mutations, &mut headers);
        assert!(!headers.iter().any(|(name, _)| name == "x-test"));
    }

    #[test]
    fn mutation_limits_propagate_in_both_phases() {
        use crate::codec::mutations::{MAX_MUTATION_BYTES, MAX_MUTATIONS};

        for hvos in [
            vec![extra_hvo("x-test", "v"); MAX_MUTATIONS + 1],
            vec![extra_hvo("x-test", &"a".repeat(MAX_MUTATION_BYTES))],
        ] {
            let mut decoder = RequestReplyDecoder::default();
            assert!(matches!(
                decoder.on_message(&headers_response_with(hvos.clone()).encode_to_vec()),
                Err(ModuleError::InvalidResponse(_))
            ));
            assert!(matches!(
                decode_response_headers_reply(
                    &response_headers_reply_with(hvos, vec![]).encode_to_vec()
                ),
                Err(ModuleError::InvalidResponse(_))
            ));
        }
    }

    #[test]
    fn request_mutation_limit_spans_headers_and_body() {
        use crate::codec::mutations::MAX_MUTATIONS;

        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(
                &headers_response_with(vec![extra_hvo("x-test", "v"); MAX_MUTATIONS])
                    .encode_to_vec(),
            )
            .unwrap();
        assert!(matches!(
            decoder.on_message(&body_response_with(vec![extra_hvo("x-test", "v")]).encode_to_vec()),
            Err(ModuleError::InvalidResponse(_))
        ));
    }

    #[test]
    #[allow(deprecated)]
    fn response_headers_append_wrapper_survives_protobuf_round_trip() {
        use envoy_types::pb::google::protobuf::BoolValue;

        for append in [None, Some(false), Some(true)] {
            let mut option = extra_hvo("x-test", "new");
            option.append = append.map(|value| BoolValue { value });
            let frame = frame_response(&response_headers_reply_with(vec![option], vec![]));
            let mutations = headers_reply(&frame).unwrap().unwrap();
            let mut headers = vec![("x-test".into(), b"old".to_vec())];
            crate::codec::mutations::apply(&mutations, &mut headers);
            assert_eq!(headers.len(), if append == Some(true) { 2 } else { 1 });
            assert_eq!(headers.last().unwrap().1, b"new");
        }
    }

    #[test]
    fn endpoint_from_request_headers_response() {
        let buf = frame_response(&headers_response_with(vec![endpoint_hvo("10.0.0.1:8080")]));
        let result = decode_responses(&buf).unwrap();
        assert_eq!(result.endpoints.len(), 1);
        assert_eq!(result.endpoints[0].address, "10.0.0.1:8080");
    }

    #[test]
    fn endpoint_from_request_body_response() {
        let mut buf = frame_response(&headers_response_with(vec![]));
        buf.extend(frame_response(&body_response_with(vec![endpoint_hvo(
            "10.0.0.2:9090",
        )])));
        let result = decode_responses(&buf).unwrap();
        assert_eq!(result.endpoints[0].address, "10.0.0.2:9090");
    }

    #[test]
    fn two_responses_endpoint_in_first() {
        let mut buf = frame_response(&headers_response_with(vec![
            endpoint_hvo("10.0.0.1:80"),
            extra_hvo("x-worker", "w1"),
        ]));
        buf.extend_from_slice(&frame_response(&body_response_with(vec![extra_hvo(
            "x-extra", "e1",
        )])));

        let result = decode_responses(&buf).unwrap();
        assert_eq!(result.endpoints[0].address, "10.0.0.1:80");
        // Both mutations collected.
        assert_eq!(result.mutations.sets.len(), 3); // endpoint + x-worker + x-extra
    }

    #[test]
    fn endpoint_in_second_response() {
        let mut buf = frame_response(&headers_response_with(vec![extra_hvo("x-first", "v")]));
        buf.extend_from_slice(&frame_response(&body_response_with(vec![endpoint_hvo(
            "[::1]:443",
        )])));

        let result = decode_responses(&buf).unwrap();
        assert_eq!(result.endpoints[0].address, "[::1]:443");
    }

    #[test]
    fn comma_separated_endpoints() {
        let buf = frame_response(&headers_response_with(vec![endpoint_hvo(
            "10.0.0.1:80, 10.0.0.2:80",
        )]));
        let result = decode_responses(&buf).unwrap();
        assert_eq!(result.endpoints.len(), 2);
        assert_eq!(result.endpoints[0].address, "10.0.0.1:80");
        assert_eq!(result.endpoints[1].address, "10.0.0.2:80");
    }

    #[test]
    fn no_endpoint_error() {
        let buf = frame_response(&headers_response_with(vec![extra_hvo("x-something", "v")]));
        let err = decode_responses(&buf).unwrap_err();
        assert!(matches!(err, ModuleError::NoEndpoint));
        assert_eq!(err.status(), 500);
    }

    #[test]
    fn immediate_response_error() {
        let resp = ProcessingResponse {
            response: Some(processing_response::Response::ImmediateResponse(
                ImmediateResponse {
                    status: Some(HttpStatus { code: 503 }),
                    ..Default::default()
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let err = decode_responses(&buf).unwrap_err();
        assert!(matches!(err, ModuleError::ImmediateResponse(503)));
        assert_eq!(err.status(), 503);
    }

    #[test]
    fn immediate_response_429() {
        let resp = ProcessingResponse {
            response: Some(processing_response::Response::ImmediateResponse(
                ImmediateResponse {
                    status: Some(HttpStatus { code: 429 }),
                    ..Default::default()
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let err = decode_responses(&buf).unwrap_err();
        assert!(matches!(err, ModuleError::ImmediateResponse(429)));
    }

    #[test]
    fn immediate_response_no_status_defaults_500() {
        let resp = ProcessingResponse {
            response: Some(processing_response::Response::ImmediateResponse(
                ImmediateResponse::default(),
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let err = decode_responses(&buf).unwrap_err();
        assert!(matches!(err, ModuleError::ImmediateResponse(500)));
    }

    #[test]
    fn endpoint_from_envoy_lb_metadata() {
        use envoy_types::pb::google::protobuf::{Struct, Value, value::Kind};
        use std::collections::HashMap;

        let inner_fields: HashMap<String, Value> = [(
            ENDPOINT_HEADER.into(),
            Value {
                kind: Some(Kind::StringValue("10.0.0.5:8080".into())),
            },
        )]
        .into();

        let lb_struct = Value {
            kind: Some(Kind::StructValue(Struct {
                fields: inner_fields,
            })),
        };

        let dm_fields: HashMap<String, Value> = [(ENVOY_LB_NS.into(), lb_struct)].into();

        let resp = ProcessingResponse {
            dynamic_metadata: Some(Struct { fields: dm_fields }),
            response: Some(processing_response::Response::RequestHeaders(
                HeadersResponse {
                    response: Some(CommonResponse::default()),
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let result = decode_responses(&buf).unwrap();
        assert_eq!(result.endpoints[0].address, "10.0.0.5:8080");
    }

    #[test]
    fn header_mutation_preferred_over_metadata() {
        use envoy_types::pb::google::protobuf::{Struct, Value, value::Kind};
        use std::collections::HashMap;

        let inner_fields: HashMap<String, Value> = [(
            ENDPOINT_HEADER.into(),
            Value {
                kind: Some(Kind::StringValue("metadata-ep:80".into())),
            },
        )]
        .into();

        let lb_struct = Value {
            kind: Some(Kind::StructValue(Struct {
                fields: inner_fields,
            })),
        };

        let dm_fields: HashMap<String, Value> = [(ENVOY_LB_NS.into(), lb_struct)].into();

        let resp = ProcessingResponse {
            dynamic_metadata: Some(Struct { fields: dm_fields }),
            response: Some(processing_response::Response::RequestHeaders(
                HeadersResponse {
                    response: Some(CommonResponse {
                        header_mutation: Some(HeaderMutation {
                            set_headers: vec![endpoint_hvo("header-ep:80")],
                            remove_headers: vec![],
                        }),
                        ..Default::default()
                    }),
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let result = decode_responses(&buf).unwrap();
        // Header mutation is the primary source; metadata is the fallback.
        assert_eq!(result.endpoints[0].address, "header-ep:80");
    }

    #[test]
    fn mutations_collected_from_both_responses() {
        let mut buf = frame_response(&headers_response_with(vec![
            endpoint_hvo("10.0.0.1:80"),
            extra_hvo("x-from-headers", "h"),
        ]));
        buf.extend_from_slice(&frame_response(&body_response_with(vec![extra_hvo(
            "x-from-body",
            "b",
        )])));

        let result = decode_responses(&buf).unwrap();
        let names: Vec<&str> = result
            .mutations
            .sets
            .iter()
            .map(|s| s.name.as_str())
            .collect();
        assert!(names.contains(&"x-gateway-destination-endpoint"));
        assert!(names.contains(&"x-from-headers"));
        assert!(names.contains(&"x-from-body"));
    }

    #[test]
    fn empty_buffer_error() {
        assert!(decode_responses(&[]).is_err());
    }

    // --- clamp_status_code ---

    #[test]
    fn clamp_status_code_valid() {
        assert_eq!(super::clamp_status_code(Some(429)), 429);
        assert_eq!(super::clamp_status_code(Some(503)), 503);
        assert_eq!(super::clamp_status_code(Some(100)), 100);
        assert_eq!(super::clamp_status_code(Some(599)), 599);
    }

    #[test]
    fn clamp_status_code_out_of_range() {
        assert_eq!(super::clamp_status_code(Some(0)), 500);
        assert_eq!(super::clamp_status_code(Some(99)), 500);
        assert_eq!(super::clamp_status_code(Some(600)), 500);
        assert_eq!(super::clamp_status_code(Some(-1)), 500);
    }

    #[test]
    fn clamp_status_code_none() {
        assert_eq!(super::clamp_status_code(None), 500);
    }

    // --- try_decode_responses ---

    #[test]
    fn try_decode_headers_only_returns_none() {
        let buf = frame_response(&headers_response_with(vec![endpoint_hvo("10.0.0.1:80")]));
        assert!(try_decode_responses(&buf).unwrap().is_none());
    }

    #[test]
    fn try_decode_body_response_returns_result() {
        let mut buf = frame_response(&headers_response_with(vec![endpoint_hvo("10.0.0.1:80")]));
        buf.extend_from_slice(&frame_response(&body_response_with(vec![extra_hvo(
            "x-body", "v",
        )])));
        let result = try_decode_responses(&buf).unwrap().unwrap();
        assert_eq!(result.endpoints[0].address, "10.0.0.1:80");
    }

    #[test]
    fn try_decode_immediate_response() {
        let resp = ProcessingResponse {
            response: Some(processing_response::Response::ImmediateResponse(
                ImmediateResponse {
                    status: Some(HttpStatus { code: 429 }),
                    ..Default::default()
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let err = try_decode_responses(&buf).unwrap_err();
        assert!(matches!(err, ModuleError::ImmediateResponse(429)));
    }

    #[test]
    fn try_decode_incomplete_frame_returns_none() {
        // Partial frame — just the length prefix, no payload.
        let buf = vec![0, 0, 0, 0, 10];
        assert!(try_decode_responses(&buf).unwrap().is_none());
    }

    // --- decode_response_headers_reply ---

    fn response_headers_reply_with(
        hvos: Vec<HeaderValueOption>,
        removes: Vec<String>,
    ) -> ProcessingResponse {
        use envoy_types::pb::envoy::service::ext_proc::v3::HeadersResponse;
        ProcessingResponse {
            response: Some(processing_response::Response::ResponseHeaders(
                HeadersResponse {
                    response: Some(CommonResponse {
                        header_mutation: Some(HeaderMutation {
                            set_headers: hvos,
                            remove_headers: removes,
                        }),
                        ..Default::default()
                    }),
                },
            )),
            ..Default::default()
        }
    }

    #[test]
    fn response_headers_reply_with_mutations() {
        let resp = response_headers_reply_with(
            vec![extra_hvo("x-served-by", "pod-1")],
            vec!["x-internal".into()],
        );
        let buf = frame_response(&resp);
        let mutations = headers_reply(&buf).unwrap().unwrap();
        assert_eq!(mutations.sets.len(), 1);
        assert_eq!(mutations.sets[0].name, "x-served-by");
        assert_eq!(mutations.removes, vec!["x-internal"]);
    }

    #[test]
    fn response_headers_reply_empty_mutation() {
        use envoy_types::pb::envoy::service::ext_proc::v3::HeadersResponse;
        let resp = ProcessingResponse {
            response: Some(processing_response::Response::ResponseHeaders(
                HeadersResponse {
                    response: Some(CommonResponse::default()),
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let mutations = headers_reply(&buf).unwrap().unwrap();
        assert!(mutations.sets.is_empty());
        assert!(mutations.removes.is_empty());
    }

    #[test]
    fn response_headers_reply_no_response_field() {
        let resp = ProcessingResponse::default();
        let buf = frame_response(&resp);
        assert!(headers_reply(&buf).unwrap().is_none());
    }

    #[test]
    fn response_headers_reply_immediate_response_error() {
        let resp = ProcessingResponse {
            response: Some(processing_response::Response::ImmediateResponse(
                ImmediateResponse {
                    status: Some(HttpStatus { code: 503 }),
                    ..Default::default()
                },
            )),
            ..Default::default()
        };
        let buf = frame_response(&resp);
        let err = headers_reply(&buf).unwrap_err();
        assert!(matches!(err, ModuleError::ImmediateResponse(503)));
    }

    // --- Multi-chunk (streamed) body reply tests ---

    /// Build a `RequestBody` reply with a `StreamedResponse` body_mutation.
    fn streamed_body_response(hvos: Vec<HeaderValueOption>, eos: bool) -> ProcessingResponse {
        ProcessingResponse {
            response: Some(processing_response::Response::RequestBody(BodyResponse {
                response: Some(CommonResponse {
                    header_mutation: Some(HeaderMutation {
                        set_headers: hvos,
                        remove_headers: vec![],
                    }),
                    body_mutation: Some(BodyMutation {
                        mutation: Some(body_mutation::Mutation::StreamedResponse(
                            StreamedBodyResponse {
                                end_of_stream: eos,
                                ..Default::default()
                            },
                        )),
                    }),
                    ..Default::default()
                }),
            })),
            ..Default::default()
        }
    }

    #[test]
    fn multi_chunk_body_reply_completes_on_eos() {
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(&headers_response_with(vec![endpoint_hvo("host:80")]).encode_to_vec())
            .unwrap();

        // Intermediate chunk (EOS = false) → not complete.
        assert!(
            !decoder
                .on_message(
                    &streamed_body_response(vec![extra_hvo("x-a", "1")], false).encode_to_vec()
                )
                .unwrap()
        );

        // Another intermediate chunk.
        assert!(
            !decoder
                .on_message(
                    &streamed_body_response(vec![extra_hvo("x-b", "2")], false).encode_to_vec()
                )
                .unwrap()
        );

        // Final chunk (EOS = true) → complete.
        assert!(
            decoder
                .on_message(
                    &streamed_body_response(vec![extra_hvo("x-c", "3")], true).encode_to_vec()
                )
                .unwrap()
        );

        let result = decoder.finish().unwrap();
        assert_eq!(result.endpoints[0].address, "host:80");
        // All three body-chunk mutations plus the endpoint mutation are collected.
        assert_eq!(result.mutations.sets.len(), 4);
    }

    #[test]
    fn single_streamed_body_reply_with_eos() {
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(&headers_response_with(vec![endpoint_hvo("host:80")]).encode_to_vec())
            .unwrap();
        // Single body reply with StreamedResponse and EOS = true.
        assert!(
            decoder
                .on_message(
                    &streamed_body_response(vec![extra_hvo("x-a", "1")], true).encode_to_vec()
                )
                .unwrap()
        );
        let result = decoder.finish().unwrap();
        assert_eq!(result.endpoints[0].address, "host:80");
    }

    #[test]
    fn duplicate_body_after_streamed_eos_is_rejected() {
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(&headers_response_with(vec![endpoint_hvo("host:80")]).encode_to_vec())
            .unwrap();
        assert!(
            decoder
                .on_message(&streamed_body_response(vec![], true).encode_to_vec())
                .unwrap()
        );
        // After EOS, another body reply should be rejected.
        assert!(matches!(
            decoder.on_message(&streamed_body_response(vec![], false).encode_to_vec()),
            Err(ModuleError::InvalidResponse(_))
        ));
    }

    #[test]
    fn endpoint_in_streamed_body_chunk() {
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(&headers_response_with(vec![]).encode_to_vec())
            .unwrap();
        // Endpoint arrives in an intermediate body chunk.
        assert!(
            !decoder
                .on_message(
                    &streamed_body_response(vec![endpoint_hvo("10.0.0.5:8080")], false)
                        .encode_to_vec()
                )
                .unwrap()
        );
        assert!(
            decoder
                .on_message(&streamed_body_response(vec![], true).encode_to_vec())
                .unwrap()
        );
        let result = decoder.finish().unwrap();
        assert_eq!(result.endpoints[0].address, "10.0.0.5:8080");
    }

    #[test]
    fn legacy_body_reply_without_streamed_mutation_completes_immediately() {
        // body_response_with() creates a BodyResponse without body_mutation.
        let mut decoder = RequestReplyDecoder::default();
        decoder
            .on_message(&headers_response_with(vec![endpoint_hvo("host:80")]).encode_to_vec())
            .unwrap();
        assert!(
            decoder
                .on_message(&body_response_with(vec![]).encode_to_vec())
                .unwrap()
        );
    }
}
