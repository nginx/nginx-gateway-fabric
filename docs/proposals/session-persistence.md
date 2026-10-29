# Enhancement Proposal-4051: Cookie-Based Session Persistence for NGINX Gateway Fabric

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4051
- Status: Completed

## Summary

Enable NGINX Gateway Fabric to support cookie-based session persistence through the Gateway API `sessionPersistence` field, with behavior that is usable by application developers across NGINX Gateway Fabric deployments. NGINX OSS also continues to support basic IP-based affinity through the `ip_hash` load-balancing method, but that mechanism is secondary to the cookie-based approach described in this document.

This API surface is experimental and temporary. The current `sessionPersistence` behavior follows an evolving Gateway API design and is expected to change as the upstream specification matures.

## Goals

- Design the translation of the Gateway API `sessionPersistence` specification, which can be configured on both HTTPRoute and GRPCRoute, into cookie-based session persistence directives in NGINX Gateway Fabric.
- Document the current temporary behavior of the experimental `sessionPersistence` API so users understand both what is supported today and that the API may change.

## Non-Goals

- Describe or implement low-level configuration details for enabling session persistence.
- Extend session persistence support to TLSRoutes or other Layer 4 route types.
- Supporting the `secure`, `httponly`, `sameSite` cookie directive for NGINX Plus session persistence, which may be considered in the future as the Gateway API `sessionPersistence` specification evolves.

## Introduction

NGINX Gateway Fabric primarily exposes session persistence through the Gateway API `sessionPersistence` field on `HTTPRouteRule` and `GRPCRouteRule`. That configuration is translated into `sticky cookie` upstream configuration, with host-only cookies and a path that defaults to `/` (or an explicit value when configured via `cookie.path`), so requests can remain pinned to a chosen backend across multiple exchanges.

This document focuses on that cookie-based approach because it is the user-facing direction of the Gateway API feature and the model most application developers should evaluate first.

As a secondary option, NGINX OSS also supports IP-based affinity by setting `loadBalancingMethod: ip_hash` on UpstreamSettingsPolicy. That is operationally useful in some environments, but it is not the main API direction for session persistence and it has different behavior and tradeoffs from cookie-based persistence.

> Note: The `sessionPersistence` API discussed here is experimental and temporary. Users should expect field shape, semantics, and supported behaviors to evolve as the upstream Gateway API work progresses.

### Understanding the NGINX directives

**sticky (cookie method)**

The [sticky](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#sticky) directive enables session persistence using a cookie to identify the upstream server handling a client’s session. When configured with the cookie parameter, NGINX sends a cookie token in the client response in a `Set-Cookie` header, allowing the browser to route subsequent requests with that cookie to the same upstream server.

Syntax:

```bash
sticky cookie name [expires=time] [domain=domain] [httponly] [samesite=strict|lax|none|$variable] [secure] [path=path];
```

Key Parameters:
cookie <name> – Defines the session cookie name.
expires=<time> – Sets cookie lifetime; omit to make it session-based. `max` – Special value for expires that sets expiry to `31 Dec 2037 23:55:55 GMT`.
domain=<domain> - Sets the domain for the cookie scope.
path=<path> - Sets the path for the cookie scope.
samesite=[strict|lax|none|$variable] - Sets the sameSite attribute for the cookie.
secure - Sets the `secure` attribute for the cookie.
httponly - Sets the `httponly` attribute for the cookie.

**ip_hash**

The [ip_hash](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#ip_hash) directive enables session persistence by routing requests from the same client IP address to the same upstream server. It uses the client IP as a hash key and can provide basic affinity when cookies are not the chosen mechanism. If the chosen server becomes unavailable, NGINX automatically selects the next available upstream server.

Syntax:

```bash
ip_hash;
```

## API, Customer Driven Interfaces, and User Experience

### Cookie-Based Session Persistence

The primary user-facing model is the Gateway API `sessionPersistence` field on `HTTPRouteRule` and `GRPCRouteRule`. NGINX Gateway Fabric maps that configuration to `sticky cookie` behavior.

This behavior is currently experimental. It is available so users can evaluate and adopt cookie-based stickiness now, but they should plan for API changes as the upstream Gateway API specification evolves.

Users can configure [sessionPersistence](https://gateway-api.sigs.k8s.io/reference/api-spec/?h=sessionpersistence#sessionpersistence) on HTTPRouteRule or GRPCRouteRule, and NGINX Gateway Fabric will map that configuration to `sticky cookie` and associated cookie attributes as described below. The current specification for Session Persistence can be found [here](https://gateway-api.sigs.k8s.io/reference/api-spec/#sessionpersistence).

#### Mapping the Gateway API fields to NGINX directives

| Spec Field | NGINX Directive | Notes / Limitations |
| ---------- | --------------- | ------------------- |
| `cookie.name` | `name` | Direct mapping to `sticky cookie` name. |
| `absoluteTimeout` | `expires` | Only used when `cookie.lifetimeType=Permanent`; not enforced for `Session` cookies. |
| `type` | `cookie` | Only cookie-based persistence is supported. If Header is specified, the sessionPersistence spec is ignored and a warning/status message is reported on the route, but the route itself remains valid. |
| `cookie.lifetimeType=Session` | _no `expires` set_ | Session cookies expire when the browser session ends. |
| `cookie.lifetimeType=Permanent` | `expires=<absoluteTimeout>` | Cookie persists until the specified timeout. `absoluteTimeout` is required when `lifetimeType` is `Permanent`. |
| no matching spec field | _no `domain` attribute_ | Cookies are host-only for both `HTTPRoute` and `GRPCRoute`. |
| `cookie.path` | `path` | Defaults to `/` when not set. See [Domain and Path selection for Routes](#domain-and-path-selection-for-routes) below. |

#### Domain and Path selection for Routes

Cookies use the [domain](https://datatracker.ietf.org/doc/html/rfc6265?#section-5.1.3) and [path](https://datatracker.ietf.org/doc/html/rfc6265?#section-5.1.4) attributes to control when the browser sends them back to the server. Domain limits the cookie to a host (and its subdomains, if set), while path limits it to URLs under a specific path prefix. Together they control where the browser sends the cookie, and therefore where session persistence actually applies.

For both **HTTPRoutes** and **GRPCRoutes**, we do not set the `domain` attribute. Deriving a broader domain (for example, a common suffix across hostnames or a parent domain) would widen the cookie scope to sibling subdomains and increase the risk of cross-host leakage. Since users cannot explicitly configure this field, inferring a shared domain would also be vulnerable to abuse. Leaving domain unset ensures each cookie is scoped to the exact host that issued it.

Per the Gateway API [`CookieConfig.Path`](https://gateway-api.sigs.k8s.io/geps/gep-1619) field, implementations MUST default the cookie `path` to `/` when it is not explicitly specified. NGINX Gateway Fabric follows this behavior for both HTTPRoutes and GRPCRoutes: unless the user sets `sessionPersistence.cookie.path` explicitly, the generated `sticky cookie` directive uses `path=/`, so the cookie is sent for all paths under the route's host. Users who want a narrower cookie scope can set `cookie.path` explicitly to any value, independent of the route's path matches.

We previously derived the cookie `path` for HTTPRoutes from the rule's path matches (e.g. using the longest common prefix of `Exact`/`Prefix` matches, and no `path` attribute for `RegularExpression` matches), and did not set a `path` attribute at all for GRPCRoutes. That derivation added complexity and produced cookie scopes that did not always match user expectations, especially when multiple path matches with different prefixes shared the same sessionPersistence configuration. The explicit `cookie.path` field, now part of the Gateway API spec, replaces the need for this derivation: implementations default to `/`, and users who need a narrower scope set it themselves.

For GRPCRoutes, we only provide basic sessionPersistence because typical gRPC clients do not implement browser-style cookie storage and replay. Cookies are treated as ordinary headers, so applications must handle them explicitly rather than relying on an automatic client-side cookie store.

### OSS note: `ip_hash` as a secondary affinity option

In OSS, a secondary way to achieve affinity is configuring upstreams to use the `ip_hash` load-balancing method. NGINX hashes the client IP to select an upstream server, so requests from the same IP are routed to the same upstream as long as it is available. If that server becomes unavailable, NGINX automatically selects another server in the upstream group.

This method is more operationally sensitive than cookie-based persistence. Its quality depends on NGINX seeing the real client IP. In environments with external load balancers or proxies, operators must ensure appropriate `real_ip_header/set_real_ip_from` configuration so that `$remote_addr` reflects the end-user address. Otherwise, stickiness will be determined by the address of the front-end proxy rather than the actual client.

To surface this behavior, UpstreamSettingsPolicy is extended with a load-balancing method field:

```go
// UpstreamSettingsPolicySpec defines the desired state of the UpstreamSettingsPolicy.
type UpstreamSettingsPolicySpec struct {
	// ZoneSize is the size of the shared memory zone used by the upstream. This memory zone is used to share
	// the upstream configuration between nginx worker processes. The more servers that an upstream has,
	// the larger memory zone is required.
	// Default: OSS: 512k, Plus: 1m.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#zone
	//
	// +optional
	ZoneSize *Size `json:"zoneSize,omitempty"`

	// KeepAlive defines the keep-alive settings.
	//
	// +optional
	KeepAlive *UpstreamKeepAlive `json:"keepAlive,omitempty"`

	// LoadBalancingMethod specifies the load balancing algorithm to be used for the upstream.
	//
	// +optional
	// +kubebuilder:default:=random two least_conn
	LoadBalancingMethod *LoadBalancingType `json:"loadBalancingMethod,omitempty"`

	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Support: Service
	//
	// TargetRefs must be _distinct_. The `name` field must be unique for all targetRef entries in the UpstreamSettingsPolicy.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRefs Kind must be: Service",rule="self.all(t, t.kind=='Service')"
	// +kubebuilder:validation:XValidation:message="TargetRefs Group must be core",rule="self.exists(t, t.group=='') || self.exists(t, t.group=='core')"
	// +kubebuilder:validation:XValidation:message="TargetRef Name must be unique",rule="self.all(p1, self.exists_one(p2, p1.name == p2.name))"
	//nolint:lll
	TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`
}

// LoadBalancingType defines supported load balancing methods.
//
// +kubebuilder:validation:Enum=ip_hash;random two least_conn
type LoadBalancingType string

const (
	// LoadBalancingTypeIPHash enables IP hash-based load balancing,
	// ensuring requests from the same client IP are routed to the same upstream server.
    // NGINX directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#ip_hash
	LoadBalancingTypeIPHash LoadBalancingType = "ip_hash"

	// LoadBalancingTypeRandomTwoLeastConnection enables a variation of least-connections
	// balancing that randomly selects two servers and forwards traffic to the one with
	// fewer active connections.
    // NGINX directive least_conn: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#least_conn
    // NGINX directive random: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#random
	LoadBalancingTypeRandomTwoLeastConnection LoadBalancingType = "random two least_conn"
)
```

Note: `LoadBalancingMethod` is optional and defaults to `random two least_conn` in NGINX Gateway Fabric, even though NGINX itself defaults to `round_robin` load balancing. Adding this optional field is a non-breaking change and does not require a version bump in alignment with the [Kubernetes API compatibility guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md#on-compatibility).

## Use Cases

This enhancement targets apps that need straightforward cookie-based session persistence, such as keeping a user on the same backend across multiple requests or supporting stateful services that keep session data in memory. Session persistence keeps a client pinned to one upstream while it’s healthy instead of re-randomizing on each request.

## Testing

There are no existing conformance tests for session persistence, so we will add functional tests to verify end-to-end behavior for cookie-based session persistence and the OSS `ip_hash` fallback. Tests will verify that `sessionPersistence` produces the expected `sticky cookie` configuration for both HTTPRoute and GRPCRoute and that requests with a valid session cookie are routed consistently to the same upstream. Separate OSS coverage can continue to validate `ip_hash` as a secondary affinity method.

## Security Considerations

The main security concern is how far session cookies reach. This design keeps cookies host-only by never setting the domain attribute. The cookie path defaults to `/`, scoping the cookie to the entire host unless the user explicitly narrows it via `cookie.path`. Users who need tighter scoping to limit cross-path leakage should set `cookie.path` explicitly to the narrowest path that meets their needs.

### Edge Cases

- If an implementation routes through Service IPs, any Gateway-level session persistence must be rejected when Service-level session affinity is enabled. In our case, the data plane routes directly to pod IPs, so Service affinity does not interfere with session persistence between the gateway and backends.
- For traffic-splitting configurations, if cookie-based session persistence is enabled, sessions must remain pinned consistently across the split backends.
- When multiple HTTPRoute rules reference the same backend but define different (or no) sessionPersistence settings, session persistence is treated as rule-scoped rather than backend-scoped. For each unique combination of backendRef and session persistence configuration, the controller generates a dedicated NGINX upstream and wires that rule’s internal location to it.

### Future work

- Align the implementation with future upstream changes to the experimental `sessionPersistence` API as the Gateway API design evolves.
- Define clear precedence and additional restrictions when SessionPersistence is configured via a separate policy.
- Add support for the `sameSite`, `secure`, `httponly` cookie attribute in a way that remains compliant with the Gateway API specification.

## Useful Links

- Session Persistence [specification](https://gateway-api.sigs.k8s.io/reference/api-spec/#sessionpersistence).
- Extended Session Persistence [GEP](https://gateway-api.sigs.k8s.io/geps/gep-1619).
- RFC standard for [Set-Cookie](https://datatracker.ietf.org/doc/html/rfc6265#section-4.1) header.
- [Security risks with subdomain](https://blog.stackademic.com/session-security-risks-with-subdomains-2802c56d681f).
- [Cookie Security](https://www.appsecmonkey.com/blog/cookie-security), read the section `Malicious Subdomains`.
- [gRPC Metadata](https://grpc.io/docs/guides/metadata/).
