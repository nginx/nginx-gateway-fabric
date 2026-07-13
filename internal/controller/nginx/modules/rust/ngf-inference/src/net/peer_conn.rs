// Async peer connection wrapper for nginx's ngx_peer_connection_t.
// Adapted from nginx-acme (Apache-2.0).
//
// Implements tokio::io::AsyncRead and tokio::io::AsyncWrite to enable h2
// client connections over nginx's native async I/O.

use core::ffi::{CStr, c_int};
use core::pin::Pin;
use core::ptr::{self, NonNull};
use core::task::{self, Poll};
use core::{fmt, future, mem};
use std::io;

use nginx_sys::{
    ngx_addr_t, ngx_connection_t, ngx_destroy_pool, ngx_event_connect_peer, ngx_event_get_peer,
    ngx_int_t, ngx_log_t, ngx_msec_t, ngx_peer_connection_t, ngx_pool_t, ngx_ssl_shutdown,
    ngx_ssl_t, ngx_str_t,
};
use ngx::core::{Pool, Status};

use super::connection::Connection;

const DEFAULT_READ_TIMEOUT: ngx_msec_t = 60000;

/// SSL certificate verification error.
#[derive(Debug)]
pub struct SslVerifyError(c_int);

impl std::error::Error for SslVerifyError {}

impl fmt::Display for SslVerifyError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let desc = unsafe {
            let s = openssl_sys::X509_verify_cert_error_string(self.0 as _);
            CStr::from_ptr(s).to_str().unwrap_or("<unknown>")
        };
        write!(
            f,
            "upstream SSL certificate verify error: ({}:{})",
            self.0, desc
        )
    }
}

/// Owned nginx pool wrapper.
pub struct OwnedPool(Pool);

impl OwnedPool {
    pub fn new(size: usize, log: NonNull<ngx_log_t>) -> Result<Self, io::Error> {
        let pool = unsafe { nginx_sys::ngx_create_pool(size, log.as_ptr()) };
        if pool.is_null() {
            return Err(io::ErrorKind::OutOfMemory.into());
        }
        Ok(Self(unsafe { Pool::from_ngx_pool(pool) }))
    }

    pub fn with_default_size(log: NonNull<ngx_log_t>) -> Result<Self, io::Error> {
        Self::new(nginx_sys::NGX_DEFAULT_POOL_SIZE as usize, log)
    }
}

impl AsRef<ngx_pool_t> for OwnedPool {
    fn as_ref(&self) -> &ngx_pool_t {
        self.0.as_ref()
    }
}

impl AsMut<ngx_pool_t> for OwnedPool {
    fn as_mut(&mut self) -> &mut ngx_pool_t {
        self.0.as_mut()
    }
}

impl AsRef<Pool> for OwnedPool {
    fn as_ref(&self) -> &Pool {
        &self.0
    }
}

impl core::ops::Deref for OwnedPool {
    type Target = Pool;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl core::ops::DerefMut for OwnedPool {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

impl Drop for OwnedPool {
    fn drop(&mut self) {
        unsafe { ngx_destroy_pool(self.0.as_mut()) };
    }
}

/// Async wrapper over an ngx_peer_connection_t.
pub struct PeerConnection {
    pub pool: OwnedPool,
    pub pc: ngx_peer_connection_t,
    pub rev: Option<task::Waker>,
    pub wev: Option<task::Waker>,
}

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

        // SAFETY: c.recv() writes into the buffer and returns the number
        // of bytes written (or NGX_AGAIN/NGX_ERROR). We call assume_init
        // only for the bytes actually written.
        let unfilled = unsafe { buf.unfilled_mut() };
        let n = c.recv(unfilled);

        if n == nginx_sys::NGX_ERROR as isize {
            return Poll::Ready(Err(io::Error::last_os_error()));
        }

        let rev = c.read();

        if Status(unsafe { nginx_sys::ngx_handle_read_event(rev, 0) }) != Status::NGX_OK {
            return Poll::Ready(Err(io::ErrorKind::UnexpectedEof.into()));
        }

        if rev.active() != 0 {
            unsafe { nginx_sys::ngx_add_timer(rev, DEFAULT_READ_TIMEOUT) };
        } else if rev.timer_set() != 0 {
            unsafe { nginx_sys::ngx_del_timer(rev) };
        }

        if n == nginx_sys::NGX_AGAIN as isize {
            self.rev = Some(cx.waker().clone());
            return Poll::Pending;
        }

        if n > 0 {
            // SAFETY: c.recv() wrote n bytes into the unfilled portion.
            unsafe { buf.assume_init(n as usize) };
            buf.advance(n as usize);
        }

        Poll::Ready(Ok(()))
    }
}

impl tokio::io::AsyncWrite for PeerConnection {
    fn poll_write(
        mut self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
        buf: &[u8],
    ) -> Poll<Result<usize, io::Error>> {
        let Some(c) = self.connection_mut() else {
            return Poll::Ready(Err(io::ErrorKind::InvalidInput.into()));
        };

        let n = c.send(buf);

        if n == nginx_sys::NGX_AGAIN as ngx_int_t {
            self.wev = Some(cx.waker().clone());
            Poll::Pending
        } else if n > 0 {
            Poll::Ready(Ok(n as usize))
        } else {
            Poll::Ready(Err(io::ErrorKind::UnexpectedEof.into()))
        }
    }

    fn poll_flush(
        self: Pin<&mut Self>,
        _cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        Poll::Ready(Ok(()))
    }

    fn poll_shutdown(
        self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        self.poll_close(cx)
    }
}

impl PeerConnection {
    pub fn new(log: NonNull<ngx_log_t>) -> Result<Self, io::Error> {
        let mut pool = OwnedPool::with_default_size(log)?;

        // Copy the log object to avoid modifying log.connection on a cycle log.
        let new_log = {
            let mut new_log = unsafe { log.read() };
            new_log.action = ptr::null_mut();
            new_log.data = ptr::null_mut();
            new_log.handler = Some(Self::log_handler);
            let p = pool.alloc(core::mem::size_of::<ngx_log_t>()) as *mut ngx_log_t;
            if p.is_null() {
                return Err(io::ErrorKind::OutOfMemory.into());
            }
            unsafe {
                ptr::write(p, new_log);
                NonNull::new_unchecked(p)
            }
        };

        pool.as_mut().log = new_log.as_ptr();

        let mut this = Self {
            pool,
            pc: unsafe { mem::zeroed() },
            rev: None,
            wev: None,
        };
        let pc = &mut this.pc;
        pc.get = Some(ngx_event_get_peer);
        pc.log = new_log.as_ptr();
        pc.set_log_error(1); // NGX_ERROR_INFO

        Ok(this)
    }

    pub async fn connect(mut self: Pin<&mut Self>, addr: &ngx_addr_t) -> Result<(), io::Error> {
        // Copy sockaddr to the memory of the current connection
        let addr = copy_sockaddr(&self.pool, addr)?;
        let name = self.pool.alloc(core::mem::size_of::<ngx_str_t>()) as *mut ngx_str_t;
        if name.is_null() {
            return Err(io::ErrorKind::OutOfMemory.into());
        }
        unsafe { ptr::write(name, addr.name) };
        self.pc.name = name;
        self.pc.sockaddr = addr.sockaddr;
        self.pc.socklen = addr.socklen;

        future::poll_fn(|cx| self.as_mut().poll_connect(cx)).await
    }

    fn connect_peer(&mut self) -> Status {
        let rc = Status(unsafe { ngx_event_connect_peer(&mut self.pc) });

        if rc == Status::NGX_ERROR || rc == Status::NGX_BUSY || rc == Status::NGX_DECLINED {
            return rc;
        }

        let c = unsafe { &mut *self.pc.connection };
        c.data = ptr::from_mut(self).cast();

        if c.pool.is_null() {
            c.pool = ptr::from_mut(self.pool.as_mut());
        }

        unsafe {
            (*c.log).connection = c.number;
            (*c.read).handler = Some(ngx_peer_conn_read_handler);
            (*c.write).handler = Some(ngx_peer_conn_write_handler);
        }

        rc
    }

    pub fn poll_connect(
        mut self: Pin<&mut Self>,
        cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        if let Some(c) = self.connection_mut() {
            let rv = if c.read().timedout() != 0 || c.write().timedout() != 0 {
                c.close();
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

        self.set_log_action(c"connecting");

        match self.connect_peer() {
            Status::NGX_OK => {
                debug_assert!(
                    self.connection_mut().is_some(),
                    "connection should be established after NGX_OK"
                );
                self.unset_log_action();
                Poll::Ready(Ok(()))
            }
            Status::NGX_AGAIN => {
                let c = self.connection_mut().unwrap();

                c.read().handler = Some(ngx_peer_conn_read_handler);
                c.write().handler = Some(ngx_peer_conn_write_handler);

                unsafe { nginx_sys::ngx_add_timer(c.read(), DEFAULT_READ_TIMEOUT) };

                self.rev = Some(cx.waker().clone());
                self.wev = Some(cx.waker().clone());

                Poll::Pending
            }
            _ => Poll::Ready(Err(io::ErrorKind::ConnectionRefused.into())),
        }
    }

    pub fn poll_ssl_handshake(
        mut self: Pin<&mut Self>,
        ssl: &ngx_ssl_t,
        ssl_name: Option<&CStr>,
        cx: &mut task::Context<'_>,
    ) -> Poll<Result<(), io::Error>> {
        let c = unsafe { self.pc.connection.as_mut() }
            .expect("SSL handshake started on established connection");
        let c = unsafe { Connection::from_ptr_mut(c) };

        if c.ssl.is_null() {
            self.set_log_action(c"SSL handshaking");
            self.ssl_create_connection(ssl, ssl_name)?;
            unsafe { nginx_sys::ngx_reusable_connection(c.as_mut(), 0) };
        }

        match Status(unsafe { nginx_sys::ngx_ssl_handshake(c.as_mut()) } as _) {
            Status::NGX_OK => {
                let ssl_conn = unsafe { (*c.ssl).connection.cast() };

                // Check verify result if verification was enabled
                if unsafe { openssl_sys::SSL_get_verify_mode(ssl_conn) }
                    != openssl_sys::SSL_VERIFY_NONE
                {
                    let rc = unsafe { openssl_sys::SSL_get_verify_result(ssl_conn) } as c_int;
                    if rc != openssl_sys::X509_V_OK {
                        self.close();
                        return Poll::Ready(Err(io::Error::other(SslVerifyError(rc))));
                    }
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

    fn ssl_create_connection(
        &mut self,
        ssl: &ngx_ssl_t,
        ssl_name: Option<&CStr>,
    ) -> Result<(), io::Error> {
        const FLAGS: usize = (nginx_sys::NGX_SSL_CLIENT | nginx_sys::NGX_SSL_BUFFER) as _;

        let c = unsafe { self.pc.connection.as_mut() }
            .expect("SSL handshake started on established connection");

        // ngx_ssl_create_connection will increment a reference count on ssl.ctx: *mut SSL_CTX.
        let sslp = ptr::from_ref(ssl).cast_mut();
        if Status(unsafe { nginx_sys::ngx_ssl_create_connection(sslp, c, FLAGS) }) != Status::NGX_OK
        {
            return Err(io::ErrorKind::ConnectionRefused.into());
        }

        let ssl_conn = unsafe { (*c.ssl).connection.cast() };

        if let Some(name) = ssl_name
            && unsafe { openssl_sys::SSL_set_tlsext_host_name(ssl_conn, name.as_ptr().cast_mut()) }
                != 1
        {
            return Err(io::Error::other("SSL_set_tlsext_host_name failed"));
        }

        Ok(())
    }

    pub fn poll_close(
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

        let pool = c.pool;
        c.close();
        self.pc.connection = ptr::null_mut();

        if !ptr::eq::<ngx_pool_t>(self.pool.as_ref(), pool) {
            unsafe { ngx_destroy_pool(pool) };
        }

        Poll::Ready(Ok(()))
    }

    pub fn connection_mut(&mut self) -> Option<&mut Connection> {
        if self.pc.connection.is_null() {
            None
        } else {
            Some(unsafe { Connection::from_ptr_mut(self.pc.connection) })
        }
    }

    fn close(&mut self) {
        let Some(c) = self.connection_mut() else {
            return;
        };

        if !c.ssl.is_null() {
            unsafe {
                (*c.ssl).set_no_wait_shutdown(1);
                let _ = ngx_ssl_shutdown(c.as_mut());
            };
        }

        let pool = c.pool;
        c.close();
        self.pc.connection = ptr::null_mut();

        if !ptr::eq::<ngx_pool_t>(self.pool.as_ref(), pool) {
            unsafe { ngx_destroy_pool(pool) };
        }
    }

    fn set_log_action(self: &Pin<&mut Self>, action: &'static CStr) {
        if let Some(log) = unsafe { self.pc.log.as_mut() } {
            log.data = ptr::from_ref(&self.pc).cast_mut().cast();
            log.action = action.as_ptr().cast_mut().cast();
        }
    }

    fn unset_log_action(&self) {
        if let Some(log) = unsafe { self.pc.log.as_mut() } {
            log.action = ptr::null_mut();
        }
    }

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
                len -= p.offset_from(buf) as usize;
                buf = p;
            }

            if !pc.name.is_null() {
                let p = nginx_sys::ngx_snprintf(buf, len, c", server: %V".as_ptr(), pc.name);
                len -= p.offset_from(buf) as usize;
                buf = p;
            }

            if pc.socklen != 0 {
                let p = nginx_sys::ngx_snprintf(buf, len, c", addr: ".as_ptr());
                len -= p.offset_from(buf) as usize;

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

unsafe extern "C" fn ngx_peer_conn_ssl_handler(c: *mut ngx_connection_t) {
    unsafe {
        let this: *mut PeerConnection = (*c).data.cast();
        // This callback is invoked when both event handlers are set to ngx_event_openssl functions.
        // Using any of the wakers would result in polling the correct future.
        if let Some(waker) = (*this).rev.take() {
            waker.wake();
        }
    }
}

unsafe extern "C" fn ngx_peer_conn_ssl_shutdown_handler(c: *mut ngx_connection_t) {
    unsafe {
        let this: *mut PeerConnection = (*c).data.cast();
        // c.ssl is gone and it's no longer safe to use the ssl module event handlers
        (*(*c).read).handler = Some(ngx_peer_conn_read_handler);
        (*(*c).write).handler = Some(ngx_peer_conn_write_handler);

        if let Some(waker) = (*this).rev.take() {
            waker.wake();
        }
    }
}

unsafe extern "C" fn ngx_peer_conn_read_handler(ev: *mut nginx_sys::ngx_event_t) {
    unsafe {
        let c: *mut ngx_connection_t = (*ev).data.cast();
        let this: *mut PeerConnection = (*c).data.cast();

        if let Some(waker) = (*this).rev.take() {
            waker.wake();
        }
    }
}

unsafe extern "C" fn ngx_peer_conn_write_handler(ev: *mut nginx_sys::ngx_event_t) {
    unsafe {
        let c: *mut ngx_connection_t = (*ev).data.cast();
        let this: *mut PeerConnection = (*c).data.cast();

        if let Some(waker) = (*this).wev.take() {
            waker.wake();
        // Handle write events posted from the ngx_event_openssl code.
        } else if Status(nginx_sys::ngx_handle_write_event(ev, 0)) != Status::NGX_OK {
            // Log error if needed
        }
    }
}

fn copy_sockaddr(pool: &Pool, addr: &ngx_addr_t) -> Result<ngx_addr_t, io::Error> {
    let sockaddr = pool.alloc(addr.socklen as usize) as *mut nginx_sys::sockaddr;
    if sockaddr.is_null() {
        return Err(io::ErrorKind::OutOfMemory.into());
    }

    unsafe {
        addr.sockaddr
            .cast::<u8>()
            .copy_to_nonoverlapping(sockaddr.cast(), addr.socklen as usize)
    };

    let name = unsafe { ngx_str_t::from_bytes(pool.as_ptr(), addr.name.as_bytes()) }
        .ok_or(io::ErrorKind::OutOfMemory)?;

    Ok(ngx_addr_t {
        sockaddr,
        socklen: addr.socklen,
        name,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- SslVerifyError ---

    #[test]
    fn ssl_verify_error_display_ok_code() {
        // X509_V_OK == 0 should produce a known message
        let err = SslVerifyError(openssl_sys::X509_V_OK);
        let msg = format!("{err}");
        assert!(msg.contains("upstream SSL certificate verify error"));
        assert!(msg.contains("0:"));
    }

    #[test]
    fn ssl_verify_error_display_expired() {
        let err = SslVerifyError(openssl_sys::X509_V_ERR_CERT_HAS_EXPIRED);
        let msg = format!("{err}");
        assert!(msg.contains("upstream SSL certificate verify error"));
        assert!(msg.contains("expired"), "should mention expiry: {msg}");
    }

    #[test]
    fn ssl_verify_error_display_self_signed() {
        let err = SslVerifyError(openssl_sys::X509_V_ERR_DEPTH_ZERO_SELF_SIGNED_CERT);
        let msg = format!("{err}");
        assert!(msg.contains("upstream SSL certificate verify error"));
        assert!(
            msg.contains("self") || msg.contains("signed"),
            "should mention self-signed: {msg}"
        );
    }

    #[test]
    fn ssl_verify_error_debug() {
        let err = SslVerifyError(42);
        let debug = format!("{err:?}");
        assert!(debug.contains("SslVerifyError"));
        assert!(debug.contains("42"));
    }

    #[test]
    fn ssl_verify_error_is_std_error() {
        // Ensure SslVerifyError implements std::error::Error
        let err = SslVerifyError(0);
        let _: &dyn std::error::Error = &err;
    }

    // --- DEFAULT_READ_TIMEOUT ---

    #[test]
    fn default_read_timeout_is_reasonable() {
        const {
            assert!(
                DEFAULT_READ_TIMEOUT >= 1000,
                "timeout should be at least 1s"
            );
            assert!(
                DEFAULT_READ_TIMEOUT <= 300_000,
                "timeout should be at most 5 minutes"
            );
        }
    }
}
