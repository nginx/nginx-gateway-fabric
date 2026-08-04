//! Single-threaded `Send + Sync` assertion wrapper.
//!
//! NGINX workers are single-threaded; a request's raw pointers and NGINX
//! structures are only ever touched from the one worker thread. The async tasks
//! spawned via `ngx::async_` are `'static` and nominally require `Send`, so any
//! raw pointer captured across an `.await` boundary must be wrapped to assert
//! `Send + Sync`. That assertion is sound *only* under this single-threaded
//! embedding, matching `ngx::async_`'s own runtime invariant.

/// Wrapper asserting `Send + Sync` for a value that is only ever accessed on the
/// single NGINX worker thread.
///
/// Used to move raw request/subrequest pointers (and other `!Send` NGINX values)
/// across the `.await` boundary of a spawned task. Access the inner value via
/// `.0`.
///
/// # Safety
///
/// Constructing this is safe, but relying on the `Send`/`Sync` impls is only
/// sound while the wrapped value is confined to the single NGINX worker thread.
/// Do not use it to share values across real OS threads.
pub(crate) struct AssertSendSync<T>(pub(crate) T);

// Safety: single-threaded embedding — the NGINX worker never touches these
// values from more than one thread.
unsafe impl<T> Send for AssertSendSync<T> {}
unsafe impl<T> Sync for AssertSendSync<T> {}
