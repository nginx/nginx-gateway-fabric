//! h2-based gRPC stream client for the Endpoint Picker Protocol.
//!
//! Drives a bidirectional ext_proc gRPC stream over h2. The h2 connection
//! future is polled alongside every stream operation so the protocol state
//! machine advances without a detached driver task.

use core::pin::Pin;
use core::task::{Context, Poll};
use std::collections::VecDeque;

use bytes::Bytes;
use h2::client::{Connection, SendRequest};
use h2::{RecvStream, SendStream};
use http::Request;
use tokio::io::{AsyncRead, AsyncWrite};

use crate::codec::grpc::FrameReader;
use crate::codec::mutations::MutationSet;
use crate::codec::notify;
use crate::codec::response::{self, EppResult, RequestReplyDecoder};
use crate::error::ModuleError;

/// The gRPC service method path for ext_proc.
const EXT_PROC_PATH: &str = "/envoy.service.ext_proc.v3.ExternalProcessor/Process";

/// Maximum framed EPP message size (1 MiB). The reader buffers up to this
/// limit, with at most one partially consumed h2 DATA chunk retained separately.
const MAX_RESPONSE_BUF: usize = 1024 * 1024;

/// Upper bound on a single queued DATA chunk, so replies are polled between sends.
const SEND_CHUNK: usize = 16 * 1024;

const GRPC_MEDIA_TYPE: &str = "application/grpc";

/// An open ext_proc stream, retained after the request phase so the
/// response phase can be sent on the same stream.
pub(crate) struct EppStream {
    send: SendStream<Bytes>,
    recv: RecvStream,
    reader: FrameReader,
    /// Unconsumed portion of one h2 DATA chunk, retained across phases.
    pending: Bytes,
    closed: bool,
}

impl EppStream {
    /// Abort the stream; used when the caller abandons it on error.
    pub(crate) fn cancel(&mut self) {
        self.send.send_reset(h2::Reason::CANCEL);
    }
}

fn transport(what: &str, e: impl core::fmt::Display) -> ModuleError {
    ModuleError::Transport(format!("{what}: {e}"))
}

/// Execute the request phase of an EPP call: send request headers and body,
/// then read replies until the RequestBody reply arrives.
///
/// The send side is kept open so the response phase can be sent later.
pub(crate) async fn epp_call<IO>(
    send_request: &mut SendRequest<Bytes>,
    conn: &mut Connection<IO, Bytes>,
    authority: &str,
    ssl: bool,
    headers_frame: Vec<u8>,
    body_frames: Vec<Vec<u8>>,
) -> Result<(EppResult, EppStream), ModuleError>
where
    IO: AsyncRead + AsyncWrite + Unpin,
{
    drive(
        conn,
        |cx, sr| sr.poll_ready(cx).map_err(|e| transport("h2 poll_ready", e)),
        send_request,
    )
    .await?;

    let scheme = if ssl { "https" } else { "http" };
    let request = Request::builder()
        .method("POST")
        .uri(format!("{scheme}://{authority}{EXT_PROC_PATH}"))
        .header("content-type", "application/grpc")
        .header("te", "trailers")
        .body(())
        .map_err(|e| transport("build request", e))?;

    let (response_fut, mut send) = send_request
        .send_request(request, false)
        .map_err(|e| transport("h2 send_request", e))?;

    let receive = async {
        let response = response_fut
            .await
            .map_err(|e| transport("h2 response", e))?;
        check_response(&response)?;
        // Only END_STREAM initial headers can carry the terminal status.
        let mut closed = response.body().is_end_stream();
        if closed {
            check_grpc_status(response.headers())?;
        }
        let mut recv = response.into_body();
        let mut reader = FrameReader::new(MAX_RESPONSE_BUF);
        let mut pending = Bytes::new();
        let mut decoder = RequestReplyDecoder::default();

        loop {
            if closed {
                return Ok((decoder.finish()?, recv, reader, pending, closed));
            }
            match read_message(&mut recv, &mut reader, &mut pending).await? {
                Some(msg) => {
                    if decoder.on_message(&msg)? {
                        return Ok((decoder.finish()?, recv, reader, pending, closed));
                    }
                }
                None => {
                    check_trailers(&mut recv).await?;
                    closed = true;
                }
            }
        }
    };
    let mut frames = Vec::with_capacity(1 + body_frames.len());
    frames.push(Bytes::from(headers_frame));
    frames.extend(body_frames.into_iter().map(Bytes::from));
    let result = exchange(conn, &mut send, &frames, false, receive).await;

    match result {
        Ok((epp, recv, reader, pending, closed)) => Ok((
            epp,
            EppStream {
                send,
                recv,
                reader,
                pending,
                closed,
            },
        )),
        Err(e) => {
            send.send_reset(h2::Reason::CANCEL);
            Err(e)
        }
    }
}

/// Send the response-phase messages (ResponseHeaders + streaming ResponseBody)
/// and consume replies through stream completion, validating the terminal
/// status.
///
/// `body_poll` is polled for upstream body chunks. It returns
/// `Ready(Some((data, is_last)))` for each chunk and `Ready(None)` when the
/// sender was dropped without a final chunk (treated as an implicit EOS with
/// empty body). The body is encoded into gRPC-framed `ResponseBody` messages,
/// chunked at ≤62 KB to match the EPP's `BodyByteLimit`.
///
/// Returns the decoded response-header mutations (empty if the EPP ended the
/// stream without replying).
pub(crate) async fn send_response_phase<IO, B>(
    stream: &mut EppStream,
    conn: &mut Connection<IO, Bytes>,
    response_headers_frame: Bytes,
    body_poll: &B,
) -> Result<MutationSet, ModuleError>
where
    IO: AsyncRead + AsyncWrite + Unpin,
    B: BodyPoll,
{
    if stream.closed {
        return Ok(MutationSet::default());
    }
    let receive = async {
        let mut reply = None;
        loop {
            if let Some(msg) =
                read_message(&mut stream.recv, &mut stream.reader, &mut stream.pending).await?
            {
                if let Some(mutations) = response::decode_response_headers_reply(&msg)? {
                    if reply.is_some() {
                        return Err(ModuleError::InvalidResponse(
                            "duplicate response headers response".into(),
                        ));
                    }
                    reply = Some(mutations);
                }
            } else {
                check_trailers(&mut stream.recv).await?;
                stream.closed = true;
                return Ok(reply.unwrap_or_default());
            }
        }
    };
    let result = exchange_streaming(
        conn,
        &mut stream.send,
        response_headers_frame,
        body_poll,
        receive,
    )
    .await;

    if result.is_err() {
        stream.cancel();
    }
    result
}

/// Trait for polling body chunks from a channel or similar source.
///
/// Used by `send_response_phase` to decouple h2 sending from the channel
/// implementation in handler.rs.
pub(crate) trait BodyPoll {
    /// Returns `Ready(Some((data, is_last)))` for each body chunk,
    /// `Ready(None)` if the sender was dropped without a final chunk,
    /// or `Pending` while waiting.
    fn poll_chunk(&self, cx: &mut Context<'_>) -> Poll<Option<(Vec<u8>, bool)>>;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/// Poll the h2 connection alongside an arbitrary operation on `state`.
async fn drive<T, IO, S, F>(
    conn: &mut Connection<IO, Bytes>,
    mut op: F,
    state: &mut S,
) -> Result<T, ModuleError>
where
    IO: AsyncRead + AsyncWrite + Unpin,
    F: FnMut(&mut Context<'_>, &mut S) -> Poll<Result<T, ModuleError>>,
{
    core::future::poll_fn(|cx| {
        let connection = Pin::new(&mut *conn).poll(cx);
        // Consume a queued response/EOF before reporting connection closure.
        if let Poll::Ready(result) = op(cx, state) {
            return Poll::Ready(result);
        }
        match connection {
            Poll::Ready(Ok(())) => {
                return Poll::Ready(Err(ModuleError::Transport(
                    "h2 connection closed unexpectedly".into(),
                )));
            }
            Poll::Ready(Err(e)) => return Poll::Ready(Err(transport("h2 connection", e))),
            Poll::Pending => {}
        }
        Poll::Pending
    })
    .await
}

/// Assemble one message within the reader's cap, retaining at most one DATA
/// chunk outside it. Never fetch another chunk while bytes remain unconsumed.
async fn read_message(
    body: &mut RecvStream,
    reader: &mut FrameReader,
    pending: &mut Bytes,
) -> Result<Option<Bytes>, ModuleError> {
    loop {
        if let Some(message) = reader.next_message()? {
            return Ok(Some(message));
        }
        if pending.is_empty() {
            let Some(chunk) = read_chunk(body).await? else {
                check_eof(reader)?;
                return Ok(None);
            };
            *pending = chunk;
        }
        reader.push_partial(pending)?;
    }
}

/// Read the next DATA chunk, releasing its flow-control capacity.
async fn read_chunk(body: &mut RecvStream) -> Result<Option<Bytes>, ModuleError> {
    core::future::poll_fn(|cx| match body.poll_data(cx) {
        Poll::Ready(Some(Ok(chunk))) => {
            body.flow_control()
                .release_capacity(chunk.len())
                .map_err(|e| transport("h2 release capacity", e))?;
            Poll::Ready(Ok(Some(chunk)))
        }
        Poll::Ready(Some(Err(e))) => Poll::Ready(Err(transport("h2 data", e))),
        Poll::Ready(None) => Poll::Ready(Ok(None)),
        Poll::Pending => Poll::Pending,
    })
    .await
}

/// Read the trailers at end of stream and fail on a non-OK `grpc-status`.
async fn check_trailers(body: &mut RecvStream) -> Result<(), ModuleError> {
    let trailers = core::future::poll_fn(|cx| {
        body.poll_trailers(cx)
            .map_err(|e| transport("h2 trailers", e))
    })
    .await?;
    match trailers {
        Some(t) => check_grpc_status(&t),
        None => Err(ModuleError::Transport(
            "missing terminal grpc-status".into(),
        )),
    }
}

/// Poll application replies before every bounded send step. In particular, a
/// peer need not consume a large request body before sending ImmediateResponse
/// or closing ext_proc cleanly. No runtime or detached connection driver is used.
async fn exchange<T, IO>(
    conn: &mut Connection<IO, Bytes>,
    stream: &mut SendStream<Bytes>,
    frames: &[Bytes],
    end_of_stream: bool,
    receive: impl core::future::Future<Output = Result<T, ModuleError>>,
) -> Result<T, ModuleError>
where
    IO: AsyncRead + AsyncWrite + Unpin,
{
    // Copy frame references so the send loop can consume them progressively.
    let mut owned: Vec<Bytes> = frames.to_vec();
    let mut index = 0;
    let mut sent = false;
    let mut receive = core::pin::pin!(receive);
    drive(
        conn,
        |cx, stream| {
            if let Poll::Ready(result) = receive.as_mut().poll(cx) {
                // Release any unused capacity reservation on early completion.
                stream.reserve_capacity(0);
                return Poll::Ready(result);
            }
            if sent {
                return Poll::Pending;
            }
            while index < owned.len() && owned[index].is_empty() {
                index += 1;
            }
            if index == owned.len() {
                if end_of_stream {
                    stream
                        .send_data(Bytes::new(), true)
                        .map_err(|e| transport("h2 send_data", e))?;
                }
                sent = true;
            } else {
                let data = &mut owned[index];
                stream.reserve_capacity(data.len().min(SEND_CHUNK));
                let capacity = match stream.poll_capacity(cx) {
                    Poll::Ready(Some(Ok(n))) if n > 0 => n,
                    Poll::Ready(Some(Ok(_))) | Poll::Pending => return Poll::Pending,
                    Poll::Ready(Some(Err(e))) => {
                        return Poll::Ready(Err(transport("h2 capacity", e)));
                    }
                    Poll::Ready(None) => {
                        return Poll::Ready(Err(ModuleError::Transport(
                            "h2 stream closed before send completed".into(),
                        )));
                    }
                };
                let chunk = data.split_to(capacity.min(data.len()).min(SEND_CHUNK));
                stream
                    .send_data(chunk, false)
                    .map_err(|e| transport("h2 send_data", e))?;
            }
            // Drive the newly queued DATA and give reception priority on the next poll.
            cx.waker().wake_by_ref();
            Poll::Pending
        },
        stream,
    )
    .await
}

/// Like [`exchange`], but after sending `initial_frame`, dynamically polls
/// `body_source` for additional body chunks, encodes them as gRPC-framed
/// `ResponseBody` messages, and sends those too. END_STREAM is set after the
/// last body chunk (or after the initial frame if no body arrives).
async fn exchange_streaming<T, IO, B>(
    conn: &mut Connection<IO, Bytes>,
    stream: &mut SendStream<Bytes>,
    initial_frame: Bytes,
    body_source: &B,
    receive: impl core::future::Future<Output = Result<T, ModuleError>>,
) -> Result<T, ModuleError>
where
    IO: AsyncRead + AsyncWrite + Unpin,
    B: BodyPoll,
{
    // Phase 1: send the initial frame (ResponseHeaders).
    // Phase 2: poll body_source, encode and send body frames.
    // Phase 3: END_STREAM once body is complete.
    //
    // All phases interleave with h2 connection driving and reply reception.
    let mut pending_send: VecDeque<Bytes> = VecDeque::new();
    pending_send.push_back(initial_frame);
    let mut body_done = false;
    let mut all_sent = false;
    let mut receive = core::pin::pin!(receive);

    drive(
        conn,
        |cx, stream| {
            // Priority: check for a completed receive (reply + trailers).
            if let Poll::Ready(result) = receive.as_mut().poll(cx) {
                stream.reserve_capacity(0);
                return Poll::Ready(result);
            }

            if all_sent {
                return Poll::Pending;
            }

            // Drain send queue.
            while let Some(front) = pending_send.front_mut() {
                if front.is_empty() {
                    pending_send.pop_front();
                    continue;
                }
                stream.reserve_capacity(front.len().min(SEND_CHUNK));
                let capacity = match stream.poll_capacity(cx) {
                    Poll::Ready(Some(Ok(n))) if n > 0 => n,
                    Poll::Ready(Some(Ok(_))) | Poll::Pending => return Poll::Pending,
                    Poll::Ready(Some(Err(e))) => {
                        return Poll::Ready(Err(transport("h2 capacity", e)));
                    }
                    Poll::Ready(None) => {
                        return Poll::Ready(Err(ModuleError::Transport(
                            "h2 stream closed before send completed".into(),
                        )));
                    }
                };
                let chunk = front.split_to(capacity.min(front.len()).min(SEND_CHUNK));
                stream
                    .send_data(chunk, false)
                    .map_err(|e| transport("h2 send_data", e))?;
                cx.waker().wake_by_ref();
                return Poll::Pending;
            }

            // Send queue is drained. Poll for more body data.
            if !body_done {
                match body_source.poll_chunk(cx) {
                    Poll::Ready(Some((data, is_last))) => {
                        let eos = is_last;
                        match notify::encode_response_body_frames(&data, eos) {
                            Ok(frames) => {
                                for frame in frames {
                                    pending_send.push_back(Bytes::from(frame));
                                }
                            }
                            Err(_) => {
                                // Encoding failure on a non-critical path: send
                                // an empty EOS to gracefully close the stream.
                                let fallback =
                                    notify::encode_response_body_frames(b"", true).unwrap();
                                pending_send
                                    .push_back(Bytes::from(fallback.into_iter().next().unwrap()));
                                body_done = true;
                                cx.waker().wake_by_ref();
                                return Poll::Pending;
                            }
                        }
                        if is_last {
                            body_done = true;
                        }
                        cx.waker().wake_by_ref();
                        return Poll::Pending;
                    }
                    Poll::Ready(None) => {
                        // Sender dropped without sending last_buf. Send
                        // empty EOS to cleanly close the gRPC stream.
                        let eos_frame = notify::encode_response_body_frames(b"", true).unwrap();
                        pending_send.push_back(Bytes::from(eos_frame.into_iter().next().unwrap()));
                        body_done = true;
                        cx.waker().wake_by_ref();
                        return Poll::Pending;
                    }
                    Poll::Pending => return Poll::Pending,
                }
            }

            // Body is done and send queue is fully drained → set END_STREAM.
            stream
                .send_data(Bytes::new(), true)
                .map_err(|e| transport("h2 send_data", e))?;
            all_sent = true;
            cx.waker().wake_by_ref();
            Poll::Pending
        },
        stream,
    )
    .await
}

fn check_response(response: &http::Response<RecvStream>) -> Result<(), ModuleError> {
    if response.status() != http::StatusCode::OK {
        return Err(ModuleError::Transport(format!(
            "unexpected HTTP status {}",
            response.status()
        )));
    }
    let content_type = response
        .headers()
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    let media_type = content_type.split(';').next().unwrap_or("").trim();
    // Accept "application/grpc" and "application/grpc+<subtype>", case-insensitively.
    let is_grpc = media_type
        .get(..GRPC_MEDIA_TYPE.len())
        .is_some_and(|p| p.eq_ignore_ascii_case(GRPC_MEDIA_TYPE))
        && matches!(
            media_type.as_bytes().get(GRPC_MEDIA_TYPE.len()),
            None | Some(b'+')
        );
    if !is_grpc {
        return Err(ModuleError::Transport(
            "expected application/grpc content-type".into(),
        ));
    }
    Ok(())
}

fn check_eof(reader: &FrameReader) -> Result<(), ModuleError> {
    if reader.has_partial() {
        return Err(ModuleError::GrpcFrame("truncated gRPC frame".into()));
    }
    Ok(())
}

/// Terminal gRPC status is mandatory, including in trailers-only responses.
fn check_grpc_status(headers: &http::HeaderMap) -> Result<(), ModuleError> {
    let Some(status_val) = headers.get("grpc-status") else {
        return Err(ModuleError::Transport(
            "missing terminal grpc-status".into(),
        ));
    };
    let code = status_val
        .to_str()
        .ok()
        .and_then(|s| s.parse::<u32>().ok())
        .ok_or_else(|| ModuleError::Transport("malformed grpc-status".into()))?;
    if code == 0 {
        return Ok(());
    }
    let msg = headers
        .get("grpc-message")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("unknown");
    Err(ModuleError::Transport(format!("gRPC error {code}: {msg}")))
}

#[cfg(test)]
mod tests {
    use super::*;
    use core::future::Future;
    use std::sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    };
    use std::task::{Wake, Waker};

    use envoy_types::pb::envoy::config::core::v3::{HeaderValue, HeaderValueOption};
    use envoy_types::pb::envoy::service::ext_proc::v3::{
        CommonResponse, HeaderMutation, HeadersResponse, ImmediateResponse, ProcessingResponse,
        processing_response::Response,
    };
    use prost::Message;

    use crate::codec::grpc;

    /// A test `BodyPoll` that immediately yields an empty EOS body chunk.
    struct ImmediateEos(std::cell::Cell<bool>);

    impl ImmediateEos {
        fn new() -> Self {
            Self(std::cell::Cell::new(false))
        }
    }

    impl BodyPoll for ImmediateEos {
        fn poll_chunk(&self, _cx: &mut Context<'_>) -> Poll<Option<(Vec<u8>, bool)>> {
            if self.0.get() {
                // Already sent EOS — no more chunks.
                Poll::Ready(None)
            } else {
                self.0.set(true);
                Poll::Ready(Some((Vec::new(), true)))
            }
        }
    }

    struct Ready(AtomicBool);

    impl Wake for Ready {
        fn wake(self: Arc<Self>) {
            self.0.store(true, Ordering::Relaxed);
        }

        fn wake_by_ref(self: &Arc<Self>) {
            self.0.store(true, Ordering::Relaxed);
        }
    }

    /// A bounded, single-thread executor: all I/O is in-memory and every
    /// pending future must arrange a wake. A flow-control deadlock fails instead
    /// of hanging the test. No Tokio runtime, timers, sockets or spawned tasks.
    fn run<T>(client: impl Future<Output = T>, server: impl Future<Output = ()>) -> T {
        let ready = Arc::new(Ready(AtomicBool::new(true)));
        let waker = Waker::from(ready.clone());
        let mut cx = Context::from_waker(&waker);
        let mut client = core::pin::pin!(client);
        let mut server = core::pin::pin!(server);
        for _ in 0..10_000 {
            assert!(
                ready.0.swap(false, Ordering::Relaxed),
                "duplex exchange stalled"
            );
            if let Poll::Ready(value) = client.as_mut().poll(&mut cx) {
                return value;
            }
            assert!(server.as_mut().poll(&mut cx).is_pending());
        }
        panic!("duplex exchange exceeded poll budget");
    }

    fn framed(response: Response) -> Vec<u8> {
        grpc::encode(
            &ProcessingResponse {
                response: Some(response),
                ..Default::default()
            }
            .encode_to_vec(),
        )
    }

    fn endpoint_reply() -> Vec<u8> {
        framed(Response::RequestHeaders(HeadersResponse {
            response: Some(CommonResponse {
                header_mutation: Some(HeaderMutation {
                    set_headers: vec![HeaderValueOption {
                        header: Some(HeaderValue {
                            key: "x-gateway-destination-endpoint".into(),
                            value: "127.0.0.1:8080".into(),
                            ..Default::default()
                        }),
                        ..Default::default()
                    }],
                    ..Default::default()
                }),
                ..Default::default()
            }),
        }))
    }

    /// The server consumes one small DATA chunk but never releases capacity.
    /// Both request and response-phase sends exceed its 32-byte stream window.
    fn duplex(
        response: http::Response<()>,
        data: Vec<u8>,
        trailers: Option<http::HeaderMap>,
        trailers_only: bool,
        response_phase: bool,
    ) -> Result<(), ModuleError> {
        let (client_io, server_io) = tokio::io::duplex(4096);
        let client = async {
            let (mut sender, mut conn) = h2::client::handshake(client_io).await.unwrap();
            let (_, mut stream) = epp_call(
                &mut sender,
                &mut conn,
                "epp",
                false,
                grpc::encode(b"headers"),
                vec![vec![0; 256 * 1024]],
            )
            .await?;
            if response_phase {
                send_response_phase(
                    &mut stream,
                    &mut conn,
                    vec![0; 256 * 1024].into(),
                    &ImmediateEos::new(),
                )
                .await?;
            }
            Ok(())
        };
        let server = async {
            let mut conn = h2::server::Builder::new()
                .initial_window_size(32)
                .handshake::<_, Bytes>(server_io)
                .await
                .unwrap();
            let mut request = None;
            let mut response = Some(response);
            let mut data = Some(data);
            let mut trailers = trailers;
            core::future::poll_fn(|cx| {
                if let Poll::Ready(Some(incoming)) = conn.poll_accept(cx) {
                    assert!(request.is_none());
                    request = Some(incoming.unwrap());
                }
                if let Some((req, send)) = &mut request
                    && response.is_some()
                    && let Poll::Ready(Some(chunk)) = req.body_mut().poll_data(cx)
                {
                    assert!(!chunk.unwrap().is_empty());
                    let mut body = send
                        .send_response(response.take().unwrap(), trailers_only)
                        .unwrap();
                    if !trailers_only {
                        body.send_data(Bytes::from(data.take().unwrap()), trailers.is_none())
                            .unwrap();
                        if let Some(t) = trailers.take() {
                            body.send_trailers(t).unwrap();
                        }
                    }
                    cx.waker().wake_by_ref();
                }
                Poll::<()>::Pending
            })
            .await;
        };
        run(client, server)
    }

    fn ok_response() -> http::Response<()> {
        http::Response::builder()
            .header("content-type", "application/grpc")
            .body(())
            .unwrap()
    }

    fn status(code: &str) -> http::HeaderMap {
        let mut headers = http::HeaderMap::new();
        headers.insert("grpc-status", code.parse().unwrap());
        headers
    }

    #[test]
    fn immediate_response_interrupts_flow_control_stall() {
        let data = framed(Response::ImmediateResponse(ImmediateResponse::default()));
        assert!(matches!(
            duplex(ok_response(), data, Some(status("0")), false, false),
            Err(ModuleError::ImmediateResponse(500))
        ));
    }

    #[test]
    fn clean_early_close_preserves_endpoint_and_skips_response_send() {
        assert!(
            duplex(
                ok_response(),
                endpoint_reply(),
                Some(status("0")),
                false,
                true
            )
            .is_ok()
        );
    }

    #[test]
    fn invalid_http_or_content_type_is_rejected_before_body_send_completes() {
        for (code, content_type) in [
            (503, "application/grpc"),
            (200, "text/plain"),
            (200, "application/grpc-web"),
        ] {
            let response = http::Response::builder()
                .status(code)
                .header("content-type", content_type)
                .body(())
                .unwrap();
            assert!(matches!(
                duplex(response, endpoint_reply(), Some(status("0")), false, false),
                Err(ModuleError::Transport(_))
            ));
        }
    }

    #[test]
    fn grpc_content_type_suffix_and_parameters_are_accepted() {
        for content_type in ["application/grpc+proto", "application/grpc; charset=utf-8"] {
            let response = http::Response::builder()
                .header("content-type", content_type)
                .body(())
                .unwrap();
            assert!(duplex(response, endpoint_reply(), Some(status("0")), false, false).is_ok());
        }
    }

    #[test]
    fn trailers_only_status_is_checked() {
        for code in [None, Some("0"), Some("14"), Some("invalid")] {
            let mut response = ok_response();
            if let Some(code) = code {
                *response.headers_mut() = status(code);
                response
                    .headers_mut()
                    .insert("content-type", "application/grpc".parse().unwrap());
            }
            let result = duplex(response, vec![], None, true, false);
            if code == Some("0") {
                // Transport succeeded; no endpoint was supplied by ext_proc.
                assert!(matches!(result, Err(ModuleError::NoEndpoint)));
            } else {
                assert!(matches!(result, Err(ModuleError::Transport(_))));
            }
        }
    }

    #[test]
    fn terminal_status_is_required_even_if_initial_headers_have_status() {
        let mut response = ok_response();
        response
            .headers_mut()
            .insert("grpc-status", "0".parse().unwrap());
        assert!(matches!(
            duplex(response, endpoint_reply(), None, false, false),
            Err(ModuleError::Transport(_))
        ));
    }

    #[test]
    fn non_ok_trailers_are_rejected() {
        assert!(matches!(
            duplex(
                ok_response(),
                endpoint_reply(),
                Some(status("14")),
                false,
                false
            ),
            Err(ModuleError::Transport(_))
        ));
    }

    #[test]
    fn response_phase_checks_status_and_partial_frame_after_reply() {
        use envoy_types::pb::envoy::service::ext_proc::v3::BodyResponse;
        let mut data = endpoint_reply();
        data.extend(framed(Response::RequestBody(BodyResponse::default())));
        data.extend(framed(
            Response::ResponseHeaders(HeadersResponse::default()),
        ));
        assert!(duplex(ok_response(), data.clone(), Some(status("0")), false, true).is_ok());
        for trailers in [None, Some(status("14"))] {
            assert!(matches!(
                duplex(ok_response(), data.clone(), trailers, false, true),
                Err(ModuleError::Transport(_))
            ));
        }
        for partial in [vec![0, 0], vec![0, 0, 0, 0, 2, 1]] {
            let mut truncated = data.clone();
            truncated.extend(partial);
            assert!(matches!(
                duplex(ok_response(), truncated, Some(status("0")), false, true),
                Err(ModuleError::GrpcFrame(_))
            ));
        }
    }

    #[test]
    fn near_limit_messages_work_across_request_and_response_phases() {
        use envoy_types::pb::envoy::service::ext_proc::v3::BodyResponse;

        // A length-delimited unknown protobuf field pads a valid reply without
        // introducing header mutations or exceeding their separate limits.
        fn padded(frame: Vec<u8>) -> Vec<u8> {
            let mut payload = grpc::decode(&frame).unwrap().0.to_vec();
            let padding = MAX_RESPONSE_BUF - 5 - payload.len() - 2 - 3;
            prost::encoding::encode_varint((100 << 3) | 2, &mut payload);
            prost::encoding::encode_varint(padding as u64, &mut payload);
            payload.resize(payload.len() + padding, 0);
            let frame = grpc::encode(&payload);
            assert_eq!(frame.len(), MAX_RESPONSE_BUF);
            frame
        }

        let mut data = endpoint_reply();
        data.extend(padded(framed(Response::RequestBody(
            BodyResponse::default(),
        ))));
        data.extend(padded(framed(Response::ResponseHeaders(
            HeadersResponse::default(),
        ))));
        data.extend(framed(Response::ResponseBody(BodyResponse::default())));
        assert!(duplex(ok_response(), data, Some(status("0")), false, true).is_ok());
    }

    #[test]
    fn grpc_status_required() {
        let mut h = http::HeaderMap::new();
        assert!(check_grpc_status(&h).is_err());
        h.insert("grpc-status", "0".parse().unwrap());
        assert!(check_grpc_status(&h).is_ok());
    }

    #[test]
    fn grpc_status_error() {
        let mut h = http::HeaderMap::new();
        h.insert("grpc-status", "14".parse().unwrap());
        h.insert("grpc-message", "unavailable".parse().unwrap());
        assert!(check_grpc_status(&h).is_err());
        h.insert("grpc-status", "x".parse().unwrap());
        assert!(check_grpc_status(&h).is_err());
    }
}
