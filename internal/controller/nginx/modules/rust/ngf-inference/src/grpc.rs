//! gRPC client for Envoy ExternalProcessor (ext_proc) protocol.
//!
//! Implements the EPP (Endpoint Picker Protocol) exchange using the h2 crate
//! for HTTP/2 with manual gRPC framing over nginx's native async I/O.
//!
//! The exchange is split into two phases on a single bidirectional gRPC stream:
//!
//! **Phase 1 — Request (access handler async task):**
//! 1. Connect to EPP server via PeerConnection (with optional SSL)
//! 2. HTTP/2 handshake using h2::client::handshake
//! 3. Open a gRPC bidirectional stream
//! 4. Send RequestHeaders and RequestBody as gRPC frames
//! 5. Read the endpoint selection response
//! 6. Keep the gRPC stream open for the response phase
//!
//! **Phase 2 — Response (header filter, fire-and-forget):**
//! 7. Send ResponseHeaders with served endpoint on the same stream
//! 8. Close the gRPC stream
//!
//! The stream state ([`EppStream`]) is preserved between phases in the
//! per-request context. Phase 2 is fire-and-forget — the client response
//! is not delayed while waiting for EPP's reply.
//!
//! ## Architecture: h2 without a separate connection driver
//!
//! Unlike hyper (which requires a separate spawned task to drive the HTTP/2
//! connection), we use h2 directly and drive the `Connection` future from the
//! same async task that sends requests and reads responses. The h2 crate's
//! `SendRequest` and `Connection` share internal state via `Arc<Mutex<>>`,
//! so polling the `Connection` advances I/O that `SendRequest`/`ResponseFuture`
//! /`RecvStream` need. This avoids the cross-task waker chain that caused
//! requests to hang with hyper on nginx's single-threaded event loop.
//!
//! After phase 1 completes, the `Connection` future is moved into a spawned
//! background task so it can continue to drive I/O for the fire-and-forget
//! response phase.

use crate::net::peer_conn::{OwnedPool, PeerConnection};
use crate::net::ssl::NgxSsl;
use crate::protos::envoy;

use core::future::Future;
use core::pin::Pin;
use core::ptr::NonNull;
use core::task::Poll;
use core::time::Duration;
use core::{future, mem};

use ngx::async_::Sleep;
use ngx::core::Pool;
use std::collections::HashMap;
use std::io;

use bytes::{Buf, BufMut, Bytes, BytesMut};
use h2::client::{Connection, ResponseFuture, SendRequest};
use h2::{RecvStream, SendStream};
use http::header::CONTENT_TYPE;
use nginx_sys::{NGX_LOG_DEBUG, ngx_log_t, ngx_msec_t, ngx_resolver_t};
use ngx::ngx_log_error;
use prost::Message;

/// Optional nginx resolver parameters extracted from the request's location
/// configuration. When `resolver` is `Some`, [`connect_peer`] will use the
/// nginx async resolver instead of the blocking `getaddrinfo(3)` fallback.
#[derive(Clone, Copy)]
pub struct ResolverParams {
    pub resolver: Option<NonNull<ngx_resolver_t>>,
    pub timeout: ngx_msec_t,
}

type ProcessingRequest = envoy::service::ext_proc::v3::ProcessingRequest;
type ProcessingResponse = envoy::service::ext_proc::v3::ProcessingResponse;
type HttpHeaders = envoy::service::ext_proc::v3::HttpHeaders;
type HeaderMap = envoy::config::core::v3::HeaderMap;

/// Type alias for the h2 Connection pinned in a Box over a PeerConnection.
type H2Conn = Pin<Box<Connection<Pin<Box<PeerConnection>>, Bytes>>>;

/// Result of the EPP request phase (phase 1).
pub struct EppRequestResult {
    /// The selected endpoint from the EPP response (e.g., "10.0.0.1:8080").
    pub selected_endpoint: Option<String>,
    /// The live gRPC stream for the response phase.
    /// `None` if the stream ended without selecting an endpoint.
    pub stream: Option<EppStream>,
}

/// Live gRPC stream state preserved between EPP request and response phases.
///
/// The ext_proc protocol uses a single bidirectional gRPC stream for the entire
/// request lifecycle. Phase 1 (request) reads the endpoint selection; phase 2
/// (response) sends the served endpoint back as a fire-and-forget notification.
/// This struct holds the handles needed to send additional data and keep the
/// HTTP/2 connection alive between the two phases.
pub struct EppStream {
    /// Outbound send stream — used to send the ResponseHeaders gRPC frame.
    send_stream: SendStream<Bytes>,
    /// Inbound receive stream — kept alive to prevent HTTP/2 stream reset.
    /// Not read after phase 1; exists solely to keep the stream open.
    _recv_stream: RecvStream,
    /// The HTTP/2 connection future, moved into a background task in phase 2
    /// so it can drive the I/O needed to flush the fire-and-forget message.
    /// Wrapped in Option so we can take() it when spawning.
    conn: Option<H2Conn>,
    /// SSL context — must outlive the connection to prevent use-after-free.
    _ssl: Option<NgxSsl>,
}

/// Timeout error type for the EPP exchange.
///
/// Returned when the timeout future completes before the wrapped future.
struct TimeoutError;

/// Run a future with a timeout using nginx's async timer.
///
/// If the future completes before the timeout, returns `Ok(result)`.
/// If the timeout expires first, returns `Err(TimeoutError)`.
///
/// This uses `ngx::async_::Sleep` which integrates with nginx's event loop,
/// avoiding the need for a separate runtime (tokio, etc.).
async fn with_timeout<F, T>(future: F, timeout: Duration, log: NonNull<ngx_log_t>) -> Result<T, TimeoutError>
where
    F: Future<Output = T>,
{
    use core::task::Context;
    use std::pin::pin;

    let mut future = pin!(future);
    let mut sleep = pin!(Sleep::new(timeout, log));

    future::poll_fn(|cx: &mut Context<'_>| {
        // Poll the main future first
        if let Poll::Ready(result) = future.as_mut().poll(cx) {
            return Poll::Ready(Ok(result));
        }

        // Poll the timeout
        if let Poll::Ready(()) = sleep.as_mut().poll(cx) {
            return Poll::Ready(Err(TimeoutError));
        }

        Poll::Pending
    })
    .await
}

/// Run EPP request phase: connect, send headers+body, read endpoint selection.
///
/// This is phase 1 of the two-phase EPP exchange. It:
/// 1. Connects to the EPP server (with optional TLS)
/// 2. Performs HTTP/2 handshake via h2
/// 3. Opens a gRPC bidirectional stream
/// 4. Sends RequestHeaders and RequestBody as gRPC frames
/// 5. Reads the endpoint selection response
///
/// The entire exchange is wrapped in a timeout (`timeout_ms`). If any step
/// stalls (DNS, connect, handshake, send, read), the timeout will fire and
/// the function returns an error so the caller can fail open/closed.
///
/// The gRPC stream is kept open — the returned [`EppStream`] must be passed
/// to [`epp_response_phase`] after the upstream responds.
///
/// The h2 `Connection` future is driven from this same async task (no separate
/// spawned driver), avoiding the cross-task waker issues that occur with hyper
/// on nginx's single-threaded event loop.
#[allow(clippy::too_many_arguments)]
pub async fn epp_request_phase(
    log: NonNull<ngx_log_t>,
    endpoint: &str,
    timeout_ms: u64,
    headers: Vec<(String, String)>,
    body: Option<Vec<u8>>,
    use_tls: bool,
    ssl: Option<NgxSsl>,
    resolver_params: ResolverParams,
) -> Result<EppRequestResult, String> {
    let timeout = Duration::from_millis(timeout_ms);

    // Wrap the entire EPP exchange in a timeout. This covers DNS resolution,
    // TCP connect, TLS handshake, HTTP/2 handshake, request send, and response
    // read. If any step stalls, the timeout fires and we return an error.
    match with_timeout(
        epp_request_phase_inner(log, endpoint, headers, body, use_tls, ssl, resolver_params),
        timeout,
        log,
    )
    .await
    {
        Ok(result) => result,
        Err(TimeoutError) => Err(format!(
            "EPP request timed out after {}ms (endpoint: {})",
            timeout_ms, endpoint
        )),
    }
}

/// Inner implementation of epp_request_phase without timeout wrapper.
///
/// Separated so `with_timeout` can wrap the entire async operation cleanly.
#[allow(clippy::too_many_arguments)]
async fn epp_request_phase_inner(
    log: NonNull<ngx_log_t>,
    endpoint: &str,
    headers: Vec<(String, String)>,
    body: Option<Vec<u8>>,
    use_tls: bool,
    ssl: Option<NgxSsl>,
    resolver_params: ResolverParams,
) -> Result<EppRequestResult, String> {
    let uri_str = normalize_endpoint(endpoint, use_tls);
    let uri: http::Uri = uri_str
        .parse()
        .map_err(|e| format!("invalid endpoint URI: {e}"))?;

    // Connect via PeerConnection
    let peer = connect_peer(log, &uri, use_tls, ssl.as_ref(), resolver_params).await?;

    // HTTP/2 handshake — peer is Pin<Box<PeerConnection>> which is Unpin
    let (send_request, conn) = h2::client::handshake(peer)
        .await
        .map_err(|e| format!("HTTP/2 handshake failed: {e}"))?;

    // Pin the connection for polling
    let mut conn = Box::pin(conn);

    // Build and encode the gRPC request messages (RequestHeaders + optional RequestBody)
    // directly into a single buffer to minimize copies of potentially large bodies.
    let request_msgs = build_request_messages(&headers, body);
    let payload = encode_grpc_messages_into(&request_msgs);

    // Build the HTTP/2 request (body sent via SendStream, not in the request itself).
    // h2 requires scheme + authority in the URI for the :scheme and :authority pseudo-headers.
    let scheme = if use_tls { "https" } else { "http" };
    let authority = uri.authority().map(|a| a.as_str()).unwrap_or_default();
    let full_uri =
        format!("{scheme}://{authority}/envoy.service.ext_proc.v3.ExternalProcessor/Process");
    let req = http::Request::builder()
        .method("POST")
        .uri(&full_uri)
        .version(http::Version::HTTP_2)
        .header(CONTENT_TYPE, "application/grpc")
        .header("te", "trailers")
        .body(())
        .map_err(|e| format!("failed to build request: {e}"))?;

    // Wait for the connection to be ready, then send the request.
    // We must poll the connection concurrently to drive I/O.
    let (resp_future, mut send_stream) =
        drive_conn_until_ready(&mut conn, send_request, req).await?;

    // Send the gRPC payload on the send stream with flow-control awareness.
    // For large payloads (e.g., LLM prompts with RAG context), this avoids
    // unbounded buffering inside h2 by only sending data as the remote peer
    // advertises capacity via WINDOW_UPDATE frames.
    // end_of_stream=false because the response phase will send more data.
    drive_conn_send_data(&mut conn, &mut send_stream, payload, false).await?;

    // Read the response headers while driving the connection
    let resp = drive_conn_until_response(&mut conn, resp_future).await?;

    let (parts, recv_stream) = resp.into_parts();

    // Check for immediate gRPC error in headers
    if let Some(status) = parts.headers.get("grpc-status") {
        let code = status.to_str().unwrap_or("?");
        if code != "0" {
            let msg = parts
                .headers
                .get("grpc-message")
                .and_then(|v| v.to_str().ok())
                .unwrap_or("unknown");
            let _ = send_stream.send_data(Bytes::new(), true);
            return Err(format!("gRPC error: status={code}, message={msg}"));
        }
    }

    // Read endpoint selection from response body.
    // We poll the connection concurrently with reading the recv stream.
    let mut resp_data = BytesMut::new();
    let mut recv_stream = recv_stream;
    let mut selected_endpoint: Option<String> = None;

    loop {
        // Try to decode any complete messages we have so far
        while let Some(response) = try_decode_grpc_message::<ProcessingResponse>(&mut resp_data) {
            if let Some(ep) = extract_endpoint_from_response(&response) {
                selected_endpoint = Some(ep);
                break;
            }
        }

        if selected_endpoint.is_some() {
            break;
        }

        // Read more data while driving the connection
        match drive_conn_until_data(&mut conn, &mut recv_stream).await {
            Some(Ok(data)) => {
                // Release flow control capacity back to the sender
                let len = data.len();
                let _ = recv_stream.flow_control().release_capacity(len);
                resp_data.extend_from_slice(&data);
            }
            Some(Err(e)) => {
                let _ = send_stream.send_data(Bytes::new(), true);
                return Err(format!("gRPC stream error: {e}"));
            }
            None => {
                // Stream ended without endpoint
                let _ = send_stream.send_data(Bytes::new(), true);
                return Ok(EppRequestResult {
                    selected_endpoint: None,
                    stream: None,
                });
            }
        }
    }

    // Keep the stream alive for the response phase.
    Ok(EppRequestResult {
        selected_endpoint,
        stream: Some(EppStream {
            send_stream,
            _recv_stream: recv_stream,
            conn: Some(conn),
            _ssl: ssl,
        }),
    })
}

/// Poll the h2 Connection concurrently until SendRequest is ready, then send the request.
///
/// Returns `(ResponseFuture, SendStream<Bytes>)` on success.
async fn drive_conn_until_ready(
    conn: &mut H2Conn,
    mut send_request: SendRequest<Bytes>,
    req: http::Request<()>,
) -> Result<(ResponseFuture, SendStream<Bytes>), String> {
    let mut req = Some(req);
    future::poll_fn(|cx| {
        // Drive connection I/O — ignore Pending (normal), error on Ready(Err)
        match conn.as_mut().poll(cx) {
            Poll::Ready(Ok(())) => {
                return Poll::Ready(Err("connection closed unexpectedly".to_string()));
            }
            Poll::Ready(Err(e)) => {
                return Poll::Ready(Err(format!("connection error: {e}")));
            }
            Poll::Pending => {}
        }

        // Check if we can send a request
        match send_request.poll_ready(cx) {
            Poll::Ready(Ok(())) => {
                // Ready to send — do it synchronously
                let r = req.take().expect("poll_ready resolved only once");
                match send_request.send_request(r, false) {
                    Ok(result) => Poll::Ready(Ok(result)),
                    Err(e) => Poll::Ready(Err(format!("failed to send request: {e}"))),
                }
            }
            Poll::Ready(Err(e)) => Poll::Ready(Err(format!("send_request ready error: {e}"))),
            Poll::Pending => Poll::Pending,
        }
    })
    .await
}

/// Poll the h2 Connection concurrently until the ResponseFuture resolves.
async fn drive_conn_until_response(
    conn: &mut H2Conn,
    mut resp_future: ResponseFuture,
) -> Result<http::Response<RecvStream>, String> {
    future::poll_fn(|cx| {
        // Drive connection I/O
        match conn.as_mut().poll(cx) {
            Poll::Ready(Ok(())) => {
                return Poll::Ready(Err("connection closed before response received".to_string()));
            }
            Poll::Ready(Err(e)) => {
                return Poll::Ready(Err(format!("connection error: {e}")));
            }
            Poll::Pending => {}
        }

        // Check for response
        match Pin::new(&mut resp_future).poll(cx) {
            Poll::Ready(Ok(resp)) => Poll::Ready(Ok(resp)),
            Poll::Ready(Err(e)) => Poll::Ready(Err(format!("response error: {e}"))),
            Poll::Pending => Poll::Pending,
        }
    })
    .await
}

/// Send data on the h2 SendStream with flow-control awareness.
///
/// Instead of calling `send_data` with the entire payload (which causes h2 to
/// buffer unboundedly), this uses `reserve_capacity` / `poll_capacity` to send
/// data only as the remote peer advertises window capacity. This keeps h2's
/// internal buffer small regardless of payload size.
///
/// The h2 Connection is polled concurrently to drive I/O (process
/// WINDOW_UPDATE frames, etc.) while waiting for capacity.
async fn drive_conn_send_data(
    conn: &mut H2Conn,
    send_stream: &mut SendStream<Bytes>,
    data: Bytes,
    end_of_stream: bool,
) -> Result<(), String> {
    let total = data.len();

    // For small payloads that fit in the initial window, skip the
    // reserve/poll loop — send_data handles it directly and the overhead
    // of one buffered frame is negligible.
    if total == 0 {
        return send_stream
            .send_data(Bytes::new(), end_of_stream)
            .map_err(|e| format!("failed to send empty frame: {e}"));
    }

    let mut sent = 0;
    send_stream.reserve_capacity(total);

    while sent < total {
        // Wait for capacity while driving the connection
        let available = future::poll_fn(|cx| {
            // Drive connection I/O to process WINDOW_UPDATE frames
            match conn.as_mut().poll(cx) {
                Poll::Ready(Ok(())) => {
                    return Poll::Ready(Err("connection closed while sending data".to_string()));
                }
                Poll::Ready(Err(e)) => {
                    return Poll::Ready(Err(format!("connection error while sending: {e}")));
                }
                Poll::Pending => {}
            }

            // Poll for send capacity
            match send_stream.poll_capacity(cx) {
                Poll::Ready(Some(Ok(n))) => Poll::Ready(Ok(n)),
                Poll::Ready(Some(Err(e))) => Poll::Ready(Err(format!("capacity error: {e}"))),
                Poll::Ready(None) => {
                    Poll::Ready(Err("stream closed while waiting for capacity".to_string()))
                }
                Poll::Pending => Poll::Pending,
            }
        })
        .await?;

        // Send exactly what's available (don't exceed remaining data)
        let chunk_end = (sent + available).min(total);
        let chunk = data.slice(sent..chunk_end);
        let is_last = chunk_end == total;

        send_stream
            .send_data(chunk, is_last && end_of_stream)
            .map_err(|e| format!("failed to send data chunk: {e}"))?;

        sent = chunk_end;
    }

    Ok(())
}

/// Poll the h2 Connection concurrently until the RecvStream produces data.
///
/// Returns `Some(Ok(data))`, `Some(Err(e))`, or `None` (end of stream).
async fn drive_conn_until_data(
    conn: &mut H2Conn,
    recv_stream: &mut RecvStream,
) -> Option<Result<Bytes, h2::Error>> {
    future::poll_fn(|cx| {
        // Drive connection I/O
        match conn.as_mut().poll(cx) {
            Poll::Ready(Ok(())) | Poll::Ready(Err(_)) => {
                // Connection done/error — check if recv has data first
            }
            Poll::Pending => {}
        }

        // Check for data
        recv_stream.poll_data(cx)
    })
    .await
}

/// Run EPP response phase: send served endpoint notification (fire-and-forget).
///
/// This is phase 2 of the two-phase EPP exchange. It is called after the
/// upstream has responded (from the header filter). It:
/// 1. Encodes the ResponseHeaders message as a gRPC frame
/// 2. Sends it on the h2 SendStream with end_of_stream=true
/// 3. Spawns a background task to drive the h2 Connection until the
///    data is flushed
///
/// This is fire-and-forget — the client response is not delayed.
///
/// Returns `Ok(())` if the data was queued for sending, `Err` if the
/// send failed (e.g. stream already reset). Either way, the background
/// task is spawned to flush the connection and clean up resources.
pub fn epp_response_phase(mut stream: EppStream, served_endpoint: &str) -> Result<(), String> {
    // Encode the ResponseHeaders message and send it on the h2 stream.
    let resp_msg = build_response_headers_msg(served_endpoint);
    let frame = encode_grpc_message(&resp_msg);

    // Send with end_of_stream=true to close our half of the stream.
    let send_result = stream
        .send_stream
        .send_data(frame, true)
        .map_err(|e| format!("failed to send response headers: {e}"));

    // Move the entire EppStream into a background task so the h2
    // Connection can flush the queued data, and the SendStream/RecvStream
    // stay alive until the connection is done (dropping them early can
    // trigger h2 stream resets that race with the connection driver).
    if stream.conn.is_some() {
        ngx::async_::spawn(async move {
            if let Some(mut conn) = stream.conn.take() {
                let _ = future::poll_fn(|cx| conn.as_mut().poll(cx)).await;
            }
            // stream (with send_stream, _recv_stream, _ssl) dropped here
            // after the connection is fully flushed
            drop(stream);
        })
        .detach();
    }

    send_result
}

/// Connect to a peer using PeerConnection with optional SSL.
///
/// When `resolver_params` contains a resolver, hostname resolution uses nginx's
/// async resolver (non-blocking, integrated with the event loop). Otherwise,
/// falls back to the blocking `getaddrinfo(3)` via [`resolve_address`].
async fn connect_peer(
    log: NonNull<ngx_log_t>,
    uri: &http::Uri,
    use_tls: bool,
    ssl: Option<&NgxSsl>,
    resolver_params: ResolverParams,
) -> Result<Pin<Box<PeerConnection>>, String> {
    let host = uri.host().ok_or("URI has no host")?;
    let port = uri.port_u16().unwrap_or(if use_tls { 443 } else { 80 });

    let mut peer = Box::pin(
        PeerConnection::new(log).map_err(|e| format!("failed to create peer connection: {e}"))?,
    );

    if let Some(resolver_ptr) = resolver_params.resolver {
        // Async resolution via nginx's resolver — non-blocking.
        let resolver =
            ngx::async_::resolver::Resolver::from_resolver(resolver_ptr, resolver_params.timeout);

        // The resolver needs an ngx_str_t for the hostname and a Pool for
        // allocating the result. We use an OwnedPool that is dropped after
        // connect copies the sockaddr.
        let pool = OwnedPool::with_default_size(log)
            .map_err(|_| "failed to create pool for resolver".to_string())?;

        let pool_ref: &Pool = pool.as_ref();
        let name = unsafe { nginx_sys::ngx_str_t::from_bytes(pool_ref.as_ptr(), host.as_bytes()) }
            .ok_or_else(|| "failed to allocate ngx_str_t for hostname".to_string())?;

        match resolver.resolve_name(&name, pool_ref).await {
            Ok(addrs) => {
                let first = addrs
                    .first()
                    .ok_or("nginx resolver returned no addresses")?;

                // The resolver returns addresses without the port set — patch it in.
                set_sockaddr_port(first.sockaddr, first.socklen, port);

                peer.as_mut()
                    .connect(first)
                    .await
                    .map_err(|e| format!("TCP connect to {host}:{port} failed: {e}"))?;
            }
            Err(e) => {
                // The async resolver does not use search domains from
                // /etc/resolv.conf, so short Kubernetes names like
                // "svc.namespace" will fail. Fall back to getaddrinfo(3)
                // which applies search domains automatically.
                ngx_log_error!(
                    NGX_LOG_DEBUG,
                    log.as_ptr(),
                    "inference: async resolver failed for {}: {}, falling back to getaddrinfo",
                    host,
                    e
                );

                let resolved = resolve_address(host, port).map_err(|e2| {
                    format!("address resolution failed: {e2} (resolver also failed: {e})")
                })?;

                peer.as_mut()
                    .connect(&resolved.addr)
                    .await
                    .map_err(|e| format!("TCP connect to {host}:{port} failed: {e}"))?;
                drop(resolved);
            }
        }
    } else {
        // Fallback: blocking getaddrinfo(3). Works without a `resolver` directive
        // but blocks the nginx event loop during DNS lookups.
        let resolved =
            resolve_address(host, port).map_err(|e| format!("address resolution failed: {e}"))?;

        peer.as_mut()
            .connect(&resolved.addr)
            .await
            .map_err(|e| format!("TCP connect to {host}:{port} failed: {e}"))?;
        drop(resolved);
    }

    // SSL handshake if TLS is enabled
    if use_tls {
        let ssl = ssl.ok_or("TLS enabled but no SSL context configured")?;
        let ssl_name = std::ffi::CString::new(host).ok();
        let ssl_name_ref = ssl_name.as_deref();

        future::poll_fn(|cx| {
            peer.as_mut()
                .poll_ssl_handshake(ssl.as_ref(), ssl_name_ref, cx)
        })
        .await
        .map_err(|e| format!("TLS handshake with {host}:{port} failed: {e}"))?;
    }

    Ok(peer)
}

/// Set the port on a raw sockaddr pointer. This is needed because nginx's
/// resolver returns addresses without a port — it resolves hostnames only.
fn set_sockaddr_port(sockaddr: *mut nginx_sys::sockaddr, socklen: nginx_sys::socklen_t, port: u16) {
    if sockaddr.is_null() || socklen == 0 {
        return;
    }
    let family = unsafe { (*sockaddr).sa_family } as i32;
    match family {
        libc::AF_INET => {
            let sin = sockaddr.cast::<libc::sockaddr_in>();
            unsafe { (*sin).sin_port = port.to_be() };
        }
        libc::AF_INET6 => {
            let sin6 = sockaddr.cast::<libc::sockaddr_in6>();
            unsafe { (*sin6).sin6_port = port.to_be() };
        }
        _ => {} // unknown family — leave port as-is
    }
}

/// Wrapper that pairs a resolved `ngx_addr_t` with the owned storage behind
/// its `sockaddr` pointer.  When the wrapper is dropped the heap allocation is
/// freed — this avoids the leak that was present when using `Box::into_raw`
/// without a corresponding `Box::from_raw`.
pub(crate) struct ResolvedAddr {
    pub addr: nginx_sys::ngx_addr_t,
    /// Tracks the address family to correctly reclaim the Box-allocated sockaddr.
    family: SockaddrFamily,
}

#[derive(Clone, Copy)]
enum SockaddrFamily {
    V4,
    V6,
}

impl Drop for ResolvedAddr {
    fn drop(&mut self) {
        if !self.addr.sockaddr.is_null() {
            // Reclaim the Box allocation created in resolve_address.
            unsafe {
                match self.family {
                    SockaddrFamily::V4 => {
                        drop(Box::from_raw(self.addr.sockaddr as *mut libc::sockaddr_in));
                    }
                    SockaddrFamily::V6 => {
                        drop(Box::from_raw(self.addr.sockaddr as *mut libc::sockaddr_in6));
                    }
                }
            }
            self.addr.sockaddr = core::ptr::null_mut();
        }
    }
}

/// Resolve a hostname:port to an [`ngx_addr_t`](nginx_sys::ngx_addr_t).
///
/// The returned [`ResolvedAddr`] owns the heap-allocated sockaddr.  It must
/// be kept alive for as long as the `addr` field is in use.
fn resolve_address(host: &str, port: u16) -> Result<ResolvedAddr, io::Error> {
    use std::net::ToSocketAddrs;

    let socket_addr = format!("{host}:{port}")
        .to_socket_addrs()?
        .next()
        .ok_or_else(|| io::Error::new(io::ErrorKind::AddrNotAvailable, "no addresses resolved"))?;

    match socket_addr {
        std::net::SocketAddr::V4(v4) => {
            let mut sin: libc::sockaddr_in = unsafe { mem::zeroed() };
            #[cfg(target_os = "macos")]
            {
                sin.sin_len = mem::size_of::<libc::sockaddr_in>() as u8;
            }
            sin.sin_family = libc::AF_INET as libc::sa_family_t;
            sin.sin_port = v4.port().to_be();
            sin.sin_addr = libc::in_addr {
                s_addr: u32::from(*v4.ip()).to_be(),
            };
            let ptr = Box::into_raw(Box::new(sin));
            Ok(ResolvedAddr {
                addr: nginx_sys::ngx_addr_t {
                    sockaddr: ptr.cast(),
                    socklen: mem::size_of::<libc::sockaddr_in>() as _,
                    name: nginx_sys::ngx_str_t {
                        len: 0,
                        data: core::ptr::null_mut(),
                    },
                },
                family: SockaddrFamily::V4,
            })
        }
        std::net::SocketAddr::V6(v6) => {
            let mut sin6: libc::sockaddr_in6 = unsafe { mem::zeroed() };
            #[cfg(target_os = "macos")]
            {
                sin6.sin6_len = mem::size_of::<libc::sockaddr_in6>() as u8;
            }
            sin6.sin6_family = libc::AF_INET6 as libc::sa_family_t;
            sin6.sin6_port = v6.port().to_be();
            sin6.sin6_flowinfo = v6.flowinfo();
            sin6.sin6_addr = libc::in6_addr {
                s6_addr: v6.ip().octets(),
            };
            sin6.sin6_scope_id = v6.scope_id();
            let ptr = Box::into_raw(Box::new(sin6));
            Ok(ResolvedAddr {
                addr: nginx_sys::ngx_addr_t {
                    sockaddr: ptr.cast(),
                    socklen: mem::size_of::<libc::sockaddr_in6>() as _,
                    name: nginx_sys::ngx_str_t {
                        len: 0,
                        data: core::ptr::null_mut(),
                    },
                },
                family: SockaddrFamily::V6,
            })
        }
    }
}

/// Build ProcessingRequest messages for headers and optional body.
///
/// Takes ownership of the body `Vec<u8>` to avoid copying potentially
/// large payloads (LLM prompts with RAG context can be multi-MB).
fn build_request_messages(
    headers: &[(String, String)],
    body: Option<Vec<u8>>,
) -> Vec<ProcessingRequest> {
    let header_entries: Vec<envoy::config::core::v3::HeaderValue> = headers
        .iter()
        .map(|(k, v)| envoy::config::core::v3::HeaderValue {
            key: k.clone(),
            value: v.clone(),
            raw_value: Vec::new(),
        })
        .collect();

    let header_map = HeaderMap {
        headers: header_entries,
    };

    let metadata_context = {
        use envoy_types::pb::google::protobuf::Struct;
        let metadata_struct = Struct {
            fields: HashMap::new(),
        };
        let mut filter_metadata = HashMap::new();
        filter_metadata.insert("envoy.lb".to_string(), metadata_struct);

        Some(envoy::config::core::v3::Metadata {
            filter_metadata,
            typed_filter_metadata: HashMap::new(),
        })
    };

    let body_vec = body.filter(|b| !b.is_empty());

    let headers_msg = ProcessingRequest {
        request: Some(
            envoy::service::ext_proc::v3::processing_request::Request::RequestHeaders(
                HttpHeaders {
                    headers: Some(header_map),
                    attributes: HashMap::new(),
                    end_of_stream: body_vec.is_none(),
                },
            ),
        ),
        metadata_context,
        attributes: HashMap::new(),
        observability_mode: false,
        protocol_config: None,
    };

    let mut msgs = vec![headers_msg];

    if let Some(body_vec) = body_vec {
        let body_msg = ProcessingRequest {
            request: Some(
                envoy::service::ext_proc::v3::processing_request::Request::RequestBody(
                    envoy::service::ext_proc::v3::HttpBody {
                        body: body_vec, // move, not clone
                        end_of_stream: true,
                        end_of_stream_without_message: false,
                        grpc_message_compressed: false,
                    },
                ),
            ),
            metadata_context: None,
            attributes: HashMap::new(),
            observability_mode: false,
            protocol_config: None,
        };
        msgs.push(body_msg);
    }

    msgs
}

/// Encode a single protobuf message with gRPC length-prefix framing.
///
/// gRPC wire format:
///   - 1 byte: compression flag (0 = no compression)
///   - 4 bytes: message length (big-endian u32)
///   - N bytes: serialized protobuf
fn encode_grpc_message<M: Message>(msg: &M) -> Bytes {
    let len = msg.encoded_len();
    let mut buf = BytesMut::with_capacity(5 + len);
    buf.put_u8(0); // no compression
    buf.put_u32(len as u32);
    msg.encode(&mut buf)
        .expect("encode into pre-sized BytesMut cannot fail");
    buf.freeze()
}

/// Encode multiple protobuf messages with gRPC framing directly into one buffer.
///
/// Uses `prost::Message::encode` to write directly into the output buffer,
/// avoiding intermediate `Vec<u8>` allocations. This is important for large
/// payloads (e.g., LLM request bodies with RAG context).
fn encode_grpc_messages_into<M: Message>(msgs: &[M]) -> Bytes {
    // Pre-compute total size to allocate once
    let total: usize = msgs.iter().map(|m| 5 + m.encoded_len()).sum();
    let mut buf = BytesMut::with_capacity(total);
    for msg in msgs {
        let len = msg.encoded_len();
        buf.put_u8(0); // no compression
        buf.put_u32(len as u32);
        msg.encode(&mut buf)
            .expect("encode into pre-sized BytesMut cannot fail");
    }
    buf.freeze()
}

/// Encode multiple protobuf messages with gRPC framing into a single buffer.
#[cfg(test)]
fn encode_grpc_messages<M: Message>(msgs: &[M]) -> Bytes {
    let mut buf = BytesMut::new();
    for msg in msgs {
        let encoded = msg.encode_to_vec();
        buf.put_u8(0);
        buf.put_u32(encoded.len() as u32);
        buf.put_slice(&encoded);
    }
    buf.freeze()
}

/// Try to decode a single gRPC message from a buffer, consuming it if successful.
///
/// Returns None if there isn't enough data for a complete message.
fn try_decode_grpc_message<M: Message + Default>(buf: &mut BytesMut) -> Option<M> {
    if buf.len() < 5 {
        return None;
    }

    let _compressed = buf[0];
    let len = u32::from_be_bytes([buf[1], buf[2], buf[3], buf[4]]) as usize;

    if buf.len() < 5 + len {
        return None; // incomplete message
    }

    // Consume the header
    buf.advance(5);
    // Consume the message
    let msg_bytes = buf.split_to(len);
    M::decode(&msg_bytes[..]).ok()
}

/// Decode gRPC length-prefixed messages from a byte slice.
#[cfg(test)]
fn decode_grpc_messages<M: Message + Default>(data: &[u8]) -> Vec<M> {
    let mut messages = Vec::new();
    let mut remaining = data;

    while remaining.len() >= 5 {
        let _compressed = remaining[0];
        let len =
            u32::from_be_bytes([remaining[1], remaining[2], remaining[3], remaining[4]]) as usize;
        remaining = &remaining[5..];

        if remaining.len() < len {
            break;
        }

        if let Ok(msg) = M::decode(&remaining[..len]) {
            messages.push(msg);
        }
        remaining = &remaining[len..];
    }

    messages
}

pub(crate) fn normalize_endpoint(endpoint: &str, use_tls: bool) -> String {
    if endpoint.starts_with("http://") || endpoint.starts_with("https://") {
        endpoint.to_string()
    } else if use_tls {
        format!("https://{}", endpoint)
    } else {
        format!("http://{}", endpoint)
    }
}

/// Build a ResponseHeaders message with the served endpoint.
///
/// Per the EPP protocol spec, the served endpoint is sent in the metadata_context
/// field with namespace "envoy.lb" and key "x-gateway-destination-endpoint-served".
pub(crate) fn build_response_headers_msg(served_endpoint: &str) -> ProcessingRequest {
    let metadata_context = {
        use envoy_types::pb::google::protobuf::{Struct, Value, value::Kind};
        let mut fields = HashMap::new();
        fields.insert(
            "x-gateway-destination-endpoint-served".to_string(),
            Value {
                kind: Some(Kind::StringValue(served_endpoint.to_string())),
            },
        );
        let metadata_struct = Struct { fields };
        let mut filter_metadata = HashMap::new();
        filter_metadata.insert("envoy.lb".to_string(), metadata_struct);

        Some(envoy::config::core::v3::Metadata {
            filter_metadata,
            typed_filter_metadata: HashMap::new(),
        })
    };

    ProcessingRequest {
        request: Some(
            envoy::service::ext_proc::v3::processing_request::Request::ResponseHeaders(
                HttpHeaders {
                    headers: Some(HeaderMap { headers: vec![] }),
                    attributes: HashMap::new(),
                    end_of_stream: true,
                },
            ),
        ),
        metadata_context,
        attributes: HashMap::new(),
        observability_mode: false,
        protocol_config: None,
    }
}

/// Extract the selected endpoint from a ProcessingResponse.
pub(crate) fn extract_endpoint_from_response(resp: &ProcessingResponse) -> Option<String> {
    use envoy::service::ext_proc::v3::processing_response::Response;

    let raw_endpoint = match &resp.response {
        Some(Response::RequestHeaders(headers_resp)) => {
            headers_resp.response.as_ref().and_then(|common| {
                extract_header_from_mutation(
                    &common.header_mutation,
                    "x-gateway-destination-endpoint",
                )
            })
        }
        Some(Response::RequestBody(body_resp)) => body_resp.response.as_ref().and_then(|common| {
            extract_header_from_mutation(&common.header_mutation, "x-gateway-destination-endpoint")
        }),
        _ => None,
    };

    // EPP may return a comma-separated list of endpoints; use only the first one.
    raw_endpoint.map(|ep| {
        ep.split(',')
            .next()
            .map(|s| s.trim().to_string())
            .unwrap_or(ep)
    })
}

/// Extract a header value from a HeaderMutation by key (case-insensitive).
///
/// Checks both `value` (string) and `raw_value` (bytes) fields of HeaderValue,
/// preferring `raw_value` if non-empty (as the EPP may use it).
pub(crate) fn extract_header_from_mutation(
    mutation: &Option<envoy::service::ext_proc::v3::HeaderMutation>,
    target_key: &str,
) -> Option<String> {
    let mutation = mutation.as_ref()?;
    let target_lower = target_key.to_ascii_lowercase();

    for hvo in &mutation.set_headers {
        if let Some(ref hv) = hvo.header
            && hv.key.to_ascii_lowercase() == target_lower
        {
            return resolve_header_value(hv);
        }
    }
    None
}

/// Resolve the string value from a HeaderValue, preferring `raw_value` over `value`.
///
/// Returns `None` if both fields are empty.
fn resolve_header_value(hv: &envoy::config::core::v3::HeaderValue) -> Option<String> {
    if !hv.raw_value.is_empty() {
        return Some(String::from_utf8_lossy(&hv.raw_value).into_owned());
    }
    if !hv.value.is_empty() {
        return Some(hv.value.clone());
    }
    None
}

#[cfg(test)]
#[allow(deprecated, clippy::needless_update)]
mod tests {
    use super::*;
    use envoy::config::core::v3::HeaderValue;
    use envoy::service::ext_proc::v3::{
        BodyResponse, CommonResponse, HeaderMutation, HeadersResponse,
        processing_response::Response,
    };

    // --- normalize_endpoint ---

    #[test]
    fn normalize_endpoint_plain_host_no_tls() {
        assert_eq!(
            normalize_endpoint("epp.default:9002", false),
            "http://epp.default:9002"
        );
    }

    #[test]
    fn normalize_endpoint_plain_host_with_tls() {
        assert_eq!(
            normalize_endpoint("epp.default:9002", true),
            "https://epp.default:9002"
        );
    }

    #[test]
    fn normalize_endpoint_already_http() {
        assert_eq!(
            normalize_endpoint("http://epp:9002", true),
            "http://epp:9002"
        );
        assert_eq!(
            normalize_endpoint("http://epp:9002", false),
            "http://epp:9002"
        );
    }

    #[test]
    fn normalize_endpoint_already_https() {
        assert_eq!(
            normalize_endpoint("https://epp:9002", false),
            "https://epp:9002"
        );
        assert_eq!(
            normalize_endpoint("https://epp:9002", true),
            "https://epp:9002"
        );
    }

    // --- extract_header_from_mutation ---

    fn make_mutation(headers: Vec<(&str, &str)>) -> Option<HeaderMutation> {
        Some(HeaderMutation {
            set_headers: headers
                .into_iter()
                .map(|(k, v)| envoy::config::core::v3::HeaderValueOption {
                    header: Some(HeaderValue {
                        key: k.to_string(),
                        value: v.to_string(),
                        raw_value: Vec::new(),
                    }),
                    append: None,
                    append_action: 0,
                    keep_empty_value: false,
                })
                .collect(),
            remove_headers: Vec::new(),
        })
    }

    fn make_mutation_raw(key: &str, raw: &[u8]) -> Option<HeaderMutation> {
        Some(HeaderMutation {
            set_headers: vec![envoy::config::core::v3::HeaderValueOption {
                header: Some(HeaderValue {
                    key: key.to_string(),
                    value: String::new(),
                    raw_value: raw.to_vec(),
                }),
                append: None,
                append_action: 0,
                keep_empty_value: false,
            }],
            remove_headers: Vec::new(),
        })
    }

    #[test]
    fn extract_header_found() {
        let mutation = make_mutation(vec![("x-gateway-destination-endpoint", "10.0.0.1:8080")]);
        let result = extract_header_from_mutation(&mutation, "x-gateway-destination-endpoint");
        assert_eq!(result, Some("10.0.0.1:8080".to_string()));
    }

    #[test]
    fn extract_header_case_insensitive() {
        let mutation = make_mutation(vec![("X-Gateway-Destination-Endpoint", "10.0.0.1:8080")]);
        let result = extract_header_from_mutation(&mutation, "x-gateway-destination-endpoint");
        assert_eq!(result, Some("10.0.0.1:8080".to_string()));
    }

    #[test]
    fn extract_header_not_found() {
        let mutation = make_mutation(vec![("x-other-header", "value")]);
        let result = extract_header_from_mutation(&mutation, "x-gateway-destination-endpoint");
        assert_eq!(result, None);
    }

    #[test]
    fn extract_header_none_mutation() {
        let result = extract_header_from_mutation(&None, "x-gateway-destination-endpoint");
        assert_eq!(result, None);
    }

    #[test]
    fn extract_header_prefers_raw_value() {
        let mutation = make_mutation_raw("x-gateway-destination-endpoint", b"10.0.0.2:8080");
        let result = extract_header_from_mutation(&mutation, "x-gateway-destination-endpoint");
        assert_eq!(result, Some("10.0.0.2:8080".to_string()));
    }

    #[test]
    fn extract_header_empty_value_returns_none() {
        let mutation = make_mutation(vec![("x-gateway-destination-endpoint", "")]);
        let result = extract_header_from_mutation(&mutation, "x-gateway-destination-endpoint");
        assert_eq!(result, None);
    }

    // --- extract_endpoint_from_response ---

    fn make_request_headers_response(headers: Vec<(&str, &str)>) -> ProcessingResponse {
        ProcessingResponse {
            response: Some(Response::RequestHeaders(HeadersResponse {
                response: Some(CommonResponse {
                    header_mutation: make_mutation(headers),
                    ..Default::default()
                }),
                ..Default::default()
            })),
            ..Default::default()
        }
    }

    fn make_request_body_response(headers: Vec<(&str, &str)>) -> ProcessingResponse {
        ProcessingResponse {
            response: Some(Response::RequestBody(BodyResponse {
                response: Some(CommonResponse {
                    header_mutation: make_mutation(headers),
                    ..Default::default()
                }),
            })),
            ..Default::default()
        }
    }

    #[test]
    fn extract_endpoint_from_request_headers() {
        let resp = make_request_headers_response(vec![(
            "x-gateway-destination-endpoint",
            "10.0.0.1:8080",
        )]);
        assert_eq!(
            extract_endpoint_from_response(&resp),
            Some("10.0.0.1:8080".to_string()),
        );
    }

    #[test]
    fn extract_endpoint_from_request_body() {
        let resp =
            make_request_body_response(vec![("x-gateway-destination-endpoint", "10.0.0.2:8080")]);
        assert_eq!(
            extract_endpoint_from_response(&resp),
            Some("10.0.0.2:8080".to_string()),
        );
    }

    #[test]
    fn extract_endpoint_comma_separated_list_uses_first() {
        // EPP may return a comma-separated list of endpoints; we use only the first.
        let resp = make_request_headers_response(vec![(
            "x-gateway-destination-endpoint",
            "10.0.0.1:8080, 10.0.0.2:8080, 10.0.0.3:8080",
        )]);
        assert_eq!(
            extract_endpoint_from_response(&resp),
            Some("10.0.0.1:8080".to_string()),
        );
    }

    #[test]
    fn extract_endpoint_comma_separated_no_spaces() {
        let resp = make_request_headers_response(vec![(
            "x-gateway-destination-endpoint",
            "10.0.0.1:8080,10.0.0.2:8080",
        )]);
        assert_eq!(
            extract_endpoint_from_response(&resp),
            Some("10.0.0.1:8080".to_string()),
        );
    }

    #[test]
    fn extract_endpoint_missing_header() {
        let resp = make_request_headers_response(vec![("x-other", "value")]);
        assert_eq!(extract_endpoint_from_response(&resp), None);
    }

    #[test]
    fn extract_endpoint_no_response() {
        let resp = ProcessingResponse {
            response: None,
            ..Default::default()
        };
        assert_eq!(extract_endpoint_from_response(&resp), None);
    }

    #[test]
    fn extract_endpoint_from_response_headers_type_returns_none() {
        // ResponseHeaders type should not return an endpoint
        let resp = ProcessingResponse {
            response: Some(Response::ResponseHeaders(HeadersResponse {
                response: Some(CommonResponse {
                    header_mutation: make_mutation(vec![(
                        "x-gateway-destination-endpoint",
                        "10.0.0.1:8080",
                    )]),
                    ..Default::default()
                }),
                ..Default::default()
            })),
            ..Default::default()
        };
        assert_eq!(extract_endpoint_from_response(&resp), None);
    }

    // --- build_response_headers_msg ---

    #[test]
    fn build_response_headers_msg_contains_metadata() {
        use envoy::service::ext_proc::v3::processing_request::Request;

        let msg = build_response_headers_msg("10.0.0.1:8080");

        // Should be a ResponseHeaders request
        match &msg.request {
            Some(Request::ResponseHeaders(h)) => {
                assert!(h.end_of_stream);
            }
            other => panic!("expected ResponseHeaders, got {:?}", other),
        }

        // Should have metadata_context with envoy.lb filter
        let metadata = msg
            .metadata_context
            .as_ref()
            .expect("missing metadata_context");
        let lb = metadata
            .filter_metadata
            .get("envoy.lb")
            .expect("missing envoy.lb");

        let endpoint_val = lb
            .fields
            .get("x-gateway-destination-endpoint-served")
            .expect("missing served endpoint field");

        use envoy_types::pb::google::protobuf::value::Kind;
        match &endpoint_val.kind {
            Some(Kind::StringValue(s)) => assert_eq!(s, "10.0.0.1:8080"),
            other => panic!("expected StringValue, got {:?}", other),
        }
    }

    // --- gRPC framing ---

    #[test]
    fn encode_decode_grpc_roundtrip() {
        let msg = build_response_headers_msg("10.0.0.1:8080");
        let encoded = encode_grpc_messages(std::slice::from_ref(&msg));
        let decoded: Vec<ProcessingRequest> = decode_grpc_messages(&encoded);
        assert_eq!(decoded.len(), 1);
        assert!(decoded[0].metadata_context.is_some());
    }

    #[test]
    fn encode_multiple_messages() {
        let msg1 = build_response_headers_msg("10.0.0.1:8080");
        let msg2 = build_response_headers_msg("10.0.0.2:8080");
        let encoded = encode_grpc_messages(&[msg1, msg2]);
        let decoded: Vec<ProcessingRequest> = decode_grpc_messages(&encoded);
        assert_eq!(decoded.len(), 2);
    }

    #[test]
    fn decode_empty_data() {
        let decoded: Vec<ProcessingRequest> = decode_grpc_messages(&[]);
        assert!(decoded.is_empty());
    }

    #[test]
    fn decode_truncated_data() {
        // Only header, no body
        let decoded: Vec<ProcessingRequest> = decode_grpc_messages(&[0, 0, 0, 0, 5]);
        assert!(decoded.is_empty());
    }

    #[test]
    fn try_decode_grpc_message_partial() {
        let msg = build_response_headers_msg("10.0.0.1:8080");
        let encoded = encode_grpc_messages(&[msg]);
        // Give it only part of the data
        let mut partial = BytesMut::from(&encoded[..3]);
        assert!(try_decode_grpc_message::<ProcessingRequest>(&mut partial).is_none());
    }

    #[test]
    fn try_decode_grpc_message_full() {
        let msg = build_response_headers_msg("10.0.0.1:8080");
        let encoded = encode_grpc_messages(&[msg]);
        let mut buf = BytesMut::from(encoded.as_ref());
        let decoded = try_decode_grpc_message::<ProcessingRequest>(&mut buf);
        assert!(decoded.is_some());
        assert!(buf.is_empty()); // consumed
    }

    #[test]
    fn build_request_messages_headers_only() {
        let headers = vec![
            (":method".to_string(), "POST".to_string()),
            (":path".to_string(), "/v1/chat".to_string()),
        ];
        let msgs = build_request_messages(&headers, None);
        assert_eq!(msgs.len(), 1);
    }

    #[test]
    fn build_request_messages_with_body() {
        let headers = vec![(":method".to_string(), "POST".to_string())];
        let msgs = build_request_messages(&headers, Some(b"{\"model\": \"llama3\"}".to_vec()));
        assert_eq!(msgs.len(), 2);
    }

    #[test]
    fn build_request_messages_empty_body() {
        let headers = vec![(":method".to_string(), "GET".to_string())];
        let msgs = build_request_messages(&headers, Some(b"".to_vec()));
        assert_eq!(msgs.len(), 1); // empty body is skipped
    }

    // --- build_request_messages: end_of_stream flag verification ---

    #[test]
    fn build_request_messages_headers_only_end_of_stream_true() {
        use envoy::service::ext_proc::v3::processing_request::Request;

        let headers = vec![(":method".to_string(), "GET".to_string())];
        let msgs = build_request_messages(&headers, None);
        assert_eq!(msgs.len(), 1);
        match &msgs[0].request {
            Some(Request::RequestHeaders(h)) => assert!(h.end_of_stream),
            other => panic!("expected RequestHeaders, got {:?}", other),
        }
    }

    #[test]
    fn build_request_messages_with_body_end_of_stream_false() {
        use envoy::service::ext_proc::v3::processing_request::Request;

        let headers = vec![(":method".to_string(), "POST".to_string())];
        let msgs = build_request_messages(&headers, Some(b"{\"model\": \"llama3\"}".to_vec()));
        assert_eq!(msgs.len(), 2);
        match &msgs[0].request {
            Some(Request::RequestHeaders(h)) => assert!(!h.end_of_stream),
            other => panic!("expected RequestHeaders, got {:?}", other),
        }
        match &msgs[1].request {
            Some(Request::RequestBody(b)) => assert!(b.end_of_stream),
            other => panic!("expected RequestBody, got {:?}", other),
        }
    }

    #[test]
    fn build_request_messages_body_content_correct() {
        use envoy::service::ext_proc::v3::processing_request::Request;

        let body_bytes = b"{\"model\": \"test-model\"}";
        let msgs = build_request_messages(
            &[("host".to_string(), "example.com".to_string())],
            Some(body_bytes.to_vec()),
        );
        assert_eq!(msgs.len(), 2);
        match &msgs[1].request {
            Some(Request::RequestBody(b)) => {
                assert_eq!(b.body, body_bytes.to_vec());
            }
            other => panic!("expected RequestBody, got {:?}", other),
        }
    }

    #[test]
    fn build_request_messages_header_values_correct() {
        use envoy::service::ext_proc::v3::processing_request::Request;

        let headers = vec![
            (":method".to_string(), "POST".to_string()),
            ("content-type".to_string(), "application/json".to_string()),
        ];
        let msgs = build_request_messages(&headers, None);
        match &msgs[0].request {
            Some(Request::RequestHeaders(h)) => {
                let hmap = h.headers.as_ref().expect("should have headers");
                assert_eq!(hmap.headers.len(), 2);
                assert_eq!(hmap.headers[0].key, ":method");
                assert_eq!(hmap.headers[0].value, "POST");
                assert_eq!(hmap.headers[1].key, "content-type");
                assert_eq!(hmap.headers[1].value, "application/json");
            }
            other => panic!("expected RequestHeaders, got {:?}", other),
        }
    }

    // --- resolve_header_value ---

    #[test]
    fn resolve_header_value_prefers_raw_value() {
        let hv = HeaderValue {
            key: "test".to_string(),
            value: "string-val".to_string(),
            raw_value: b"raw-val".to_vec(),
        };
        assert_eq!(resolve_header_value(&hv), Some("raw-val".to_string()));
    }

    #[test]
    fn resolve_header_value_falls_back_to_value() {
        let hv = HeaderValue {
            key: "test".to_string(),
            value: "string-val".to_string(),
            raw_value: Vec::new(),
        };
        assert_eq!(resolve_header_value(&hv), Some("string-val".to_string()));
    }

    #[test]
    fn resolve_header_value_empty_both_returns_none() {
        let hv = HeaderValue {
            key: "test".to_string(),
            value: String::new(),
            raw_value: Vec::new(),
        };
        assert_eq!(resolve_header_value(&hv), None);
    }

    #[test]
    fn resolve_header_value_raw_value_lossy_utf8() {
        let hv = HeaderValue {
            key: "test".to_string(),
            value: String::new(),
            raw_value: vec![0xff, 0xfe, 0x41], // invalid UTF-8 + 'A'
        };
        let result = resolve_header_value(&hv);
        assert!(result.is_some());
        assert!(result.unwrap().contains('A'));
    }

    // --- encode_grpc_message (single message framing) ---

    #[test]
    fn encode_grpc_message_framing_valid() {
        let msg = build_response_headers_msg("10.0.0.1:8080");
        let encoded = encode_grpc_message(&msg);

        // gRPC frame: 1 byte compressed flag + 4 bytes length + payload
        assert!(encoded.len() >= 5);
        assert_eq!(encoded[0], 0); // not compressed

        let len = u32::from_be_bytes([encoded[1], encoded[2], encoded[3], encoded[4]]) as usize;
        assert_eq!(len, encoded.len() - 5);

        // Payload should be decodable as the original message type
        let decoded = ProcessingRequest::decode(&encoded[5..]).expect("should decode");
        assert!(decoded.metadata_context.is_some());
    }

    // --- EppRequestResult ---

    #[test]
    fn epp_request_result_no_endpoint() {
        let result = EppRequestResult {
            selected_endpoint: None,
            stream: None,
        };
        assert!(result.selected_endpoint.is_none());
        assert!(result.stream.is_none());
    }

    #[test]
    fn epp_request_result_with_endpoint() {
        let result = EppRequestResult {
            selected_endpoint: Some("10.0.0.1:8080".to_string()),
            stream: None, // stream requires live h2 connection, so None in unit test
        };
        assert_eq!(result.selected_endpoint.as_deref(), Some("10.0.0.1:8080"));
    }

    // --- ResolvedAddr ---

    #[test]
    fn resolve_address_localhost_v4() {
        let resolved = resolve_address("127.0.0.1", 8080).expect("should resolve localhost");
        assert!(!resolved.addr.sockaddr.is_null());
        assert_eq!(
            resolved.addr.socklen as usize,
            core::mem::size_of::<libc::sockaddr_in>()
        );
    }

    #[test]
    fn resolve_address_localhost_v6() {
        let resolved = resolve_address("::1", 8080).expect("should resolve IPv6 localhost");
        assert!(!resolved.addr.sockaddr.is_null());
        assert_eq!(
            resolved.addr.socklen as usize,
            core::mem::size_of::<libc::sockaddr_in6>()
        );
    }

    #[test]
    fn resolve_address_invalid_host() {
        let result = resolve_address("this.host.definitely.does.not.exist.invalid", 80);
        assert!(result.is_err());
    }

    #[test]
    fn resolve_address_port_preserved() {
        let resolved = resolve_address("127.0.0.1", 9999).expect("should resolve");
        let sin = unsafe { &*(resolved.addr.sockaddr as *const libc::sockaddr_in) };
        assert_eq!(u16::from_be(sin.sin_port), 9999);
    }

    // --- resolve_address: Drop safety ---

    #[test]
    fn resolved_addr_v4_drops_cleanly() {
        let resolved = resolve_address("127.0.0.1", 80).expect("should resolve");
        assert!(!resolved.addr.sockaddr.is_null());
        drop(resolved);
        // No use-after-free or double-free
    }

    #[test]
    fn resolved_addr_v6_drops_cleanly() {
        let resolved = resolve_address("::1", 80).expect("should resolve");
        assert!(!resolved.addr.sockaddr.is_null());
        drop(resolved);
    }

    // --- set_sockaddr_port ---

    #[test]
    fn set_sockaddr_port_v4() {
        let mut sin: libc::sockaddr_in = unsafe { mem::zeroed() };
        sin.sin_family = libc::AF_INET as libc::sa_family_t;
        sin.sin_port = 0;
        let ptr = &raw mut sin as *mut nginx_sys::sockaddr;
        set_sockaddr_port(ptr, mem::size_of::<libc::sockaddr_in>() as _, 9999);
        assert_eq!(u16::from_be(sin.sin_port), 9999);
    }

    #[test]
    fn set_sockaddr_port_v6() {
        let mut sin6: libc::sockaddr_in6 = unsafe { mem::zeroed() };
        sin6.sin6_family = libc::AF_INET6 as libc::sa_family_t;
        sin6.sin6_port = 0;
        let ptr = &raw mut sin6 as *mut nginx_sys::sockaddr;
        set_sockaddr_port(ptr, mem::size_of::<libc::sockaddr_in6>() as _, 8443);
        assert_eq!(u16::from_be(sin6.sin6_port), 8443);
    }

    #[test]
    fn set_sockaddr_port_null_is_noop() {
        // Should not panic
        set_sockaddr_port(core::ptr::null_mut(), 0, 80);
    }

    // --- ResolverParams ---

    #[test]
    fn resolver_params_none_is_copy() {
        let params = ResolverParams {
            resolver: None,
            timeout: 0,
        };
        let copy = params;
        assert!(copy.resolver.is_none());
    }

    // --- normalize_endpoint: additional edge cases ---

    #[test]
    fn normalize_endpoint_with_path() {
        assert_eq!(
            normalize_endpoint("epp:9002/some/path", false),
            "http://epp:9002/some/path"
        );
    }

    #[test]
    fn normalize_endpoint_empty_string() {
        assert_eq!(normalize_endpoint("", false), "http://");
        assert_eq!(normalize_endpoint("", true), "https://");
    }

    // --- try_decode_grpc_message: multiple messages ---

    #[test]
    fn try_decode_grpc_message_multiple_in_buffer() {
        let msg1 = build_response_headers_msg("10.0.0.1:8080");
        let msg2 = build_response_headers_msg("10.0.0.2:8080");
        let encoded = encode_grpc_messages(&[msg1, msg2]);
        let mut buf = BytesMut::from(encoded.as_ref());

        let decoded1 = try_decode_grpc_message::<ProcessingRequest>(&mut buf);
        assert!(decoded1.is_some());
        assert!(!buf.is_empty()); // second message still in buffer

        let decoded2 = try_decode_grpc_message::<ProcessingRequest>(&mut buf);
        assert!(decoded2.is_some());
        assert!(buf.is_empty()); // all consumed
    }

    #[test]
    fn try_decode_grpc_message_empty_buffer() {
        let mut buf = BytesMut::new();
        assert!(try_decode_grpc_message::<ProcessingRequest>(&mut buf).is_none());
    }

    #[test]
    fn try_decode_grpc_message_header_only_no_body() {
        // 5-byte header says 100 bytes but buffer only has the header
        let mut buf = BytesMut::from(&[0u8, 0, 0, 0, 100][..]);
        assert!(try_decode_grpc_message::<ProcessingRequest>(&mut buf).is_none());
        assert_eq!(buf.len(), 5); // buffer not consumed
    }

    // --- build_response_headers_msg: edge cases ---

    #[test]
    fn build_response_headers_msg_empty_endpoint() {
        let msg = build_response_headers_msg("");
        let metadata = msg.metadata_context.as_ref().unwrap();
        let lb = metadata.filter_metadata.get("envoy.lb").unwrap();
        let val = lb
            .fields
            .get("x-gateway-destination-endpoint-served")
            .unwrap();
        use envoy_types::pb::google::protobuf::value::Kind;
        match &val.kind {
            Some(Kind::StringValue(s)) => assert_eq!(s, ""),
            other => panic!("expected empty StringValue, got {:?}", other),
        }
    }

    // --- encode_grpc_message: zero-length message ---

    #[test]
    fn encode_grpc_message_minimal() {
        // A default ProcessingRequest with no fields set
        let msg = ProcessingRequest {
            request: None,
            metadata_context: None,
            attributes: HashMap::new(),
            observability_mode: false,
            protocol_config: None,
        };
        let encoded = encode_grpc_message(&msg);
        // Should have 5-byte header + 0 bytes payload (empty message)
        assert_eq!(encoded[0], 0); // no compression
        let len = u32::from_be_bytes([encoded[1], encoded[2], encoded[3], encoded[4]]);
        assert_eq!(len as usize, encoded.len() - 5);
    }

    // --- extract_endpoint_from_response: comma-separated edge cases ---

    #[test]
    fn extract_endpoint_single_trailing_comma() {
        let resp = make_request_headers_response(vec![(
            "x-gateway-destination-endpoint",
            "10.0.0.1:8080,",
        )]);
        assert_eq!(
            extract_endpoint_from_response(&resp),
            Some("10.0.0.1:8080".to_string()),
        );
    }

    #[test]
    fn extract_endpoint_whitespace_trimmed() {
        let resp = make_request_headers_response(vec![(
            "x-gateway-destination-endpoint",
            "  10.0.0.1:8080  , 10.0.0.2:8080  ",
        )]);
        assert_eq!(
            extract_endpoint_from_response(&resp),
            Some("10.0.0.1:8080".to_string()),
        );
    }
}
