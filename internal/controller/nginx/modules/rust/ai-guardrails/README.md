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
    - GuardrailsConfig{ Enabled, APIURL, APITokenAuthFileID, InternalPath }
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
    guardrails_api_token_file /etc/nginx/secrets/guardrails_token_default_guardrails-token;
    guardrails_internal_uri /_ngf-internal-guardrails-default_route1_rule0;
    # ...proxy_pass to the LLM upstream...
}

# The control plane also emits a deduplicated internal location that both the
# request and response paths subrequest into (one per distinct guardrails_internal_uri).
# The backend URL lives here, NOT on the module. There is no configurable timeout;
# the backend call is bounded only by NGINX's default proxy_*_timeout (60s per operation).
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

The backend URL is derived from the `PayloadProcessor` policy's `backendRef` Service by
`resolveExtProcessURL` (`internal/controller/state/graph/payloadprocessor.go`). It is **not** passed
to the module (there is no `guardrails_api_url` directive); the control plane uses it solely to build
the internal location's `proxy_pass`. The URL scheme is chosen from the Service *type*:

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
| `src/lib.rs` | Slim crate root / registration hub. Declares the submodules, defines the `Module` type and its `HttpModule` / `HttpModuleLocationConf` impls, and in `postconfiguration` registers the access-phase handler + header / request-body / response-body filters (wiring the `directives`, `ctx`, `request_path`, and `response_path` modules together). Holds the `ngx_modules!` registration and the `ngx_http_guardrails_module` static. |
| `src/directives.rs` | The directive table (`NGX_HTTP_GUARDRAILS_COMMANDS`) and the three config-parsing handlers (`enable`, `api_token_file`, `internal_uri`) plus the `ngx_conf_handler!` macro they share. |
| `src/ctx.rs` | The FFI seam shared by both paths: the stored next-filter statics (`NGX_HTTP_NEXT_*`) and their `call_next_*` wrappers, per-request `StreamContext` allocation/cleanup (`get_module_ctx_mut`, `alloc_stream_ctx`), and SSE content-type detection (`is_sse_response` / `is_sse_content_type`). Co-locates the SSE + `call_next` unit tests. |
| `src/decision.rs` | Pure, NGINX-free decision logic shared by both paths, lifted out of the FFI handlers so it can be unit-tested: `verdict_from_inspection` (fail-closed allow/block mapping), `decide_access_action` (access-handler state machine), `decide_response_action` (body-filter branch precedence), and `block_commit_kind` (SSE-vs-buffered block selector). All tests co-located. |
| `src/request_path.rs` | The async request-inspection path: the access-phase handler and request-body filter (body read → spawn task → phase re-drive), prompt extraction (`extract_inspection_content`), per-request `RequestInspectState`, and the 403 sender. Consumes `decision::{decide_access_action, verdict_from_inspection}`. Co-locates the prompt-extraction + request-block-body unit tests. |
| `src/response_path.rs` | The async response-inspection path: the header filter and response-body filter (buffer → spawn task → `resume_output` → single finalize), the `BodyCommit` commit-status enum, and the stream-termination / blocked-response senders. Co-locates the `BodyCommit` unit test. |
| `src/config.rs` | `ModuleConfig` — the per-`location` configuration struct (`enabled`, `api_token`, `api_token_file`, `internal_uri`), its derived `Default`, the `MAX_RESPONSE_BYTES` constant, and the `inspect_requests()` / `inspect_responses()` helpers (both return `enabled`; when on, both directions are inspected). Holds no backend-URL or timeout config: the backend URL lives control-plane side in the internal location's `proxy_pass`; there is no configurable timeout, so the backend call is bounded only by NGINX's default `proxy_*_timeout` (60s per operation). |
| `src/subrequest_client.rs` | The **shared** async inspection client used by **both** the request and response paths. `inspect_content_async` synthesizes the Guardrails JSON request, issues an in-memory NGINX **subrequest** into `guardrails_internal_uri`, and bridges the subrequest completion callback back to the awaiting task via a `oneshot` channel (`PostSubrequest`). Non-blocking: the worker keeps serving other connections while the scan runs. |
| `src/error.rs` | The path-agnostic `GuardrailsError` type (fail-closed on any `Err`) and the shared `GUARDRAILS_USER_AGENT` constant. Used by `subrequest_client.rs` for both directions. |
| `src/sync_ptr.rs` | The canonical `AssertSendSync<T>` wrapper used to move raw NGINX pointers into the single-threaded `'static` async tasks spawned by both paths. |
| `src/stream.rs` | `StreamContext` — the streaming buffer and "checkpoint" logic. Parses SSE / OpenAI / Ollama chunk formats, accumulates text, decides when to inspect, and holds the termination/error message bodies. Also holds the response-path async state (`ResponseInspect` / `ResponseVerdict` + the in-flight `Task`). |
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
| `guardrails_internal_uri` | path | *(none)* | Yes | URI of the internal NGINX location that **both** the request and response paths subrequest into for inspection. Points at a generated `internal;` location that `proxy_pass`es to the backend's `/backend/v1/scans`. If unset, inspection fails **closed** (request path returns `403`; response path blocks). |
| `guardrails_api_token_file` | path | *(none)* | Yes (when a token Secret is configured) | Reads the bearer token from a file at config-load time. |

> The backend URL is **not** a module directive. It is baked into the internal location's
> `proxy_pass` by the control plane. There is no configurable timeout: the backend call is bounded
> only by that internal location's NGINX default `proxy_*_timeout` (60s per operation). The earlier
> `guardrails_api_url` and `guardrails_timeout_ms` directives, which were inert in the module, have
> been removed, as has the `PayloadProcessor` `Timeout` field.
>
> When the filter is enabled, **both** the request and response directions are inspected; there is
> no directive to select a single direction. The response buffer cap is a fixed module constant
> (`MAX_RESPONSE_BYTES`, 10 MB): a response exceeding it is blocked (fail-closed) rather than
> buffered unbounded.

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
| Backend addressing | `guardrails_internal_uri` → internal location → `proxy_pass <backend-url>/backend/v1/scans` (bounded by NGINX default `proxy_*_timeout`) | Same internal location |
| Suspend / resume | Return `NGX_DONE`, then re-drive the **phase engine** (`resume_phases`) | Return `NGX_OK` without forwarding, then push output + finalize once (`resume_output`) |

**Why they resume differently.** A body filter cannot suspend-and-be-called-again the way an
access-phase handler can (there is no "re-run the output filters" entry point). So instead of
suspending, the response-body filter buffers the whole response, returns `NGX_OK` **without
forwarding** the final buffer, and lets the in-flight subrequest hold the request open
(`ngx_http_subrequest` bumps `r->main->count`). When the subrequest completes, `resume_output`
commits the suppressed headers, flushes the buffered body (or sends the block/termination body), and
calls `ngx_http_finalize_request` **exactly once**. The single-finalize contract is guarded by the
`ResponseInspect` (`Idle → Pending → Done`) state machine.

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
   responses) so nothing is committed to the client yet. SSE responses are let
   through immediately because they cannot be fully buffered; SSE is detected by matching a
   `text/event-stream` media type in `Content-Type` case-insensitively, ignoring any `;`-delimited
   parameters (e.g. `; charset=utf-8`). Error responses (status ≥ 400,
   e.g. our own injected 403) are passed through unchanged.
2. **Response-body filter** — buffers each upstream chunk in a `StreamContext`, extracting text from
   OpenAI/Ollama chunk formats as it goes. When the stream is complete (or the `MAX_RESPONSE_BYTES`
   cap is exceeded), it starts one **async** Guardrails "checkpoint" inspection over the accumulated text:
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

### Block message (`error.message`)

The `error.message` text combines a per-path default with an optional message supplied by the
Guardrails backend.

Scan requests are sent with `verbose:false`. The operator-configured block text is still delivered on
a flagged scan as `result.scannerResults[].message`, so `verbose:true` is not needed — and is
deliberately avoided because it inflates the response with a large top-level scanner-config block,
which can exceed the default NGINX `subrequest_output_buffer_size` ("too big subrequest response" →
empty body → fail-closed block).

On a block, the module selects the **first failed** guardrail (`outcome == "failed"`) and reads its
`message` field. Empty/whitespace values are treated as absent.

**Composition.** The per-path default is always present; the backend message, when available, is
**appended** to it (it does not replace it):

- No backend message → the default verbatim.
- Backend message present → `"<default> Message: <backend text>"`.

Per-path defaults:

| Block path | Default `error.message` |
| ------------ | -------------------------- |
| Request (client input) | `Request blocked by guardrails policy.` |
| Response, non-SSE | `Response blocked by guardrails policy.` |
| Response, SSE stream | `Stream terminated by guardrails policy.` |

Example (request path, guardrail configured with a flag message):

```text
Request blocked by guardrails policy. Message: This message has been blocked by IP guardrail FLAG MESSAGE@!!!!
```

The backend message is JSON-escaped before injection, so operator-configured text cannot break the
error envelope. The `type`/`code` fields are never overridden by the backend, so the request-vs-response
distinction above always holds.

### Scan direction (`scanDirection`)

Each scan request includes a `scanDirection` field that correlates with the LLM exchange, so the
backend applies the guardrails configured for that side:

- **Request path** (access-phase handler, inspecting the client prompt) sends `scanDirection: "request"`.
- **Response path** (response-body filter, inspecting the model output) sends `scanDirection: "response"`.

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

### What the unit tests cover — and what they cannot

The FFI handlers (`guardrails_access_handler`, `guardrails_response_body_filter`,
`guardrails_resume_write_handler`, the `send_*` emitters) are inseparable from raw
`*mut ngx_http_request_t` work, the NGINX filter chain, and the async scheduler, so they cannot be
exercised by `cargo test`. To keep as much of the decision-making testable as possible, the **pure
decisions** those handlers make are lifted into `src/decision.rs` — plain functions over ordinary
Rust values, unit-tested exhaustively, that the handlers then translate into FFI side effects:

| Decision function | What it decides | Consumed by |
| ------------------- | ----------------- | ------------- |
| `verdict_from_inspection` | cleared → allow (drop message); flagged → block (keep message); error (`None`) → block, no message (**fail-closed**) | both async tasks (request + response) |
| `decide_access_action` | access-handler state machine: `GrantAccess` / `Block` / `Wait` / `StartInspection` from `(verdict, started)` | `guardrails_access_handler` |
| `decide_response_action` | body-filter branch: `HoldForPending` / `EmitBlock` / `BlockOverLimit` / `SpawnInspection` / `FlushBuffered` / `KeepBuffering` (precedence pinned by tests) | `guardrails_response_body_filter` |
| `block_commit_kind` | headers-suppressed → non-SSE 403 body (`send_blocked_response`) vs. SSE termination frame (`send_termination`) | every response block site (via `commit_block`) |

This isolates the *policy* (fail-closed mapping, branch precedence, SSE-vs-buffered selection) from
the *mechanism* (buffers, finalize, chain calls), so a regression in policy is caught at
`make rust-unit-test` time rather than only in live NGINX.

### End-to-end validation matrix (live NGINX required)

The decision logic above is unit-covered, but the mechanism — suppressed-header commit, single
finalize from a posted write event, body flush before connection close — can only be validated
against a running NGINX. The following scenarios **must** be re-run against a live NGINX after any
change to the request/response paths:

| # | Scenario | Expected result |
| --- | ---------- | ----------------- |
| E1 | Clean **non-SSE** response (e.g. `/v1/chat/completions`, non-streaming) | `200` with **byte-identical** upstream body; headers committed once; request finalized exactly once |
| E2 | Blocked **non-SSE** response | `403` with `api_error` JSON body (non-empty — **not** `403 0`) |
| E3 | Clean **SSE** stream (`text/event-stream`) | Buffered `data:` frames flushed; stream completes with no client timeout |
| E4 | Blocked **SSE** stream | Injected SSE termination frame carrying the block message; `200` (headers already flushed) |
| E5 | Blocked **request** prompt (input path) | `403` with `invalid_request_error` JSON body; upstream never contacted |
| E6 | Backend **error / unreachable** (both directions) | **Fail-closed**: request → `403`; response → block body (never released unfiltered) |
| E7 | Response exceeds `MAX_RESPONSE_BYTES` (10 MB) | Stream blocked via the buffer-limit path; block/termination body emitted |
| E8 | `guardrails_internal_uri` misconfigured / absent | **Fail-closed** on both paths (403 / block body); logged at `ERR` |
| E9 | Non-LLM JSON body with no extractable text (e.g. `/v1/models`) | Suppressed headers committed and buffered body released (client does **not** hang) |
| E10 | Double-finalize / leaked-request smoke across E1–E4 | No `403 0` empty bodies, no leaked requests, no worker crash under repeated runs |

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

- **Logging via NGINX's log macros.** All log lines go through the NGINX logging macros
  (`ngx_log_error!` / `ngx_conf_log_error!`) and land in the standard NGINX error log — no
  `eprintln!`/stderr paths remain. Genuine failures (fail-closed branches, alloc failures,
  inspection errors) log at `NGX_LOG_ERR` / `NGX_LOG_WARN`; verbose lifecycle traces (per-chunk
  progress, header suppression, resume steps) log at `NGX_LOG_DEBUG_HTTP`, so they appear only when
  NGINX is built with `--with-debug` and `error_log ... debug;` is configured. Log lines carry only
  metadata (status codes, byte counts, config, lifecycle) — scanned prompt/model content is never
  logged.

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
