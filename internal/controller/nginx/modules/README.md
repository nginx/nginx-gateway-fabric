# NGINX Gateway Fabric Modules

This directory contains the custom NGINX modules for NGINX Gateway Fabric.

## Directory Structure

```text
modules/
├── njs/                     # NJS (NGINX JavaScript) modules
│   ├── src/                 # NJS source files
│   ├── test/                # NJS unit tests
│   └── README.md            # NJS development guide
└── rust/                    # Native Rust NGINX modules
    ├── ngf-inference/       # Inference Extension module
    └── README.md            # Rust development guide
```

## Module Types

### NJS Modules

[NJS](https://nginx.org/en/docs/njs/) (NGINX JavaScript) modules are lightweight scripting extensions that run within
NGINX. They're useful for request routing logic and header manipulation without requiring native compilation.

See the [NJS README](./njs/README.md) for development details.

**Current modules:**

- [httpmatches](./njs/src/httpmatches.js): A location handler that redirects requests to internal location blocks
  based on request headers, arguments, and method.

### Rust Modules

Native Rust modules are compiled directly into NGINX using the [ngx-rust](https://github.com/nginx/ngx-rust) crate.
They provide high-performance, type-safe integrations for complex functionality like gRPC streaming.

See the [Rust README](./rust/README.md) for development details.

**Current modules:**

- [ngf-inference](./rust/ngf-inference/): Implements the Endpoint Picker Protocol (EPP) for the Gateway API Inference
  Extension, enabling AI endpoint selection via bidirectional gRPC streaming.
