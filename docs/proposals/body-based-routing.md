# Enhancement Proposal-5778: Body-Based Routing

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5778
- Status: Implementable

## Summary

This Enhancement Proposal enables NGINX Gateway Fabric to make routing decisions based on fields extracted from a
request's JSON body, in addition to the host, path, method, headers, and query parameters it can already match on
today. This is powered by a new set of NGINX directives (`client_body_early_read`, `json_set`, `predicate`, `match`)
that read the request body, extract named JSON fields into variables, and evaluate them against ordered rules to
select a backend, all while forwarding the original body to the backend unchanged.

## Goals

- Allow Application Developers to route requests to different backends based on the value of one or more fields in a
  JSON request body, including nested fields.
- Allow a single routing rule to combine body field conditions with existing match types (host, path, method,
  headers, query parameters).
- Support matching extracted body fields by exact value or by regular expression.
- Handle malformed, incomplete, or oversized request bodies safely: an oversized body or a body that doesn't
  arrive in time fails closed via NGINX's existing `client_max_body_size`/`client_body_timeout` behavior, while a
  malformed/incomplete JSON body (including one that spilled to a temporary file) is treated as an absent value
  for matching purposes, using native `json_set`/predicate semantics, rather than a forced error.
- Forward the original, unmodified request body to the selected backend.
- Ensure listeners with no body-based `PayloadProcessor` attached, whether directly or via an attached route, are
  completely unaffected in behavior and performance.
- Ensure existing NGINX Gateway Fabric capabilities (TLS termination, load balancing, rate limiting, logging,
  tracing, etc.) continue to work unchanged for routes that use body-based matching, including capabilities that
  themselves read, buffer, or suppress the request body (request mirroring, `ExternalAuth`'s body forwarding, the
  inference extension's endpoint-picker, AI Guardrails).

## Non-Goals

- Supporting request body formats other than JSON (e.g. XML, Protobuf, form-encoded).
- Supporting matching on response bodies.
- Supporting streaming or chunked evaluation of the request body; the body must be fully read before matching
  occurs.
- Supporting actions triggered by body content other than backend/route selection (e.g. body mutation, rejection
  based on arbitrary business logic, header injection from body fields).
- Defining body-based matching for TCP, UDP, or TLS (stream) routes.
- Supporting a dedicated way to match on an extracted body field being empty or absent, as distinct from matching
  a specific value.

## Introduction

Today, NGF can select a backend for a request using the standard Gateway API `HTTPRouteMatch` conditions: path,
method, headers, and query parameters. None of these can look inside the request body. This is a growing gap for
workloads like AI inference and MCP (Model Context Protocol), where the information needed to make a routing
decision -- the model name, the RPC method being invoked, a tenant identifier, etc. -- lives in a JSON body field
rather than in the URL or headers.

Upstream Gateway API is exploring the broader problem of payload processing via
[GEP-5091](https://github.com/kubernetes-sigs/gateway-api/pull/5092), which introduces a `PayloadProcessor`
resource. `PayloadProcessor` is intentionally general-purpose: it's a mechanism for extracting information from (or
acting on) a request/response payload, and routing is only one of several intended use cases, alongside things like
inspection (e.g. validating or logging payload content) and manipulation (e.g. rewriting or redacting fields). For
the routing use case specifically, the GEP's example works by having a `PayloadProcessor` extract a body field and
write it into a request header using a CEL expression (`json(request.body).model`), and an
`HTTPRoute` then matches on that header. The ideal, most direct way to express a body-based routing decision would
be a native body condition on `HTTPRouteMatch` itself, but no such condition exists in Gateway API today; the
header-extraction approach is a practical way to achieve the same outcome using `PayloadProcessor` as it exists
now.

NGF already has its own `PayloadProcessor` CRD, currently used only for
the `ExtProcess` (AI Guardrails) use case, where an external service inspects and can reject requests/responses.
This proposal extends that same CRD with a new `InProcess` processor type executed natively by NGINX rather than by
an external service. Like its upstream counterpart, `InProcess` is a general extraction/action mechanism; this
proposal, however, is scoped specifically to the routing use case -- extracting a body field and making it
available for `HTTPRoute` matching.

### Prerequisite

Replacing our existing NJS matching module with predicate matching must be completed before implementing this feature. NJS and predicate routing can't coexist.

### Understanding the new NGINX directives

This feature is built on a new set of NGINX directives:

- [`client_body_early_read`](https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_early_read) --
  reads the request body into memory immediately after the request headers are received, before the
  backend/location is selected.
- [`json_set`](https://nginx.org/en/docs/http/ngx_http_json_module.html) -- extracts a named field (including
  nested fields, e.g. `params.clientInfo.name`) from a JSON document held in one NGINX variable into another
  NGINX variable.
- `predicate` / `match` -- defines a named rule made up of one or more conditions (on the extracted variables, or
  on any other NGINX variable such as headers, method, or path) that must all be true (logical AND) for the
  predicate to match.
- Predicate-based `location` selection -- chooses a `location` block based on which predicate matched.

At a high level:

```nginx
client_body_early_read on;

json_set $rpc_method $request_body method;

predicate $mcp_init {
    match $rpc_method = initialize;
}

server {
    location $mcp_init {
        proxy_pass http://mcp_control;
    }
    location / {
        return 404;
    }
}
```

## API, Customer Driven Interfaces, and User Experience

We will extend the existing `PayloadProcessor` Inherited Policy rather than introduce a new CRD. This keeps
a single, general-purpose policy for "do something with the payload" and mirrors the shape of the upstream GEP-5091
resource, which also models `InProcess` as a processor type alongside `ExtProcess`. Note that the `InProcessConfig`
defined below is intentionally named and structured to leave room for future, non-routing `InProcessTransform` fields
(inspection, mutation, etc.); this proposal only defines the `SetHeaders` action needed for routing. Aligning our
shape with upstream now reduces the migration cost if/when NGF adopts the upstream resource directly.

NGF adopts the same phase concept as the upstream PayloadProcessor's `PreRouting`/`PostRouting` split, but maps it
directly onto the two fixed processor types instead of a generic ordered list: `InProcess` processing runs in the
`PreRouting` phase, ahead of route matching, and `ExtProcess` processing runs in the `PostRouting` phase, after a
route has been selected. Because `InProcess` runs before routing, NGF can evaluate the extracted body field
natively as part of route matching itself -- body-based conditions are therefore treated the same as any other
`HTTPRouteMatch` condition (header, method, etc.). The `TargetRef` on an `InProcess` `PayloadProcessor` used for
routing can target an `HTTPRoute` (or a `Gateway`, to apply across all attached routes), consistent with how other
Inherited Policies work today.

The existing `PayloadProcessorSpec.Processors` list is capped at one entry (`MaxItems=1`) and documented as an
ordered pipeline, which is sufficient while `ExtProcess` is the only processor type. This proposal raises that cap
to one entry per `ProcessorType` (an `ExtProcess` entry and an `InProcess` entry), so a single `PayloadProcessor`
can carry AI Guardrails and body-based routing configuration together. With only two fixed types, list order is no
longer a meaningful way to express execution order once entries can come from different scopes (Gateway vs.
Route), so this proposal replaces list order with the fixed, phase-based execution order described above:
`InProcess` (`PreRouting`) always executes first, since its output (the extracted body field) is needed to select
the route in the first place, and `ExtProcess` (`PostRouting`) always executes second, against the already-routed
request -- matching how AI Guardrails operates today. `Processors` list order is no longer significant and is not
validated.

Selection across the Gateway/Route hierarchy also follows this phase order: for each `ProcessorType`/phase, NGF
resolves the effective entry by considering the Gateway-level entry of that type first and the HTTPRoute-level
entry of that type second, so an HTTPRoute-level entry overrides a Gateway-level entry of the same type when both
are present. This preserves a Gateway-level `ExtProcess` (Guardrails) policy for a Route that only attaches its
own `InProcess` entry for routing, rather than the Route's policy silently overriding Guardrails for that Route.
If two `PayloadProcessor` resources instead target the *same* phase with the *same* target reference (for example,
two Gateway-scoped `PayloadProcessor` resources that both contain an `InProcess` entry and target the same
`Gateway`), the newer resource is ignored: NGF applies its existing Inherited Policy conflict-resolution rule,
under which the older resource (by `CreationTimestamp`) wins and the newer, conflicting resource is marked invalid
via a `PolicyConflicted` status condition, consistent with the [Gateway API conflict resolution
guidelines](https://gateway-api.sigs.k8s.io/concepts/guidelines/?h=conflict#conflicts).

### Go

```go
// PayloadProcessorSpec defines the desired state of a PayloadProcessor.
type PayloadProcessorSpec struct {
    // TargetRef identifies the Gateway or HTTPRoute this policy applies to.
    TargetRef gatewayv1.LocalPolicyTargetReference `json:"targetRef"`

    // Processors is a list of processing steps applied to the request and response payloads,
    // with at most one entry per ProcessorType. Execution order is fixed by type (InProcess, then
    // ExtProcess) rather than by list order.
    //
    // +kubebuilder:validation:MinItems=1
    // +kubebuilder:validation:MaxItems=2
    // +kubebuilder:validation:XValidation:message="processors must not contain duplicate types",rule="self.all(x, self.exists_one(y, y.type == x.type))"
    Processors []PayloadProcessorEntry `json:"processors"`
}

// ProcessorType specifies how the processor executes.
// ExtProcess calls an external service. InProcess is executed by NGINX itself, without a network hop.
//
// +kubebuilder:validation:Enum=ExtProcess;InProcess
type ProcessorType string

const (
    // ProcessorTypeExtProcess delegates processing to an external service.
    ProcessorTypeExtProcess ProcessorType = "ExtProcess"

    // ProcessorTypeInProcess is executed within the NGINX data plane.
    ProcessorTypeInProcess ProcessorType = "InProcess"
)

// PayloadProcessorEntry defines a single processing step in the pipeline.
//
// +kubebuilder:validation:XValidation:message="extProcess must be set if and only if type is ExtProcess",rule="(self.type == 'ExtProcess') == has(self.extProcess)"
// +kubebuilder:validation:XValidation:message="inProcess must be set if and only if type is InProcess",rule="(self.type == 'InProcess') == has(self.inProcess)"
type PayloadProcessorEntry struct {
    // Type specifies how the processor executes.
    Type ProcessorType `json:"type"`

    // ExtProcess defines the configuration for an ExtProcess processor that delegates to an external service.
    //
    // +optional
    ExtProcess *ExtProcessConfig `json:"extProcess,omitempty"`

    // InProcess defines the configuration for an InProcess processor that runs within NGINX.
    //
    // +optional
    InProcess *InProcessConfig `json:"inProcess,omitempty"`
}

// InProcessConfig configures payload processing that runs directly
// in the gateway process using CEL expressions.
type InProcessConfig struct {
    // Request defines the actions to take on the request payload.
    //
    // +optional
    Request *InProcessTransform `json:"request,omitempty"`
}

// InProcessTransform defines header mutations.
// CEL expressions can access request.body via the json() function,
// e.g. json(request.body).model
type InProcessTransform struct {
    // SetHeaders is a list of headers to set (overwrite if existing).
    // The value is a CEL expression.
    //
    // +listType=map
    // +listMapKey=name
    // +kubebuilder:validation:MinItems=1
    // +kubebuilder:validation:MaxItems=16
    // +optional
    SetHeaders []HeaderTransformation `json:"setHeaders,omitempty"`
}

// HeaderTransformation sets a header to a CEL-evaluated value.
type HeaderTransformation struct {
	// Name is the HTTP header name.
	Name gatewayv1.HeaderName `json:"name"`

	// Value is the CEL expression that produces the header value.
	// Use json(request.body).fieldName to extract from the JSON body.
	Value CELExpression `json:"value"`
}

// CELExpression is a string containing a CEL expression.
//
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=1024
// +kubebuilder:validation:Pattern=`^json\(request\.body\)(\.[A-Za-z_][A-Za-z0-9_]*)+$`
// +k8s:deepcopy-gen=false
type CELExpression string
```

#### A note on CEL

NGF and NGINX do not have a built-in CEL engine. Rather than implement one, this proposal constrains `CELExpression`
to a single supported grammar -- `json(request.body).field[.nestedField...]` -- validated by a CRD `Pattern`, and
translates it directly into the corresponding NGINX config. In other words, `json(request.body).model` translates
to `json_set $body_field $request_body model;`. Any value that doesn't match this pattern is rejected by the API
server at admission time, so there's no runtime notion of an "invalid CEL expression" to handle: the field is
named and typed as `CELExpression` to align with the upstream GEP-5091 shape and leave room for real CEL support
later, but today it accepts only this fixed grammar.

#### Header name uniqueness

`SetHeaders`'s `+listType=map`/`+listMapKey=name` markers only give us CRD-level uniqueness on the `name` field as
an exact string match, but HTTP header names are case-insensitive, so the API as declared would still accept both
`X-Model` and `x-model` in the same list. Compiling that to NGINX config would make the resulting overwrite
behavior and generated `proxy_set_header` directives ambiguous (whichever entry happens to be processed last would
silently win). To prevent this, NGF will validate `SetHeaders` names for case-insensitive uniqueness the same way
it already validates `HTTPHeaderFilter`'s `Add`/`Set` header names today, rejecting
the `PayloadProcessor` with a `PolicyInvalid`-style condition if two `SetHeaders` entries collide case-insensitively.

### YAML

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: PayloadProcessor
metadata:
  name: extract-model
  namespace: default
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: gpt4-route
  processors:
  - type: InProcess
    inProcess:
      request:
        setHeaders:
        - name: X-Gateway-Model-Name
          value: 'json(request.body).model'
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: gpt4-route
  namespace: default
spec:
  parentRefs:
  - name: ai-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v1/chat/completions
      method: POST
      headers:
      - name: X-Gateway-Model-Name
        value: gpt-4
    backendRefs:
    - name: gpt4-backend
      port: 8080
```

A `method` match is included above alongside the body-derived header match to illustrate that body conditions
compose with the existing match types exactly like any other condition on the same rule: all conditions in a
single `matches` entry are ANDed together, so this rule only selects `gpt4-backend` for `POST` requests to
`/v1/chat/completions` whose body's `model` field is `gpt-4`.

### NGINX Configuration

Above resources roughly translated into nginx config:

```nginx
server {
    client_body_early_read on;
    json_set $body_model $request_body model;

    predicate $gpt4_route {
        match $request_method = POST;
        match $uri ~ ^/v1/chat/completions(/|$);
        match $body_model = gpt-4;
    }

    location $gpt4_route {
        proxy_set_header X-Gateway-Model-Name gpt-4;
        proxy_pass http://gpt4-backend;
    }

    location / {
        return 404;
    }
}
```

Note that the `predicate`'s `match $body_model = gpt-4` condition is compiled directly from the `HTTPRoute`'s
`X-Gateway-Model-Name: gpt-4` header match plus the `PayloadProcessor`'s `json(request.body).model` extraction --
NGF resolves the indirection between the two at translation time, rather than at request time. The header value
itself, `gpt-4`, comes straight from the `HTTPRoute`'s header match condition, so it's a static, already-validated
string known at config-build time.

From the user's perspective, this is identical to the upstream GEP-5091 pattern: define a `PayloadProcessor` that
extracts a body field into a header, then write a normal `HTTPRoute` that matches on that header. NGF will honor
that API exactly as written and will still set `X-Gateway-Model-Name` on the request, since the user explicitly
configured it via `setHeaders` and other consumers (the backend, offload services) may depend
on seeing it.

Internally, NGF translates this configuration into `client_body_early_read` and `json_set` directives, and compiles
the `HTTPRoute`'s `X-Gateway-Model-Name: gpt-4` header match into a `predicate`/`match` rule evaluated directly
against the `json_set`-extracted variable (e.g. `$model = gpt-4`), rather than against the header NGINX itself
sets on the request. The observable result matches the literal "set header, then match on header" semantics implied
by the API -- the header is present with the correct value, and the request is routed as if that header had been
matched -- without NGF depending on the header for the match itself.

## Use Cases

- As an Application Developer, I want to route inference requests to the correct model backend based on a `model`
  field in the JSON request body, without modifying my application or writing NGINX configuration by hand.
- As an Application Developer building an MCP server, I want to route requests to different backends based on the
  JSON-RPC `method` field (e.g. `initialize`, `tools/list`, `tools/call`), so that discovery, control, and tool
  invocation traffic can be handled independently.
- As a Cluster Operator, I want routes on listeners with no body-based `PayloadProcessor` attached, directly or via
  another route on the same listener, to see no change in behavior or performance. For a route that shares a
  listener with a body-matched route, I accept that its requests will also be preread and buffered (since the
  listener must buffer bodies for the sibling route regardless), and that its own `body.maxSize`, `bufferSize`, and
  `timeout` settings are overridden by the listener's Gateway-scoped values for that pre-routing read (see Security
  Considerations below), but I expect no other change to that route's routing correctness, security, or backend
  behavior as a result.

## Testing

- Unit tests
- Functional tests validating end-to-end routing decisions based on body content, including nested fields, multiple
  combined conditions, existing path/method/header/query matches, malformed/missing/oversized bodies Gateway/Route
  inheritance and conflicts.
- Functional compatibility tests for request mirroring, `ExternalAuth` body forwarding, the inference extension's
  endpoint-picker, and AI Guardrails, verifying that each still receives the complete, unchanged request body.


## Security Considerations

- Reading the request body before routing requires buffering it in memory, and the settings that normally govern
  that read -- `client_max_body_size`, `client_body_buffer_size`, and `client_body_timeout` -- are ordinarily
  applied per-location, after routing has already selected one. `client_body_early_read`, however, reads the body
  at the server level, *before* any location (and therefore any location-scoped `ClientSettingsPolicy`) has been
  selected. This timing problem applies equally to all three settings, not just buffer size: whichever effective
  values are in force for the pre-routing read are the only ones that actually govern that request, since a
  route-scoped override of any of them cannot retroactively re-check a read that already happened at the server
  level under different values -- a smaller Route-scoped `client_max_body_size` can't reject an oversized body the
  server-level value already accepted, and a larger Route-scoped value can't rescue a body the server-level value
  already rejected.

  For this reason, `client_max_body_size`, `client_body_buffer_size`, and `client_body_timeout` for the
  pre-routing read are always taken from the listener's **Gateway-scoped** `ClientSettingsPolicy` `body` settings
  (falling back to NGINX's own platform defaults -- e.g. `1m`, `8k`/`16k`, `60s` -- for any that are unset), for
  every listener that has at least one body-based `PayloadProcessor` attached, regardless of whether that
  `PayloadProcessor` is itself attached to the `Gateway` or to one of the listener's `HTTPRoute`s. This
  Gateway-scoped fallback is what governs the primary use case in this proposal, where `InProcess` is attached
  directly to an `HTTPRoute` (see the [YAML example](#yaml) above): even though the triggering `PayloadProcessor`
  is Route-scoped, the values that bound the pre-routing read still have to come from the Gateway's
  `ClientSettingsPolicy`, because the Gateway scope is the only one in effect before any route has been selected.
  This puts the choice of these limits in the hands of the Cluster Operator, who already controls `body` settings
  for their own reasons and understands their own body sizes, rather than NGF guessing a one-size-fits-all
  constant.

  A `body` setting on an `HTTPRoute`-scoped `ClientSettingsPolicy` is therefore **incompatible with body-based
  routing on that route** and has no effect on the pre-routing read: by the time a location is selected, the body
  has already been fully read (or the request already rejected) using the Gateway-scoped values, so a
  Route-scoped `body.maxSize`, `body.bufferSize`, or `body.timeout` can never be enforced or relaxed for that
  request. NGF will report this with a status condition on the Route-scoped `ClientSettingsPolicy` (indicating it
  is overridden/ineffective) whenever it targets a route that shares a listener with a body-based
  `PayloadProcessor`, rather than silently ignoring it.

  A request that exceeds `client_max_body_size` or `client_body_timeout` during the pre-routing read is rejected
  before a route is even selected, with the same status NGINX already returns for each of these conditions today
  (`413` for size, `408` for timeout) -- this is unchanged, native NGINX behavior; NGF isn't introducing anything
  new here, only fixing which `ClientSettingsPolicy` governs the values these checks are made against.

  A body that exceeds `client_body_buffer_size` is not an error condition in NGINX at all: NGINX spills the excess
  to a temporary file and continues, exactly as it does today for a normally-routed request. `$request_body` is
  only populated when the body was read into memory; a spilled body leaves `$request_body` -- and so every
  `json_set`-extracted field derived from it -- **not found**, per the `ngx_http_json_module` documentation. This
  isn't a special case NGF needs to invent handling for: it's the same "not found" state produced by a missing
  field or malformed JSON (see below), and NGF doesn't add any interception, custom module, or forced status code
  on top of it, unlike AI Guardrails' custom `ngx_http_ai_guardrails_module`, which deliberately fails closed on a
  spilled body because it must inspect the full prompt to make a safety decision. Body-based routing has no
  equivalent requirement to inspect the whole body -- only the specific extracted fields matter -- so there's
  nothing to fail closed over: whatever locations don't depend on the missing field still work normally, and
  whichever location predicate would have depended on it simply doesn't match, the same as if the field were
  absent from a well-formed body.
- `client_body_early_read` is scoped to listeners with at least one body-based `PayloadProcessor` attached;
  listeners without one are unaffected, consistent with the Goals above. Within a listener that has one attached,
  a missing field, a malformed/incomplete JSON body, or a body that spilled to disk are all indistinguishable to
  `json_set`: every extracted variable is simply **not found**. Per native `location $variable { ... }` semantics,
  a predicate whose condition depends on a not-found variable is not empty and not "0", so it simply does not
  match -- there is no special error path or forced status code. Request handling falls through to whichever
  `location` would otherwise apply next (typically the deployment's catch-all/default backend, per the compiled
  example above), exactly as an ordinary unmatched regex or predicate location falls through today.
- A header set from a body-derived match is never the raw, request-derived value: it's the literal value declared
  in the `HTTPRoute`'s header match condition, which NGF already validates as a well-formed HTTP header value
  today, the same as any other header value it sets. The `json_set`-extracted variable itself is only ever used on
  the left-hand side of a `predicate`/`match` comparison to decide whether a rule matches -- it's never forwarded
  to the backend directly -- so there's no path by which unvalidated, request-controlled body content reaches a
  proxied header, and no additional runtime sanitization is needed.
- Because extracted values may be echoed into a request header that is forwarded to the backend, users should be
  aware that sensitive body content (tokens, PII, etc.) used for routing will also be visible to the backend and to
  anything that logs request headers.
- Because NGF matches directly against the `json_set`-extracted value rather than the header value, the routing
  decision cannot be spoofed by a client setting its own `X-Gateway-Model-Name` header. NGF overwrites or strips
  the client-supplied header value before proxying, using the value derived from the actual request body, even
  when that value is an empty string because the field is absent from the body.

## Alternatives

- **Extend `HTTPRouteMatch` directly with a body-based condition.** This was considered but impossible for now,
since `HTTPRouteMatch` is a closed Gateway API struct. If upstream Gateway API eventually adds a native body
condition  to `HTTPRouteMatch`, we should reevaluate using it directly as a simpler alternative to the
`PayloadProcessor` + header-match indirection described in this proposal. This is an initiative that we could drive forward.
- **Add body matching as an `HTTPRoute` filter.** Rather than extending `PayloadProcessor`, we could add a new
  extension filter attached inline to an `HTTPRouteRule`, which would both
  extract the body field and gate whether the rule matches. This was rejected because filters run against a rule
  that has already been selected -- they are not inputs to route selection itself -- so a filter cannot express
  "only match this rule if the body satisfies X" without reworking how filters participate in route matching.
  This goes against Gateway API semantics.
- **Introduce a new, separate route type (e.g. a `BodyMatchRoute` CRD) dedicated to body-based routing.** Rejected
because it would fragment routing configuration across multiple route types for what is conceptually just another
match condition alongside path, method, headers, and query parameters.

## References

- [GEP-5091: PayloadProcessor Resource - Internal Processing](https://github.com/kubernetes-sigs/gateway-api/pull/5092)
- [`client_body_early_read`](https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_early_read)
- [`json_set` (ngx_http_json_module)](https://nginx.org/en/docs/http/ngx_http_json_module.html)
