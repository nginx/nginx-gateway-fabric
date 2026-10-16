//! gRPC stream client and timeout utilities.
//!
//! Drives a bidirectional ext_proc gRPC stream over h2 on the
//! `PeerConnection`. The h2 connection future is polled alongside every
//! stream operation in a `poll_fn` loop to keep the protocol state machine
//! advancing.

pub(crate) mod client;
pub(crate) mod timeout;
