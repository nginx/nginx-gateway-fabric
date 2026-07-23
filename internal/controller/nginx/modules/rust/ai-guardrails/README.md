# AI Guardrails NGINX Module

This directory contains `ai-guardrails`, an **NGINX dynamic module written in Rust**.
It inspects the request and response bodies of LLM traffic (chat/completion APIs), forwards the
extracted text to an external **Guardrails API**, and blocks content that the policy rejects.

This module is the **data-plane half** of the NGF `PayloadProcessor` feature. It does not decide
*which* routes to guard or *where* the Guardrails API lives — the Go control plane resolves all of
that and writes it into the generated `nginx.conf` as a set of `guardrails_*` directives. This
module simply reads those directives and acts on them at request time.

> **Audience:** This README assumes you know Go and the NGF codebase but may be new to Rust and to
> NGINX C modules. Rust/NGINX concepts are explained inline as they come up.

---

## Table of contents

- [How it fits into NGF](#how-it-fits-into-ngf)
- [Rust primer for this module](#rust-primer-for-this-module)
- [Source layout](#source-layout)
- [Configuration directives](#configuration-directives)
- [Request / response lifecycle](#request--response-lifecycle)
- [Build & test workflow](#build--test-workflow)
- [Gotchas & notes](#gotchas--notes)

---

## How it fits into NGF

The feature spans four layers. This module is the last one.

```text
  PayloadProcessor CRD  (apis/v1alpha1/payloadprocessor_types.go)
          │
          ▼
  Graph resolution      (internal/controller/state/graph/payloadprocessor.go)
   - resolves backend Service  -> APIURL string
   - resolves authTokenRef Secret -> bearer token
          │
          ▼
  Dataplane config      (internal/controller/state/dataplane/configuration.go)
   - GuardrailsConfig{ Filter, APIURL, APITokenAuthFileID, TimeoutMS }
   - Configuration.GuardrailsEnabled  (true if any route has guardrails)
          │
          ▼
  nginx.conf generation (internal/controller/nginx/config/*)
   - main_config_template.go: load_module modules/libai_guardrails.so;   (if GuardrailsEnabled)
   - servers_template.go:     guardrails_* directives inside a location   (per route)
          │
          ▼
  THIS MODULE (libai_guardrails.so)
   - reads the guardrails_* directives, inspects traffic, calls the Guardrails API
```

The exact directives the control plane emits into each `location` block
(see `servers_template.go`):

```nginx
guardrails_filter on;
guardrails_api_url http://guardrails-api.default.svc.cluster.local:443;
guardrails_api_token_file /etc/nginx/secrets/guardrails_token_default_guardrails-token;
guardrails_timeout_ms 2000;
```

The module is only loaded when at least one route uses guardrails, via this line in
`main_config_template.go`:

```nginx
load_module modules/libai_guardrails.so;
```

### Backend addressing (in-cluster vs external)

The value of `guardrails_api_url` is derived from the `PayloadProcessor` policy's `backendRef`
Service by `resolveExtProcessURL` (`internal/controller/state/graph/payloadprocessor.go`). The URL
scheme is chosen from the Service *type*:

| Backend location | Service type | Resolved URL |
| ------------------ | ------------- | -------------- |
| External | `ExternalName` | `https://<externalName>:<backendRef.port>` |
| In-cluster | `ClusterIP` (or any non-`ExternalName`) | `http://<name>.<namespace>.svc.cluster.local:<backendRef.port>` |

The module itself is scheme-agnostic — the `minreq` client (`src/client.rs`) handles both `http://`
and `https://`, so no module change is needed to support either backend. Two consequences worth
knowing:

- The port comes from the policy's `backendRef.port`, **not** the Service's `.spec.ports`.
- Externally-addressed backends are always called over https and in-cluster ones over http; the
  scheme cannot currently be overridden independently of the Service type.
- HTTPS verification uses the **operating system trust store** (rustls + `rustls-native-certs`),
  so the runtime image must ship `ca-certificates`. This is installed in the NGINX Dockerfiles
  (`build/Dockerfile.nginx[plus]`, `build/ubi/Dockerfile.nginx[plus]`). There is no bundled root
  certificate crate.

See [`examples/guardrails/README.md`](../../../../../../examples/guardrails/README.md) for
configuration walkthroughs of both backend styles.

---

## Rust primer for this module

A few Rust/NGINX concepts you need in order to read the code:

- **`.so` / `cdylib`.** NGINX loads modules as shared objects (`.so` files). Rust builds a shared
  object when a crate's `crate-type` is `cdylib` ("C dynamic library"). Our `Cargo.toml` sets
  `crate-type = ["cdylib", "rlib"]` — `cdylib` produces `libai_guardrails.so` for NGINX, and `rlib`
  is additionally needed so `cargo test` can link the unit-test harness.

- **FFI and `unsafe`.** NGINX exposes a C API. Rust talks to C through **FFI** (Foreign Function
  Interface). Calling C, dereferencing raw pointers (`*mut ngx_http_request_t`), and mutating
  global statics are all operations the Rust compiler cannot prove are memory-safe, so they must be
  wrapped in `unsafe { ... }`. The `unsafe` blocks and `extern "C"` functions throughout `lib.rs`
  are the boundary between Rust and NGINX's C runtime. This is expected for an NGINX module and is
  not a code smell by itself.

- **The `ngx` / `nginx-sys` crates.** These are third-party crates that provide Rust bindings to the
  NGINX API. `nginx-sys` is the low-level layer: at build time it runs **`bindgen`**, a tool that
  reads NGINX's C header files and auto-generates the matching Rust type/function declarations.

- **Why a configured NGINX source tree is required to build.** Because `bindgen` reads NGINX headers,
  compiling this module needs a *configured* NGINX source checkout (`./configure --with-compat` must
  have been run so generated headers exist). That is exactly what the `nginx-source` stage in the
  production Dockerfiles and in `Dockerfile.testing` prepares, exposed to the build via the
  `NGINX_SOURCE_DIR` environment variable. You do **not** need NGINX source on your laptop — the
  Docker-based `make` targets handle it (see [Build & test workflow](#build--test-workflow)).

---

## Source layout

| File | What it contains |
| ------ | ------------------ |
| `src/lib.rs` | Module entry point. Declares the directive table (`ngx_command_t`), the config-parsing handlers, registers the three filters in `postconfiguration`, and implements the request-body / header / response-body filters plus the 403 and stream-termination senders. |
| `src/config.rs` | `ModuleConfig` — the per-`location` configuration struct, its `Default` values, and the `inspect_requests()` / `inspect_responses()` helpers derived from `enabled` + `inspect_mode`. |
| `src/client.rs` | The Guardrails API client. A blocking `minreq` HTTP client that `POST`s to `<api_url>/backend/v1/scans`, the request/response JSON shapes, and the fail-closed `GuardrailsError` type. Returns `Ok(true)` when content is *cleared*, `Ok(false)` when *blocked*. TLS uses rustls verified against the OS trust store (`rustls-native-certs`). Unit tests use a hand-rolled `std::net::TcpListener` mock server (no mock-server dependency). |
| `src/stream.rs` | `StreamContext` — the streaming buffer and "checkpoint" logic. Parses SSE / OpenAI / Ollama chunk formats, accumulates text, decides when to inspect, and holds the termination/error message bodies. Contains the module's unit tests. |
| `Cargo.toml` | Crate manifest: dependencies (`ngx`, `nginx-sys`, `minreq`, `serde`, `serde_json`, …), `crate-type`, and release profile. See the comment on the `minreq` dependency for why it is chosen over `ureq`/`reqwest`. |
| `Cargo.lock` | Pinned exact dependency versions. Committed for reproducible builds. |
| `build.rs` | Build script that sets platform-specific linker flags so undefined NGINX symbols are resolved at module-load time rather than at link time. |
| `Dockerfile.testing` | CI/dev image used by `make rust-lint` and `make rust-unit-test`. Not used to produce the shipped `.so`. |

---

## Configuration directives

All directives are valid in the `location` context. Defaults come from
`ModuleConfig::default()` in `src/config.rs`.

| Directive | Argument | Default | Set by NGF? | Purpose |
| ----------- | ---------- | --------- | ------------- | --------- |
| `guardrails_filter` | `on` / `off` | `off` | Yes | Master enable switch for the location. |
| `guardrails_api_url` | URL | *(none)* | Yes | Base URL of the Guardrails API. `/backend/v1/scans` is appended by the client. |
| `guardrails_api_token_file` | path | *(none)* | Yes (when a token Secret is configured) | Reads the bearer token from a file at config-load time. Preferred over inline tokens. |
| `guardrails_api_token` | string | *(none)* | No | Inline bearer token. Supported by the module but NGF always uses the file form. |
| `guardrails_timeout_ms` | integer (ms) | `5000` | Yes (when `timeout` is set on the policy) | Per-request timeout for the API call. The `minreq` client's timeout is second-granular, so the value is rounded **up** to the nearest whole second (minimum 1s). |
| `guardrails_inspect_mode` | `request` / `response` / `both` / `off` | `both` | No | Which directions to inspect. NGF does not emit this, so the `both` default applies. |
| `guardrails_max_response_bytes` | integer (bytes) | `10485760` (10 MB) | No | Max response bytes buffered before the stream is blocked. `0` = unlimited. |

> The columns marked "No" are directives the module understands but the current NGF control plane
> does not generate. They fall back to the Rust defaults. If you add API knobs for them later, wire
> them through the graph → dataplane → template layers (see [How it fits into NGF](#how-it-fits-into-ngf)).

---

## Request / response lifecycle

The module registers three filters in `postconfiguration` (`src/lib.rs`). NGINX filters are chained:
each filter does its work and then calls the "next" filter it saved during registration.

### Request path (request-body filter)

1. Runs only for the main request (skips subrequests).
2. If `inspect_requests()` is false, passes through untouched.
3. Collects the full request body, parses it as JSON, and extracts the text to inspect:
   - `prompt` field → `/v1/completions` style, or
   - `messages[].content` → `/v1/chat/completions` style (joined together).
4. Calls the Guardrails API **synchronously** (`client::inspect_content`).
5. **Cleared** → forwards the body to the upstream. **Blocked or error** → returns `403 Forbidden`
   with a JSON error body (`send_403_and_finalize`) and the request never reaches the LLM.

### Response path (header filter + response-body filter)

The response is trickier because NGINX streams it, and we may need to *change a 200 into a 403*
after the upstream has already produced headers.

1. **Header filter** — on the first pass it *suppresses* the upstream response headers (for non-SSE
   responses) so nothing is committed to the client yet. SSE (`text/event-stream`) responses are let
   through immediately because they cannot be fully buffered. Error responses (status ≥ 400,
   e.g. our own injected 403) are passed through unchanged.
2. **Response-body filter** — buffers each upstream chunk in a `StreamContext`, extracting text from
   OpenAI/Ollama chunk formats as it goes. When the stream is complete (or `max_response_bytes` is
   exceeded), it runs one Guardrails "checkpoint" inspection over the accumulated text:
   - **Cleared** → commits the previously-suppressed headers, then releases all buffered chunks to
     the client.
   - **Blocked** → discards the buffer and sends either a proper `403` (non-SSE, via
     `send_blocked_response`) or an SSE termination event (streaming, via `send_termination`).

### Fail-closed behavior

If the Guardrails API errors or times out, the module **blocks** the traffic (treats it as
disallowed) rather than letting it through. This matches a `FailClosed` policy: when in doubt, deny.

---

## Build & test workflow

You do **not** need Rust installed locally. Everything runs in Docker via `make` targets defined in
the repository root `Makefile`. (On macOS in particular there is usually no host `cargo`, so use
these targets.)

| Command | What it does |
| --------- | -------------- |
| `make rust-fmt` | Runs `cargo fmt` to auto-format the code (formatting is enforced). |
| `make rust-lint` | Runs `clippy` (Rust's linter) with `-D warnings`, so any warning fails the build. Uses `Dockerfile.testing`. |
| `make rust-unit-test` | Runs the unit tests (`cargo test --lib`). Uses `Dockerfile.testing`. |

`make dev-all` runs `rust-fmt` and `rust-unit-test` alongside the Go and NJS checks.

### How the shipped module is built

The production `.so` is **not** built by the `make` targets above. It is built as part of the NGINX
container images. Each of `build/Dockerfile.nginx`, `build/Dockerfile.nginxplus`,
`build/ubi/Dockerfile.nginx`, and `build/ubi/Dockerfile.nginxplus` contains two extra build stages:

1. `nginx-source` — downloads the matching NGINX source and runs `./configure --with-compat` so
   bindgen has headers.
2. `guardrails-builder` — compiles this crate (`cargo build --release`) against that source.

The final image then copies the result in:

```dockerfile
COPY --from=guardrails-builder /build/target/release/libai_guardrails.so \
     /usr/lib/nginx/modules/libai_guardrails.so
```

That path is exactly what `load_module modules/libai_guardrails.so;` resolves to at runtime.

---

## Gotchas & notes

- **NGINX version pinning matters (ABI).** The module must be compiled against the *same* NGINX
  version that runs it. The OSS Dockerfiles pin the current OSS version (e.g. `1.31.3`); the Plus
  Dockerfiles pin `1.27.5` because NGINX Plus R37 is based on that OSS release. `Dockerfile.testing`
  pins its own version purely for bindgen — nothing is deployed from it, so the exact version there
  matters less.

- **Alpine (musl) vs UBI (glibc).** The OSS/Plus images build against Alpine/musl and pass
  `RUSTFLAGS="-C target-feature=-crt-static"` to force dynamic linking. The UBI images build against
  Debian/glibc instead, so their libc ABI matches the UBI runtime. The linker flags differ
  accordingly — this is why there are separate builder stages per image family.

- **Test-only linker flag.** `Dockerfile.testing` adds
  `-C link-arg=-Wl,--unresolved-symbols=ignore-all`. When running `cargo test`, Rust links a real
  test executable that references NGINX symbols (`ngx_pnalloc`, etc.) which normally only exist
  inside the NGINX process at module-load time. This flag lets the test binary link without them.
  It is **not** used for the production build.

- **Debug logging via `eprintln!`.** The code contains many `eprintln!("[guardrails] ...")` calls
  that write to stderr. These are development/debug traces; production log lines use the NGINX
  logging macros (`ngx_log_error!`). Treat the `eprintln!` output as verbose debug aid.

- **Ported verbatim.** The module source was ported from an upstream reference implementation. If
  you change directive names or config fields here, remember to update the corresponding Go layers
  (`servers_template.go`, `main_config_template.go`, and the dataplane/graph types) so the generated
  config and the module stay in sync.

- **`minreq` is chosen for license/dependency hygiene — do not swap it for `ureq`/`reqwest`.** The
  HTTP client is `minreq` with the `https-rustls-probe` and `json-using-serde` features. `minreq`
  does **not** depend on the `url` crate, so — unlike `ureq`, `reqwest`, `attohttpc`, and `ehttp` —
  it avoids pulling `idna` → the ICU4X (`icu_*`) crate stack, which is licensed `Unicode-3.0` and is
  rejected by the `dependency-review` CI workflow. `https-rustls-probe` verifies TLS against the OS
  trust store via `rustls-native-certs`, so no bundled-roots crate (`webpki-roots` /
  `webpki-root-certs`) is pulled in either. These crates are absent from **both** the compiled tree
  (`cargo tree`) **and** `Cargo.lock`, so no `dependency-review` allowlist entries are required for
  them. (`unicode-ident`, licensed in part `Unicode-3.0`, remains as an unavoidable build-time
  proc-macro dependency of `serde_derive`; it is independent of the HTTP client choice.) If you
  change the client or its features, regenerate `Cargo.lock` and confirm the `icu_*` / `url` /
  `webpki-root-certs` families do not reappear.

- **HTTP client unit tests use a std-only mock.** `src/client.rs` tests spin up a one-shot
  `std::net::TcpListener` HTTP/1.1 server on an ephemeral loopback port instead of using a
  mock-server crate. This avoids adding a dev-dependency (the previous `httpmock` dragged in
  license-incompatible transitive crates). If you add client tests, extend the `mock_once` helper
  rather than reintroducing a mock-server dependency.
