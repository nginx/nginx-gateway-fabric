# Rust NGINX Modules

This directory contains native Rust NGINX modules for NGINX Gateway Fabric, built using the
[ngx-rust](https://github.com/nginx/ngx-rust) crate.

## Directory Structure

```text
rust/
└── ngf-inference/           # Inference Extension module
    ├── src/                 # Rust source files
    │   ├── lib.rs           # Module entry point (access handler, header filter, variable)
    │   ├── config.rs        # Location configuration (directives & defaults)
    │   ├── grpc.rs          # gRPC client (h2 HTTP/2 + manual gRPC framing)
    │   ├── protos.rs        # Protobuf type re-exports (envoy-types)
    │   ├── stream.rs        # Request processing and EPP stream lifecycle
    │   └── net/             # Async networking over nginx I/O
    │       ├── mod.rs
    │       ├── connection.rs  # ngx_connection_t wrapper
    │       ├── peer_conn.rs   # Async peer connection (tokio AsyncRead/AsyncWrite)
    │       └── ssl.rs         # ngx_ssl_t wrapper (TLS context)
    ├── .cargo/              # Cargo configuration
    ├── Cargo.toml           # Dependencies and metadata
    ├── Dockerfile.testing   # Test and lint container
    └── coverage/            # Test coverage output (generated)
```

## Modules

### ngf-inference

Implements the [Endpoint Picker Protocol (EPP)](https://github.com/kubernetes-sigs/gateway-api-inference-extension/blob/main/docs/proposals/004-endpoint-picker-protocol/README.md)
for the [Gateway API Inference Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main).
EPP uses Envoy's [ext_proc](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ext_proc/v3/ext_proc.proto)
bidirectional gRPC streaming protocol to communicate between the data plane and the Endpoint Picker.

This module enables NGINX Gateway Fabric to route traffic to AI/ML inference endpoints selected by an external EPP server.

**Key features:**

- Bidirectional gRPC streaming using the ext_proc protocol
- Manual gRPC framing over the h2 HTTP/2 crate
- Async networking via nginx's native event loop (`ngx::async_::spawn`)
- Request phase: Extracts headers and body, sends to EPP, receives endpoint selection
- Response phase: After the upstream responds, spawns a background task to send the served endpoint back to EPP on the still-open gRPC stream
- EPP may return a comma-separated list of endpoints; the first one is selected
- Failopen support: on EPP failure, falls back to a configured upstream instead of returning 502
- TLS support via nginx's SSL subsystem (`--with-http_ssl_module`)
- uses nginx's DNS resolver if configured, falls back to getaddrinfo to preserve backwards compatibility

**NGINX directives:**

- `inference_epp on|off` — Enable or disable EPP for this location
- `inference_epp_endpoint <host:port>` — EPP gRPC server address (e.g., `inference_epp_endpoint epp-server:9002`)
- `inference_failopen <upstream_name>` — Enable failopen; on EPP failure, `$inference_endpoint` resolves to this upstream so `proxy_pass` falls back to it instead of returning 502
- `inference_epp_tls on|off` — Enable TLS for the EPP gRPC connection
- `inference_epp_tls_skip_verify on|off` — Skip server certificate verification when using TLS

**Variables:**

- `$inference_endpoint` — The selected endpoint address, set by the module after EPP response (used in `proxy_pass`)

## Architecture

### NGINX Integration

The module registers:

1. **Access phase handler** — Intercepts requests, reads the body, initiates the EPP gRPC exchange as an async task, and returns `NGX_AGAIN` while waiting for the endpoint selection. Once the endpoint is resolved, sets `$inference_endpoint` and returns `NGX_DECLINED` to let nginx continue to `proxy_pass`.
2. **Header filter** — After the upstream responds, spawns a fire-and-forget async task to send the served endpoint back to EPP on the still-open gRPC stream, then closes it.
3. **Variable handler** — Provides `$inference_endpoint` for use in `proxy_pass`. Returns the EPP-selected endpoint, or the failopen upstream if EPP failed and failopen is configured.

### Async Model

NGINX is single-threaded and event-driven. The module runs entirely on the nginx
event loop without any external runtime:

- **`ngx::async_::spawn`** — Spawns futures on the nginx event loop
- **`PeerConnection`** — Wraps `ngx_peer_connection_t` with tokio `AsyncRead`/`AsyncWrite` trait impls,
  enabling the h2 client to operate over nginx's native async I/O
- **NGINX event posting** — The async task posts to `ngx_posted_events` / `ngx_posted_next_events`
  to re-enter the access handler when work completes
- **No cross-thread communication** — All state is single-threaded within the nginx worker; no atomics or mutexes are needed

### gRPC Client

The gRPC client uses the **h2 crate** for HTTP/2 with **manual gRPC framing** (5-byte length-prefixed
protobuf messages):

- `h2::client::handshake` — Performs the HTTP/2 handshake, returning a `SendRequest` handle and `Connection` driver
- `encode_grpc_message` / `try_decode_grpc_message` — Manual gRPC frame encoding/decoding using prost
- `EppStream` — Holds the `SendStream`, `RecvStream`, and `Connection` for the bidirectional gRPC stream
- Phase 1 (request): async task on the nginx event loop; stores `EppStream` in `RequestCtx` when done
- Phase 2 (response): spawns a background task that takes ownership of the entire `EppStream`, sends the notification, and closes the stream while continuing to drive the `Connection`

**Important:** The h2 crate's `SendStream`, `RecvStream`, and `Connection` share internal state. Dropping
`SendStream`/`RecvStream` while another task polls `Connection` can cause panics. The response phase moves
the entire `EppStream` into the background task to ensure safe concurrent access.

### Request Flow

```text
1. Client request arrives → Access handler triggered
2. Read client request body → Return NGX_AGAIN while waiting
3. Body ready → Spawn async EPP task:
   a. Resolve EPP address, open PeerConnection (TCP + optional TLS handshake)
   b. HTTP/2 handshake via h2::client::handshake
   c. Open bidirectional stream with send_request()
   d. Send RequestHeaders and RequestBody to EPP as gRPC frames
   e. EPP responds with chosen endpoint(s) in header mutation
   f. Store selected endpoint + EppStream in RequestCtx, set done=true
4. NGINX event loop detects task completion → re-enters access handler
5. Access handler finalizes: sets $inference_endpoint → Return NGX_DECLINED
6. NGINX proxy_passes request to $inference_endpoint
   (or to failopen upstream if EPP communication failed and failopen is configured)
7. Upstream responds → Header filter fires:
   a. Spawn background task with entire EppStream (SendStream, RecvStream, Connection)
   b. Background task: send ResponseHeaders, close stream, drive Connection to completion
   c. Pass headers and body through to the client
```

## Development

All commands are run from the **repository root** using the top-level Makefile. The Rust toolchain runs inside Docker
containers to ensure consistent builds with the correct NGINX source headers.

### Format Code

Run `rustfmt` to format the code:

```shell
make rust-fmt
```

### Lint

Run `clippy` with warnings as errors:

```shell
make rust-lint
```

### Unit Tests

Run tests with code coverage (output to `ngf-inference/coverage/lcov.info`):

```shell
make rust-unit-test
```

### Build

The module is built as part of the NGINX image build process. To build the full image:

```shell
make build-nginx-image
```
