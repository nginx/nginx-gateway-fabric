// SSL context wrapper for nginx's ngx_ssl_t.
// Adapted from nginx-acme (Apache-2.0).
//
// Creates and manages an SSL_CTX via nginx's SSL subsystem for use
// with PeerConnection::poll_ssl_handshake.

use core::mem;
use core::ptr;

use nginx_sys::{ngx_ssl_create, ngx_ssl_t};

/// Wrapper around ngx_ssl_t that manages the SSL context lifecycle.
///
/// Supports skip-verify mode (SSL_VERIFY_NONE) for connecting to endpoints
/// that don't support custom certificates.
#[derive(Debug)]
pub struct NgxSsl(ngx_ssl_t);

impl AsRef<ngx_ssl_t> for NgxSsl {
    fn as_ref(&self) -> &ngx_ssl_t {
        &self.0
    }
}

impl AsMut<ngx_ssl_t> for NgxSsl {
    fn as_mut(&mut self) -> &mut ngx_ssl_t {
        &mut self.0
    }
}

impl NgxSsl {
    /// Initialize the SSL context with TLS 1.2 and 1.3 protocols.
    ///
    /// Sets ALPN to advertise `h2` (HTTP/2), which is required for gRPC over TLS.
    /// Without ALPN, the server will not know to speak HTTP/2 and will reject the
    /// HTTP/2 connection preface after the TLS handshake.
    pub fn init(&mut self) -> Result<(), &'static str> {
        let protocols =
            (nginx_sys::NGX_SSL_TLSv1_2 | nginx_sys::NGX_SSL_TLSv1_3) as nginx_sys::ngx_uint_t;

        let rc = unsafe { ngx_ssl_create(&mut self.0, protocols, ptr::null_mut()) };
        if rc != ngx::core::Status::NGX_OK.0 {
            return Err("ngx_ssl_create failed");
        }

        // Advertise HTTP/2 via ALPN -- required for gRPC over TLS.
        // Wire format is length-prefixed protocol strings: [0x02, 'h', '2'].
        let alpn: &[u8] = &[0x02, b'h', b'2'];
        let ret = unsafe {
            openssl_sys::SSL_CTX_set_alpn_protos(
                self.0.ctx.cast(),
                alpn.as_ptr(),
                alpn.len() as libc::c_uint,
            )
        };
        if ret != 0 {
            return Err("SSL_CTX_set_alpn_protos failed");
        }

        Ok(())
    }

    /// Configure SSL certificate verification.
    ///
    /// If `skip_verify` is true, sets SSL_VERIFY_NONE (accept any certificate).
    /// Otherwise uses default verify paths (system trust store).
    pub fn set_verify(&mut self, skip_verify: bool) {
        unsafe {
            if skip_verify {
                openssl_sys::SSL_CTX_set_verify(
                    self.0.ctx.cast(),
                    openssl_sys::SSL_VERIFY_NONE,
                    None,
                );
            } else {
                openssl_sys::SSL_CTX_set_verify(
                    self.0.ctx.cast(),
                    openssl_sys::SSL_VERIFY_PEER,
                    None,
                );
                // Use system trust store
                openssl_sys::SSL_CTX_set_default_verify_paths(self.0.ctx.cast());
            }
        }
    }
}

impl Default for NgxSsl {
    fn default() -> Self {
        Self(unsafe { mem::zeroed() })
    }
}

impl Drop for NgxSsl {
    fn drop(&mut self) {
        if !self.0.ctx.is_null() {
            unsafe { nginx_sys::ngx_ssl_cleanup_ctx(ptr::addr_of_mut!(self.0).cast()) }
        }
    }
}
