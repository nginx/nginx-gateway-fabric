# Enhancement Proposal-5778: Body-Based Routing

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5778
- Status: Provisional

## Summary

This Enhancement Proposal enables NGINX Gateway Fabric to make routing decisions based on fields extracted from a
request's JSON body, in addition to the host, path, method, headers, and query parameters it can already match on
today. This is powered by a new set of NGINX directives (`client_body_preread`, `json_parse`, `predicate`, `match`)
that read the request body, extract named JSON fields into variables, and evaluate them against ordered rules to
select a backend, all while forwarding the original body to the backend unchanged.

## Goals

- Allow Application Developers to route requests to different backends based on the value of one or more fields in a
  JSON request body, including nested fields.
- Allow a single routing rule to combine body field conditions with existing match types (host, path, method,
  headers, query parameters).
- Support matching extracted body fields by exact value and by regular expression.
- Support matching on the presence, absence, or emptiness of a body field.
- Handle malformed JSON and oversized request bodies safely, failing closed by default.
- Forward the original, unmodified request body to the selected backend.
- Ensure routes that do not use body-based matching are completely unaffected in behavior and performance.
- Ensure existing NGINX Gateway Fabric capabilities (TLS termination, load balancing, rate limiting, logging,
  tracing, etc.) continue to work unchanged for routes that use body-based matching.

## Non-Goals

- Supporting request body formats other than JSON (e.g. XML, Protobuf, form-encoded).
- Supporting matching on response bodies.
- Supporting streaming or chunked evaluation of the request body; the body must be fully read before matching
  occurs.
- Supporting actions triggered by body content other than backend/route selection (e.g. body mutation, rejection
  based on arbitrary business logic, header injection from body fields).
- Defining body-based matching for TCP, UDP, or TLS (stream) routes.
