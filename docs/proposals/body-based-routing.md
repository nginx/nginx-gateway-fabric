# Enhancement Proposal-5778: Body-Based Routing

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5778
- Status: Implementable

## Summary

This Enhancement Proposal enables NGINX Gateway Fabric to make routing decisions based on fields extracted from a
request's JSON body, in addition to the host, path, method, headers, and query parameters it can already match on
today. This is powered by a new set of NGINX directives (`client_body_early_read`, `json_set`, `condition`)
that read the request body, extract named JSON fields into variables, and evaluate them against named condition
blocks to select a backend, all while forwarding the original body to the backend unchanged.

## Goals

- Allow Application Developers to route requests to different backends based on the value of one or more fields in a
  JSON request body, including nested fields.
- Allow a single routing rule to combine body field conditions with existing match types (host, path, method,
  headers, query parameters).
- Support matching extracted body fields by exact value or by regular expression.
- Handle malformed, incomplete, or oversized request bodies safely: an oversized body or a body that doesn't
  arrive in time fails closed via NGINX's existing `client_max_body_size`/`client_body_timeout` behavior, while a
  malformed/incomplete JSON body (including one that spilled to a temporary file) is treated as an absent value
  for matching purposes, using native `json_set`/`condition` semantics, rather than a forced error.
- Forward the original, unmodified request body to the selected backend.
- Ensure requests that don't match a body-based route's path and method are never preread, even on a listener
  whose `filters.requests` list includes a `RequestHeaderModifier` pre-routing filter with a body-sourced header
  value, by using `client_body_early_read` with a variable that NGF derives from the union of body-matched
  routes' path/method conditions (see [NGINX Configuration](#nginx-configuration) and
  [Security Considerations](#security-considerations) below). Listeners with no such filter attached at all
  remain completely unaffected.
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

Upstream Gateway API defines the attachment point for this kind of problem via
[GEP-5224: Pre-Routing Filters](https://gateway-api.sigs.k8s.io/geps/gep-5224/), which adds a new
`Listener.filters.requests []ListenerFilter` field: an ordered list of filters that run on every request accepted
by a Listener, before route matching happens. Filters attach to a `Listener` rather than an `HTTPRoute`
specifically because a filter that needs to run *before* a route is selected can't be scoped to the very
`HTTPRoute` it's supposed to help select -- by the time route-scoped configuration would apply, the route has
already been chosen. `ListenerFilter` has three variants: `ExternalAuth`, for establishing identity before
routing; `RequestHeaderModifier`, for setting, adding, or removing request headers before routing; and
`ExtensionRef`, the escape hatch for any other kind of pre-routing processing. `RequestHeaderModifier` is the
piece this proposal relies on, reused completely unchanged from its existing post-routing `HTTPRouteFilter`
definition (`HTTPHeaderFilter`/`HTTPHeader{Name, Value}`) -- no new Go type or field. The one capability that
makes it useful pre-routing is that its `Value` now supports a dynamic value, not just a static literal. For
NGINX, a dynamic value is a body-reference expression such as `json(request.body.model)` (the exact expression
syntax is still TBD), which NGF resolves to the live JSON field at the given path, rather than to a literal
string. This is exactly the primitive GEP-5224's own body-based-routing user story describes, as the same
filter type used post-routing today, rather than an opaque extension.

This proposal adopts GEP-5224's `Listener.filters.requests` mechanism directly, implementing support for the
`RequestHeaderModifier` pre-routing filter's dynamic-value header support (see [API, Customer Driven Interfaces,
and User Experience](#api-customer-driven-interfaces-and-user-experience) below). NGF introduces no CRD or Go
API of its own: a Gateway author adds a `RequestHeaderModifier` entry directly to a Listener's
`filters.requests` list, setting a header's `Value` to a body-reference expression (`json(request.body.model)`,
exact syntax TBD), and any `HTTPRoute` attached to that Listener then matches on the resulting header using
ordinary `HTTPHeaderMatch` conditions, exactly as GEP-5224's own body-based routing user story describes.
Because `filters.requests` is a field directly on the Gateway API `Listener` struct, such a filter is inherently
scoped to the single Listener it's attached to -- there's no `TargetRef`/`SectionName` narrowing to model, and
no cross-resource conflict resolution to define, since each Listener owns its own ordered filter list.

GEP-5224 is implemented and gated behind the Gateway API experimental release channel.

### Prerequisite

Replacing our existing NJS matching module with [Predicate Routing](https://github.com/nginx/nginx-gateway-fabric/issues/5737)'s condition-based matching must be completed before implementing this feature. NJS and condition-based routing can't coexist.

### Understanding the new NGINX directives

This feature is built on a new set of NGINX directives:

- [`client_body_early_read`](https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_early_read) --
  reads the request body into memory immediately after the request headers are received, before the
  backend/location is selected. The directive accepts one or more `string` parameters that may contain
  variables; the body is only read early if at least one of those values is non-empty and non-`"0"`. This
  lets NGF gate the read on a `map`-derived variable so that only requests matching body-based routes'
  path/method conditions trigger the preread.
- [`json_set`](https://nginx.org/en/docs/http/ngx_http_json_module.html) -- extracts a named field (including
  nested fields, e.g. `params.clientInfo.name`) from a JSON document held in one NGINX variable into another
  NGINX variable.
- `condition` -- defines a named condition block made up of one or more comparisons (on the extracted variables,
  or on any other NGINX variable such as headers, method, or path), combined with an explicit `and;`/`or;`
  combinator when more than one comparison is present, all of which (or, for `or;`, any of which) must evaluate
  true for the condition to match.
- Condition-based `location` selection -- chooses a `location` block based on which condition matched.

At a high level:

```nginx
# Only preread requests whose path matches a body-based route.
# $uri is the normalized path without query string; $request_uri includes
# the query string and would break prefix matching (e.g. /api/rpc?debug=1).
map $uri $need_body_read {
    ~^/api/rpc(/|$)  1;
    default          0;
}

client_body_early_read $need_body_read;

json_set $rpc_method $request_body "method";

condition $mcp_init {
    $rpc_method = initialize;
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

NGF introduces **no new CRD or Go API of its own** for body-based routing. This proposal implements GEP-5224's
native `Listener.filters.requests` mechanism, specifically its `RequestHeaderModifier` pre-routing filter type,
reused unmodified from its existing post-routing definition: NGF's Gateway/Listener reconciliation and config
generation are extended to recognize `Listener.Filters.Requests`, and for any `RequestHeaderModifier` entry
whose header `Value` is a body-reference expression (`json(request.body.<path>)`, exact syntax TBD), to compile
that body-field extraction down to NGINX config before any `location` is selected.

Once the header is set, ordinary `HTTPRoute`s attached to the same Listener match on it using standard
`HTTPHeaderMatch` conditions -- body-based routing conditions are therefore evaluated as input to route matching
using the same mechanism as any other `HTTPRouteMatch` condition, with no new match type required. Because a
`RequestHeaderModifier` pre-routing filter is set directly on a single `Listener`'s `filters.requests` list, its
scope is unambiguous: it applies only to requests accepted by that Listener, and there is no competing
Gateway-wide or HTTPRoute-scoped variant to reconcile against. Conflict resolution is therefore simpler than a
`TargetRef`-based policy: the Gateway API's existing `ListenerFilters` ordering and uniqueness rules (an atomic,
ordered list, validated at admission) govern how multiple filters on the same Listener interact, and NGF does
not need to invent its own conflict-resolution semantics.

### Upstream GEP-5224 types

```go
// Excerpt of the upstream GEP-5224 types this proposal relies on.
type Listener struct {
	// ... existing fields (Name, Hostname, Port, Protocol, TLS, AllowedRoutes) ...

	// Filters define the filters that are applied to requests/responses accepted by this Listener,
	// before route matching occurs.
	//
	// +optional
	Filters *ListenerFilters `json:"filters,omitempty"`
}

type ListenerFilters struct {
	// Requests is an ordered list of filters applied to requests before route matching.
	//
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=16
	Requests []ListenerFilter `json:"requests,omitempty"`
}

type ListenerFilter struct {
	// Type identifies the kind of filter.
	//
	// +kubebuilder:validation:Enum=ExternalAuth;RequestHeaderModifier;ExtensionRef
	Type ListenerFilterType `json:"type"`

	// ExternalAuth defines configuration for establishing authentication before routing.
	//
	// +optional
	ExternalAuth *HTTPExternalAuthFilter `json:"externalAuth,omitempty"`

	// RequestHeaderModifier defines headers to set, add, or remove before routing.
	//
	// +optional
	RequestHeaderModifier *HTTPHeaderFilter `json:"requestHeaderModifier,omitempty"`

	// ExtensionRef references an implementation-specific resource that performs pre-routing processing.
	//
	// +optional
	ExtensionRef *LocalObjectReference `json:"extensionRef,omitempty"`
}

// HTTPHeaderFilter is the exact same type used by the existing post-routing
// HTTPRouteFilter.RequestHeaderModifier, reused here unchanged.
type HTTPHeaderFilter struct {
	Set    []HTTPHeader `json:"set,omitempty"`
	Add    []HTTPHeader `json:"add,omitempty"`
	Remove []string     `json:"remove,omitempty"`
}

type HTTPHeader struct {
	// Name is the name of the header to set, add, or remove.
	Name gatewayv1.HTTPHeaderName `json:"name"`

	// Value is the header value. For a pre-routing RequestHeaderModifier, Value may be a dynamic value
	// instead of a static literal -- for NGF, a body-reference expression such as
	// json(request.body.model) (exact expression syntax TBD), rather than a plain string.
	Value string `json:"value"`
}
```

A Gateway author sets a header from a JSON body field directly on a Listener:

```yaml
listeners:
- name: https
  # ... hostname, port, protocol, tls ...
  filters:
    requests:
    - type: RequestHeaderModifier
      requestHeaderModifier:
        set:
        - name: X-Gateway-Model-Name
          value: json(request.body.model) # body-reference expression, exact syntax TBD
```

#### Field selection

NGF recognizes a header `Value` of the form `json(request.body.<path>)` (e.g. `json(request.body.model)` or
`json(request.body.params.clientInfo.name)`; exact expression syntax TBD) and compiles the dot-path directly
into `ngx_http_json_module`'s own nested-field addressing syntax: `json_set $model $request_body "model";` or
`json_set $client_info_name $request_body "params.clientInfo.name";`, generating its own NGINX variable name for
each extracted field. Since the path is a closed, statically-analyzable dot-path rather than a general
expression language, every valid value translates 1:1 to a `json_set` field path, with no partial-support cases
to reject.

#### Header name uniqueness

HTTP header names are case-insensitive, so NGF validates `RequestHeaderModifier.Set`/`Add` header names for
case-insensitive uniqueness, the same way it already validates `HTTPHeaderFilter`'s `Add`/`Set` header names on
route-scoped `RequestHeaderModifier` filters today -- rejecting the filter with an invalid-filter status
condition on a collision (e.g. both `X-Model` and `x-model` present), rather than silently letting whichever is
processed last win.

### YAML

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: ai-gateway
  namespace: default
spec:
  gatewayClassName: nginx
  listeners:
  - name: https
    hostname: ai.example.com
    port: 443
    protocol: HTTPS
    tls:
      certificateRefs:
      - name: ai-gateway-tls
    filters:
      requests:
      - type: RequestHeaderModifier
        requestHeaderModifier:
          set:
          - name: X-Gateway-Model-Name
            value: json(request.body.model) # body-reference expression, exact syntax TBD
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: gpt4-route
  namespace: default
spec:
  parentRefs:
  - name: ai-gateway
    sectionName: https
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

Note that the `RequestHeaderModifier` pre-routing filter is set directly on the `https` Listener's
`filters.requests`, not on the `HTTPRoute` -- it has to run and take effect before any `HTTPRoute` is selected.
Any number of `HTTPRoute`s attached to that Listener can match on `X-Gateway-Model-Name`; a second route for a
different model would simply add its own `HTTPRouteMatch` with a different header value, reusing the same
Listener-level header-setting configuration.

A `method` match is included above alongside the body-derived header match to illustrate that body conditions
compose with the existing match types exactly like any other condition on the same rule: all conditions in a
single `matches` entry are ANDed together, so this rule only selects `gpt4-backend` for `POST` requests to
`/v1/chat/completions` whose body's `model` field is `gpt-4`.

### NGINX Configuration

Above resources roughly translated into nginx config:

```nginx
# Gate the early read: only fire for paths/methods that have a body-based route.
# Uses $uri (path only) rather than $request_uri (path + query string) so that
# query parameters don't break prefix matching (e.g. /v1/chat/completions?stream=true).
map $request_method:$uri $need_body_read {
    ~^POST:/v1/chat/completions(/|$)  1;
    default                           0;
}

server {
    client_body_early_read $need_body_read;
    json_set $model $request_body "model";

    condition $gpt4_route {
        and;

        $request_method = POST;
        $uri ~ ^/v1/chat/completions(/|$);
        $model = gpt-4;
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

The `map` block is compiled by NGF from the union of all body-based `HTTPRoute` match conditions (paths and
methods) attached to this server block. `client_body_early_read $need_body_read;` evaluates the variable
per-request: a `GET /healthz` or `POST /legacy-api` request on the same listener is never preread, because
`$need_body_read` evaluates to `0`. Only requests whose method and path overlap with at least one body-matched
route trigger the early read. If the server block has multiple body-matched routes with different paths, their
patterns are combined into the same `map`.

When compiling path conditions into the `map`, NGF handles each `HTTPRoute` path type as follows:

- **PathPrefix** (the common case): the path is converted to a PCRE prefix expression. PCRE metacharacters in
  the literal path (e.g. `$`, `.`, `+`) are escaped before embedding, the same way NGF already escapes them
  when generating `location` regex rewrites today. The root prefix `/` is special-cased: because every URI
  starts with `/`, `PathPrefix: /` compiles to `~^POST:/` (matching all paths under the given method) rather
  than `~^POST:/(/|$)`, which would only match `/` and `//`. For non-root paths the leading `/` is retained
  in the escaped form; for example, a PathPrefix of `/$coffee` becomes `~^POST:/\$coffee(/|$)` in the `map`.
- **Exact**: the path is escaped the same way and anchored with `$` so that only an exact path match triggers
  the read (e.g. `~^POST:/v1/chat/completions$`).
- **RegularExpression**: the path regex is wrapped in a non-capturing group before prepending the method prefix,
  so that PCRE alternation inside the original expression stays scoped correctly. For example, a path regex of
  `/v[12]/chat/.*` becomes `~^POST:(?:/v[12]/chat/.*)$`, and an alternation like `/foo|/bar` becomes
  `~^POST:(?:/foo|/bar)$` rather than `~^POST:/foo|/bar$` (which would let the second branch match any method).

Note that the `condition`'s `$model = gpt-4` comparison is compiled directly from the `HTTPRoute`'s
`X-Gateway-Model-Name: gpt-4` header match plus the Listener's `RequestHeaderModifier` filter's
`value: json(request.body.model)` extraction -- NGF resolves the indirection between the two at translation
time, rather than at request time.

#### Header value: exact match vs. regular-expression match

The value set by `proxy_set_header` depends on the `HTTPRoute`'s header match type:

- **Exact match** (the common case, shown above): the header value is the **static literal** from the
  `HTTPRoute`'s `HTTPHeaderMatch.Value` (e.g. `gpt-4`). It's a config-time constant, already validated as a
  well-formed header value, and requires no runtime sanitization.
- **RegularExpression match**: there is no single literal to set, so NGF sets the header to the actual extracted
  value (e.g. `proxy_set_header X-Gateway-Model-Name $model;`). The `condition`'s regex comparison ensures only
  values matching the expected pattern reach the `proxy_set_header` directive. The value is forwarded as-is
  after passing the regex gate -- NGF does not apply additional sanitization beyond the match itself -- so users
  who need tighter control should write a sufficiently restrictive regex (e.g. anchored, character-class-limited).
  The forwarded value is request-derived content and should be treated with the same care as any other
  client-supplied header (see the sensitivity note below).

From the user's perspective, this is the same two-stage pattern GEP-5224 defines: a Listener-scoped
`RequestHeaderModifier` pre-routing filter extracts a body field into a header, then a normal `HTTPRoute` matches
on that header. NGF honors that API exactly as written and still sets `X-Gateway-Model-Name` on the request,
since the user explicitly configured it via the filter's `set`/`value: json(request.body.model)` field and
other consumers (the backend, offload services) may depend on seeing it. Preserving the literal "set header,
then match header" semantics keeps the experience portable across Gateway API implementations rather than
leaking an NGINX-specific shortcut into the API, even though NGINX could in principle match the extracted value
directly without ever setting the header.

Internally, NGF translates this configuration into `client_body_early_read` and `json_set` directives, and compiles
the `HTTPRoute`'s header match into a `condition` evaluated directly against the `json_set`-extracted variable
(e.g. `$model = gpt-4` for exact, or `$model ~ ^gpt-[45]$` for regex), rather than against the header NGINX
itself sets on the request. The observable result matches the literal "set header, then match on header"
semantics implied by the API -- the header is present with the correct value, and the request is routed as if
that header had been matched -- without NGF depending on the header for the match itself.

## Use Cases

- As an Application Developer, I want to route inference requests to the correct model backend based on a `model`
  field in the JSON request body, without modifying my application or writing NGINX configuration by hand.
- As an Application Developer building an MCP server, I want to route requests to different backends based on the
  JSON-RPC `method` field (e.g. `initialize`, `tools/list`, `tools/call`), so that discovery, control, and tool
  invocation traffic can be handled independently.
- As a Cluster Operator, I want sibling routes on the same listener as a body-matched route to be unaffected
  whenever their requests don't overlap in path and method with the body-matched route's conditions.

## Testing

- Unit tests
- Functional tests validating end-to-end routing decisions based on body content, including nested fields, multiple
  combined conditions, existing path/method/header/query matches, malformed/missing/oversized bodies Gateway/Route
  inheritance and conflicts.
- Functional compatibility tests for request mirroring, `ExternalAuth` body forwarding, the inference extension's
  endpoint-picker, and AI Guardrails, verifying that each still receives the complete, unchanged request body.


## Security Considerations

- `client_body_early_read` is gated by a `map`-derived variable that NGF compiles from the union of all
  body-based routes' path and method conditions on each server block (see
  [NGINX Configuration](#nginx-configuration)). Only requests whose method and URI match that `map` trigger the
  early read; all other requests on the same listener are never preread and their Route-scoped
  `ClientSettingsPolicy` `body` settings remain fully effective.

  For the requests that *do* trigger the early read, the body settings that govern the read --
  `client_max_body_size`, `client_body_buffer_size`, and `client_body_timeout` -- are ordinarily applied
  per-location, after routing has already selected one. `client_body_early_read`, however, reads the body at the
  server level, *before* any location (and therefore any location-scoped `ClientSettingsPolicy`) has been
  selected. This timing problem applies equally to all three settings, not just buffer size: whichever effective
  values are in force for the pre-routing read are the only ones that actually govern that request, since a
  route-scoped override of any of them cannot retroactively re-check a read that already happened at the server
  level under different values -- a smaller Route-scoped `client_max_body_size` can't reject an oversized body
  the server-level value already accepted, and a larger Route-scoped value can't rescue a body the server-level
  value already rejected.

  For this reason, `client_max_body_size`, `client_body_buffer_size`, and `client_body_timeout` for the
  pre-routing read are always taken from the listener's **Gateway-scoped** `ClientSettingsPolicy` `body` settings
  (falling back to NGINX's own platform defaults -- e.g. `1m`, `8k`/`16k`, `60s` -- for any that are unset), for
  for every server block whose Listener has a `RequestHeaderModifier` pre-routing filter with a body-sourced
  header value attached via `filters.requests`. This is a natural consequence of such a filter always being
  Listener-scoped (see [API, Customer Driven
  Interfaces, and User Experience](#api-customer-driven-interfaces-and-user-experience) above): since there is no
  Route-scoped variant of this filter to begin with, there is no competing Route-scoped source of these limits to
  reconcile against -- the Gateway/Listener scope is simply the only scope in effect before any route has been
  selected, and is also the only scope such a filter can ever be attached to. This puts the choice of
  these limits in the hands of the Cluster Operator, who already controls `body` settings for their own reasons
  and understands their own body sizes, rather than NGF guessing a one-size-fits-all constant.

  A `body` setting on an `HTTPRoute`-scoped `ClientSettingsPolicy` is therefore **incompatible with body-based
  routing on that route** and has no effect on the pre-routing read: by the time a location is selected, the body
  has already been fully read (or the request already rejected) using the Gateway-scoped values, so a
  Route-scoped `body.maxSize`, `body.bufferSize`, or `body.timeout` can never be enforced or relaxed for that
  request. NGF will report this with a status condition on the Route-scoped `ClientSettingsPolicy` (indicating it
  is overridden/ineffective) whenever it targets a route whose requests trigger the early read, rather than
  silently ignoring it. Routes on the same listener whose path/method conditions don't overlap with any
  body-based route are **not** affected by this constraint -- their requests never trigger the early read, so
  their Route-scoped `body` settings work normally.

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
  whichever location's condition would have depended on it simply doesn't match, the same as if the field were
  absent from a well-formed body.
- Within a server block whose Listener has a `RequestHeaderModifier` pre-routing filter with a body-sourced
  header value attached, a missing field, a malformed/incomplete JSON body, or a body that spilled to disk are
  all indistinguishable to
  `json_set`: every extracted variable is simply **not found**. Per native `location $variable { ... }` semantics,
  a condition that depends on a not-found variable is not empty and not "0", so it simply does not
  match -- there is no special error path or forced status code. Request handling falls through to whichever
  `location` would otherwise apply next (typically the deployment's catch-all/default backend, per the compiled
  example above), exactly as an ordinary unmatched regex or condition location falls through today.
- For exact header matches, the header set by `proxy_set_header` is the static literal from the `HTTPRoute`'s
  match value (after the config-safe validation described in [Header value: exact match vs. regular-expression
  match](#header-value-exact-match-vs-regular-expression-match)). For regular-expression header matches, the
  header is set to the `json_set`-extracted variable (e.g. `$model`), which is request-derived content
  forwarded as-is after passing the route's regex gate. Users who need tighter control should write a
  sufficiently restrictive regex. See the same subsection above for the full rationale.
- Because extracted values may be echoed into a request header that is forwarded to the backend, users should be
  aware that sensitive body content (tokens, PII, etc.) used for routing will also be visible to the backend and to
  anything that logs request headers.

## Alternatives

- **Extend NGF's own `PayloadProcessor` CRD with a new `InProcess` processor type**, modeled on the chicken-and-egg
  resolution GEP-5224 eventually settled on (Gateway/Listener-scoped attachment, since a pre-routing processor
  can't be scoped to the very `HTTPRoute` it's meant to help select) but implemented as an NGF-specific extension
  to an existing CRD rather than adopting the native upstream mechanism. This was rejected for the same reason as
  above: GEP-5224 already defines the pre-routing attachment point, ordering, and admission-validation semantics
  NGF would otherwise have to reinvent on `PayloadProcessor`, and adopting the native upstream mechanism keeps
  body-based routing portable and recognizable to users of other Gateway API implementations, rather than being
  expressed as an NGF-only policy shape.
- **Extend `HTTPRouteMatch` directly with a body-based condition.** This was considered but impossible for now,
  since `HTTPRouteMatch` is a closed Gateway API struct. If upstream Gateway API eventually adds a native body
  condition to `HTTPRouteMatch`, we should reevaluate using it directly as a simpler alternative to the
  pre-routing-filter-plus-header-match indirection described in this proposal. This is an initiative that we
  could drive forward.
- **Add body matching as an `HTTPRoute` filter**, attached inline to an `HTTPRouteRule`, which would both extract
  the body field and gate whether the rule matches. This was rejected because filters run against a rule that has
  already been selected -- they are not inputs to route selection itself -- so a filter cannot express "only
  match this rule if the body satisfies X" without reworking how filters participate in route matching. This is
  the same reasoning GEP-5224 itself used to require pre-routing filters to attach to a `Listener` rather than an
  `HTTPRoute`.
- **Introduce a new, separate route type (e.g. a `BodyMatchRoute` CRD) dedicated to body-based routing.** Rejected
  because it would fragment routing configuration across multiple route types for what is conceptually just
  another match condition alongside path, method, headers, and query parameters.

## References

- [GEP-5224: Pre-Routing Filters](https://gateway-api.sigs.k8s.io/geps/gep-5224/)
- [`client_body_early_read`](https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_early_read)
- [`json_set` (ngx_http_json_module)](https://nginx.org/en/docs/http/ngx_http_json_module.html)
