# Enhancement Proposal-5848: Access Control Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5848
- Status: Provisional

## Summary

This Enhancement Proposal introduces the `AccessPolicy` API that allows Cluster Operators and Application Developers
to configure IP-based access control (allowlists and denylists) for traffic flowing through NGINX Gateway Fabric.
The API is designed to be structurally compatible with the upstream
[kube-agentic-networking XAccessPolicy](https://github.com/kubernetes-sigs/kube-agentic-networking/blob/main/api/v1alpha1/accesspolicy_types.go)
so that a future migration to the community API is possible without breaking changes for users.

## Goals

- Define an `AccessPolicy` custom resource for IP-based allowlist and denylist enforcement.
- Support specific IP addresses and CIDR ranges (IPv4 and IPv6).
- Support both default-allow (denylist) and default-deny (allowlist) modes through an action-based rule model.
- Support attachment to Gateway, HTTPRoute, and GRPCRoute as an Inherited Attached Policy.
- Design the API structure (rule shape, source types, action semantics) to be forward-compatible with the upstream
  kube-agentic-networking XAccessPolicy, so that future addition of identity-based sources (ServiceAccount, SPIFFE)
  and MCP authorization attributes can extend the same CRD without restructuring.
- Handle X-Forwarded-For and other proxy headers for accurate client IP detection.

## Non-Goals

- Identity-based access control sources (ServiceAccount, SPIFFE). Future extension.
- MCP-specific authorization attributes (method-level, tool-level). Future extension.
- CEL-based authorization expressions. Future extension.
- Attachment to TLSRoute or TCPRoute.
- Geographic (GeoIP) based access control. May be added as a future extension to the same API.
