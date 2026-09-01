# Enhancement Proposal-5778: Body-Based Routing

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5778
- Status: Implementable

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
- Support matching extracted body fields by exact value or by regular expression.
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

### Understanding the new NGINX directives

This feature is built on a new set of NGINX directives:

- `client_body_preread` -- reads the request body into memory before the backend/location is selected.
- `json_parse` -- extracts a named field (including nested fields, e.g. `params.clientInfo.name`) from a JSON
  string into an NGINX variable.
- `predicate` / `match` -- defines a named rule made up of one or more conditions (on the extracted variables, or
  on any other NGINX variable such as headers, method, or path) that must all be true (logical AND) for the
  predicate to match.
- Predicate-based `location` selection -- chooses a `location` block based on which predicate matched.

At a high level:

```nginx
client_body_preread on;

json_parse $request_body $rpc_method method;

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

Unlike the uptream PayloadProcessor's `PreRouting`/`PostRouting` split -- which exists because extraction there happens ahead of, and
separately from, route matching -- NGF can evaluate the extracted body field natively as part of route matching
itself. Body-based conditions are therefore treated the same as any other `HTTPRouteMatch` condition (header,
method, etc.): the `TargetRef` on an `InProcess` `PayloadProcessor` used for routing can target an `HTTPRoute` (or a
`Gateway`, to apply across all attached routes), consistent with how other Inherited Policies work today.

### Go

```go
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
// +kubebuilder:validation:XValidation:message="extProcess must be set when type is ExtProcess",rule="self.type != 'ExtProcess' || has(self.extProcess)"
// +kubebuilder:validation:XValidation:message="inProcess must be set when type is InProcess",rule="self.type != 'InProcess' || has(self.inProcess)"
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

// InProcessTransform defines header and body mutations.
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
// +k8s:deepcopy-gen=false
type CELExpression string
```

#### A note on CEL

NGF and NGINX do not have a built-in CEL engine. We can write a simple one in NGF (essentially a string parser) so that we can translate the expected CEL format of `json(request.body).fieldName` into the proper nginx config for reading the body. In other words, this would just translate to `json_parse $request_body $body_field fieldName;`

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
    client_body_preread on;
    json_parse $request_body $body_model model;

    predicate $gpt4_route {
        match $request_method = POST;
        match $uri = /v1/chat/completions;
        match $body_model = gpt-4;
    }

    proxy_set_header X-Gateway-Model-Name $body_model;

    location $gpt4_route {
        proxy_pass http://gpt4-backend;
    }

    location / {
        return 404;
    }
}
```

Note that the `predicate`'s `match $body_model = gpt-4` condition is compiled directly from the `HTTPRoute`'s
`X-Gateway-Model-Name: gpt-4` header match plus the `PayloadProcessor`'s `json(request.body).model` extraction --
NGF resolves the indirection between the two at translation time, rather than at request time.

From the user's perspective, this is identical to the upstream GEP-5091 pattern: define a `PayloadProcessor` that
extracts a body field into a header, then write a normal `HTTPRoute` that matches on that header. NGF will honor
that API exactly as written and will still set `X-Gateway-Model-Name` on the request, since the user explicitly
configured it via `setHeaders` and other consumers (the backend, other filters/policies, request logs) may depend
on seeing it.

Internally, NGF translates this configuration into `client_body_preread` and `json_parse` directives, and compiles
the `HTTPRoute`'s `X-Gateway-Model-Name: gpt-4` header match into a `predicate`/`match` rule evaluated directly
against the `json_parse`-extracted variable (e.g. `$model = gpt-4`), rather than against the header NGINX itself
sets on the request. The observable result matches the literal "set header, then match on header" semantics implied
by the API -- the header is present with the correct value, and the request is routed as if that header had been
matched -- without NGF depending on the header for the match itself.

## Use Cases

- As an Application Developer, I want to route inference requests to the correct model backend based on a `model`
  field in the JSON request body, without modifying my application or writing NGINX configuration by hand.
- As an Application Developer building an MCP server, I want to route requests to different backends based on the
  JSON-RPC `method` field (e.g. `initialize`, `tools/list`, `tools/call`), so that discovery, control, and tool
  invocation traffic can be handled independently.
- As a Cluster Operator, I want routes that don't use body-based matching to see no change in behavior or
  performance.

## Testing

- Unit tests
- Functional tests validating end-to-end routing decisions based on body content, including
  nested fields, multiple combined conditions, and interaction with existing path/method/header/query matches.


## Security Considerations

- Reading the request body before routing requires buffering it in memory. Existing body size limits (e.g.
  `client_max_body_size`, `ClientSettingsPolicy`'s body buffer settings) apply, and requests exceeding configured
  limits will fail closed (rejected) rather than silently skipping body-based matching.
- Malformed or non-JSON bodies on a route that requires body-based matching will fail closed by default, rather
  than falling through to an unintended backend.
- Values extracted from the body and written into headers will be sanitized to prevent header injection (e.g.
  stripping or rejecting CR/LF characters) before being set on the proxied request.
- Because extracted values may be echoed into a request header that is forwarded to the backend, users should be
  aware that sensitive body content (tokens, PII, etc.) used for routing will also be visible to the backend and to
  anything that logs request headers.

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
