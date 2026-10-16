# Inference (Endpoint Picker) NGINX Module

`inference` is an NGINX dynamic module written in Rust (`libinference.so`). It implements the Gateway API
Inference Extension
[Endpoint Picker Protocol](https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main/docs/proposals/004-endpoint-picker-protocol),
which lets NGINX ask an Endpoint Picker (EPP) which model server should handle each inference request.

## How it works

For each inference request, the module opens a gRPC bidirectional stream to the
Endpoint Picker (EPP) over the ext_proc protocol.

1. **PRECONTENT phase** — The module reads the request body, encodes it as
   ext_proc `RequestHeaders` + `RequestBody` gRPC messages, and sends them
   to the configured EPP endpoint over h2. The EPP replies with the selected
   backend address in the `x-gateway-destination-endpoint` header mutation,
   plus any additional header mutations for the upstream. The module sets the
   `$inference_endpoint` variable to the chosen address and applies header
   mutations to the upstream request. Running in PRECONTENT (after ACCESS)
   allows auth modules like `auth_basic` and `auth_request` to reject
   requests before the module contacts the EPP.
2. **Proxy** — nginx forwards the request to the selected endpoint via
   `proxy_pass http://$inference_endpoint`.
3. **Response phase** — After the upstream responds, a header filter triggers
   the module to send `ResponseHeaders` (with served-endpoint metadata)
   followed by one or more `ResponseBody` messages containing the upstream
   response body back to the EPP on the same gRPC stream. The body is
   streamed progressively as chunks arrive from the upstream via the body
   filter, with each gRPC message capped at 62 KB to match the EPP's
   `BodyByteLimit`. The final `ResponseBody` carries `end_of_stream`. This
   tells the EPP which endpoint handled the request, provides token-count
   data from the response body, and releases in-flight accounting.
   `inference_epp_response_phase` controls whether EPP response-header
   mutations are applied before sending the response to the client.

### Response modes

- **`notify` (default)** — The client response proceeds without waiting for
  the EPP response phase. EPP replies are consumed to complete the protocol
  exchange, but response-header mutations are not applied.
- **`headers`** — The module waits for the EPP response phase to complete and
  applies permitted mutations from its `ResponseHeaders` reply before sending
  headers to the client. Response output is buffered while waiting, subject to
  a 1 MiB payload limit and a 1,024-chunk limit per request. Exceeding either
  limit terminates the request. This mode can add latency before the client receives headers.

Both modes stream the upstream response body to the EPP via one or more
`ResponseBody` messages. The response phase has its own timeout using
`inference_epp_timeout`.

### Failure modes

- **Request-phase fail-open** — With `inference_failopen` configured, EPP errors set
  `$inference_endpoint` to the fallback upstream and the request proceeds
  (logged at `warn`).
- **Request-phase fail-close** — Without it, the request is rejected with the EPP's status
  code (504 on timeout, otherwise 500, logged at `error`).
- **Request-phase ImmediateResponse** — If the EPP rejects the request with an
  `ImmediateResponse`, it follows the same fail-open policy as other request-phase
  errors. Without `inference_failopen`, its status code (typically 503 or 429)
  is returned through nginx's response finalization. Informational and success
  codes other than 201 and 204 are mapped to 500 because they cannot generate a
  terminal response through the PRECONTENT return path. Response bodies and headers
  from `ImmediateResponse` are not passed through.
- **Response-phase failure** — EPP errors, timeouts, and `ImmediateResponse`
  replies are logged at `warn`; the upstream response proceeds without EPP
  response-header mutations, regardless of `inference_failopen`.

## Configuration

The module ships in the NGINX images at `modules/libinference.so`.

### Directives

All directives are valid in `location` blocks.

| Directive | Values | Default | Description |
| --------- | ------ | ------- | ----------- |
| `inference_epp` | `on` \| `off` | `off` | Enable or disable the module. |
| `inference_epp_endpoint` | `host:port` | — | EPP gRPC address. Required when enabled. |
| `inference_epp_timeout` | nginx time value | `5s` | Timeout applied separately to the EPP request and response phases (e.g. `5s`, `10m`). |
| `inference_epp_response_phase` | `notify` \| `headers` | `notify` | Notify the EPP without delaying the client response, or wait and apply EPP response-header mutations. |
| `inference_epp_ssl_verify` | `on` \| `off` | `off` | Verify EPP TLS certificate. Requires `inference_epp_ssl_trusted_certificate`. |
| `inference_epp_ssl_trusted_certificate` | file path | — | PEM CA bundle for EPP certificate verification. Implies TLS is enabled. |
| `inference_epp_ssl_name` | hostname | — | Hostname for SNI and certificate SAN check. Implies TLS is enabled. |
| `inference_failopen` | upstream name | — | Upstream to use when the EPP fails. Omit for fail-close. |

### Variables

| Variable | Description |
| -------- | ----------- |
| `$inference_endpoint` | The `host:port` address selected by the EPP (or the failopen upstream on error). |

### Example

```nginx
upstream inference_pool {
  server 192.0.2.10:8000;  # replace with an actual fallback model-server address
}

server {
    listen 80;

    location /v1/completions {
        inference_epp            on;
        inference_epp_endpoint   10.96.0.42:9002;  # ClusterIP:port of the EPP service
        inference_epp_timeout    5s;
        inference_epp_response_phase notify; # use headers to apply response-header mutations
        inference_failopen       inference_pool;

        proxy_pass http://$inference_endpoint;
    }
}
```

## Source layout

```text
inference/
├── Cargo.toml
├── Cargo.lock
├── build.rs              # linker flags for nginx symbol resolution at load time
├── Dockerfile.testing    # lint/test environment image
└── src/
    ├── lib.rs            # module definition, ngx_modules! export, phase/filter registration
    ├── handler.rs        # PRECONTENT handler, header/body filters, variable getter, mutations
    ├── error.rs          # ModuleError enum with HTTP status codes
    ├── codec/
    │   ├── endpoint.rs   # host:port parsing, IPv6 bracket notation, comma lists
    │   ├── grpc.rs       # gRPC length-prefixed frame encoder/decoder
    │   ├── mutations.rs  # apply EPP header mutations (set/append/remove with denylist)
    │   ├── notify.rs     # build response-phase messages (served-endpoint metadata)
    │   ├── request.rs    # build RequestHeaders and RequestBody messages
    │   └── response.rs   # decode ProcessingResponse, extract endpoint and mutations
    ├── config/
    │   ├── types.rs      # LocationConfig struct, validation, nginx time parsing
    │   └── directives.rs # nginx directive handlers and command table
    ├── net/
    │   ├── pool.rs       # OwnedPool — RAII nginx memory pool
    │   ├── connection.rs # Connection wrapper over ngx_connection_t
    │   └── peer_conn.rs  # PeerConnection — async TCP + TLS over ngx_peer_connection_t
    └── stream/
        ├── timeout.rs    # nginx-event-loop-aware timeout via ngx::async_::sleep
        └── client.rs     # h2 gRPC stream client: epp_call, send_response_phase
```

## Development

Everything runs in Docker, because `nginx-sys` needs a configured nginx source tree for bindgen.

| Command | What it does |
| ------- | ------------ |
| `make rust-fmt` | `cargo fmt` |
| `make rust-lint` | `cargo clippy` |
| `make rust-unit-test` | `cargo test` |

Unit tests run without nginx, so they cannot call nginx functions.

The production `.so` is **not** built by the `make` targets above. It is built by
`build/Dockerfile.rustmodules`, which downloads the matching NGINX source, runs
`./configure --with-compat` for bindgen headers, and compiles both Rust modules
(`cargo build --release`). CI uploads the resulting `.so` files as GitHub Actions
artifacts; the NGINX image Dockerfiles (`build/Dockerfile.nginx*`,
`build/ubi/Dockerfile.nginx*`) then `COPY` them into the final image at
`/usr/lib/nginx/modules/`. See the header comment in `build/Dockerfile.rustmodules`
for usage examples (OSS vs Plus, musl vs glibc).
