# Enhancement Proposal-4051: Session Persistence for NGINX Plus and OSS

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4051
- Status: Provisional

## Summary

Enable NGINX Gateway Fabric to support session persistence for both NGINX Plus and NGINX OSS, allowing application developers to configure basic session persistence using the `ip_hash` load balancing method in OSS and cookie-based session persistence in NGINX Plus.

## Goals

- Extend the Upstream Settings Policy API to allow specifying `ip_hash` load balancing method to support basic session persistence.
- Design the translation of the Gateway API `sessionPersistence` specification, which can be configured on both HTTPRoute and GRPCRoute, into NGINX Plus cookie-based session persistence directives with `secure` and `httpOnly` mode enforced by default.

## Non-Goals

- Describe or implement low-level configuration details for enabling session persistence.
- Extend session persistence support to TLSRoutes or other Layer 4 route types.
- Supporting the `sameSite` cookie directive for NGINX Plus session persistence, which may be considered in the future as the Gateway API `sessionPersistence` specification evolves.

## Useful Links

- Session Persistence [specification](https://gateway-api.sigs.k8s.io/reference/spec/#sessionpersistence).
- Extended Session Persistence [GEP](https://gateway-api.sigs.k8s.io/geps/gep-1619).
- RFC standard for [Set-Cookie](https://datatracker.ietf.org/doc/html/rfc6265#section-4.1) header.
