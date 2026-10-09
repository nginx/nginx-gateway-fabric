//! Owned nginx memory pool wrapper.
//!
//! `OwnedPool` wraps an `ngx_pool_t` and destroys it on drop, providing a safe
//! RAII lifetime for pool-allocated memory used by peer connections.

use core::ops::Deref;
use core::ptr::NonNull;

use nginx_sys::{NGX_DEFAULT_POOL_SIZE, ngx_create_pool, ngx_destroy_pool, ngx_log_t, ngx_pool_t};

/// An owned nginx memory pool that is destroyed on drop.
pub(crate) struct OwnedPool(*mut ngx_pool_t);

impl OwnedPool {
    /// Create a new pool with the given size.
    pub(crate) fn new(size: usize, log: NonNull<ngx_log_t>) -> Result<Self, std::io::Error> {
        let pool = unsafe { ngx_create_pool(size, log.as_ptr()) };
        if pool.is_null() {
            return Err(std::io::ErrorKind::OutOfMemory.into());
        }
        Ok(Self(pool))
    }

    /// Create a new pool with the default nginx pool size.
    pub(crate) fn with_default_size(log: NonNull<ngx_log_t>) -> Result<Self, std::io::Error> {
        Self::new(NGX_DEFAULT_POOL_SIZE as usize, log)
    }

    /// Get the raw pointer to the underlying `ngx_pool_t`.
    pub(crate) fn as_ptr(&self) -> *mut ngx_pool_t {
        self.0
    }
}

impl Deref for OwnedPool {
    type Target = ngx_pool_t;

    fn deref(&self) -> &Self::Target {
        unsafe { &*self.0 }
    }
}

impl Drop for OwnedPool {
    fn drop(&mut self) {
        if !self.0.is_null() {
            unsafe { ngx_destroy_pool(self.0) };
        }
    }
}
