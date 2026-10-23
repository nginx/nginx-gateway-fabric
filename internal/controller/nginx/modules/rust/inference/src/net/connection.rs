//! Safe wrapper around `ngx_connection_t`.
//!
//! Provides ergonomic access to read/write events, connect verification, data
//! transfer, and connection close. The `#[repr(transparent)]` layout allows
//! zero-cost casting from raw `ngx_connection_t` pointers.

use core::ffi::{CStr, c_int};
use core::mem::MaybeUninit;
use core::{mem, ops};

use nginx_sys::{ngx_close_connection, ngx_connection_t, ngx_err_t, ngx_event_t, ngx_int_t};

/// Log-error severity levels for `ngx_connection_t.log_error`.
#[allow(dead_code)]
#[repr(u32)]
pub(crate) enum ConnectionLogError {
    Alert = nginx_sys::ngx_connection_log_error_e_NGX_ERROR_ALERT,
    Err = nginx_sys::ngx_connection_log_error_e_NGX_ERROR_ERR,
    Info = nginx_sys::ngx_connection_log_error_e_NGX_ERROR_INFO,
    IgnoreConnectionReset = nginx_sys::ngx_connection_log_error_e_NGX_ERROR_IGNORE_ECONNRESET,
    IgnoreInvalidArgument = nginx_sys::ngx_connection_log_error_e_NGX_ERROR_IGNORE_EINVAL,
}

/// Transparent wrapper over `ngx_connection_t`.
///
/// Provides safe accessors for read/write events, connect verification,
/// data transfer, and close operations.
#[repr(transparent)]
pub(crate) struct Connection(ngx_connection_t);

impl AsRef<ngx_connection_t> for Connection {
    #[inline]
    fn as_ref(&self) -> &ngx_connection_t {
        &self.0
    }
}

impl AsMut<ngx_connection_t> for Connection {
    #[inline]
    fn as_mut(&mut self) -> &mut ngx_connection_t {
        &mut self.0
    }
}

impl ops::Deref for Connection {
    type Target = ngx_connection_t;

    #[inline]
    fn deref(&self) -> &Self::Target {
        self.as_ref()
    }
}

impl ops::DerefMut for Connection {
    #[inline]
    fn deref_mut(&mut self) -> &mut Self::Target {
        self.as_mut()
    }
}

impl Connection {
    /// Create a mutable reference from a raw `ngx_connection_t` pointer.
    ///
    /// # Safety
    ///
    /// The pointer must be non-null and point to a valid, properly aligned
    /// `ngx_connection_t`. The caller must ensure the lifetime is valid.
    pub(crate) unsafe fn from_ptr_mut<'a>(c: *mut ngx_connection_t) -> &'a mut Self {
        unsafe { &mut *c.cast::<Self>() }
    }

    /// Access the read event.
    pub(crate) fn read(&mut self) -> &mut ngx_event_t {
        debug_assert!(!self.0.read.is_null());
        unsafe { &mut *self.0.read }
    }

    /// Access the write event.
    pub(crate) fn write(&mut self) -> &mut ngx_event_t {
        debug_assert!(!self.0.write.is_null());
        unsafe { &mut *self.0.write }
    }

    /// Verify the connect() result after an async connect completes.
    ///
    /// Returns `Ok(())` on success or the OS error code on failure.
    pub(crate) fn test_connect(&mut self) -> Result<(), ngx_err_t> {
        let mut err: c_int = 0;
        let mut len: libc::socklen_t = mem::size_of_val(&err) as _;

        if unsafe {
            libc::getsockopt(
                self.0.fd,
                libc::SOL_SOCKET,
                libc::SO_ERROR,
                &mut err as *mut c_int as *mut _,
                &mut len,
            )
        } == -1
        {
            err = std::io::Error::last_os_error().raw_os_error().unwrap_or(0);
        }

        if err != 0 {
            self.error(err as ngx_err_t, c"connect() failed");
            return Err(err as ngx_err_t);
        }

        Ok(())
    }

    /// Log a connection error via nginx's error handling.
    pub(crate) fn error(&mut self, err: ngx_err_t, msg: &CStr) -> ngx_int_t {
        unsafe { nginx_sys::ngx_connection_error(self.as_mut(), err, msg.as_ptr().cast_mut()) }
    }

    /// Receive data from the connection into the provided buffer.
    ///
    /// Returns the number of bytes read, `NGX_AGAIN` if the socket would block,
    /// or `NGX_ERROR` on failure.
    pub(crate) fn recv(&mut self, buf: &mut [MaybeUninit<u8>]) -> isize {
        // Safety: recv is always set on a valid connection.
        unsafe {
            self.as_ref().recv.unwrap_unchecked()(self.as_mut(), buf.as_mut_ptr().cast(), buf.len())
        }
    }

    /// Send data on the connection.
    ///
    /// Returns the number of bytes sent, `NGX_AGAIN` if the socket would block,
    /// or `NGX_ERROR` on failure.
    pub(crate) fn send(&mut self, buf: &[u8]) -> isize {
        // Safety: send is always set on a valid connection.
        unsafe {
            self.as_ref().send.unwrap_unchecked()(self.as_mut(), buf.as_ptr().cast_mut(), buf.len())
        }
    }

    /// Close the connection via `ngx_close_connection`.
    pub(crate) fn close(&mut self) {
        unsafe { ngx_close_connection(self.as_mut()) }
    }
}
