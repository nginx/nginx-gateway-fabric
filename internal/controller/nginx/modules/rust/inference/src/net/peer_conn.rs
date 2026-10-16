//! Async peer connection wrapping `ngx_peer_connection_t`.
//!
//! Implements `tokio::io::AsyncRead` and `tokio::io::AsyncWrite` so the h2 crate
//! can drive HTTP/2 framing directly on nginx's event loop. On `EAGAIN`, wakers are
//! stored and nginx event handlers wake the task when the socket becomes ready.

use core::ffi::{CStr, c_int};
use core::pin::Pin;
use core::ptr::{self, NonNull};
use core::task::{self, Poll};
use core::{fmt, future, mem};
use std::io;

use nginx_sys::{
    ngx_addr_t, ngx_connection_t, ngx_destroy_pool, ngx_event_connect_peer, ngx_event_get_peer,
    ngx_int_t, ngx_log_t, ngx_peer_connection_t, ngx_pool_t, ngx_ssl_shutdown, ngx_ssl_t,
    ngx_str_t,
};
use ngx::core::Status;
use openssl_sys::{
    SSL_get_verify_mode, SSL_get_verify_result, X509_VERIFY_PARAM_set1_host,
    X509_VERIFY_PARAM_set1_ip,
};

use super::connection::{Connection, ConnectionLogError};
use super::pool::OwnedPool;

/// Error returned when SSL certificate verification fails after the TLS handshake.
#[derive(Debug)]
pub(crate) struct SslVerifyError(c_int);

impl core::error::Error for SslVerifyError {}

impl fmt::Display for SslVerifyError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "SSL certificate verify error ({})", self.0)
    }
}

/// Async wrapper over `ngx_peer_connection_t`.
///
/// Owns a memory pool, the peer connection state, and optional wakers for
/// read/write events. When nginx fires an event on the underlying socket,
/// the stored waker is taken and woken, which re-polls the future.
pub(crate) struct PeerConnection {
    pool: OwnedPool,
    pc: ngx_peer_connection_t,
    rev: Option<task::Waker>,
    wev: Option<task::Waker>,
}

// -- tokio::io::AsyncRead implementation --

impl tokio::io::AsyncRead for PeerConnection {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
        buf: &mut tokio::io::ReadBuf<'_>,
    ) -> Poll<io::Result<()>> {
        let Some(c) = self.connection_mut() else {
            return Poll::Ready(Err(io::ErrorKind::InvalidInput.into()));
        };

        if c.read().timedout() != 0 {
            return Poll::Ready(Err(io::ErrorKind::TimedOut.into()));
        }

        // SAFETY: recv only writes into the slice and never de-initializes it;
        // exactly `n` returned bytes are marked initialized below.
        let n = c.recv(unsafe { buf.unfilled_mut() });

        if n == nginx_sys::NGX_ERROR as isize {
            return Poll::Ready(Err(io::Error::last_os_error()));
        }

        let rev = c.read();
        if Status(unsafe { nginx_sys::ngx_handle_read_event(rev, 0) }) != Status::NGX_OK {
            return Poll::Ready(Err(io::Error::other("ngx_handle_read_event failed")));
        }

        if n == nginx_sys::NGX_AGAIN as isize {
            self.rev = Some(cx.waker().clone());
            return Poll::Pending;
        }

        if let Ok(n) = usize::try_from(n)
            && n > 0
        {
            // SAFETY: recv initialized the first `n` bytes of the unfilled region.
            unsafe { buf.assume_init(n) };
            buf.advance(n);
        }

        Poll::Ready(Ok(()))
    }
}

// -- tokio::io::AsyncWrite implementation --

impl tokio::io::AsyncWrite for PeerConnection {
    fn poll_write(
        mut self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
        buf: &[u8],
    ) -> Poll<io::Result<usize>> {
        let Some(c) = self.connection_mut() else {
            return Poll::Ready(Err(io::ErrorKind::InvalidInput.into()));
        };

        let n = c.send(buf);

        if n == nginx_sys::NGX_AGAIN as ngx_int_t {
            self.wev = Some(cx.waker().clone());
            Poll::Pending
        } else if n > 0 {
            Poll::Ready(Ok(n as usize))
        } else if n == nginx_sys::NGX_ERROR as ngx_int_t {
            Poll::Ready(Err(io::Error::last_os_error()))
        } else {
            Poll::Ready(Err(io::ErrorKind::WriteZero.into()))
        }
    }

    fn poll_flush(self: Pin<&mut Self>, _cx: &mut task::Context<'_>) -> Poll<io::Result<()>> {
        Poll::Ready(Ok(()))
    }

    fn poll_shutdown(self: Pin<&mut Self>, cx: &mut task::Context<'_>) -> Poll<io::Result<()>> {
        self.poll_ssl_shutdown(cx)
    }
}

// -- Core PeerConnection implementation --

impl PeerConnection {
    /// Create a new peer connection with its own memory pool.
    ///
    /// Allocates a pool, copies the log, and initializes the `ngx_peer_connection_t`
    /// with the default `ngx_event_get_peer` callback.
    pub(crate) fn new(log: NonNull<ngx_log_t>) -> Result<Self, io::Error> {
        let pool = OwnedPool::with_default_size(log).map_err(|_| io::ErrorKind::OutOfMemory)?;

        // Copy the log so we don't modify log.connection on a shared cycle log.
        let new_log = {
            let mut new_log = unsafe { log.read() };
            new_log.action = ptr::null_mut();
            new_log.data = ptr::null_mut();
            new_log.handler = Some(Self::log_handler);

            // Allocate the log copy in our pool.
            let p = pool.as_ptr();
            let log_ptr =
                unsafe { nginx_sys::ngx_palloc(p, mem::size_of::<ngx_log_t>()) } as *mut ngx_log_t;
            if log_ptr.is_null() {
                return Err(io::ErrorKind::OutOfMemory.into());
            }
            unsafe { log_ptr.write(new_log) };
            log_ptr
        };

        unsafe { (*pool.as_ptr()).log = new_log };

        let mut this = Self {
            pool,
            pc: unsafe { mem::zeroed() },
            rev: None,
            wev: None,
        };

        let pc = &mut this.pc;
        pc.get = Some(ngx_event_get_peer);
        pc.log = new_log;
        pc.set_log_error(ConnectionLogError::Info as _);

        Ok(this)
    }

    /// Initiate an async TCP connect to the given address.
    ///
    /// Copies the sockaddr into pool memory and polls until connected.
    pub(crate) async fn connect(
        mut self: Pin<&mut Self>,
        addr: &ngx_addr_t,
    ) -> Result<(), io::Error> {
        let addr = copy_sockaddr(self.pool.as_ptr(), addr)?;

        // Allocate the name in pool memory so the pointer remains valid.
        let name_ptr = unsafe {
            let p = nginx_sys::ngx_palloc(self.pool.as_ptr(), mem::size_of::<ngx_str_t>())
                as *mut ngx_str_t;
            if p.is_null() {
                return Err(io::ErrorKind::OutOfMemory.into());
            }
            p.write(addr.name);
            p
        };

        self.pc.name = name_ptr;
        self.pc.sockaddr = addr.sockaddr;
        self.pc.socklen = addr.socklen;

        future::poll_fn(|cx| self.as_mut().poll_connect(cx)).await
    }

    /// Call `ngx_event_connect_peer` and install our event handlers.
    fn connect_peer(&mut self) -> Status {
        let rc = Status(unsafe { ngx_event_connect_peer(&mut self.pc) });

        if rc == Status::NGX_ERROR || rc == Status::NGX_BUSY || rc == Status::NGX_DECLINED {
            return rc;
        }

        let c = unsafe { &mut *self.pc.connection };
        c.data = ptr::from_mut(self).cast();

        if c.pool.is_null() {
            c.pool = self.pool.as_ptr();
        }

        unsafe {
            (*c.log).connection = c.number;
            (*c.read).handler = Some(ngx_peer_conn_read_handler);
            (*c.write).handler = Some(ngx_peer_conn_write_handler);
        }

        rc
    }

    /// Poll for TCP connect completion.
    ///
    /// On first call, initiates the connect. On subsequent calls, checks whether
    /// the connect has completed or timed out.
    fn poll_connect(
        mut self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        // If we already have a connection, this is a re-poll after EAGAIN.
        if let Some(c) = self.connection_mut() {
            let rv = if c.read().timedout() != 0 || c.write().timedout() != 0 {
                Err(io::ErrorKind::TimedOut.into())
            } else if let Err(err) = c.test_connect() {
                Err(io::Error::from_raw_os_error(err))
            } else {
                c.read().handler = Some(ngx_peer_conn_read_handler);
                c.write().handler = Some(ngx_peer_conn_write_handler);
                Ok(())
            };

            self.unset_log_action();
            return Poll::Ready(rv);
        }

        // First call: initiate the connect.
        self.set_log_action(c"connecting");

        match self.connect_peer() {
            Status::NGX_OK => {
                let Some(c) = self.connection_mut() else {
                    return Poll::Ready(Err(io::ErrorKind::NotConnected.into()));
                };
                c.read().handler = Some(ngx_peer_conn_read_handler);
                c.write().handler = Some(ngx_peer_conn_write_handler);
                self.unset_log_action();
                Poll::Ready(Ok(()))
            }
            Status::NGX_AGAIN => {
                // Store wakers — nginx will fire the event handler when the
                // socket becomes writable (connect completed) or readable (error).
                self.rev = Some(cx.waker().clone());
                self.wev = Some(cx.waker().clone());
                Poll::Pending
            }
            _ => Poll::Ready(Err(io::ErrorKind::ConnectionRefused.into())),
        }
    }

    /// Poll for TLS handshake completion.
    ///
    /// Creates the SSL connection on first call, then drives `ngx_ssl_handshake`
    /// to completion. After a successful handshake, verifies the peer certificate
    /// if verification mode is enabled and requires negotiated ALPN h2.
    pub(crate) fn poll_ssl_handshake(
        mut self: Pin<&mut Self>,
        ssl: &ngx_ssl_t,
        ssl_name: Option<&CStr>,
        cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        let Some(c) = (unsafe { self.pc.connection.as_mut() }) else {
            return Poll::Ready(Err(io::ErrorKind::NotConnected.into()));
        };
        let c = unsafe { Connection::from_ptr_mut(c) };

        if c.ssl.is_null() {
            self.set_log_action(c"SSL handshaking");
            self.ssl_create_connection(ssl, ssl_name)?;
            unsafe { nginx_sys::ngx_reusable_connection(c.as_mut(), 0) };
        }

        match Status(unsafe { nginx_sys::ngx_ssl_handshake(c.as_mut()) } as _) {
            Status::NGX_OK => {
                let ssl_conn = unsafe { (*c.ssl).connection.cast() };

                // nginx's verify callback allows the handshake to finish;
                // check the result explicitly.
                if unsafe { SSL_get_verify_mode(ssl_conn) } != openssl_sys::SSL_VERIFY_NONE {
                    let rc = unsafe { SSL_get_verify_result(ssl_conn) } as c_int;
                    if rc != openssl_sys::X509_V_OK {
                        self.close();
                        return Poll::Ready(Err(io::Error::other(SslVerifyError(rc))));
                    }
                }

                // Advertising h2 is not enough: the peer must select it, even
                // when certificate verification is disabled.
                let mut selected = ptr::null();
                let mut selected_len = 0;
                unsafe {
                    openssl_sys::SSL_get0_alpn_selected(ssl_conn, &mut selected, &mut selected_len);
                }
                let selected = if selected.is_null() || selected_len == 0 {
                    &[][..]
                } else {
                    // OpenSSL owns these bytes until the SSL object is freed.
                    unsafe { core::slice::from_raw_parts(selected, selected_len as usize) }
                };
                if let Err(err) = validate_h2_alpn(selected) {
                    self.close();
                    return Poll::Ready(Err(err));
                }

                c.read().handler = Some(ngx_peer_conn_read_handler);
                c.write().handler = Some(ngx_peer_conn_write_handler);
                self.unset_log_action();
                Poll::Ready(Ok(()))
            }
            Status::NGX_AGAIN => {
                unsafe { (*c.ssl).handler = Some(ngx_peer_conn_ssl_handler) };
                self.rev = Some(cx.waker().clone());
                Poll::Pending
            }
            _ => Poll::Ready(Err(io::ErrorKind::ConnectionRefused.into())),
        }
    }

    /// Set up the TLS connection with SNI, ALPN h2, and verification parameters.
    fn ssl_create_connection(
        &mut self,
        ssl: &ngx_ssl_t,
        ssl_name: Option<&CStr>,
    ) -> Result<(), io::Error> {
        const FLAGS: usize = (nginx_sys::NGX_SSL_CLIENT | nginx_sys::NGX_SSL_BUFFER) as _;

        let Some(c) = (unsafe { self.pc.connection.as_mut() }) else {
            return Err(io::ErrorKind::NotConnected.into());
        };

        // ngx_ssl_create_connection increments the SSL_CTX reference count.
        let sslp = ptr::from_ref(ssl).cast_mut();
        if Status(unsafe { nginx_sys::ngx_ssl_create_connection(sslp, c, FLAGS) }) != Status::NGX_OK
        {
            return Err(io::ErrorKind::ConnectionRefused.into());
        }

        let ssl_conn: *mut openssl_sys::SSL = unsafe { (*c.ssl).connection.cast() };

        // Set ALPN to h2 for HTTP/2 negotiation.
        // Wire format: length-prefixed protocol list: \x02h2
        let alpn: &[u8] = b"\x02h2";
        if unsafe { openssl_sys::SSL_set_alpn_protos(ssl_conn, alpn.as_ptr(), alpn.len() as _) }
            != 0
        {
            return Err(io::Error::other("SSL_set_alpn_protos failed"));
        }

        // Set SNI hostname if provided.
        if let Some(name) = ssl_name
            && unsafe { openssl_sys::SSL_set_tlsext_host_name(ssl_conn, name.as_ptr().cast_mut()) }
                != 1
        {
            return Err(openssl::error::ErrorStack::get().into());
        }

        // Set up hostname/IP verification parameters if verify mode is enabled.
        if unsafe { SSL_get_verify_mode(ssl_conn) } != openssl_sys::SSL_VERIFY_NONE {
            let vp = unsafe { openssl_sys::SSL_get0_param(ssl_conn) };

            if let Some(name) = ssl_name {
                // Hostname provided — verify against the certificate SAN/CN.
                // Do NOT also set IP verification; the EPP cert is issued for
                // the hostname, not the resolved IP address.
                if unsafe { X509_VERIFY_PARAM_set1_host(vp, name.as_ptr(), name.count_bytes()) }
                    != 1
                {
                    return Err(openssl::error::ErrorStack::get().into());
                }
            } else {
                // No hostname — fall back to IP-based verification against the
                // peer's socket address.
                let mut addr_buf = [0u8; 16];
                if let Some(ip) = unsafe {
                    self.pc
                        .sockaddr
                        .cast::<libc::sockaddr>()
                        .as_ref()
                        .and_then(|x| sockaddr_to_in_addr_buf(x, &mut addr_buf))
                } && unsafe { X509_VERIFY_PARAM_set1_ip(vp, ip.as_ptr(), ip.len()) } != 1
                {
                    return Err(openssl::error::ErrorStack::get().into());
                }
            }
        }

        Ok(())
    }

    /// Poll for graceful SSL shutdown.
    fn poll_ssl_shutdown(
        mut self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        self.set_log_action(c"closing connection");

        let Some(c) = self.connection_mut() else {
            return Poll::Ready(Ok(()));
        };

        if !c.ssl.is_null() {
            let rc = Status(unsafe { ngx_ssl_shutdown(c.as_mut()) });
            if rc == Status::NGX_AGAIN {
                unsafe { (*c.ssl).handler = Some(ngx_peer_conn_ssl_shutdown_handler) };
                self.rev = Some(cx.waker().clone());
                return Poll::Pending;
            }
        }

        self.release_connection();
        Poll::Ready(Ok(()))
    }

    /// Close the nginx connection and destroy its pool if it is not ours.
    fn release_connection(&mut self) {
        let Some(c) = self.connection_mut() else {
            return;
        };
        let pool = c.pool;
        c.close();
        self.pc.connection = ptr::null_mut();

        if !pool.is_null() && !ptr::eq::<ngx_pool_t>(self.pool.as_ptr(), pool) {
            unsafe { ngx_destroy_pool(pool) };
        }
    }

    /// Get a mutable reference to the underlying connection, if one exists.
    pub(crate) fn connection_mut(&mut self) -> Option<&mut Connection> {
        if self.pc.connection.is_null() {
            None
        } else {
            Some(unsafe { Connection::from_ptr_mut(self.pc.connection) })
        }
    }

    /// Close the connection, performing an immediate SSL shutdown if needed.
    fn close(&mut self) {
        let Some(c) = self.connection_mut() else {
            return;
        };

        if !c.ssl.is_null() {
            unsafe {
                prepare_immediate_ssl_shutdown(&mut *c.ssl);
                let _ = ngx_ssl_shutdown(c.as_mut());
            };
        }

        self.release_connection();
    }

    /// Set a log action string for the current operation.
    fn set_log_action(self: &Pin<&mut Self>, action: &'static CStr) {
        if let Some(log) = unsafe { self.pc.log.as_mut() } {
            log.data = ptr::from_ref(&self.pc).cast_mut().cast();
            log.action = action.as_ptr().cast_mut().cast();
        }
    }

    /// Clear the log action string.
    fn unset_log_action(&self) {
        if let Some(log) = unsafe { self.pc.log.as_mut() } {
            log.action = ptr::null_mut();
        }
    }

    /// Custom log handler that appends connection context (server name, address)
    /// to error messages.
    unsafe extern "C" fn log_handler(
        log: *mut ngx_log_t,
        mut buf: *mut u8,
        mut len: usize,
    ) -> *mut u8 {
        unsafe {
            let log = &mut *log;
            let Some(pc) = log.data.cast::<ngx_peer_connection_t>().as_ref() else {
                return buf;
            };

            if !log.action.is_null() {
                let p = nginx_sys::ngx_snprintf(buf, len, c" while %s".as_ptr(), log.action);
                len = len.saturating_sub(p.offset_from(buf) as usize);
                buf = p;
            }

            if !pc.name.is_null() {
                let p = nginx_sys::ngx_snprintf(buf, len, c", server: %V".as_ptr(), pc.name);
                len = len.saturating_sub(p.offset_from(buf) as usize);
                buf = p;
            }

            if pc.socklen != 0 {
                let p = nginx_sys::ngx_snprintf(buf, len, c", addr: ".as_ptr());
                len = len.saturating_sub(p.offset_from(buf) as usize);

                let n = nginx_sys::ngx_sock_ntop(pc.sockaddr, pc.socklen, p, len, 1);
                buf = p.byte_add(n);
            }

            buf
        }
    }
}

impl Drop for PeerConnection {
    fn drop(&mut self) {
        self.close();
    }
}

// -- Event handlers --
// These are installed on the nginx connection events. When nginx detects I/O readiness,
// it calls the handler which wakes the stored waker, causing the async task to re-poll.

/// Resolve the owning `PeerConnection` from an nginx connection, or `None`
/// if the connection has already been detached (e.g. after close).
unsafe fn peer_of(c: *mut ngx_connection_t) -> Option<&'static mut PeerConnection> {
    unsafe {
        if c.is_null() {
            return None;
        }
        (*c).data.cast::<PeerConnection>().as_mut()
    }
}

unsafe extern "C" fn ngx_peer_conn_read_handler(ev: *mut nginx_sys::ngx_event_t) {
    unsafe {
        if let Some(this) = peer_of((*ev).data.cast())
            && let Some(waker) = this.rev.take()
        {
            waker.wake();
        }
    }
}

unsafe extern "C" fn ngx_peer_conn_write_handler(ev: *mut nginx_sys::ngx_event_t) {
    unsafe {
        let Some(this) = peer_of((*ev).data.cast()) else {
            return;
        };
        match this.wev.take() {
            Some(waker) => waker.wake(),
            // Write events posted from ngx_event_openssl code with no waiter:
            // re-arm the event so it is not lost (best effort).
            None => {
                let _ = nginx_sys::ngx_handle_write_event(ev, 0);
            }
        }
    }
}

unsafe extern "C" fn ngx_peer_conn_ssl_handler(c: *mut ngx_connection_t) {
    unsafe {
        if let Some(this) = peer_of(c)
            && let Some(waker) = this.rev.take()
        {
            waker.wake();
        }
    }
}

unsafe extern "C" fn ngx_peer_conn_ssl_shutdown_handler(c: *mut ngx_connection_t) {
    unsafe {
        let Some(this) = peer_of(c) else {
            return;
        };
        (*(*c).read).handler = Some(ngx_peer_conn_read_handler);
        (*(*c).write).handler = Some(ngx_peer_conn_write_handler);

        if let Some(waker) = this.rev.take() {
            waker.wake();
        }
    }
}

// -- Helpers --

/// Prevent nginx from deferring SSL cleanup while the connection is destroyed.
fn prepare_immediate_ssl_shutdown(ssl: &mut nginx_sys::ngx_ssl_connection_t) {
    ssl.set_no_wait_shutdown(1);
    // Without this, sending close_notify can return NGX_AGAIN and leak SSL
    // state when the socket and pool are immediately closed below.
    ssl.set_no_send_shutdown(1);
}

/// TLS gRPC requires an exact h2 ALPN selection; no selection is not a fallback.
fn validate_h2_alpn(selected: &[u8]) -> Result<(), io::Error> {
    if selected == b"h2" {
        Ok(())
    } else {
        Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "EPP TLS peer did not negotiate ALPN h2",
        ))
    }
}

/// Copy a sockaddr into pool-allocated memory so the pointer remains valid
/// for the lifetime of the peer connection.
fn copy_sockaddr(pool: *mut ngx_pool_t, addr: &ngx_addr_t) -> Result<ngx_addr_t, io::Error> {
    let sockaddr =
        unsafe { nginx_sys::ngx_palloc(pool, addr.socklen as usize) } as *mut nginx_sys::sockaddr;
    if sockaddr.is_null() {
        return Err(io::ErrorKind::OutOfMemory.into());
    }

    unsafe {
        addr.sockaddr
            .cast::<u8>()
            .copy_to_nonoverlapping(sockaddr.cast(), addr.socklen as usize);
    }

    // Copy the name string into pool memory.
    let name = if addr.name.len > 0 && !addr.name.data.is_null() {
        let data = unsafe { nginx_sys::ngx_palloc(pool, addr.name.len) } as *mut u8;
        if data.is_null() {
            return Err(io::ErrorKind::OutOfMemory.into());
        }
        unsafe { addr.name.data.copy_to_nonoverlapping(data, addr.name.len) };
        ngx_str_t {
            len: addr.name.len,
            data,
        }
    } else {
        ngx_str_t {
            len: 0,
            data: ptr::null_mut(),
        }
    };

    Ok(ngx_addr_t {
        sockaddr,
        socklen: addr.socklen,
        name,
    })
}

/// Extract the binary IP address from a `sockaddr` for X509 verification.
///
/// Returns the 4-byte IPv4 or 16-byte IPv6 address, matching the format
/// expected by `X509_VERIFY_PARAM_set1_ip`.
fn sockaddr_to_in_addr_buf<'a>(sa: &libc::sockaddr, out: &'a mut [u8; 16]) -> Option<&'a [u8]> {
    match sa.sa_family as c_int {
        libc::AF_INET => {
            let sin: &libc::sockaddr_in = unsafe { NonNull::from(sa).cast().as_ref() };
            out[..4].copy_from_slice(&sin.sin_addr.s_addr.to_ne_bytes());
            Some(&out[..4])
        }
        libc::AF_INET6 => {
            let sin6: &libc::sockaddr_in6 = unsafe { NonNull::from(sa).cast().as_ref() };
            out.copy_from_slice(&sin6.sin6_addr.s6_addr);
            Some(out)
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn immediate_ssl_shutdown_disables_wait_and_send() {
        let mut ssl: nginx_sys::ngx_ssl_connection_t = unsafe { mem::zeroed() };
        prepare_immediate_ssl_shutdown(&mut ssl);
        assert_eq!(ssl.no_wait_shutdown(), 1);
        assert_eq!(ssl.no_send_shutdown(), 1);
    }

    #[test]
    fn negotiated_alpn_requires_exact_h2() {
        assert!(validate_h2_alpn(b"h2").is_ok());
        for selected in [
            b"".as_slice(),
            b"http/1.1",
            b"h2c",
            b"H2",
            b"\x02h2",
            b"h2\0",
        ] {
            let err = validate_h2_alpn(selected).unwrap_err();
            assert_eq!(err.kind(), io::ErrorKind::InvalidData);
        }
    }

    #[test]
    fn sockaddr_to_in_addr_buf_ipv4() {
        let mut sin: libc::sockaddr_in = unsafe { mem::zeroed() };
        sin.sin_family = libc::AF_INET as _;
        sin.sin_addr.s_addr = u32::from_ne_bytes([127, 0, 0, 1]);
        let sa: &libc::sockaddr = unsafe { &*ptr::from_ref(&sin).cast() };
        let mut buf = [0u8; 16];
        let result = sockaddr_to_in_addr_buf(sa, &mut buf);
        assert!(result.is_some());
        assert_eq!(result.unwrap(), &[127, 0, 0, 1]);
    }

    #[test]
    fn sockaddr_to_in_addr_buf_ipv6() {
        let mut sin6: libc::sockaddr_in6 = unsafe { mem::zeroed() };
        sin6.sin6_family = libc::AF_INET6 as _;
        sin6.sin6_addr.s6_addr = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1];
        let sa: &libc::sockaddr = unsafe { &*ptr::from_ref(&sin6).cast() };
        let mut buf = [0u8; 16];
        let result = sockaddr_to_in_addr_buf(sa, &mut buf);
        assert!(result.is_some());
        assert_eq!(
            result.unwrap(),
            &[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1]
        );
    }

    #[test]
    fn sockaddr_to_in_addr_buf_unknown_family() {
        let mut sa: libc::sockaddr = unsafe { mem::zeroed() };
        sa.sa_family = libc::AF_UNIX as _;
        let mut buf = [0u8; 16];
        assert!(sockaddr_to_in_addr_buf(&sa, &mut buf).is_none());
    }

    #[test]
    fn ssl_verify_error_display() {
        let err = SslVerifyError(20);
        assert_eq!(format!("{err}"), "SSL certificate verify error (20)");
    }
}
