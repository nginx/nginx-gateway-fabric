// Wrapper struct for an ngx_connection_t.
// Adapted from nginx-acme (Apache-2.0).

use core::mem::MaybeUninit;
use core::ops;

use ngx::ffi::{ngx_close_connection, ngx_connection_t, ngx_event_t, ngx_int_t};

/// Wrapper struct for an [`ngx_connection_t`].
#[repr(transparent)]
pub struct Connection(ngx_connection_t);

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
    /// Creates a new connection wrapper from a raw pointer.
    ///
    /// # Safety
    /// The pointer must be valid and properly aligned.
    pub unsafe fn from_ptr_mut<'a>(c: *mut ngx_connection_t) -> &'a mut Self {
        unsafe { &mut *c.cast::<Self>() }
    }

    /// Returns a read event reference.
    pub fn read(&mut self) -> &mut ngx_event_t {
        debug_assert!(!self.0.read.is_null());
        unsafe { &mut *self.0.read }
    }

    /// Returns a write event reference.
    pub fn write(&mut self) -> &mut ngx_event_t {
        debug_assert!(!self.0.write.is_null());
        unsafe { &mut *self.0.write }
    }

    /// Check `connect` result.
    pub fn test_connect(&mut self) -> Result<(), i32> {
        let mut err: libc::c_int = 0;
        let mut len: libc::socklen_t = core::mem::size_of_val(&err) as _;

        if unsafe {
            libc::getsockopt(
                self.0.fd,
                libc::SOL_SOCKET,
                libc::SO_ERROR,
                core::ptr::addr_of_mut!(err).cast(),
                &mut len,
            ) == -1
        } {
            #[cfg(target_os = "macos")]
            {
                err = unsafe { *libc::__error() };
            }
            #[cfg(not(target_os = "macos"))]
            {
                err = unsafe { *libc::__errno_location() };
            }
        }

        if err != 0 { Err(err) } else { Ok(()) }
    }

    /// Receive data from the connection.
    pub fn recv(&mut self, buf: &mut [MaybeUninit<u8>]) -> isize {
        unsafe {
            self.as_ref().recv.unwrap_unchecked()(self.as_mut(), buf.as_mut_ptr().cast(), buf.len())
        }
    }

    /// Send data to the connection.
    pub fn send(&mut self, buf: &[u8]) -> ngx_int_t {
        unsafe {
            self.as_ref().send.unwrap_unchecked()(self.as_mut(), buf.as_ptr().cast_mut(), buf.len())
        }
    }

    /// Close the connection.
    pub fn close(&mut self) {
        unsafe { ngx_close_connection(self.as_mut()) }
    }
}
