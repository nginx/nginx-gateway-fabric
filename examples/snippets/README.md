# Snippets

This directory contains the YAML files used in the [Snippets](https://docs.nginx.com/nginx-gateway-fabric/traffic-management/snippets/) guide.

## Stream snippet contexts

`SnippetsPolicy` supports the additional `stream` and `stream.server` contexts when it targets a `Gateway` that serves TCP, UDP, or TLS stream traffic.

- `stream` snippets are inserted at the top level of the generated `stream {}` configuration.
- `stream.server` snippets are inserted inside each generated `server {}` block in `stream.conf`.

These stream contexts are supported only by `Gateway`-targeted `SnippetsPolicy` resources. `SnippetsFilter` remains limited to HTTP contexts.
