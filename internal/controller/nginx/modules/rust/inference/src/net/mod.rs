//! Networking primitives for nginx event-loop-integrated I/O.
//!
//! This module provides async wrappers around nginx's connection and peer-connection
//! types, implementing tokio I/O traits so the h2 crate can drive gRPC streams
//! directly on nginx's event loop.

pub(crate) mod connection;
pub(crate) mod peer_conn;
pub(crate) mod pool;
