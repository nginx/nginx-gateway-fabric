//! PRECONTENT-phase handler for the inference module.
//!
//! Implements a three-state machine:
//!
//! 1. First entry: allocate per-request context, trigger body read, return `NGX_DONE`.
//! 2. Waiting: the async EPP task is in flight, return `NGX_AGAIN`.
//! 3. Done: finalize the result — set `$inference_endpoint` and return `NGX_DECLINED`
//!    (success / fail-open), or an HTTP error status (fail-close).

use core::cell::{Cell, RefCell};
use core::ffi::c_void;
use core::pin::Pin;
use core::ptr::{self, NonNull};
use core::task::{Context, Poll, Waker};
use std::collections::VecDeque;
use std::net::SocketAddr;
use std::rc::Rc;

use bytes::Bytes;
use nginx_sys::{
    ngx_http_cleanup_add, ngx_http_finalize_request, ngx_http_read_client_request_body,
    ngx_http_request_t, ngx_int_t, ngx_log_t, ngx_post_event, ngx_posted_events,
};
use ngx::core::{Pool, Status};
use ngx::ffi::{
    NGX_HTTP_INTERNAL_SERVER_ERROR, NGX_HTTP_SPECIAL_RESPONSE, NGX_LOG_ERR, NGX_LOG_INFO,
    NGX_LOG_WARN, ngx_chain_t, ngx_http_output_body_filter_pt, ngx_http_output_header_filter_pt,
    ngx_http_top_body_filter, ngx_http_top_header_filter, ngx_list_t, ngx_str_t, ngx_table_elt_t,
};
use ngx::http::{HttpModule, HttpModuleLocationConf};
use ngx::ngx_log_error;

use crate::codec::mutations::{self, MutationSet};
use crate::codec::request::{Header, PseudoHeaders};
use crate::codec::response::EppResult;
use crate::codec::{notify, request};
use crate::config::types::{LocationConfig, ResponsePhase};
use crate::error::ModuleError;
use crate::net::peer_conn::PeerConnection;
use crate::stream::client::{self, BodyPoll, EppStream};
use crate::stream::timeout::with_timeout;

/// h2 connection over the EPP peer connection.
type EppConnection = h2::client::Connection<Pin<Box<PeerConnection>>, Bytes>;

/// Saved next header filter in the chain.
static mut NEXT_HEADER_FILTER: ngx_http_output_header_filter_pt = None;

/// Saved next body filter in the chain.
static mut NEXT_BODY_FILTER: ngx_http_output_body_filter_pt = None;

// Headers mode fails the request rather than buffering without a bound while
// EPP is pending. Bound metadata as well as payload (including empty markers).
const MAX_RESPONSE_BYTES: usize = 1024 * 1024;
const MAX_RESPONSE_CHUNKS: usize = 1024;

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
struct ResponseFlags {
    flush: bool,
    sync: bool,
    last_buf: bool,
    last_in_chain: bool,
}

struct ResponseChunk {
    data: Vec<u8>,
    flags: ResponseFlags,
}

#[derive(Default)]
struct ResponseBuffer {
    chunks: Vec<ResponseChunk>,
    bytes: usize,
}

impl ResponseBuffer {
    fn push(&mut self, data: &[u8], flags: ResponseFlags) -> Result<(), ()> {
        if data.is_empty() && flags == ResponseFlags::default() {
            return Ok(());
        }
        if data.len() > MAX_RESPONSE_BYTES - self.bytes || self.chunks.len() == MAX_RESPONSE_CHUNKS
        {
            return Err(());
        }
        let mut copy = Vec::new();
        copy.try_reserve_exact(data.len()).map_err(|_| ())?;
        self.chunks.try_reserve_exact(1).map_err(|_| ())?;
        copy.extend_from_slice(data);
        self.bytes += data.len();
        self.chunks.push(ResponseChunk { data: copy, flags });
        Ok(())
    }
}

/// Per-request context for the inference module.
///
/// Box-allocated and stored in the module's `ctx` slot on the request.
///
/// Two cleanup handlers manage its lifetime:
/// - **HTTP cleanup** (`ngx_http_cleanup_add`): cancels the async task early,
///   before logging. The Box is *not* freed here—only the task is taken and dropped.
/// - **Pool cleanup** (`ngx_pool_cleanup_add`): drops the Box after access logging,
///   when the request pool is destroyed. This keeps `$inference_endpoint` alive
///   through log-phase variable evaluation.
struct RequestCtx {
    /// The spawned async task. Dropping it cancels the future.
    task: Option<ngx::async_::Task<()>>,
    /// Whether the EPP (precontent-phase) async task has completed.
    done: bool,
    /// The EPP call result, written by the async task.
    result: Option<Result<EppResult, ModuleError>>,
    /// The primary endpoint string for `$inference_endpoint`.
    endpoint: Option<String>,
    /// Set by the async task when an EPP stream is open and awaiting the
    /// response phase. The header filter only engages when this is set.
    response_pending: bool,
    /// Shared signal for the detached response reporter task (Notify mode).
    /// The header filter fires this to tell the detached task "upstream
    /// responded, send response-phase messages now." `None` until the
    /// detached task is spawned; the header filter falls back to
    /// `notify_waker` when this is `None` (Headers mode).
    notify_signal: Option<Rc<NotifySignal>>,
    /// Signal from the header filter to the async task: "upstream responded,
    /// send response-phase messages now." Used only in Headers mode, where
    /// the response work stays in the original (cancellable) task.
    notify_ready: bool,
    /// Waker stored by the async task while it waits for `notify_ready`.
    /// Used only in Headers mode.
    notify_waker: Option<Waker>,
    /// The configured response phase for this request.
    response_phase: ResponsePhase,
    /// True when the header filter suppressed the downstream chain (headers mode).
    headers_suppressed: bool,
    /// True when the async task has finished the EPP response phase and
    /// applied any header mutations. The resume handler checks this to
    /// commit headers + flush the buffered body.
    response_phase_done: bool,
    /// Bounded payload and original output markers while headers are suppressed.
    buffered_body: ResponseBuffer,
    /// Owns one main-request reference until output resumes or nginx terminates.
    response_held: bool,
    /// Disable response-task effects after a buffering failure.
    response_failed: bool,
    /// Body channel sender for streaming upstream response data to the EPP.
    /// Created when the EPP stream is opened; the body filter pushes chunks.
    body_sender: Option<BodySender>,
    /// Stable in the boxed context; removed from the queue during HTTP cleanup.
    response_event: nginx_sys::ngx_event_t,
}

impl RequestCtx {
    fn new(response_phase: ResponsePhase) -> Self {
        Self {
            task: None,
            done: false,
            result: None,
            endpoint: None,
            response_pending: false,
            notify_signal: None,
            notify_ready: false,
            notify_waker: None,
            response_phase,
            headers_suppressed: false,
            response_phase_done: false,
            buffered_body: ResponseBuffer::default(),
            response_held: false,
            response_failed: false,
            body_sender: None,
            response_event: unsafe { core::mem::zeroed() },
        }
    }
}

/// The request connection's log, or null if the connection is gone.
unsafe fn request_log(r: *mut ngx_http_request_t) -> *mut ngx_log_t {
    unsafe { (*r).connection.as_ref().map_or(ptr::null_mut(), |c| c.log) }
}

/// Get or allocate the per-request context.
///
/// On first call, allocates a `Box<RequestCtx>`, stores it in the module's ctx slot,
/// and registers two cleanup handlers:
/// 1. HTTP cleanup — cancels the async task before access logging.
/// 2. Pool cleanup — drops the Box after access logging (when the pool is destroyed).
///
/// The handler runs in the PRECONTENT phase (after ACCESS/auth, before CONTENT).
///
/// Returns null if cleanup registration fails, in which case the Box is freed immediately.
unsafe fn get_or_create_ctx(
    r: *mut ngx_http_request_t,
    response_phase: ResponsePhase,
) -> *mut RequestCtx {
    let module = crate::Module::module();
    let slot = unsafe { (*r).ctx.add(module.ctx_index) };
    let existing = unsafe { *slot };
    if !existing.is_null() {
        return existing as *mut RequestCtx;
    }

    let mut boxed = Box::new(RequestCtx::new(response_phase));
    // The event lives inside the Box, so its address is stable from here on.
    boxed.response_event.data = r.cast();
    boxed.response_event.log = unsafe { request_log(r) };
    boxed.response_event.handler = Some(inference_response_event);
    let raw = Box::into_raw(boxed);

    // Pool cleanup: drops the Box after access logging, when the request pool
    // is destroyed. This keeps the endpoint string alive for $inference_endpoint.
    let pool_cln = unsafe { nginx_sys::ngx_pool_cleanup_add((*r).pool, 0) };
    if pool_cln.is_null() {
        // Cannot register cleanup — free immediately to avoid leak.
        drop(unsafe { Box::from_raw(raw) });
        return ptr::null_mut();
    }
    unsafe {
        (*pool_cln).handler = Some(request_ctx_pool_cleanup);
        (*pool_cln).data = raw as *mut c_void;
    }

    // HTTP cleanup: cancels the async task before access logging runs.
    // If this fails the task will still be cancelled at pool destruction,
    // just later than ideal.
    let http_cln = unsafe { ngx_http_cleanup_add(r, 0) };
    if !http_cln.is_null() {
        unsafe {
            (*http_cln).handler = Some(request_ctx_cancel_task);
            (*http_cln).data = raw as *mut c_void;
        }
    }

    unsafe { *slot = raw as *mut c_void };
    raw
}

/// Get the existing per-request context, if any, without inventing a lifetime.
///
/// The pointer is valid only until pool cleanup. Callers must keep as_ref/as_mut
/// borrows short: never retain a context or request borrow across nginx callbacks,
/// task polling/cancellation, waker clone/wake/drop, or request finalization.
unsafe fn get_ctx(r: *mut ngx_http_request_t) -> Option<NonNull<RequestCtx>> {
    let module = crate::Module::module();
    let slot = unsafe { *(*r).ctx.add(module.ctx_index) };
    NonNull::new(slot.cast())
}

/// HTTP cleanup handler — cancels the async task before access logging.
///
/// Takes (and drops) only the task; the Box itself stays alive until pool cleanup.
unsafe extern "C" fn request_ctx_cancel_task(data: *mut c_void) {
    if !data.is_null() {
        let task = {
            let ctx = unsafe { &mut *(data as *mut RequestCtx) };
            ctx.response_failed = true;
            if ctx.response_event.posted() != 0 {
                unsafe { nginx_sys::ngx_delete_posted_event(&mut ctx.response_event) };
            }
            ctx.task.take()
        };
        // Cancellation can execute future destructors; do not retain a ctx borrow.
        drop(task);
    }
}

/// Pool cleanup handler — drops the `Box<RequestCtx>` after access logging.
///
/// At this point `$inference_endpoint` has already been evaluated, so the
/// endpoint string can be freed safely.
unsafe extern "C" fn request_ctx_pool_cleanup(data: *mut c_void) {
    if !data.is_null() {
        unsafe { request_ctx_cancel_task(data) };
        drop(unsafe { Box::from_raw(data as *mut RequestCtx) });
    }
}

// ---------------------------------------------------------------------------
// PRECONTENT-phase handler
// ---------------------------------------------------------------------------

/// PRECONTENT-phase handler for the inference module.
///
/// Three-state machine:
/// 1. No ctx → allocate, trigger body read, return `NGX_DONE`.
/// 2. Ctx not done → return `NGX_AGAIN`.
/// 3. Ctx done → finalize (set endpoint / fail-open / fail-close).
pub(crate) unsafe extern "C" fn inference_precontent_handler(
    r: *mut ngx_http_request_t,
) -> ngx_int_t {
    // Skip subrequests — only process the main request.
    if unsafe { r != (*r).main } {
        return Status::NGX_DECLINED.into();
    }

    // Get location config. If the module isn't enabled, decline.
    let conf = unsafe { crate::Module::location_conf(&*r) };
    let Some(conf) = conf else {
        return Status::NGX_DECLINED.into();
    };
    if !conf.enabled() {
        return Status::NGX_DECLINED.into();
    }

    // Check for existing context (re-entry after body read or async completion).
    if let Some(ctx) = unsafe { get_ctx(r) } {
        if unsafe { ctx.as_ref().done } {
            // The async task has completed — finalize the result.
            return unsafe { finalize_result(r, ctx, conf) };
        }
        // Still waiting for the async task.
        return Status::NGX_AGAIN.into();
    }

    // First entry: allocate context and trigger body read.
    let ctx = unsafe { get_or_create_ctx(r, conf.response_phase()) };
    if ctx.is_null() {
        return NGX_HTTP_INTERNAL_SERVER_ERROR as ngx_int_t;
    }

    // Request a single contiguous buffer for the body.
    unsafe { (*r).set_request_body_in_single_buf(1) };

    // Trigger body read. This increments r->main->count internally.
    // It may invoke body_read_handler synchronously; no request/ctx borrow
    // spans this call. The callback posts continuation rather than finalizing.
    let rc = unsafe { ngx_http_read_client_request_body(r, Some(body_read_handler)) };
    if rc == Status::NGX_ERROR.into() || rc >= NGX_HTTP_SPECIAL_RESPONSE as ngx_int_t {
        return rc;
    }

    // Release the count that the phase engine holds on the request.
    // The body read handler (or its async continuation) will resume phases.
    unsafe { ngx_http_finalize_request(r, Status::NGX_DONE.into()) };
    Status::NGX_DONE.into()
}

// ---------------------------------------------------------------------------
// Body-read callback
// ---------------------------------------------------------------------------

/// Called when the request body has been fully read.
///
/// Collects headers and body, builds the gRPC frames, and spawns the async EPP
/// task. The task keeps the h2 connection and stream alive through the
/// response phase.
unsafe extern "C" fn body_read_handler(r: *mut ngx_http_request_t) {
    let conf = unsafe { crate::Module::location_conf(&*r) };
    let Some(conf) = conf else {
        unsafe { ngx_http_finalize_request(r, NGX_HTTP_INTERNAL_SERVER_ERROR as ngx_int_t) };
        return;
    };
    let log = unsafe { request_log(r) };

    // Collection and both encoders share the existing failure/resume path.
    let frames = unsafe { collect_body(r).and_then(|body| encode_request_frames(r, body)) };
    let (headers_frame, body_frames) = match frames {
        Ok(frames) => frames,
        Err(err) => {
            if let Some(mut ctx) = unsafe { get_ctx(r) } {
                let ctx = unsafe { ctx.as_mut() };
                ctx.result = Some(Err(err));
                ctx.done = true;
            }
            unsafe { resume_phases(r) };
            return;
        }
    };

    // Location config lives for the configuration cycle, which outlives the
    // request; the task is cancelled by request cleanup, so borrow it directly.

    // spawn can poll inline. Capture owned frames/config and raw nginx handles,
    // never a request or context reference; continuation remains event-posted.
    let task = ngx::async_::spawn(async move {
        let result = run_epp_call(log, conf, headers_frame, body_frames).await;

        // Extract the endpoint address from the EPP result before storing
        // it in ctx. In Notify mode the detached task needs the endpoint
        // string, but ctx.endpoint is only set later by finalize_result
        // (after resume_phases), which may run after HTTP cleanup cancels
        // this task. So we grab it here while we still have the result.
        let (mut stream, mut conn, endpoint_for_report) = match result {
            Ok((epp_result, stream, conn)) => {
                let endpoint_addr = epp_result.endpoints.first().map(|ep| ep.address.clone());
                let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
                    return;
                };
                let ctx = unsafe { ctx.as_mut() };
                ctx.result = Some(Ok(epp_result));
                ctx.response_pending = true;
                ctx.done = true;
                (stream, conn, endpoint_addr)
            }
            Err(err) => {
                if let Some(mut ctx) = unsafe { get_ctx(r) } {
                    let ctx = unsafe { ctx.as_mut() };
                    ctx.result = Some(Err(err));
                    ctx.done = true;
                }
                unsafe { resume_phases(r) };
                return;
            }
        };

        let phase = conf.response_phase();
        let timeout = conf.timeout();

        if phase == ResponsePhase::Notify {
            // --- Notify mode: detach the response reporter ---
            //
            // Extract all needed data NOW, before resume_phases(r), because
            // once phases resume the upstream can complete and HTTP cleanup
            // can fire at any point — cancelling this task. The detached
            // task uses a shared NotifySignal (Rc) that the header filter
            // fires when the upstream responds. It never touches `r`.
            let signal = Rc::new(NotifySignal::new());
            let (body_tx, body_rx) = body_channel();
            {
                let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
                    return;
                };
                let ctx = unsafe { ctx.as_mut() };
                ctx.notify_signal = Some(Rc::clone(&signal));
                ctx.body_sender = Some(body_tx);
            }
            // Resume phases AFTER setting up the signal, so the header
            // filter can find it if the upstream responds synchronously.
            unsafe { resume_phases(r) };

            // The detached task is NOT stored in ctx.task — it runs to
            // completion on the worker event loop, cancelled only on
            // worker shutdown (global task scheduler drop).
            ngx::async_::spawn(response_reporter(
                log,
                endpoint_for_report,
                stream,
                conn,
                timeout,
                signal,
                body_rx,
            ))
            .detach();
            // The original task returns. HTTP cleanup cancels an
            // already-finished task (no-op).
            return;
        }

        // --- Headers mode: response work stays in the original task ---
        //
        // The response IS held (headers_suppressed), so the request won't
        // finalize until this task unblocks it. No cancellation race.
        let (body_tx, body_rx) = body_channel();
        {
            let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
                return;
            };
            unsafe { ctx.as_mut().body_sender = Some(body_tx) };
        }
        unsafe { resume_phases(r) };

        // Encode while the endpoint is borrowed; no await happens in this scope.
        let headers = {
            let Some(ctx) = (unsafe { get_ctx(r) }) else {
                return;
            };
            let ctx = unsafe { ctx.as_ref() };
            if ctx.response_failed {
                return;
            }
            ctx.endpoint.as_deref().map(notify::encode_response_headers)
        };
        if let Some(headers) = headers {
            // Encoding failures must also cancel/log and unblock suppressed
            // output below, not return early and strand the response hold.
            let phase_fut = async {
                let headers = Bytes::from(headers?);
                client::send_response_phase(&mut stream, &mut conn, headers, &body_rx).await
            };
            match with_timeout(timeout, phase_fut).await.and_then(|r| r) {
                Ok(resp_mutations) => unsafe {
                    if get_ctx(r).is_none_or(|ctx| ctx.as_ref().response_failed) {
                        return;
                    }
                    if let Err(err) = apply_header_mutations(r, true, &resp_mutations) {
                        ngx_log_error!(
                            NGX_LOG_WARN,
                            log,
                            "inference: response header mutations not applied: {}",
                            err
                        );
                    }
                },
                Err(e) => {
                    stream.cancel();
                    ngx_log_error!(
                        NGX_LOG_WARN,
                        log,
                        "inference: EPP response phase failed: {}",
                        e
                    );
                }
            }
        }

        {
            let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
                return;
            };
            let ctx = unsafe { ctx.as_mut() };
            if ctx.response_failed {
                return;
            }
            ctx.response_phase_done = true;
        }
        // Output must run outside the async task's poll stack.
        unsafe { resume_response_output(r) };
    });

    if let Some(mut ctx) = unsafe { get_ctx(r) } {
        let previous = unsafe { ctx.as_mut().task.replace(task) };
        drop(previous);
    }
}

// ---------------------------------------------------------------------------
// Async EPP call
// ---------------------------------------------------------------------------

/// Run the request phase of the EPP call: parse endpoint → connect →
/// optional TLS → h2 → ext_proc request, bounded by the configured timeout.
///
/// Returns the EPP result plus the open stream and h2 connection, which the
/// caller keeps alive through the response phase.
async fn run_epp_call(
    log: *mut ngx_log_t,
    conf: &LocationConfig,
    headers_frame: Vec<u8>,
    body_frames: Vec<Vec<u8>>,
) -> Result<(EppResult, EppStream, EppConnection), ModuleError> {
    let endpoint = conf.endpoint();
    let inner = async {
        let mut addr = parse_endpoint_addr(endpoint)?;

        let log_nn =
            NonNull::new(log).ok_or_else(|| ModuleError::Transport("null log pointer".into()))?;
        let mut pc = Box::pin(
            PeerConnection::new(log_nn)
                .map_err(|e| ModuleError::Transport(format!("peer connection init: {e}")))?,
        );

        pc.as_mut()
            .connect(&addr.as_ngx_addr())
            .await
            .map_err(|e| ModuleError::Transport(format!("connect to {endpoint}: {e}")))?;

        if let Some(ref ssl_ctx) = conf.ssl_ctx {
            ssl_handshake(pc.as_mut(), ssl_ctx).await?;
        }

        let (mut send_request, mut conn) = h2::client::handshake(pc)
            .await
            .map_err(|e| ModuleError::Transport(format!("h2 handshake: {e}")))?;

        let (result, stream) = client::epp_call(
            &mut send_request,
            &mut conn,
            endpoint,
            conf.ssl(),
            headers_frame,
            body_frames,
        )
        .await?;

        Ok((result, stream, conn))
    };

    with_timeout(conf.timeout(), inner).await?
}

/// Parse an `IP:port` endpoint string into a `ResolvedAddr`.
///
/// The controller emits the EPP Service's ClusterIP directly.
/// The endpoint must be a valid `IP:port` or `[IPv6]:port` string.
fn parse_endpoint_addr(endpoint: &str) -> Result<ResolvedAddr, ModuleError> {
    let sock_addr: SocketAddr = endpoint.parse().map_err(|e| {
        ModuleError::Transport(format!("invalid EPP endpoint address '{endpoint}': {e}"))
    })?;

    match sock_addr {
        SocketAddr::V4(v4) => {
            let mut sa: libc::sockaddr_in = unsafe { core::mem::zeroed() };
            sa.sin_family = libc::AF_INET as _;
            sa.sin_port = v4.port().to_be();
            sa.sin_addr.s_addr = u32::from_ne_bytes(v4.ip().octets());
            Ok(ResolvedAddr::V4(sa))
        }
        SocketAddr::V6(v6) => {
            let mut sa: libc::sockaddr_in6 = unsafe { core::mem::zeroed() };
            sa.sin6_family = libc::AF_INET6 as _;
            sa.sin6_port = v6.port().to_be();
            sa.sin6_addr.s6_addr = v6.ip().octets();
            Ok(ResolvedAddr::V6(sa))
        }
    }
}

/// Stack-allocated resolved address. Provides a temporary `ngx_addr_t` view.
enum ResolvedAddr {
    V4(libc::sockaddr_in),
    V6(libc::sockaddr_in6),
}

impl ResolvedAddr {
    /// Borrow as an `ngx_addr_t` pointing into this stack allocation.
    fn as_ngx_addr(&mut self) -> nginx_sys::ngx_addr_t {
        let (sockaddr, socklen) = match self {
            ResolvedAddr::V4(sa) => (ptr::from_mut(sa).cast(), size_of::<libc::sockaddr_in>()),
            ResolvedAddr::V6(sa) => (ptr::from_mut(sa).cast(), size_of::<libc::sockaddr_in6>()),
        };
        nginx_sys::ngx_addr_t {
            sockaddr,
            socklen: socklen as _,
            name: ngx_str_t::empty(),
        }
    }
}

/// Perform the TLS handshake on an established connection.
///
/// Uses the pre-built `SslContext` from the location config. The `SSL_CTX`
/// refcount is incremented by `ngx_ssl_create_connection` and decremented
/// when the connection is closed, so the shared context is safe to reuse.
async fn ssl_handshake(
    mut pc: Pin<&mut PeerConnection>,
    ssl_ctx: &crate::config::types::SslContext,
) -> Result<(), ModuleError> {
    let ssl = ssl_ctx.as_ngx_ssl();

    core::future::poll_fn(|cx| pc.as_mut().poll_ssl_handshake(&ssl, ssl_ctx.name(), cx))
        .await
        .map_err(|e| ModuleError::Transport(format!("TLS handshake: {e}")))
}

// ---------------------------------------------------------------------------
// Detached response reporter (Notify mode)
// ---------------------------------------------------------------------------

/// Shared one-shot signal between the header filter and the detached response
/// reporter task. Single-threaded (nginx worker), so `Cell` is sufficient.
struct NotifySignal {
    ready: Cell<bool>,
    waker: Cell<Option<Waker>>,
}

impl NotifySignal {
    fn new() -> Self {
        Self {
            ready: Cell::new(false),
            waker: Cell::new(None),
        }
    }

    /// Called by the header filter (via `inference_response_event`) to signal
    /// that the upstream has responded.
    fn fire(&self) {
        self.ready.set(true);
        if let Some(w) = self.waker.take() {
            w.wake();
        }
    }

    /// Poll until fired. Returns `Ready` when the upstream has responded.
    fn poll_ready(&self, cx: &mut core::task::Context<'_>) -> Poll<()> {
        if self.ready.get() {
            return Poll::Ready(());
        }
        self.waker.set(Some(cx.waker().clone()));
        Poll::Pending
    }
}

// ---------------------------------------------------------------------------
// Body channel: single-threaded SPSC queue for response body streaming
// ---------------------------------------------------------------------------

/// Bounded body-chunk channel for streaming upstream response data to the
/// detached EPP response task.
///
/// Both ends share an `Rc<RefCell<BodyChannelInner>>`. The body filter pushes
/// chunks (data + is-last flag); the async task polls them. The channel is
/// bounded by total bytes to prevent runaway buffering if the EPP is slow.
const BODY_CHANNEL_CAP: usize = 256 * 1024;

struct BodyChannelInner {
    queue: VecDeque<(Vec<u8>, bool)>,
    waker: Option<Waker>,
    total_bytes: usize,
    sender_alive: bool,
}

/// Sender half — stored in `RequestCtx`, used by the body filter.
struct BodySender(Rc<RefCell<BodyChannelInner>>);

/// Receiver half — moved into the async response task.
struct BodyReceiver(Rc<RefCell<BodyChannelInner>>);

fn body_channel() -> (BodySender, BodyReceiver) {
    let inner = Rc::new(RefCell::new(BodyChannelInner {
        queue: VecDeque::new(),
        waker: None,
        total_bytes: 0,
        sender_alive: true,
    }));
    (BodySender(Rc::clone(&inner)), BodyReceiver(inner))
}

impl BodySender {
    /// Push a body chunk. Returns `false` if the channel is at capacity
    /// (the chunk is dropped). `last` should be true for the final chunk.
    fn send(&self, data: Vec<u8>, last: bool) -> bool {
        let waker = {
            let mut inner = self.0.borrow_mut();
            if inner.total_bytes + data.len() > BODY_CHANNEL_CAP && !data.is_empty() && !last {
                // Drop non-final chunks that would exceed capacity.
                // Final chunks are always accepted so the EOS is never lost.
                return false;
            }
            inner.total_bytes += data.len();
            inner.queue.push_back((data, last));
            inner.waker.take()
        };
        // Wake AFTER releasing the borrow — wake() can immediately re-enter
        // the task's poll, which calls borrow_mut() on the same RefCell.
        if let Some(w) = waker {
            w.wake();
        }
        true
    }
}

impl Drop for BodySender {
    fn drop(&mut self) {
        let waker = {
            let mut inner = self.0.borrow_mut();
            inner.sender_alive = false;
            inner.waker.take()
        };
        // Wake AFTER releasing the borrow — see BodySender::send.
        if let Some(w) = waker {
            w.wake();
        }
    }
}

impl BodyReceiver {
    /// Poll for the next body chunk.
    ///
    /// Returns `Ready(Some((data, is_last)))` when a chunk is available,
    /// `Ready(None)` when the sender was dropped without sending a final chunk,
    /// and `Pending` while waiting.
    fn poll_recv(&self, cx: &mut Context<'_>) -> Poll<Option<(Vec<u8>, bool)>> {
        let mut inner = self.0.borrow_mut();
        if let Some(chunk) = inner.queue.pop_front() {
            inner.total_bytes -= chunk.0.len();
            Poll::Ready(Some(chunk))
        } else if !inner.sender_alive {
            Poll::Ready(None)
        } else {
            inner.waker = Some(cx.waker().clone());
            Poll::Pending
        }
    }
}

impl BodyPoll for BodyReceiver {
    fn poll_chunk(&self, cx: &mut Context<'_>) -> Poll<Option<(Vec<u8>, bool)>> {
        self.poll_recv(cx)
    }
}

/// Send the response-phase messages (ResponseHeaders + ResponseBody) to the
/// EPP, using only owned data that does not reference the nginx request.
///
/// Spawned as a detached task in Notify mode so that HTTP cleanup (which
/// cancels the original task) does not abort the response report. This is
/// critical for non-streaming responses where the upstream completes quickly.
///
/// The task runs on the worker event loop. It is only cancelled if the worker
/// shuts down (the global task scheduler drops all pending tasks).
async fn response_reporter(
    log: *mut ngx_log_t,
    endpoint: Option<String>,
    mut stream: EppStream,
    mut conn: EppConnection,
    timeout: core::time::Duration,
    signal: Rc<NotifySignal>,
    body_rx: BodyReceiver,
) {
    // Wait for the header filter to signal that the upstream responded.
    core::future::poll_fn(|cx| signal.poll_ready(cx)).await;

    let headers = match endpoint.as_deref().map(notify::encode_response_headers) {
        Some(Ok(h)) => h,
        Some(Err(e)) => {
            stream.cancel();
            ngx_log_error!(
                NGX_LOG_WARN,
                log,
                "inference: response headers encoding failed: {}",
                e
            );
            return;
        }
        None => return,
    };

    let phase_fut = async {
        let headers = Bytes::from(headers);
        client::send_response_phase(&mut stream, &mut conn, headers, &body_rx).await
    };
    match with_timeout(timeout, phase_fut).await.and_then(|r| r) {
        Ok(_) => {}
        Err(e) => {
            stream.cancel();
            ngx_log_error!(
                NGX_LOG_WARN,
                log,
                "inference: EPP response phase failed: {}",
                e
            );
        }
    }
}

// ---------------------------------------------------------------------------
// Resume and finalize
// ---------------------------------------------------------------------------

/// Post the connection's write event to resume the phase engine.
///
/// Deferred to the next event-loop iteration so we don't re-enter the phase
/// engine from inside the async task's continuation.
unsafe fn resume_phases(r: *mut ngx_http_request_t) {
    unsafe {
        let conn = (*r).connection;
        if conn.is_null() {
            return;
        }
        (*r).write_event_handler = Some(inference_resume_handler);
        ngx_post_event((*conn).write, ptr::addr_of_mut!(ngx_posted_events));
    }
}

/// Write-event handler that resumes the phase engine.
unsafe extern "C" fn inference_resume_handler(r: *mut ngx_http_request_t) {
    unsafe {
        (*r).write_event_handler = Some(ngx::ffi::ngx_http_core_run_phases);
        ngx::ffi::ngx_http_core_run_phases(r);
    }
}

/// Post a private event; upstream may replace its write handler before dispatch.
unsafe fn resume_response_output(r: *mut ngx_http_request_t) {
    if let Some(ctx) = unsafe { get_ctx(r) } {
        // Pass a raw field pointer; no Rust borrow of ctx spans the nginx call.
        unsafe {
            ngx_post_event(
                &raw mut (*ctx.as_ptr()).response_event,
                ptr::addr_of_mut!(ngx_posted_events),
            )
        };
    }
}

/// Wake only after the header/upstream stack has unwound. An inline EPP reply
/// then posts this same event again, separating polling from downstream output.
unsafe extern "C" fn inference_response_event(ev: *mut nginx_sys::ngx_event_t) {
    let r = unsafe { (*ev).data.cast::<ngx_http_request_t>() };
    let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
        return;
    };
    let ctx = unsafe { ctx.as_mut() };
    if ctx.response_failed {
        return;
    }

    // Notify mode: fire the shared signal for the detached task.
    if let Some(ref signal) = ctx.notify_signal {
        signal.fire();
        return;
    }

    // Headers mode: wake the original task or resume output.
    let (ready, waker) = (ctx.response_phase_done, ctx.notify_waker.take());
    if ready {
        unsafe { inference_response_resume_handler(r) };
    } else if let Some(waker) = waker {
        waker.wake();
    }
}

/// Write-event handler that commits suppressed headers and flushes the
/// buffered response body (headers mode).
unsafe extern "C" fn inference_response_resume_handler(r: *mut ngx_http_request_t) {
    let (body, handler) = {
        let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
            return;
        };
        let ctx = unsafe { ctx.as_mut() };
        if !ctx.response_held || ctx.response_failed || !ctx.response_phase_done {
            return;
        }
        ctx.headers_suppressed = false;
        ctx.response_held = false;
        // The private event never replaces nginx's handler. Use the handler
        // installed at dispatch, since upstream may have completed meanwhile.
        (core::mem::take(&mut ctx.buffered_body), unsafe {
            (*r).write_event_handler
        })
    };

    let rc = call_next_header_filter(r);
    if rc == Status::NGX_ERROR.into() || rc > Status::NGX_OK.into() {
        // Upstream may still own a reference. Terminate instead of generating
        // a second response with ambiguous ownership of the suppressed body.
        unsafe { ngx_http_finalize_request(r, Status::NGX_ERROR.into()) };
        return;
    }

    let rc = unsafe { flush_buffered_body(r, &body.chunks) };
    if rc == Status::NGX_ERROR.into() {
        unsafe { ngx_http_finalize_request(r, rc) };
        return;
    }

    unsafe {
        if (*r).done() != 0 {
            // Upstream already finalized and released its reference. Our hold
            // now owns completion: NGX_OK/AGAIN installs the core writer if
            // replay is buffered, rather than closing a backpressured response.
            ngx_http_finalize_request(r, rc);
        } else {
            // NGX_DONE releases only our extra reference. NGX_OK here would
            // mark a still-streaming request done and replace upstream's handler.
            // The not-done upstream still owns its reference, keeping r alive
            // for its handler. Do not access r after that handler returns.
            ngx_http_finalize_request(r, Status::NGX_DONE.into());
            if let Some(handler) = handler {
                // Let upstream drain output and arm its normal write timeout.
                handler(r);
            }
        }
    }
}

/// Finalize the EPP result: set the endpoint variable, apply mutations, and
/// return the appropriate status.
unsafe fn finalize_result(
    r: *mut ngx_http_request_t,
    mut ctx: NonNull<RequestCtx>,
    conf: &LocationConfig,
) -> ngx_int_t {
    // Own the result before logging or crossing the mutation/FFI boundary.
    let result = unsafe { ctx.as_mut().result.take() };
    let epp_result = match result {
        Some(Ok(epp_result)) => epp_result,
        Some(Err(err)) => return unsafe { handle_epp_failure(r, ctx, conf, &err) },
        None => {
            let err = ModuleError::Transport("no result from EPP task".into());
            return unsafe { handle_epp_failure(r, ctx, conf, &err) };
        }
    };

    let Some(ep) = epp_result.endpoints.into_iter().next() else {
        return unsafe { handle_epp_failure(r, ctx, conf, &ModuleError::NoEndpoint) };
    };
    ngx_log_error!(
        NGX_LOG_INFO,
        unsafe { request_log(r) },
        "inference: EPP selected endpoint: {}",
        ep.address
    );
    if let Err(err) = unsafe { apply_header_mutations(r, false, &epp_result.mutations) } {
        return unsafe { handle_epp_failure(r, ctx, conf, &err) };
    }
    unsafe { ctx.as_mut().endpoint = Some(ep.address) };

    Status::NGX_DECLINED.into()
}

/// Stage all pool allocations before changing any request-visible state.
/// List parts reference stable elements, rather than copying elements whose
/// addresses nginx (or another filter) may have cached. Each part has capacity
/// one: a subsequent ngx_list_push must never overwrite an adjacent old element.
unsafe fn apply_header_mutations(
    r: *mut ngx_http_request_t,
    response: bool,
    ms: &MutationSet,
) -> Result<(), ModuleError> {
    if ms.sets.is_empty() && ms.removes.is_empty() {
        return Ok(());
    }
    let input_headers = if response {
        &[][..]
    } else {
        unsafe { input_header_table() }
    };
    let mut storage = HeaderMutationPool(unsafe { Pool::from_ngx_pool((*r).pool) });
    unsafe { apply_header_mutations_with(r, response, ms, input_headers, &mut storage) }
        .map_err(|why| ModuleError::InvalidResponse(why.into()))
}

/// nginx's static request-header table (terminated by an empty name), viewed
/// in place. Using it, rather than hard-coded offsets, keeps optional fields
/// correct for the nginx build the module is loaded into.
unsafe fn input_header_table() -> &'static [nginx_sys::ngx_http_header_t] {
    unsafe extern "C" {
        static ngx_http_headers_in: nginx_sys::ngx_http_header_t;
    }
    let base = &raw const ngx_http_headers_in;
    let mut len = 0;
    unsafe {
        while (*base.add(len)).name.len != 0 {
            len += 1;
        }
        core::slice::from_raw_parts(base, len)
    }
}

// Only allocation/parsing crosses the nginx FFI boundary. Tests exercise the
// actual staging/commit logic with owned FFI structs, without nginx symbols.
trait HeaderMutationStorage {
    fn list(&mut self) -> Result<ngx_list_t, &'static str>;
    fn part(&mut self) -> Result<*mut nginx_sys::ngx_list_part_t, &'static str>;
    fn header(&mut self, name: &str, value: &[u8]) -> Result<*mut ngx_table_elt_t, &'static str>;
    fn time(&self, value: &ngx_str_t) -> nginx_sys::time_t;
}

struct HeaderMutationPool(Pool);

impl HeaderMutationStorage for HeaderMutationPool {
    fn list(&mut self) -> Result<ngx_list_t, &'static str> {
        let mut list = unsafe { core::mem::zeroed() };
        if unsafe {
            nginx_sys::ngx_list_init(&mut list, self.0.as_ptr(), 1, size_of::<ngx_table_elt_t>())
        } != Status::NGX_OK.into()
        {
            return Err("header list allocation failed");
        }
        // The embedded part moves on return. Never retain its stack address.
        list.last = ptr::null_mut();
        Ok(list)
    }

    fn part(&mut self) -> Result<*mut nginx_sys::ngx_list_part_t, &'static str> {
        NonNull::new(self.0.calloc_type::<nginx_sys::ngx_list_part_t>())
            .map(NonNull::as_ptr)
            .ok_or("header list part allocation failed")
    }

    fn header(&mut self, name: &str, value: &[u8]) -> Result<*mut ngx_table_elt_t, &'static str> {
        let h = self.0.calloc_type::<ngx_table_elt_t>();
        unsafe { nginx_sys::add_to_ngx_table(h, self.0.as_ptr(), name, value) }
            .ok_or("header allocation failed")?;
        Ok(h)
    }

    fn time(&self, value: &ngx_str_t) -> nginx_sys::time_t {
        unsafe { nginx_sys::ngx_parse_http_time(value.data, value.len) }
    }
}

fn mutation_touches(ms: &MutationSet, name: &[u8]) -> bool {
    ms.removes
        .iter()
        .any(|n| n.as_bytes().eq_ignore_ascii_case(name))
        || ms
            .sets
            .iter()
            .any(|op| op.name.as_bytes().eq_ignore_ascii_case(name))
}

/// Install a staged list, repairing the embedded-part self pointer only once
/// the list is at its final address. Non-embedded parts are pool allocated.
unsafe fn install_header_list(dst: *mut ngx_list_t, staged: ngx_list_t) {
    unsafe {
        *dst = staged;
        let mut last = ptr::addr_of_mut!((*dst).part);
        while !(*last).next.is_null() {
            last = (*last).next;
        }
        (*dst).last = last;
    }
}

unsafe fn apply_header_mutations_with(
    r: *mut ngx_http_request_t,
    response: bool,
    ms: &MutationSet,
    input_headers: &[nginx_sys::ngx_http_header_t],
    storage: &mut impl HeaderMutationStorage,
) -> Result<(), &'static str> {
    if ms.sets.is_empty() && ms.removes.is_empty() {
        return Ok(());
    }
    let list = unsafe {
        if response {
            ptr::addr_of_mut!((*r).headers_out.headers)
        } else {
            ptr::addr_of_mut!((*r).headers_in.headers)
        }
    };
    let mut kept = Vec::new();
    let mut retired = Vec::new();
    let mut snapshot = Vec::new();
    unsafe {
        let mut part = ptr::addr_of!((*list).part);
        while !part.is_null() {
            for i in 0..(*part).nelts {
                let h = (*part).elts.cast::<ngx_table_elt_t>().add(i);
                let name = (*h).key.as_bytes();
                if mutation_touches(ms, name) {
                    if !response || (*h).hash != 0 {
                        snapshot.push((
                            String::from_utf8_lossy(name).to_ascii_lowercase(),
                            (*h).value.as_bytes().to_vec(),
                        ));
                    }
                    retired.push(h);
                } else {
                    kept.push(h);
                }
            }
            part = (*part).next;
        }
        if response
            && mutation_touches(ms, b"content-type")
            && (*r).headers_out.content_type.len != 0
        {
            let out = &(*r).headers_out;
            let mut value = out.content_type.as_bytes().to_vec();
            if out.content_type_len == out.content_type.len && out.charset.len != 0 {
                value.extend_from_slice(b"; charset=");
                value.extend_from_slice(out.charset.as_bytes());
            }
            snapshot.push(("content-type".into(), value));
        }
    }
    mutations::apply(ms, &mut snapshot);

    // nginx shortcut consumers assume these are single-valued. Reject the
    // entire transaction rather than emit one value while using another.
    let singletons: &[&str] = if response {
        &[
            "content-type",
            "server",
            "date",
            "content-encoding",
            "location",
            "refresh",
            "last-modified",
            "content-range",
            "accept-ranges",
            "expires",
            "etag",
        ]
    } else {
        &[
            "if-modified-since",
            "if-unmodified-since",
            "if-match",
            "if-none-match",
            "content-range",
            "if-range",
            "expect",
            "authorization",
            "proxy-authorization",
        ]
    };
    for name in singletons {
        if snapshot.iter().filter(|(n, _)| n == name).count() > 1 {
            return Err("multiple values for a singleton nginx header");
        }
    }
    let mut staged = storage.list()?;
    let mut replacements = Vec::new();
    for (name, value) in &snapshot {
        replacements.push((name.as_str(), storage.header(name, value)?));
    }
    // Build only the new headers' next chains; never copy stale original next
    // pointers or touch the original elements before the commit boundary.
    for (i, (name, h)) in replacements.iter().enumerate() {
        unsafe {
            (**h).next = replacements[i + 1..]
                .iter()
                .find(|(n, _)| n == name)
                .map_or(ptr::null_mut(), |(_, h)| *h);
        }
    }
    let first = |name: &str| {
        replacements
            .iter()
            .find(|(n, _)| *n == name)
            .map_or(ptr::null_mut(), |(_, h)| *h)
    };
    kept.extend(
        replacements
            .iter()
            .filter_map(|(name, h)| (!(response && *name == "content-type")).then_some(*h)),
    );
    let mut previous: *mut nginx_sys::ngx_list_part_t = ptr::null_mut();
    for (i, h) in kept.iter().enumerate() {
        if i == 0 {
            staged.part.elts = h.cast();
            staged.part.nelts = 1;
        } else {
            let part = storage.part()?;
            unsafe {
                *part = nginx_sys::ngx_list_part_t {
                    elts: h.cast(),
                    nelts: 1,
                    next: ptr::null_mut(),
                };
                if previous.is_null() {
                    staged.part.next = part;
                } else {
                    (*previous).next = part;
                }
            }
            previous = part;
        }
    }
    let time = |name| {
        let h = first(name);
        if h.is_null() {
            -1
        } else {
            storage.time(unsafe { &(*h).value })
        }
    };
    let last_modified_time = time("last-modified");
    let date_time = time("date");

    // COMMIT: no allocation, parsing, callbacks or fallible operations below.
    unsafe {
        for h in retired {
            (*h).hash = 0;
            (*h).next = ptr::null_mut();
        }
        install_header_list(list, staged);
        if response {
            let out = &mut (*r).headers_out;
            macro_rules! shortcut {
                ($($name:literal => $field:ident),* $(,)?) => {$(
                    if mutation_touches(ms, $name.as_bytes()) { out.$field = first($name); }
                )*};
            }
            shortcut!(
                "server" => server, "date" => date, "content-encoding" => content_encoding,
                "location" => location, "refresh" => refresh, "last-modified" => last_modified,
                "content-range" => content_range, "accept-ranges" => accept_ranges,
                "www-authenticate" => www_authenticate, "proxy-authenticate" => proxy_authenticate,
                "expires" => expires, "etag" => etag, "cache-control" => cache_control, "link" => link,
            );
            if mutation_touches(ms, b"last-modified") {
                out.last_modified_time = last_modified_time;
            }
            if mutation_touches(ms, b"date") {
                out.date_time = date_time;
            }
            if mutation_touches(ms, b"content-type") {
                let h = first("content-type");
                out.content_type = if h.is_null() {
                    core::mem::zeroed()
                } else {
                    (*h).value
                };
                out.content_type_len = out
                    .content_type
                    .as_bytes()
                    .iter()
                    .position(|&b| b == b';')
                    .unwrap_or(out.content_type.len);
                out.charset = core::mem::zeroed();
                out.override_charset = ptr::null_mut();
                out.content_type_lowcase = ptr::null_mut();
                out.content_type_hash = 0;
            }
        } else {
            let input = &mut (*r).headers_in;
            for h in input_headers {
                let name = h.name.as_bytes();
                if h.offset != 0 && mutation_touches(ms, name) {
                    let replacement = replacements
                        .iter()
                        .find(|(n, _)| n.as_bytes().eq_ignore_ascii_case(name))
                        .map_or(ptr::null_mut(), |(_, h)| *h);
                    (ptr::from_mut(input)
                        .cast::<u8>()
                        .add(h.offset)
                        .cast::<*mut ngx_table_elt_t>())
                    .write(replacement);
                }
            }
            input.count = kept.len();
            // Generic duplicate links may have been built before precontent phase.
            // Rebuild all links later when nginx needs them; special-header
            // chains already point to the staged replacement elements.
            input.set_multi_linked(0);
            input.set_multi(1);
            if mutation_touches(ms, b"authorization") {
                input.user = core::mem::zeroed();
                input.passwd = core::mem::zeroed();
            }
            if mutation_touches(ms, b"user-agent") {
                refresh_mutated_user_agent(input);
            }
        }
    }
    Ok(())
}

/// Refresh the browser flags cached by ngx_http_process_user_agent. Do not
/// invoke nginx's request parser here: it can finalize the live request.
unsafe fn refresh_mutated_user_agent(input: &mut nginx_sys::ngx_http_headers_in_t) {
    let value = if input.user_agent.is_null() {
        &[][..]
    } else {
        unsafe { (*input.user_agent).value.as_bytes() }
    };
    let contains = |needle: &[u8]| value.windows(needle.len()).any(|w| w == needle);
    let msie = value
        .windows(5)
        .position(|w| w == b"MSIE ")
        .filter(|&i| i + 7 < value.len());
    let opera = contains(b"Opera");
    let legacy = msie.is_some() && !opera;
    input.set_msie(u32::from(legacy));
    input.set_msie6(u32::from(
        legacy
            && msie.is_some_and(|i| {
                value[i + 6] == b'.'
                    && (matches!(value[i + 5], b'4' | b'5')
                        || (value[i + 5] == b'6'
                            && !value[i + 8..].windows(3).any(|w| w == b"SV1")))
            }),
    ));
    input.set_opera(u32::from(opera));
    let gecko = !legacy && !opera && contains(b"Gecko/");
    let chrome = !legacy && !opera && !gecko && contains(b"Chrome/");
    let safari =
        !legacy && !opera && !gecko && !chrome && contains(b"Safari/") && contains(b"Mac OS X");
    input.set_gecko(u32::from(gecko));
    input.set_chrome(u32::from(chrome));
    input.set_safari(u32::from(safari));
    input.set_konqueror(u32::from(
        !legacy && !opera && !gecko && !chrome && !safari && contains(b"Konqueror"),
    ));
}

/// Invoke `f` on every live (`hash != 0`) element of an nginx header list.
unsafe fn for_each_elt<'a>(list: &'a ngx_list_t, mut f: impl FnMut(&'a ngx_table_elt_t)) {
    let mut part = &list.part as *const nginx_sys::ngx_list_part_t;
    while !part.is_null() {
        let p = unsafe { &*part };
        let elts = p.elts as *mut ngx_table_elt_t;
        for i in 0..p.nelts {
            let h = unsafe { &*elts.add(i) };
            if h.hash != 0 {
                f(h);
            }
        }
        part = p.next;
    }
}

/// Handle an EPP failure: fail open to the configured endpoint, or return the
/// error's HTTP status.
unsafe fn handle_epp_failure(
    r: *mut ngx_http_request_t,
    mut ctx: NonNull<RequestCtx>,
    conf: &LocationConfig,
    err: &ModuleError,
) -> ngx_int_t {
    let log = unsafe { request_log(r) };

    if let Some(ref failopen) = conf.failopen {
        ngx_log_error!(
            NGX_LOG_WARN,
            log,
            "inference: EPP '{}' failed (fail-open to '{}'): {}",
            conf.endpoint(),
            failopen,
            err
        );
        unsafe { ctx.as_mut().endpoint = Some(failopen.clone()) };
        return Status::NGX_DECLINED.into();
    }

    let status = err.status();
    ngx_log_error!(
        NGX_LOG_ERR,
        log,
        "inference: EPP '{}' failed (status {}, timeout {}ms): {}",
        conf.endpoint(),
        status,
        conf.timeout().as_millis(),
        err
    );
    status as ngx_int_t
}

// ---------------------------------------------------------------------------
// Response header filter
// ---------------------------------------------------------------------------

/// Register the header filter in the nginx filter chain.
///
/// Called from `postconfiguration`. Inserts our filter at the top of the chain.
pub(crate) unsafe fn register_header_filter() {
    unsafe {
        NEXT_HEADER_FILTER = ngx_http_top_header_filter;
        ngx_http_top_header_filter = Some(inference_header_filter);
    }
}

/// Call the next header filter in the chain.
fn call_next_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
    unsafe {
        match NEXT_HEADER_FILTER {
            Some(filter) => filter(r),
            None => Status::NGX_OK.into(),
        }
    }
}

/// Header filter — signals the async task to send response-phase messages.
///
/// When an EPP response phase is pending, this sets `notify_ready` and posts
/// a private event to wake the task after the upstream stack unwinds.
///
/// In **headers** mode, the filter does NOT call `next_header_filter` —
/// it returns `NGX_OK` to suppress sending the response downstream.
/// A separate event dispatch resumes output after the task applies mutations.
unsafe extern "C" fn inference_header_filter(r: *mut ngx_http_request_t) -> ngx_int_t {
    // Only process main requests.
    if unsafe { r != (*r).main } {
        return call_next_header_filter(r);
    }

    let Some(mut ctx) = (unsafe { get_ctx(r) }) else {
        return call_next_header_filter(r);
    };
    let (suppressed, post) = {
        let ctx = unsafe { ctx.as_mut() };
        if ctx.response_failed {
            return Status::NGX_ERROR.into();
        }
        let post = ctx.response_pending && !ctx.notify_ready;
        if post {
            ctx.notify_ready = true;
            if ctx.response_phase == ResponsePhase::Headers {
                ctx.headers_suppressed = true;
                ctx.response_held = true;
            }
        }
        // Notify mode with signal: fire it directly from the header filter
        // when the event dispatch path might not run (fast inline response).
        if post && let Some(ref signal) = ctx.notify_signal {
            signal.fire();
        }
        (ctx.headers_suppressed, post)
    };
    if post {
        if suppressed {
            unsafe {
                (*r).set_filter_need_in_memory(1);
                (*r).set_count((*r).count() + 1);
            }
        }
        // A private event avoids waking inline on the header-filter
        // stack, before upstream installs its downstream write handler.
        unsafe { resume_response_output(r) };
    }
    if suppressed {
        Status::NGX_OK.into()
    } else {
        call_next_header_filter(r)
    }
}

// ---------------------------------------------------------------------------
// Response body filter
// ---------------------------------------------------------------------------

/// Register the body filter in the nginx filter chain.
///
/// Called from `postconfiguration`. Inserts our filter at the top of the chain.
pub(crate) unsafe fn register_body_filter() {
    unsafe {
        NEXT_BODY_FILTER = ngx_http_top_body_filter;
        ngx_http_top_body_filter = Some(inference_body_filter);
    }
}

/// Call the next body filter in the chain.
fn call_next_body_filter(r: *mut ngx_http_request_t, chain: *mut ngx_chain_t) -> ngx_int_t {
    unsafe {
        match NEXT_BODY_FILTER {
            Some(filter) => filter(r, chain),
            None => Status::NGX_OK.into(),
        }
    }
}

/// Response body filter — streams upstream body data to the EPP and, in
/// `headers` mode, also buffers output until the response phase completes.
///
/// In `notify` mode the body passes through to the client immediately while
/// copies are pushed into the body channel for the detached response reporter.
/// In `headers` mode the header filter suppressed the downstream header chain,
/// so this filter absorbs each body chain into `ctx.buffered_body` AND pushes
/// copies to the body channel.
unsafe extern "C" fn inference_body_filter(
    r: *mut ngx_http_request_t,
    in_chain: *mut ngx_chain_t,
) -> ngx_int_t {
    // Only process main requests.
    if unsafe { r != (*r).main } {
        return call_next_body_filter(r, in_chain);
    }

    let Some(mut ctx) =
        (unsafe { get_ctx(r) }).filter(|ctx| unsafe { ctx.as_ref().response_pending })
    else {
        return call_next_body_filter(r, in_chain);
    };

    let suppressed = unsafe { ctx.as_ref().headers_suppressed };

    // Push body chunks into the channel for the EPP response task.
    unsafe { feed_body_channel(ctx.as_mut(), in_chain) };

    if !suppressed {
        // Notify mode: pass body downstream immediately.
        return call_next_body_filter(r, in_chain);
    }

    // Headers mode: buffer as well.
    let failed = {
        let ctx = unsafe { ctx.as_mut() };
        if ctx.response_failed {
            return Status::NGX_ERROR.into();
        }
        if unsafe { buffer_response_chain(&mut ctx.buffered_body, in_chain) }.is_err() {
            ctx.response_failed = true;
            ctx.buffered_body = ResponseBuffer::default();
            true
        } else {
            false
        }
    };
    if failed {
        ngx_log_error!(
            NGX_LOG_ERR,
            unsafe { request_log(r) },
            "inference: response buffering failed (1 MiB/1024 buffers limit, allocation, or unsupported file buffer)"
        );
        // nginx's caller terminates on NGX_ERROR, running HTTP cleanup and
        // canceling the task. Do not cancel a possibly-running task inline.
        return Status::NGX_ERROR.into();
    }
    Status::NGX_OK.into()
}

/// Walk the chain and push each buffer's data into the body channel.
///
/// Best-effort: silently drops chunks that exceed the channel capacity.
/// The final `last_buf` chunk is always accepted so the EOS signal is not lost.
unsafe fn feed_body_channel(ctx: &mut RequestCtx, mut link: *mut ngx_chain_t) {
    let Some(ref sender) = ctx.body_sender else {
        return;
    };
    while !link.is_null() {
        let buf = unsafe { (*link).buf };
        if buf.is_null() {
            link = unsafe { (*link).next };
            continue;
        }
        let b = unsafe { &*buf };
        let in_memory = b.temporary() != 0 || b.memory() != 0 || b.mmap() != 0;
        let data = if in_memory && !b.pos.is_null() && !b.last.is_null() && b.last >= b.pos {
            unsafe { core::slice::from_raw_parts(b.pos, b.last.offset_from(b.pos) as usize) }
        } else {
            &[]
        };
        let last = b.last_buf() != 0;
        sender.send(data.to_vec(), last);
        if last {
            // Drop the sender so the receiver sees completion even if nginx
            // doesn't call the body filter again.
            ctx.body_sender = None;
            return;
        }
        link = unsafe { (*link).next };
    }
}

unsafe fn buffer_response_chain(
    body: &mut ResponseBuffer,
    mut link: *mut ngx_chain_t,
) -> Result<(), ()> {
    while !link.is_null() {
        let buf = unsafe { (*link).buf };
        if buf.is_null() {
            return Err(());
        }
        let b = unsafe { &mut *buf };
        let in_memory = b.temporary() != 0 || b.memory() != 0 || b.mmap() != 0;
        let data = if in_memory && !b.pos.is_null() && !b.last.is_null() && b.last >= b.pos {
            unsafe { core::slice::from_raw_parts(b.pos, b.last.offset_from(b.pos) as usize) }
        } else if in_memory && b.pos != b.last {
            return Err(());
        } else {
            &[]
        };
        // filter_need_in_memory requests conversion by nginx's copy filter.
        // Reject a remaining file-only (or inconsistent dual-backed) payload.
        if b.in_file() != 0
            && (!in_memory
                || b.file_pos < 0
                || b.file_last
                    .checked_sub(b.file_pos)
                    .and_then(|len| usize::try_from(len).ok())
                    != Some(data.len()))
        {
            return Err(());
        }
        body.push(
            data,
            ResponseFlags {
                flush: b.flush() != 0,
                sync: b.sync() != 0,
                last_buf: b.last_buf() != 0,
                last_in_chain: b.last_in_chain() != 0,
            },
        )?;
        b.pos = b.last;
        if b.in_file() != 0 {
            b.file_pos = b.file_last;
        }
        link = unsafe { (*link).next };
    }
    Ok(())
}

/// Build an `ngx_chain_t` from buffered chunks and pass it to the next body
/// filter.
///
/// Returns the result of `call_next_body_filter`.
unsafe fn flush_buffered_body(r: *mut ngx_http_request_t, chunks: &[ResponseChunk]) -> ngx_int_t {
    let pool = unsafe { (*r).pool };
    if pool.is_null() {
        return Status::NGX_ERROR.into();
    }

    let mut first_link: *mut ngx_chain_t = ptr::null_mut();
    let mut prev_link: *mut ngx_chain_t = ptr::null_mut();

    for chunk in chunks {
        // Allocate at least one byte for special empty buffers; all allocations
        // must succeed before passing any part of this chain downstream.
        let buf = unsafe { ngx::ffi::ngx_create_temp_buf(pool, chunk.data.len().max(1)) };
        if buf.is_null() {
            return Status::NGX_ERROR.into();
        }

        unsafe {
            ptr::copy_nonoverlapping(chunk.data.as_ptr(), (*buf).pos, chunk.data.len());
            (*buf).last = (*buf).pos.add(chunk.data.len());
            (*buf).set_temporary(u32::from(!chunk.data.is_empty()));
            (*buf).set_flush(u32::from(chunk.flags.flush));
            // last_in_chain alone is not ngx_buf_special(): an empty marker
            // needs sync so the write filter does not reject a zero-size buffer.
            (*buf).set_sync(u32::from(
                chunk.flags.sync || (chunk.data.is_empty() && chunk.flags.last_in_chain),
            ));
            (*buf).set_last_buf(u32::from(chunk.flags.last_buf));
            (*buf).set_last_in_chain(u32::from(chunk.flags.last_in_chain));
        }

        let link = unsafe { ngx::ffi::ngx_alloc_chain_link(pool) };
        if link.is_null() {
            return Status::NGX_ERROR.into();
        }

        unsafe {
            (*link).buf = buf;
            (*link).next = ptr::null_mut();
        }

        if first_link.is_null() {
            first_link = link;
        } else {
            unsafe { (*prev_link).next = link };
        }
        prev_link = link;
    }

    if first_link.is_null() {
        return Status::NGX_OK.into();
    }

    call_next_body_filter(r, first_link)
}

// ---------------------------------------------------------------------------
// $inference_endpoint variable
// ---------------------------------------------------------------------------

/// Register the `$inference_endpoint` variable.
///
/// Called from `preconfiguration`. The variable is `NOCACHEABLE` because its
/// value changes during the request (set by the precontent handler).
pub(crate) unsafe fn register_variable(cf: *mut nginx_sys::ngx_conf_t) -> ngx_int_t {
    let mut name = ngx::ngx_string!("inference_endpoint");

    let var = unsafe {
        nginx_sys::ngx_http_add_variable(
            cf,
            &mut name as *mut _,
            nginx_sys::NGX_HTTP_VAR_NOCACHEABLE as _,
        )
    };
    if var.is_null() {
        return Status::NGX_ERROR.into();
    }

    unsafe {
        (*var).get_handler = Some(inference_endpoint_getter);
        (*var).data = 0;
    }

    Status::NGX_OK.into()
}

/// Variable getter for `$inference_endpoint`.
///
/// Reads the endpoint value from the per-request context.
unsafe extern "C" fn inference_endpoint_getter(
    r: *mut ngx_http_request_t,
    v: *mut nginx_sys::ngx_http_variable_value_t,
    _data: usize,
) -> ngx_int_t {
    let v = unsafe { &mut *v };

    let Some(ctx) = (unsafe { get_ctx(r) }) else {
        v.set_not_found(1);
        return Status::NGX_OK.into();
    };
    let ctx = unsafe { ctx.as_ref() };

    // The variable length is a 28-bit bitfield; never truncate silently.
    let Some((ep, len)) = ctx.endpoint.as_deref().and_then(|ep| {
        u32::try_from(ep.len())
            .ok()
            .filter(|&n| n < 1 << 28)
            .map(|n| (ep, n))
    }) else {
        v.set_not_found(1);
        return Status::NGX_OK.into();
    };

    // The ctx (and this string) lives until request pool destruction, after
    // log-phase evaluation, so the value can borrow it directly.
    v.set_len(len);
    v.data = ep.as_ptr() as *mut u8;
    v.set_valid(1);
    v.set_no_cacheable(1);
    v.set_not_found(0);

    Status::NGX_OK.into()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Collect the request body (memory and temp-file buffers) into a `Vec<u8>`.
unsafe fn collect_body(r: *mut ngx_http_request_t) -> Result<Vec<u8>, ModuleError> {
    unsafe {
        collect_body_with(r, |file, bytes, offset| {
            nginx_sys::ngx_read_file(file, bytes.as_mut_ptr(), bytes.len(), offset)
        })
    }
}

/// Check the aggregate wire length before reserving or touching body storage.
fn reserve_body(data: &mut Vec<u8>, len: usize) -> Result<usize, ModuleError> {
    let end = data
        .len()
        .checked_add(len)
        .ok_or_else(|| ModuleError::GrpcFrame("request body length overflow".into()))?;
    request::check_body_len(end)?;
    data.try_reserve(len)
        .map_err(|_| ModuleError::Transport("request body allocation failed".into()))?;
    Ok(end)
}

// Keep file I/O injectable so malformed ranges and short reads can be tested
// without nginx's global file/log machinery. No request borrow crosses I/O.
unsafe fn collect_body_with(
    r: *mut ngx_http_request_t,
    mut read: impl FnMut(*mut nginx_sys::ngx_file_t, &mut [u8], nginx_sys::off_t) -> isize,
) -> Result<Vec<u8>, ModuleError> {
    let rb = unsafe { (*r).request_body };
    if rb.is_null() {
        return Ok(Vec::new());
    }

    let mut data = Vec::new();
    let mut chain = unsafe { (*rb).bufs };
    while !chain.is_null() {
        let buf = unsafe { (*chain).buf };
        chain = unsafe { (*chain).next };
        if buf.is_null() {
            continue;
        }
        let (in_file, file, file_pos, file_last, pos, last) = unsafe {
            let b = &*buf;
            (
                b.in_file() != 0,
                b.file,
                b.file_pos,
                b.file_last,
                b.pos,
                b.last,
            )
        };
        if in_file {
            let invalid = || ModuleError::Transport("invalid request body file range".into());
            if file.is_null() || file_pos < 0 {
                return Err(invalid());
            }
            let len = file_last.checked_sub(file_pos).ok_or_else(invalid)?;
            let len = usize::try_from(len)
                .map_err(|_| ModuleError::Transport("invalid request body file range".into()))?;
            let start = data.len();
            let end = reserve_body(&mut data, len)?;
            if len == 0 {
                continue;
            }
            data.resize(end, 0);
            let n = read(file, &mut data[start..end], file_pos);
            if usize::try_from(n).ok() != Some(len) {
                return Err(ModuleError::Transport(
                    "failed to read buffered request body".into(),
                ));
            }
        } else if pos != last {
            let invalid = || ModuleError::Transport("invalid request body memory range".into());
            if pos.is_null() || last.is_null() {
                return Err(invalid());
            }
            let len = (last as usize)
                .checked_sub(pos as usize)
                .ok_or_else(invalid)?;
            reserve_body(&mut data, len)?;
            // nginx owns this initialized buffer; validate length before creating
            // the slice (and avoid offset_from on malformed/reversed pointers).
            data.extend_from_slice(unsafe { core::slice::from_raw_parts(pos, len) });
        }
    }
    Ok(data)
}

/// Encode the gRPC request frames directly from the nginx request, borrowing
/// header data from `r->headers_in`.
unsafe fn encode_request_frames(
    r: *mut ngx_http_request_t,
    body: Vec<u8>,
) -> Result<(Vec<u8>, Vec<Vec<u8>>), ModuleError> {
    let req = unsafe { &*r };
    let hi = &req.headers_in;
    let authority = unsafe { request_authority(req) };
    let ssl = !req.connection.is_null() && unsafe { !(*req.connection).ssl.is_null() };

    let pseudo = PseudoHeaders {
        method: req.method_name.as_bytes(),
        // nginx's original path/query (not the absolute-form authority).
        path: req.unparsed_uri.as_bytes(),
        authority,
        scheme: if ssl { b"https" } else { b"http" },
    };

    let mut headers: Vec<Header<'_>> = Vec::new();
    unsafe {
        for_each_elt(&hi.headers, |h| {
            if h.lowcase_key.is_null() {
                return;
            }
            let lower = core::slice::from_raw_parts(h.lowcase_key, h.key.len);
            if let Ok(name) = core::str::from_utf8(lower) {
                headers.push(Header {
                    name,
                    value: h.value.as_bytes(),
                });
            }
        });
    }

    Ok((
        request::encode_request_headers(&pseudo, &headers)?,
        request::encode_request_body_frames(&body)?,
    ))
}

/// nginx validates the absolute-form host before processing Host and keeps it
/// authoritative even when Host disagrees. host_end excludes the port; use its
/// offset in the preserved request_line and scan the validated numeric port.
/// Do not use uri_start as the end: a pathless target points it at the scheme's
/// slash. headers_in.server is normalized (lowercase, no port/trailing dot).
unsafe fn request_authority(req: &ngx_http_request_t) -> &[u8] {
    if !req.host_start.is_null() && !req.host_end.is_null() {
        let line = req.request_line.as_bytes();
        let base = line.as_ptr() as usize;
        if let (Some(start), Some(end)) = (
            (req.host_start as usize).checked_sub(base),
            (req.host_end as usize).checked_sub(base),
        ) && start < end
            && end <= line.len()
        {
            let mut end = end;
            if line.get(end) == Some(&b':') {
                end += 1;
                while line.get(end).is_some_and(u8::is_ascii_digit) {
                    end += 1;
                }
            }
            return &line[start..end];
        }
    }
    if !req.headers_in.host.is_null() {
        let host = unsafe { (*req.headers_in.host).value.as_bytes() };
        if !host.is_empty() {
            return host;
        }
    }
    req.headers_in.server.as_bytes()
}

#[cfg(test)]
mod header_mutation_tests {
    use super::*;
    use envoy_types::pb::envoy::config::core::v3::{HeaderValue, HeaderValueOption};
    use envoy_types::pb::envoy::service::ext_proc::v3::HeaderMutation;
    use envoy_types::pb::google::protobuf::BoolValue;

    // Boxes keep C pointers stable even when the owning vectors grow.
    #[derive(Default)]
    #[allow(clippy::vec_box)]
    struct Arena {
        headers: Vec<Box<ngx_table_elt_t>>,
        parts: Vec<Box<nginx_sys::ngx_list_part_t>>,
        bytes: Vec<Box<[u8]>>,
        allocations: usize,
        fail_at: Option<usize>,
    }

    impl Arena {
        fn allocation(&mut self) -> Result<(), &'static str> {
            let index = self.allocations;
            self.allocations += 1;
            if self.fail_at == Some(index) {
                Err("injected allocation failure")
            } else {
                Ok(())
            }
        }

        fn string(&mut self, value: &[u8]) -> Result<ngx_str_t, &'static str> {
            self.allocation()?;
            let mut bytes = value.to_vec();
            bytes.push(0);
            let mut bytes = bytes.into_boxed_slice();
            let s = ngx_str_t {
                len: value.len(),
                data: bytes.as_mut_ptr(),
            };
            self.bytes.push(bytes);
            Ok(s)
        }

        fn request(&mut self, response: bool, values: &[(&str, &[u8])]) -> Box<ngx_http_request_t> {
            let mut r: Box<ngx_http_request_t> = Box::new(unsafe { core::mem::zeroed() });
            let list = if response {
                &mut r.headers_out.headers
            } else {
                &mut r.headers_in.headers
            };
            *list = self.list().unwrap();
            let mut previous = ptr::addr_of_mut!(list.part);
            for (i, (name, value)) in values.iter().enumerate() {
                let h = self.header(name, value).unwrap();
                unsafe {
                    if i != 0 {
                        let next = self.part().unwrap();
                        (*previous).next = next;
                        previous = next;
                    }
                    (*previous).elts = h.cast();
                    (*previous).nelts = 1;
                }
            }
            list.last = previous;
            r
        }
    }

    impl HeaderMutationStorage for Arena {
        fn list(&mut self) -> Result<ngx_list_t, &'static str> {
            let h = self.header("", b"")?;
            let mut list: ngx_list_t = unsafe { core::mem::zeroed() };
            list.part.elts = h.cast();
            list.nalloc = 1;
            list.size = size_of::<ngx_table_elt_t>();
            Ok(list)
        }

        fn part(&mut self) -> Result<*mut nginx_sys::ngx_list_part_t, &'static str> {
            self.allocation()?;
            let mut p = Box::new(unsafe { core::mem::zeroed() });
            let ptr = ptr::from_mut(&mut *p);
            self.parts.push(p);
            Ok(ptr)
        }

        fn header(
            &mut self,
            name: &str,
            value: &[u8],
        ) -> Result<*mut ngx_table_elt_t, &'static str> {
            self.allocation()?;
            let mut h: Box<ngx_table_elt_t> = Box::new(unsafe { core::mem::zeroed() });
            h.key = self.string(name.as_bytes())?;
            h.value = self.string(value)?;
            h.lowcase_key = self.string(name.to_ascii_lowercase().as_bytes())?.data;
            h.hash = 1;
            let ptr = ptr::from_mut(&mut *h);
            self.headers.push(h);
            Ok(ptr)
        }

        fn time(&self, _: &ngx_str_t) -> nginx_sys::time_t {
            123
        }
    }

    #[allow(deprecated)]
    fn batch(sets: &[(&str, &[u8], bool)], removes: &[&str]) -> HeaderMutation {
        HeaderMutation {
            set_headers: sets
                .iter()
                .map(|(name, value, append)| HeaderValueOption {
                    header: Some(HeaderValue {
                        key: (*name).into(),
                        raw_value: value.to_vec(),
                        ..Default::default()
                    }),
                    append: Some(BoolValue { value: *append }),
                    ..Default::default()
                })
                .collect(),
            remove_headers: removes.iter().map(|s| (*s).into()).collect(),
        }
    }

    fn mutation(sets: &[(&str, &[u8], bool)], removes: &[&str]) -> MutationSet {
        let mut ms = MutationSet::default();
        ms.merge(&batch(sets, removes)).unwrap();
        ms
    }

    unsafe fn elements(list: &ngx_list_t) -> Vec<*mut ngx_table_elt_t> {
        let mut result = Vec::new();
        let mut part = ptr::addr_of!(list.part);
        unsafe {
            loop {
                for i in 0..(*part).nelts {
                    result.push((*part).elts.cast::<ngx_table_elt_t>().add(i));
                }
                if (*part).next.is_null() {
                    assert_eq!(list.last, part.cast_mut());
                    break;
                }
                part = (*part).next;
            }
        }
        result
    }

    fn input_metadata() -> [nginx_sys::ngx_http_header_t; 3] {
        [
            (
                "Cookie",
                core::mem::offset_of!(nginx_sys::ngx_http_headers_in_t, cookie),
            ),
            (
                "Authorization",
                core::mem::offset_of!(nginx_sys::ngx_http_headers_in_t, authorization),
            ),
            (
                "User-Agent",
                core::mem::offset_of!(nginx_sys::ngx_http_headers_in_t, user_agent),
            ),
        ]
        .map(|(name, offset)| nginx_sys::ngx_http_header_t {
            name: ngx_str_t {
                len: name.len(),
                data: name.as_ptr().cast_mut(),
            },
            offset,
            handler: None,
        })
    }

    #[test]
    fn allocation_failures_leave_list_elements_and_shortcuts_unmodified() {
        for response in [false, true] {
            let mut reached_success = false;
            for fail_at in 0..100 {
                let mut arena = Arena::default();
                let mut r =
                    arena.request(response, &[("cache-control", b"old"), ("x-keep", b"keep")]);
                let list = if response {
                    &r.headers_out.headers
                } else {
                    &r.headers_in.headers
                };
                let mut old = unsafe { elements(list) };
                r.headers_out.cache_control = old[0];
                unsafe {
                    (*old[0]).next = old[1];
                }
                let original_last = list_last(&r, response);
                arena.allocations = 0;
                arena.fail_at = Some(fail_at);
                let ms = mutation(
                    &[("cache-control", b"new", false), ("x-new", b"value", false)],
                    &[],
                );
                let result =
                    unsafe { apply_header_mutations_with(&mut *r, response, &ms, &[], &mut arena) };
                if result.is_ok() {
                    reached_success = true;
                    break;
                }
                let list = if response {
                    &r.headers_out.headers
                } else {
                    &r.headers_in.headers
                };
                assert_eq!(unsafe { elements(list) }, old);
                assert_eq!(list.last, original_last);
                assert_eq!(r.headers_out.cache_control, old[0]);
                unsafe {
                    assert_eq!((*old[0]).hash, 1);
                    assert_eq!((*old[0]).next, old[1]);
                    assert_eq!((*old[0]).value.as_bytes(), b"old");
                }
            }
            assert!(reached_success);
        }
    }

    fn list_last(r: &ngx_http_request_t, response: bool) -> *mut nginx_sys::ngx_list_part_t {
        if response {
            r.headers_out.headers.last
        } else {
            r.headers_in.headers.last
        }
    }

    #[test]
    fn input_removal_is_physical_and_shortcuts_and_next_chains_are_current() {
        let mut arena = Arena::default();
        let mut r = arena.request(
            false,
            &[
                ("host", b"host"),
                ("Cookie", b"old"),
                ("authorization", b"old"),
            ],
        );
        let old = unsafe { elements(&r.headers_in.headers) };
        r.headers_in.host = old[0];
        r.headers_in.cookie = old[1];
        r.headers_in.authorization = old[2];
        r.headers_in.user = arena.string(b"cached-user").unwrap();
        r.headers_in.passwd = arena.string(b"cached-password").unwrap();
        r.headers_in.set_multi_linked(1);
        let ms = mutation(
            &[("cookie", b"a=1", false), ("cookie", b"b=2", true)],
            &["authorization"],
        );
        unsafe {
            apply_header_mutations_with(&mut *r, false, &ms, &input_metadata(), &mut arena)
                .unwrap();
            let current = elements(&r.headers_in.headers);
            assert_eq!(current.len(), 3);
            assert_eq!(current[0], old[0]);
            assert_eq!(r.headers_in.host, old[0]);
            assert!(!current.contains(&old[1]));
            assert!(!current.contains(&old[2]));
            assert_eq!(r.headers_in.cookie, current[1]);
            assert_eq!((*current[1]).next, current[2]);
            assert!((*current[2]).next.is_null());
            assert!(r.headers_in.authorization.is_null());
            assert_eq!(r.headers_in.user.len, 0);
            assert_eq!(r.headers_in.passwd.len, 0);
            assert_eq!(r.headers_in.multi_linked(), 0);
        }
    }

    #[test]
    fn response_special_fields_and_content_type_are_committed_together() {
        let mut arena = Arena::default();
        let mut r = arena.request(
            true,
            &[
                ("location", b"/old"),
                ("cache-control", b"old"),
                ("etag", b"keep"),
            ],
        );
        let old = unsafe { elements(&r.headers_out.headers) };
        r.headers_out.location = old[0];
        r.headers_out.cache_control = old[1];
        r.headers_out.etag = old[2];
        r.headers_out.content_type = arena.string(b"text/plain").unwrap();
        r.headers_out.content_type_len = 10;
        r.headers_out.charset = arena.string(b"utf-8").unwrap();
        r.headers_out.content_type_lowcase = r.headers_out.content_type.data;
        r.headers_out.content_type_hash = 9;
        let ms = mutation(
            &[
                ("location", b"/new", false),
                ("cache-control", b"private", false),
                ("cache-control", b"no-store", true),
                ("content-type", b"application/json; charset=utf-8", false),
            ],
            &["last-modified"],
        );
        unsafe {
            apply_header_mutations_with(&mut *r, true, &ms, &[], &mut arena).unwrap();
            let current = elements(&r.headers_out.headers);
            assert_eq!(current[0], old[2]);
            assert_eq!(r.headers_out.etag, old[2]);
            assert_eq!((*r.headers_out.location).value.as_bytes(), b"/new");
            assert_eq!((*r.headers_out.cache_control).value.as_bytes(), b"private");
            assert_eq!(
                (*(*r.headers_out.cache_control).next).value.as_bytes(),
                b"no-store"
            );
            assert!((*(*r.headers_out.cache_control).next).next.is_null());
            assert_eq!(
                r.headers_out.content_type.as_bytes(),
                b"application/json; charset=utf-8"
            );
            assert_eq!(r.headers_out.content_type_len, 16);
            assert_eq!(r.headers_out.charset.len, 0);
            assert!(r.headers_out.content_type_lowcase.is_null());
            assert_eq!(r.headers_out.content_type_hash, 0);
            assert_eq!(r.headers_out.last_modified_time, -1);
            assert!(
                current
                    .iter()
                    .all(|h| (**h).key.as_bytes() != b"content-type")
            );
        }
    }

    #[test]
    fn content_type_append_is_rejected_atomically_and_removal_clears_dedicated_value() {
        let mut arena = Arena::default();
        let mut r = arena.request(true, &[]);
        r.headers_out.content_type = arena.string(b"text/plain").unwrap();
        r.headers_out.content_type_len = 10;
        r.headers_out.charset = arena.string(b"utf-8").unwrap();
        let old = r.headers_out.content_type.data;
        unsafe {
            let ms = mutation(
                &[
                    ("x-test", b"not-applied", false),
                    ("content-type", b"other", true),
                ],
                &[],
            );
            assert!(apply_header_mutations_with(&mut *r, true, &ms, &[], &mut arena).is_err());
            assert!(elements(&r.headers_out.headers).is_empty());
            assert_eq!(r.headers_out.content_type.data, old);
            assert_eq!(r.headers_out.charset.len, 5);
            let ms = mutation(&[], &["content-type"]);
            apply_header_mutations_with(&mut *r, true, &ms, &[], &mut arena).unwrap();
            assert_eq!(r.headers_out.content_type.len, 0);
            assert_eq!(r.headers_out.charset.len, 0);
            assert!(elements(&r.headers_out.headers).is_empty());
        }
    }

    #[test]
    fn singleton_empty_and_multiple_parts_have_valid_last_and_capacity() {
        for values in [
            vec![],
            vec![("x-keep", b"one".as_slice())],
            vec![("x-keep", b"one".as_slice()), ("x-two", b"two")],
        ] {
            let mut arena = Arena::default();
            let mut r = arena.request(false, &values);
            let ms = mutation(&[], &["absent"]);
            unsafe {
                apply_header_mutations_with(&mut *r, false, &ms, &[], &mut arena).unwrap();
                assert_eq!(elements(&r.headers_in.headers).len(), values.len());
                assert_eq!(r.headers_in.headers.nalloc, 1);
                // ngx_list_push's empty-list fast path is safe without calling
                // nginx: the embedded part has allocated room for one element.
                if values.is_empty() {
                    let list = &mut r.headers_in.headers;
                    assert_eq!(list.last, ptr::addr_of_mut!(list.part));
                    assert!(!list.part.elts.is_null());
                    list.part
                        .elts
                        .cast::<ngx_table_elt_t>()
                        .write(core::mem::zeroed());
                }
            }
        }
    }

    #[test]
    fn ordered_batches_and_binary_empty_values_survive_staging() {
        let mut arena = Arena::default();
        let mut r = arena.request(false, &[("x-remove", b"old")]);
        let mut ms = mutation(&[("x-remove", b"new", false)], &[]);
        ms.merge(&batch(
            &[("x-empty", b"", false), ("x-binary", &[0xff, 0x80], false)],
            &["x-remove"],
        ))
        .unwrap();
        unsafe {
            apply_header_mutations_with(&mut *r, false, &ms, &[], &mut arena).unwrap();
            let current = elements(&r.headers_in.headers);
            assert_eq!(current.len(), 2);
            assert_eq!((*current[0]).value.as_bytes(), b"");
            assert_eq!((*current[1]).value.as_bytes(), &[0xff, 0x80]);
        }
    }

    #[test]
    fn response_allocation_failures_preserve_dedicated_fields_and_untouched_chains() {
        let mut succeeded = false;
        for fail_at in 0..100 {
            let mut arena = Arena::default();
            let mut r = arena.request(
                true,
                &[
                    ("cache-control", b"private"),
                    ("cache-control", b"no-store"),
                    ("location", b"/old"),
                ],
            );
            let mut old = unsafe { elements(&r.headers_out.headers) };
            unsafe {
                (*old[0]).next = old[1];
                // Output filters need not initialize lowcase_key.
                (*old[0]).lowcase_key = ptr::null_mut();
            }
            r.headers_out.cache_control = old[0];
            r.headers_out.location = old[2];
            r.headers_out.content_type = arena.string(b"text/plain").unwrap();
            r.headers_out.content_type_len = 10;
            r.headers_out.charset = arena.string(b"utf-8").unwrap();
            r.headers_out.override_charset = ptr::addr_of_mut!(r.headers_out.charset);
            let original_type = r.headers_out.content_type.data;
            let original_override = r.headers_out.override_charset;
            arena.allocations = 0;
            arena.fail_at = Some(fail_at);
            let ms = mutation(
                &[
                    ("content-type", b"application/json", false),
                    ("x-new", b"new", false),
                ],
                &["location"],
            );
            unsafe {
                let result = apply_header_mutations_with(&mut *r, true, &ms, &[], &mut arena);
                assert_eq!(r.headers_out.cache_control, old[0]);
                assert_eq!((*old[0]).next, old[1]);
                assert!((*old[0]).lowcase_key.is_null());
                if result.is_err() {
                    assert_eq!(elements(&r.headers_out.headers), old);
                    assert_eq!(r.headers_out.location, old[2]);
                    assert_eq!((*old[2]).hash, 1);
                    assert_eq!(r.headers_out.content_type.data, original_type);
                    assert_eq!(r.headers_out.charset.len, 5);
                    assert_eq!(r.headers_out.override_charset, original_override);
                } else {
                    let current = elements(&r.headers_out.headers);
                    assert_eq!(&current[..2], &old[..2]);
                    assert!(r.headers_out.location.is_null());
                    assert!(!current.contains(&old[2]));
                    assert_eq!((*old[2]).hash, 0);
                    assert_eq!(r.headers_out.content_type.as_bytes(), b"application/json");
                    assert!(r.headers_out.override_charset.is_null());
                    succeeded = true;
                    break;
                }
            }
        }
        assert!(succeeded);
    }

    #[test]
    fn user_agent_flags_are_refreshed_then_cleared_on_removal() {
        let mut arena = Arena::default();
        let mut r = arena.request(false, &[("User-Agent", b"old")]);
        r.headers_in.user_agent = unsafe { elements(&r.headers_in.headers)[0] };
        r.headers_in.set_opera(1);
        r.headers_in.set_safari(1);
        unsafe {
            let ms = mutation(&[("user-agent", b"Mozilla MSIE 6.0 legacy", false)], &[]);
            apply_header_mutations_with(&mut *r, false, &ms, &input_metadata(), &mut arena)
                .unwrap();
            assert_eq!(r.headers_in.msie(), 1);
            assert_eq!(r.headers_in.msie6(), 1);
            assert_eq!(r.headers_in.opera(), 0);
            assert_eq!(r.headers_in.safari(), 0);
            let ms = mutation(&[], &["user-agent"]);
            apply_header_mutations_with(&mut *r, false, &ms, &input_metadata(), &mut arena)
                .unwrap();
            assert!(r.headers_in.user_agent.is_null());
            assert_eq!(r.headers_in.msie(), 0);
            assert_eq!(r.headers_in.msie6(), 0);
            assert!(elements(&r.headers_in.headers).is_empty());
        }
    }
}

#[cfg(test)]
mod tests {
    use core::sync::atomic::Ordering;

    use super::*;

    fn string(value: &[u8]) -> ngx_str_t {
        ngx_str_t {
            len: value.len(),
            data: value.as_ptr().cast_mut(),
        }
    }

    #[test]
    fn authority_preserves_original_host_port_case_and_ipv6() {
        let mut r: ngx_http_request_t = unsafe { core::mem::zeroed() };
        let mut host: ngx_table_elt_t = unsafe { core::mem::zeroed() };
        r.headers_in.server = string(b"normalized.example");
        for value in [
            b"Example.COM.:8443".as_slice(),
            b"[2001:db8::1]:443",
            b"[::1]",
            b"example:80",
        ] {
            host.value = string(value);
            r.headers_in.host = &mut host;
            assert_eq!(unsafe { request_authority(&r) }, value);
        }
        r.headers_in.host = ptr::null_mut();
        assert_eq!(unsafe { request_authority(&r) }, b"normalized.example");
        r.headers_in.server = string(b"");
        assert!(unsafe { request_authority(&r) }.is_empty());
    }

    #[test]
    fn absolute_authority_precedes_host_including_pathless_and_query_targets() {
        for (line, expected, host_len) in [
            (
                "GET http://Example.COM:8080/path HTTP/1.1",
                "Example.COM:8080",
                11,
            ),
            (
                "GET http://[2001:db8::1]:8443/path HTTP/1.1",
                "[2001:db8::1]:8443",
                13,
            ),
            (
                "GET http://Example.COM:8080 HTTP/1.1",
                "Example.COM:8080",
                11,
            ),
            (
                "GET http://Example.COM:8080?q=1 HTTP/1.1",
                "Example.COM:8080",
                11,
            ),
            ("GET http://[::1]/ HTTP/1.1", "[::1]", 5),
            ("GET http://Example.COM HTTP/1.1", "Example.COM", 11),
        ] {
            let mut r: ngx_http_request_t = unsafe { core::mem::zeroed() };
            let mut host: ngx_table_elt_t = unsafe { core::mem::zeroed() };
            host.value = string(b"different.example:9090");
            r.headers_in.host = &mut host;
            r.headers_in.server = string(b"normalized");
            r.request_line = string(line.as_bytes());
            let start = line.find("://").unwrap() + 3;
            // Match nginx parser semantics: host_end is before the port,
            // and pathless uri_start is a slash in the scheme, not the end.
            r.host_start = unsafe { r.request_line.data.add(start) };
            r.host_end = unsafe { r.host_start.add(host_len) };
            r.uri_start = unsafe { r.request_line.data.add(start - 2) };
            assert_eq!(unsafe { request_authority(&r) }, expected.as_bytes());
        }
    }

    #[test]
    fn encoded_request_uses_preserved_authority() {
        use envoy_types::pb::envoy::service::ext_proc::v3::{
            ProcessingRequest, processing_request,
        };
        use prost::Message;
        let mut r: ngx_http_request_t = unsafe { core::mem::zeroed() };
        let mut host: ngx_table_elt_t = unsafe { core::mem::zeroed() };
        host.value = string(b"[::1]:8080");
        r.headers_in.host = &mut host;
        r.headers_in.server = string(b"[::1]");
        r.method_name = string(b"POST");
        r.unparsed_uri = string(b"/v1?q=1");
        let (headers, _) = unsafe { encode_request_frames(&mut r, vec![]) }.unwrap();
        let decoded = ProcessingRequest::decode(&headers[5..]).unwrap();
        let Some(processing_request::Request::RequestHeaders(headers)) = decoded.request else {
            panic!("expected request headers");
        };
        let headers = headers.headers.unwrap().headers;
        assert_eq!(headers[2].raw_value, b"[::1]:8080");
        assert_eq!(headers[1].raw_value, b"/v1?q=1");
    }

    #[test]
    fn collection_checks_aggregate_wire_length_before_reserving() {
        let mut data = vec![1, 2, 3];
        let capacity = data.capacity();
        // usize::MAX overflows the checked_add.
        assert!(reserve_body(&mut data, usize::MAX).is_err());
        assert_eq!(data, [1, 2, 3]);
        assert_eq!(data.capacity(), capacity);
        // isize::MAX + 1 exceeds the allocation limit check.
        assert!(reserve_body(&mut data, isize::MAX as usize + 1).is_err());
        assert_eq!(data, [1, 2, 3]);
        assert_eq!(reserve_body(&mut data, 2).unwrap(), 5);
        assert_eq!(data, [1, 2, 3]);
    }

    #[test]
    fn collection_accepts_absent_body_and_empty_memory_markers() {
        let mut r: ngx_http_request_t = unsafe { core::mem::zeroed() };
        assert!(
            unsafe { collect_body_with(&mut r, |_, _, _| panic!("unexpected I/O")) }
                .unwrap()
                .is_empty()
        );
        let mut rb: nginx_sys::ngx_http_request_body_t = unsafe { core::mem::zeroed() };
        let mut buf: nginx_sys::ngx_buf_t = unsafe { core::mem::zeroed() };
        let mut link = ngx_chain_t {
            buf: &mut buf,
            next: ptr::null_mut(),
        };
        rb.bufs = &mut link;
        r.request_body = &mut rb;
        assert!(
            unsafe { collect_body_with(&mut r, |_, _, _| panic!("unexpected I/O")) }
                .unwrap()
                .is_empty()
        );
    }

    #[test]
    fn collection_concatenates_memory_and_file_without_consuming_nginx_buffers() {
        let mut r: ngx_http_request_t = unsafe { core::mem::zeroed() };
        let mut rb: nginx_sys::ngx_http_request_body_t = unsafe { core::mem::zeroed() };
        let mut memory: nginx_sys::ngx_buf_t = unsafe { core::mem::zeroed() };
        let mut file_buf: nginx_sys::ngx_buf_t = unsafe { core::mem::zeroed() };
        let mut file: nginx_sys::ngx_file_t = unsafe { core::mem::zeroed() };
        let mut bytes = *b"abc";
        memory.pos = bytes.as_mut_ptr();
        memory.last = unsafe { memory.pos.add(bytes.len()) };
        file_buf.set_in_file(1);
        file_buf.file = &mut file;
        file_buf.file_pos = 7;
        file_buf.file_last = 10;
        let mut last = ngx_chain_t {
            buf: &mut file_buf,
            next: ptr::null_mut(),
        };
        let mut first = ngx_chain_t {
            buf: &mut memory,
            next: &mut last,
        };
        rb.bufs = &mut first;
        r.request_body = &mut rb;
        let body = unsafe {
            collect_body_with(&mut r, |_, dst, offset| {
                assert_eq!(offset, 7);
                dst.copy_from_slice(b"def");
                3
            })
        }
        .unwrap();
        assert_eq!(body, b"abcdef");
        assert_eq!(memory.pos, bytes.as_mut_ptr());
        assert_eq!(file_buf.file_pos, 7);
        for result in [-1, 0, 2] {
            assert!(unsafe { collect_body_with(&mut r, |_, _, _| result) }.is_err());
        }
    }

    #[test]
    fn collection_rejects_invalid_or_unrepresentable_ranges_before_io() {
        let mut r: ngx_http_request_t = unsafe { core::mem::zeroed() };
        let mut rb: nginx_sys::ngx_http_request_body_t = unsafe { core::mem::zeroed() };
        let mut buf: nginx_sys::ngx_buf_t = unsafe { core::mem::zeroed() };
        let mut file: nginx_sys::ngx_file_t = unsafe { core::mem::zeroed() };
        let mut link = ngx_chain_t {
            buf: &mut buf,
            next: ptr::null_mut(),
        };
        rb.bufs = &mut link;
        r.request_body = &mut rb;
        buf.set_in_file(1);
        buf.file = &mut file;
        for (start, end) in [(2, 1), (-1, i64::MAX), (0, i64::MAX), (i64::MAX, i64::MIN)] {
            buf.file_pos = start;
            buf.file_last = end;
            assert!(
                unsafe { collect_body_with(&mut r, |_, _, _| panic!("unexpected I/O")) }.is_err()
            );
        }
        buf.file_pos = 0;
        buf.file_last = 1;
        buf.file = ptr::null_mut();
        assert!(unsafe { collect_body_with(&mut r, |_, _, _| panic!("unexpected I/O")) }.is_err());
        buf.set_in_file(0);
        let mut bytes = [0; 2];
        unsafe {
            (*link.buf).pos = bytes.as_mut_ptr().add(2);
            (*link.buf).last = bytes.as_mut_ptr();
        }
        assert!(unsafe { collect_body_with(&mut r, |_, _, _| panic!("unexpected I/O")) }.is_err());
        unsafe { (*link.buf).pos = ptr::null_mut() };
        assert!(unsafe { collect_body_with(&mut r, |_, _, _| panic!("unexpected I/O")) }.is_err());
    }

    #[test]
    fn cleanup_unlinks_posted_event_and_preserves_endpoint_until_pool_cleanup() {
        let mut ctx = Box::new(RequestCtx::new(ResponsePhase::Headers));
        ctx.endpoint = Some("backend:8080".into());
        let mut queue: nginx_sys::ngx_queue_t = unsafe { core::mem::zeroed() };
        let head = ptr::from_mut(&mut queue);
        queue.prev = head;
        queue.next = head;
        let raw = ptr::from_mut(&mut *ctx).cast();
        unsafe {
            ngx_post_event(&mut ctx.response_event, head);
            // Duplicate posts must not link the same event twice.
            ngx_post_event(&mut ctx.response_event, head);
            assert_eq!(ctx.response_event.posted(), 1);
            request_ctx_cancel_task(raw);
            assert_eq!(ctx.response_event.posted(), 0);
            assert_eq!(queue.next, head);
            assert_eq!(queue.prev, head);
            assert!(ctx.response_failed);
            assert_eq!(ctx.endpoint.as_deref(), Some("backend:8080"));
            request_ctx_cancel_task(raw);
            assert_eq!(queue.next, head);
            request_ctx_pool_cleanup(Box::into_raw(ctx).cast());
            assert_eq!(queue.next, head);
            assert_eq!(queue.prev, head);
        }
    }

    #[test]
    fn response_buffer_failure_does_not_consume_rejected_buffer() {
        let mut body = ResponseBuffer::default();
        body.push(&vec![0; MAX_RESPONSE_BYTES], ResponseFlags::default())
            .unwrap();
        let mut bytes = *b"x";
        let mut buf: nginx_sys::ngx_buf_t = unsafe { core::mem::zeroed() };
        buf.set_memory(1);
        buf.pos = bytes.as_mut_ptr();
        buf.last = unsafe { buf.pos.add(1) };
        let mut link = ngx_chain_t {
            buf: &mut buf,
            next: ptr::null_mut(),
        };
        assert!(unsafe { buffer_response_chain(&mut body, &mut link) }.is_err());
        assert_eq!(buf.pos, bytes.as_mut_ptr());
        assert_eq!(body.chunks.len(), 1);
        assert_eq!(body.bytes, MAX_RESPONSE_BYTES);
    }

    #[test]
    fn pool_cleanup_unlinks_event_and_drops_waker_without_waking() {
        use std::sync::{Arc, atomic::AtomicUsize};
        use std::task::Wake;

        struct Signal(Arc<AtomicUsize>);
        impl Wake for Signal {
            fn wake(self: Arc<Self>) {
                self.0.fetch_add(100, Ordering::SeqCst);
            }
        }
        impl Drop for Signal {
            fn drop(&mut self) {
                self.0.fetch_add(1, Ordering::SeqCst);
            }
        }

        let calls = Arc::new(AtomicUsize::new(0));
        let mut ctx = Box::new(RequestCtx::new(ResponsePhase::Headers));
        ctx.notify_waker = Some(Waker::from(Arc::new(Signal(calls.clone()))));
        let mut queue: nginx_sys::ngx_queue_t = unsafe { core::mem::zeroed() };
        let head = ptr::from_mut(&mut queue);
        queue.prev = head;
        queue.next = head;
        unsafe {
            ngx_post_event(&mut ctx.response_event, head);
            // Exercise pool cleanup directly, as when HTTP cleanup registration
            // failed. It must remove the event before freeing its embedded node.
            request_ctx_pool_cleanup(Box::into_raw(ctx).cast());
        }
        assert_eq!(queue.next, head);
        assert_eq!(queue.prev, head);
        assert_eq!(calls.load(Ordering::SeqCst), 1);
    }

    #[test]
    fn response_buffer_rejects_invalid_dual_backed_ranges_without_consuming() {
        let mut bytes = *b"x";
        let mut buf: nginx_sys::ngx_buf_t = unsafe { core::mem::zeroed() };
        buf.set_memory(1);
        buf.set_in_file(1);
        buf.pos = bytes.as_mut_ptr();
        buf.last = unsafe { buf.pos.add(1) };
        let mut link = ngx_chain_t {
            buf: &mut buf,
            next: ptr::null_mut(),
        };
        for (start, end) in [(1, 0), (0, 2), (i64::MIN, i64::MAX)] {
            buf.file_pos = start;
            buf.file_last = end;
            let mut body = ResponseBuffer::default();
            assert!(unsafe { buffer_response_chain(&mut body, &mut link) }.is_err());
            assert_eq!(buf.pos, bytes.as_mut_ptr());
            assert_eq!(buf.file_pos, start);
            assert!(body.chunks.is_empty());
        }
    }

    #[test]
    fn response_buffer_byte_limit_is_atomic() {
        let mut body = ResponseBuffer::default();
        body.push(&vec![7; MAX_RESPONSE_BYTES], ResponseFlags::default())
            .unwrap();
        assert!(body.push(b"x", ResponseFlags::default()).is_err());
        assert_eq!(body.bytes, MAX_RESPONSE_BYTES);
        assert_eq!(body.chunks.len(), 1);
        assert_eq!(body.chunks[0].data.len(), MAX_RESPONSE_BYTES);
    }

    #[test]
    fn response_buffer_bounds_empty_markers() {
        let mut body = ResponseBuffer::default();
        let flags = ResponseFlags {
            flush: true,
            ..ResponseFlags::default()
        };
        for _ in 0..MAX_RESPONSE_CHUNKS {
            body.push(&[], flags).unwrap();
        }
        assert!(body.push(&[], flags).is_err());
        assert_eq!(body.bytes, 0);
        assert_eq!(body.chunks.len(), MAX_RESPONSE_CHUNKS);
    }

    #[test]
    fn response_buffer_preserves_flags_and_empty_eof() {
        let mut body = ResponseBuffer::default();
        let flags = ResponseFlags {
            flush: true,
            sync: true,
            last_buf: true,
            last_in_chain: true,
        };
        body.push(b"first", ResponseFlags::default()).unwrap();
        body.push(&[], flags).unwrap();
        assert_eq!(body.chunks[0].data, b"first");
        assert_eq!(body.chunks[0].flags, ResponseFlags::default());
        assert!(body.chunks[1].data.is_empty());
        assert_eq!(body.chunks[1].flags, flags);
    }

    #[test]
    fn response_buffer_ignores_empty_non_markers() {
        let mut body = ResponseBuffer::default();
        for _ in 0..MAX_RESPONSE_CHUNKS + 1 {
            body.push(&[], ResponseFlags::default()).unwrap();
        }
        assert!(body.chunks.is_empty());
        assert_eq!(body.bytes, 0);
    }

    #[test]
    fn response_buffer_rejects_file_only_without_consuming() {
        let mut body = ResponseBuffer::default();
        let mut buf: ngx::ffi::ngx_buf_t = unsafe { core::mem::zeroed() };
        buf.set_in_file(1);
        buf.file_last = 10;
        let mut chain = ngx_chain_t {
            buf: &mut buf,
            next: ptr::null_mut(),
        };
        assert!(unsafe { buffer_response_chain(&mut body, &mut chain) }.is_err());
        assert_eq!(buf.file_pos, 0);
        assert!(body.chunks.is_empty());
    }

    #[test]
    fn response_buffer_consumes_memory_and_preserves_eof() {
        let mut body = ResponseBuffer::default();
        let mut data = *b"abc";
        let mut buf: ngx::ffi::ngx_buf_t = unsafe { core::mem::zeroed() };
        buf.set_memory(1);
        buf.set_last_buf(1);
        buf.pos = data.as_mut_ptr();
        buf.last = unsafe { buf.pos.add(data.len()) };
        let mut chain = ngx_chain_t {
            buf: &mut buf,
            next: ptr::null_mut(),
        };
        unsafe { buffer_response_chain(&mut body, &mut chain) }.unwrap();
        assert_eq!(buf.pos, buf.last);
        assert_eq!(body.chunks[0].data, b"abc");
        assert!(body.chunks[0].flags.last_buf);
    }

    #[test]
    fn parse_endpoint_addr_ipv4() {
        let result = parse_endpoint_addr("10.96.0.42:9002").unwrap();
        match result {
            ResolvedAddr::V4(sa) => {
                assert_eq!(sa.sin_family as i32, libc::AF_INET);
                assert_eq!(sa.sin_port, 9002u16.to_be());
                assert_eq!(sa.sin_addr.s_addr, u32::from_ne_bytes([10, 96, 0, 42]));
            }
            ResolvedAddr::V6(_) => panic!("expected V4"),
        }
    }

    #[test]
    fn parse_endpoint_addr_ipv6() {
        let result = parse_endpoint_addr("[::1]:443").unwrap();
        match result {
            ResolvedAddr::V6(sa) => {
                assert_eq!(sa.sin6_family as i32, libc::AF_INET6);
                assert_eq!(sa.sin6_port, 443u16.to_be());
                assert_eq!(
                    sa.sin6_addr.s6_addr,
                    [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1]
                );
            }
            ResolvedAddr::V4(_) => panic!("expected V6"),
        }
    }

    #[test]
    fn parse_endpoint_addr_rejects_hostname() {
        assert!(parse_endpoint_addr("my-svc.ns.svc.cluster.local:9002").is_err());
    }

    #[test]
    fn parse_endpoint_addr_rejects_missing_port() {
        assert!(parse_endpoint_addr("10.96.0.42").is_err());
    }
}
