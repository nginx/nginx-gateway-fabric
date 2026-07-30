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
   - GuardrailsConfig{ Filter, APIURL, APITokenAuthFileID, TimeoutMS, InternalPath }
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
location /coffee {
    guardrails_filter on;
    guardrails_api_url http://guardrails-api.default.svc.cluster.local:443;
    guardrails_api_token_file /etc/nginx/secrets/guardrails_token_default_guardrails-token;
    guardrails_timeout_ms 2000;
    guardrails_internal_uri /_ngf-internal-guardrails-default_route1_rule0;
    # ...proxy_pass to the LLM upstream...
}

# The control plane also emits a deduplicated internal location that both the
# request and response paths subrequest into (one per distinct guardrails_internal_uri):
location /_ngf-internal-guardrails-default_route1_rule0 {
    internal;
    proxy_pass http://guardrails-api.default.svc.cluster.local:443/backend/v1/scans;
}
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

The module itself does not make outbound HTTP calls — both inspection directions issue an NGINX
**subrequest** into the generated internal location, which `proxy_pass`es to the resolved URL. TLS
to an `https://` backend is therefore handled by NGINX's own `proxy_ssl`, not by the module. Two
consequences worth knowing:

- The port comes from the policy's `backendRef.port`, **not** the Service's `.spec.ports`.
- Externally-addressed backends are always called over https and in-cluster ones over http; the
  scheme cannot currently be overridden independently of the Service type.
- HTTPS verification is performed by NGINX (`proxy_ssl_*` on the internal location) against the
  system trust store, so the runtime image must ship `ca-certificates` (installed in the NGINX
  Dockerfiles `build/Dockerfile.nginx[plus]`, `build/ubi/Dockerfile.nginx[plus]`). The module no
  longer links a Rust TLS stack (rustls/aws-lc-rs) — that dependency existed solely for the old
  blocking `minreq` client, which has been removed.

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
| `src/lib.rs` | Module entry point. Declares the directive table (`ngx_command_t`), the config-parsing handlers, registers the access-phase handler + header / response-body filters in `postconfiguration`, and implements the async request-inspection handler (body read → spawn task → phase re-drive) and the async response-inspection flow (buffer → spawn task → `resume_output` → single finalize), the header / response-body filters, and the 403 and stream-termination senders. |
| `src/config.rs` | `ModuleConfig` — the per-`location` configuration struct (including `internal_uri`), its `Default` values, and the `inspect_requests()` / `inspect_responses()` helpers derived from `enabled` + `inspect_mode`. `api_url` / `timeout_ms` are retained only so their directives parse; both are inert in the module (see the field docs). |
| `src/subrequest_client.rs` | The **shared** async inspection client used by **both** the request and response paths. `inspect_content_async` synthesizes the Guardrails JSON request, issues an in-memory NGINX **subrequest** into `guardrails_internal_uri`, and bridges the subrequest completion callback back to the awaiting task via a `oneshot` channel (`PostSubrequest`). Non-blocking: the worker keeps serving other connections while the scan runs. |
| `src/error.rs` | The path-agnostic `GuardrailsError` type (fail-closed on any `Err`) and the shared `GUARDRAILS_USER_AGENT` constant. Used by `subrequest_client.rs` for both directions. |
| `src/stream.rs` | `StreamContext` — the streaming buffer and "checkpoint" logic. Parses SSE / OpenAI / Ollama chunk formats, accumulates text, decides when to inspect, and holds the termination/error message bodies. Also holds the response-path async state (`ResponseInspect` / `ResponseVerdict` + the in-flight `Task`). Contains the module's unit tests. |
| `Cargo.toml` | Crate manifest: dependencies (`ngx`, `nginx-sys`, `futures`, `serde`, `serde_json`, …), `crate-type`, and release profile. `futures` is used only for its `oneshot` channel (the subrequest bridge). There is deliberately no blocking HTTP client or Rust TLS stack — inspection is done via NGINX subrequests, and backend TLS is NGINX's `proxy_ssl`. |
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
| `guardrails_api_url` | URL | *(none)* | Yes | Base URL of the Guardrails API. Now **inert in the module** — the backend URL lives in the internal location's `proxy_pass` (`<api_url>/backend/v1/scans`). The directive is still emitted/parsed for config visibility. |
| `guardrails_internal_uri` | path | *(none)* | Yes | URI of the internal NGINX location that **both** the request and response paths subrequest into for inspection. Points at a generated `internal;` location that `proxy_pass`es to `<api_url>/backend/v1/scans`. If unset, inspection fails **closed** (request path returns `403`; response path blocks). |
| `guardrails_api_token_file` | path | *(none)* | Yes (when a token Secret is configured) | Reads the bearer token from a file at config-load time. Preferred over inline tokens. |
| `guardrails_api_token` | string | *(none)* | No | Inline bearer token. Supported by the module but NGF always uses the file form. |
| `guardrails_timeout_ms` | integer (ms) | `5000` | Yes (when `timeout` is set on the policy) | Now **inert in the module** — backend timeouts are governed by the internal location's `proxy_*_timeout` (the subrequest inherits them). The directive is still emitted/parsed for config visibility. |
| `guardrails_inspect_mode` | `request` / `response` / `both` / `off` | `both` | No | Which directions to inspect. NGF does not emit this, so the `both` default applies. |
| `guardrails_max_response_bytes` | integer (bytes) | `10485760` (10 MB) | No | Max response bytes buffered before the stream is blocked. `0` = unlimited. |

> The columns marked "No" are directives the module understands but the current NGF control plane
> does not generate. They fall back to the Rust defaults. If you add API knobs for them later, wire
> them through the graph → dataplane → template layers (see [How it fits into NGF](#how-it-fits-into-ngf)).

---

## Request / response lifecycle

In `postconfiguration` (`src/lib.rs`) the module registers an **access-phase handler** (request
path) plus a **header filter** and **response-body filter** (response path). NGINX filters are
chained: each filter does its work and then calls the "next" filter it saved during registration.

### Request path vs. response path

Both inspection directions reach the Guardrails API through the **same non-blocking mechanism** — an
NGINX subrequest into `guardrails_internal_uri`, driven by an async task
(`subrequest_client::inspect_content_async`). They differ only in the NGINX construct they hang off
and how each hands control back once the verdict is known:

| | Request path | Response path |
| --- | -------------- | --------------- |
| NGINX phase | Access phase (a phase **handler**) | Header + body **filters** |
| Guardrails call | **Non-blocking** NGINX **subrequest** into `guardrails_internal_uri` (`ngx::async_::spawn`) | **Non-blocking** NGINX **subrequest** into `guardrails_internal_uri` (`ngx::async_::spawn`) |
| Worker impact | Worker keeps serving other connections while the Guardrails backend is slow | Worker keeps serving other connections while the Guardrails backend is slow |
| DNS / connection | Through NGINX's own upstream + resolver machinery (the internal `proxy_pass` location) | Same — the same internal `proxy_pass` location |
| Backend addressing | `guardrails_internal_uri` → internal location → `proxy_pass <api_url>/backend/v1/scans` | Same internal location |
| Suspend / resume | Return `NGX_DONE`, then re-drive the **phase engine** (`resume_phases`) | Return `NGX_OK` without forwarding, then push output + finalize once (`resume_output`) |

**Why they resume differently.** A body filter cannot suspend-and-be-called-again the way an
access-phase handler can (there is no "re-run the output filters" entry point). So instead of
suspending, the response-body filter buffers the whole response, returns `NGX_OK` **without
forwarding** the final buffer, and lets the in-flight subrequest hold the request open
(`ngx_http_subrequest` bumps `r->main->count`). When the subrequest completes, `resume_output`
commits the suppressed headers, flushes the buffered body (or sends the block/termination body), and
calls `ngx_http_finalize_request` **exactly once**. The single-finalize contract is guarded by the
`ResponseInspect` (`Idle → Pending → Done`) state machine. See
[`DESIGN-async-response-path.md`](./DESIGN-async-response-path.md) for the full design and the
subrequest-count reasoning.

### Request path (async access-phase handler)

1. Runs only for the main request (skips subrequests) and only when `inspect_requests()` is true;
   otherwise `NGX_DECLINED` (pass through).
2. If `guardrails_internal_uri` is not configured, fails **closed** with a `403`.
3. **First invocation:** reads the full client request body
   (`ngx_http_read_client_request_body`) and returns `NGX_DONE`.
4. The body-read callback parses the body as JSON and extracts the text to inspect:
   - `prompt` field → `/v1/completions` style, or
   - `messages[].content` → `/v1/chat/completions` style (joined together).
   It then **spawns an async task** (`subrequest_client::inspect_content_async`) that issues an
   in-memory subrequest into `guardrails_internal_uri`. The worker is **not** blocked while the
   Guardrails backend processes the scan.
5. When the subrequest completes, the task records the verdict and re-drives the phase engine
   (`ngx_http_core_run_phases`). The handler is re-invoked: `NGX_AGAIN` while pending, `NGX_OK` if
   **cleared** (request proceeds to the LLM), or a `403` if **blocked or errored**
   (`send_403_and_finalize`) — the request never reaches the LLM.

See [`src/subrequest_client.rs`](#source-layout) for the subrequest ↔ `oneshot` bridge that wakes
the awaiting task from the subrequest completion callback.

### Response path (header filter + response-body filter)

The response is trickier because NGINX streams it, and we may need to *change a 200 into a 403*
after the upstream has already produced headers.

1. **Header filter** — on the first pass it *suppresses* the upstream response headers (for non-SSE
   responses) so nothing is committed to the client yet. SSE (`text/event-stream`) responses are let
   through immediately because they cannot be fully buffered. Error responses (status ≥ 400,
   e.g. our own injected 403) are passed through unchanged.
2. **Response-body filter** — buffers each upstream chunk in a `StreamContext`, extracting text from
   OpenAI/Ollama chunk formats as it goes. When the stream is complete (or `max_response_bytes` is
   exceeded), it starts one **async** Guardrails "checkpoint" inspection over the accumulated text:
   it sets `ResponseInspect::Pending`, spawns the subrequest task, and returns `NGX_OK` without
   forwarding (the in-flight subrequest keeps the request alive). When the task completes,
   `resume_output` acts on the verdict:
   - **Cleared** → commits the previously-suppressed headers, then releases all buffered chunks to
     the client.
   - **Blocked / errored (fail-closed)** → discards the buffer and sends either a proper `403`
     (non-SSE, via `send_blocked_response`) or an SSE termination event (streaming, via
     `send_termination`).

   In all cases `resume_output` issues the single `ngx_http_finalize_request`.

### HTTP status vs. error type

The JSON error body follows the OpenAI error shape (`error.{message,type,param,code}`). The `type`
and HTTP status depend on *which side* was blocked:

| Block path | HTTP status | `error.type` | `error.code` |
| ------------ | ------------- | -------------- | -------------- |
| Request (client input) | `403` | `invalid_request_error` | `content_policy_violation` |
| Response, non-SSE | `403` | `api_error` | `content_policy_violation` |
| Response, SSE stream | `200`* | `api_error` | `content_policy_violation` |

Request blocks use `invalid_request_error` because the *client's request* was rejected. Response
blocks use `api_error` because the client request was valid and it was the *model's output* that
tripped the policy — a server/output-side decision.

*For SSE (`text/event-stream`) responses the upstream `200` headers are flushed to the client
immediately (SSE cannot be buffered), so the status can no longer be changed by the time the block
is decided. The block is delivered as an `api_error` payload inside an SSE `data:` frame. A `200`
carrying an `api_error` body is an unavoidable consequence of SSE header timing, not a bug.

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

- **Both paths are async (subrequest); they resume differently, and that is intentional.** NGINX
  body filters cannot suspend-and-be-called-again the way an access-phase handler can, so the
  response path does not "suspend" — it buffers, returns `NGX_OK` without forwarding, lets the
  in-flight subrequest hold the request open, and pushes output + finalizes from `resume_output`.
  Do **not** try to make the body filter itself await; that entry point does not exist in NGINX.
  The spawned inspection `Task` **must be kept alive** for the duration of the inspection (dropping
  a `ngx::async_` `Task` cancels it): the request path stores it in the per-request
  `RequestInspectState`; the response path stores it on the `StreamContext`, which is heap-boxed
  with a pool cleanup (`stream_ctx_cleanup` / `alloc_stream_ctx`) so its `Drop` — and therefore the
  task cancellation — actually runs at request teardown. A raw `pool().allocate` would **not** run
  the destructor, so it is deliberately not used for `StreamContext`.

- **`resume_output` owns the single finalize.** It is the sole caller of
  `ngx_http_finalize_request` for the response path; the send helpers (`send_chunks`,
  `send_blocked_response`, `send_termination`) deliberately do **not** finalize. The
  `ResponseInspect` state machine (`Idle → Pending → Done`) guarantees this runs exactly once —
  the same double-finalize / lost-body hazard class documented on `send_403_and_finalize`.

- **`futures` is used only for `oneshot`.** The subrequest completion callback and the awaiting task
  communicate through a `futures::channel::oneshot`. No `futures` networking/executor code is pulled
  in — keep it that way to preserve dependency-review hygiene.

- **No blocking HTTP client and no Rust TLS stack — do not reintroduce one.** The module previously
  shipped a blocking `minreq` client (with a rustls/aws-lc-rs TLS stack) for the response path; it
  has been removed now that both paths inspect via NGINX subrequests. Backend TLS is handled by
  NGINX's `proxy_ssl` on the internal location. Reintroducing an in-module HTTP client would
  re-stall the worker **and** risk pulling the `idna` → ICU4X (`icu_*`, `Unicode-3.0`) crate stack
  that the `dependency-review` CI workflow rejects — the `icu_*` / `url` / `idna` / rustls / aws-lc
  families must stay absent from both the compiled tree and `Cargo.lock`. (`unicode-ident`, licensed
  in part `Unicode-3.0`, remains as an unavoidable build-time proc-macro dependency of
  `serde_derive`; it is independent of any HTTP client choice.)

- **Inspection unit tests parse JSON only — no network mock.** `src/subrequest_client.rs` tests
  cover `build_request_body` and `parse_outcome` directly; there is no HTTP mock server (the old
  `std::net::TcpListener` mock lived in the deleted `src/client.rs`). The subrequest itself is
  exercised by live NGINX, not by a Rust unit test.
