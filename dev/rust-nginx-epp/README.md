NGINX Rust EPP Module POC (Gateway API Inference Extension)
===============================================================

Goal
----
Replace the current NJS + Go shim approach for communicating with the Endpoint Picker (EPP) using a native NGINX HTTP module written in Rust. The module will:
- Accept the client request (headers + body) at an inference location.
- Call the EPP using Envoy ext_proc gRPC protocol to obtain `X-Gateway-Destination-Endpoint`.
- Set an NGINX variable (e.g., `$inference_workload_endpoint`) and perform an internal redirect to the configured internal path, preserving query args.
- Support Fail-Open / Fail-Close behavior consistent with existing NGF logic.

References
----------
- NGF PR implementing inference extension: https://github.com/nginx/nginx-gateway-fabric/pull/4091
- NGF proposal: docs/proposals/gateway-inference-extension.md (Integration with EPP)
- Endpoint Picker Protocol (ext_proc): https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main/docs/proposals/004-endpoint-picker-protocol

Current NGF Integration Points (to mirror)
------------------------------------------
From existing implementation:
- NJS module: `internal/controller/nginx/modules/src/epp.js`
  - Reads variables `epp_host`, `epp_port`, `epp_internal_path`
  - Sends client method, headers, and body to shim at `http://127.0.0.1:54800`
  - Expects `X-Gateway-Destination-Endpoint` header from shim, assigns to `$inference_workload_endpoint`
  - Performs `internalRedirect($epp_internal_path + args)`
- Go shim: `cmd/gateway/endpoint_picker.go`
  - Listens at 127.0.0.1:54800
  - Reads `X-EPP-Host`, `X-EPP-Port` headers to determine the EPP Service DNS & port
  - Opens gRPC ext_proc stream, sends headers + body, receives response
  - Returns `X-Gateway-Destination-Endpoint` header to NJS client

Rust Module Design
------------------
We will implement a dynamic NGINX HTTP module using Rust (via ngx-rust) that:
1) Provides an HTTP content handler directive, e.g.:
   - `epp_get_endpoint on;` or `inference_epp on;`
   - The directive triggers the module to execute for that location.

2) Configuration / variables:
   - The module will read the following variables configured by NGF templates (already used today):
     - `$epp_host` -> EPP DNS name (ServiceName[.Namespace])
     - `$epp_port` -> EPP port number
     - `$epp_internal_path` -> internal location to redirect to after endpoint selection
   - The module will write:
     - `$inference_workload_endpoint` -> endpoint value returned from EPP (ip:port)
   - Fail mode:
     - If EPP returns an ImmediateResponse or errors:
       - Fail-Close: set invalid backend and return 503/500
       - Fail-Open: fall back to upstream configured for the inference pool

3) Request handling flow:
   - Parse request headers (normalize keys to lowercase) and body (if present).
   - Initiate a gRPC ext_proc stream to the EPP at `$epp_host:$epp_port`.
   - Send ProcessingRequest:
     - HttpHeaders (EndOfStream=false) containing client headers (plus any hint headers needed by EPP)
     - HttpBody (EndOfStream=true) containing full request body (if Content-Length>0)
   - Receive ProcessingResponse:
     - If ImmediateResponse: map status code (429/503) to appropriate action based on fail mode
     - If RequestHeaders response with header mutations: look for `X-Gateway-Destination-Endpoint`
       - Use first endpoint (if multiple are provided comma-separated)
       - Set `$inference_workload_endpoint`
   - Perform internal redirect to `$epp_internal_path` preserving `args` (query string).

4) NGINX config snippet example (for POC):
```
load_module modules/ngx_http_rust_epp_module.so;

http {
  # ... other http config

  server {
    listen 80;
    server_name _;

    location /inference {
      # Variables set via NGF template today:
      set $epp_host test-epp.default;
      set $epp_port 8080;
      set $epp_internal_path /_ngf-internal-rule0-route0-inference;
      # Target variable to be set by module:
      # js_var $inference_workload_endpoint;  # Not needed with Rust; declare as standard variable

      # Invoke the Rust module content handler:
      epp_get_endpoint on;

      # After module sets $inference_workload_endpoint and does internal redirect,
      # the internal location proxies to $inference_workload_endpoint:
    }

    # Internal location built by NGF (example):
    location /_ngf-internal-rule0-route0-inference {
      proxy_http_version 1.1;

      # Use the selected endpoint:
      proxy_pass http://$inference_workload_endpoint;

      # Optional: headers & TLS verification configured by NGF policy
    }
  }
}
```

Initial Module API Sketch (Rust)
--------------------------------
- Crate: `ngx-http-rust-epp`
- Exposed directive: `epp_get_endpoint` (takes no args or `on/off`)
- Handler lifecycle:
  - Phase: content handler for `location` blocks tagged for inference
  - Reads `epp_host`, `epp_port`, `epp_internal_path`
  - Builds gRPC client using `tonic` + `prost` for `ext_proc` proto
  - Writes `$inference_workload_endpoint` via NGINX variable API
  - Executes internal redirect (ngx_http_internal_redirect) to `$epp_internal_path` with `r->args` preserved

Security & TLS
--------------
- Per current NGF proposal, EPP connection is insecure due to EPP limitations.
- Once EPP supports TLS:
  - Provide module options: `epp_tls on; epp_tls_verify on; epp_tls_server_name <...>; epp_tls_ca <path>;`
  - Honor BackendTLSPolicy or equivalent via NGF control plane integration.

Fail-Open / Fail-Close Behavior
-------------------------------
- The control plane maps failure mode per `EndpointPickerRef.FailureMode` into NGINX location behavior.
- The Rust module will:
  - On failure:
    - Fail-Close: return 503 or set `$inference_workload_endpoint` to an invalid marker and avoid redirect
    - Fail-Open: set `$inference_workload_endpoint` to an upstream fallback (provided by NGF via map),
      or skip setting and allow subsequent config to proxy to upstream as in today’s template logic.

Testing Plan
------------
- Provide a Dockerfile that builds NGINX OSS + ngx-rust + the Rust module, and runs an example config.
- Provide simple mock EPP or use the reference EPP chart to validate end-to-end.
- Send requests with JSON bodies containing model names and verify the module drives routing to the endpoint given by EPP.

Project Structure (to be added)
-------------------------------
- `Cargo.toml`
- `src/lib.rs`         (NGINX module registration + directive + handler)
- `src/epp_client.rs`  (tonic ext_proc client, header/body mapping)
- `Dockerfile`         (build NGINX with module)
- `nginx.conf`         (POC config wiring directive and internal locations)

Limitations
-----------
- Initial POC focuses on basic `DestinationEndpoint` header handling and internal redirect flow.
- Subsetting and dynamic metadata may be added in later iterations if needed by the conformance suite.
- TLS to EPP is not supported until EPP implementation adds support.

Build & Run (planned)
---------------------
- `docker build -t ngf-rust-epp:dev dev/rust-nginx-epp/`
- `docker run --rm -p 8080:80 ngf-rust-epp:dev`
- Use curl to send inference requests; observe endpoint chosen.

Next Steps
----------
- Scaffold Cargo.toml and Rust sources
- Implement gRPC client with tonic for ext_proc
- Implement NGINX handler using ngx-rust
- Provide Dockerfile and sample nginx.conf
- Validate end-to-end with mock EPP
