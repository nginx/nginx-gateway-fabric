//! Timeout wrapper using nginx timers.
//!
//! `with_timeout` races a future against `ngx::async_::sleep`, which is backed
//! by the nginx event timer tree. A single deadline covers the entire EPP
//! request lifecycle: DNS, connect, TLS handshake, h2 handshake, send, and
//! receive.

use core::future::Future;
use core::pin::Pin;
use core::task::Poll;
use core::time::Duration;

use crate::error::ModuleError;

/// Race `fut` against an nginx timer of the given duration.
///
/// Returns the future's output on success, or `ModuleError::Timeout` if the
/// timer fires first.
pub(crate) async fn with_timeout<F>(duration: Duration, fut: F) -> Result<F::Output, ModuleError>
where
    F: Future,
{
    if duration.is_zero() {
        return Err(ModuleError::Timeout);
    }

    let mut sleep = ngx::async_::sleep(duration);
    let mut sleep = Pin::new(&mut sleep);
    let mut fut = core::pin::pin!(fut);

    core::future::poll_fn(|cx| {
        // Check the timer first — if it fired, we're done.
        if sleep.as_mut().poll(cx).is_ready() {
            return Poll::Ready(Err(ModuleError::Timeout));
        }

        // Poll the real work.
        if let Poll::Ready(val) = fut.as_mut().poll(cx) {
            return Poll::Ready(Ok(val));
        }

        Poll::Pending
    })
    .await
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn zero_duration_is_immediate_timeout() {
        let fut = async { 42u32 };
        let mut timeout_fut = core::pin::pin!(with_timeout(Duration::ZERO, fut));
        let waker = core::task::Waker::noop();
        let mut cx = core::task::Context::from_waker(waker);
        match timeout_fut.as_mut().poll(&mut cx) {
            Poll::Ready(Err(ModuleError::Timeout)) => {}
            other => panic!("expected Ready(Err(Timeout)), got {other:?}"),
        }
    }
}
