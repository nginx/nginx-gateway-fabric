# Enhancement Proposal-4051: Session Persistence for NGINX Plus and OSS

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4051
- Status: Implementable

## Summary

Enable NGINX Gateway Fabric to support session persistence for both NGINX Plus and NGINX OSS, allowing application developers to configure basic session persistence using the `ip_hash` load balancing method in OSS and cookie-based session persistence in NGINX Plus.

## Goals

- Extend the Upstream Settings Policy API to allow specifying `ip_hash` load balancing method to support basic session persistence.
- Design the translation of the Gateway API `sessionPersistence` specification, which can be configured on both HTTPRoute and GRPCRoute, into NGINX Plus cookie-based session persistence directives with `secure` and `httpOnly` mode enforced by default.

## Non-Goals

- Describe or implement low-level configuration details for enabling session persistence.
- Extend session persistence support to TLSRoutes or other Layer 4 route types.
- Supporting the `sameSite` cookie directive for NGINX Plus session persistence, which may be considered in the future as the Gateway API `sessionPersistence` specification evolves.

## Introduction

For NGINX OSS, session persistence is enabled by setting `loadBalancingMethod: ip_hash` on UpstreamSettingsPolicy, which adds the `ip_hash` directive to upstreams and provides IP-based affinity.
For NGINX Plus, session persistence defined on `HTTPRouteRule/GRPCRouteRule` is translated into sticky cookie upstream configuration with secure, httpOnly, host-only cookies and a path derived from HTTPRoute matches (or defaulted for GRPCRoutes), so sessions stick to a chosen backend.

### Understanding the NGINX directives

**ip_hash**

The [ip_hash](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#ip_hash) directive enables session persistence by routing requests from the same client IP address to the same upstream server. It uses the client’s IP address as a hash key to determine the target server, ensuring consistent routing for users behind a single IP. If the chosen server becomes unavailable, NGINX automatically selects the next available upstream server.

Syntax:

```bash
ip_hash;
```

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
httpOnly - Sets the `httpOnly` attribute for the cookie.

## API, Customer Driven Interfaces, and User Experience

### Session Persistence for NGINX OSS users

In OSS, session persistence is provided by configuring upstreams to use the `ip_hash` load-balancing method. NGINX hashes the client IP to select an upstream server, so requests from the same IP are routed to the same upstream as long as it is available. If that server becomes unavailable, NGINX automatically selects another server in the upstream group. Session affinity quality with `ip_hash` depends on NGINX seeing the real client IP. In environments with external load balancers or proxies, operators must ensure appropriate `real_ip_header/set_real_ip_from` configuration so that `$remote_addr` reflects the end-user address otherwise, stickiness will be determined by the address of the front-end proxy rather than the actual client.

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

### Session Persistence for NGINX Plus users

In NGINX Plus, session persistence is implemented with the `sticky` directive.The directive supports cookie, header, and learn modes; this design only discusses the cookie-based method and the rest are out of scope.
Users can configure [sessionPersistence](https://gateway-api.sigs.k8s.io/reference/spec/?h=sessionpersistence#sessionpersistence) on HTTPRouteRule or GRPCRouteRule, and NGINX Gateway Fabric will map that configuration to `sticky cookie` and associated cookie attributes as described below. The current specification for Session Persistence can be found [here](https://gateway-api.sigs.k8s.io/reference/spec/#sessionpersistence).

#### Mapping the Gateway API fields to NGINX directives

| Spec Field                             | NGINX Directive            | Notes / Limitations                                                                                                                 |
|----------------------------------------|----------------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| `sessionName`                          | `name`                     | Direct mapping to `sticky cookie` name.                                                                                             |
| `absoluteTimeout`                      | `expires`                  | Only used when `cookieConfig.lifetimeType=Permanent`; not enforced for `Session` cookies.                                           |
| `idleTimeout`                          | _not supported_            | NGINX does not support idle-based invalidation for sticky cookies. Sessions expire only when the cookie expires or the session ends.|
| `type`                                 | `cookie`                   | Only cookie-based persistence is supported. If Header is specified, the sessionPersistence spec is ignored and a warning/status message is reported on the route, but the route itself remains valid. |
| `cookieConfig.lifetimeType=Session`    | _no `expires` set_         | Session cookies expire when the browser session ends.                                                                               |
| `cookieConfig.lifetimeType=Permanent`  | `expires=<absoluteTimeout>`| Cookie persists until the specified timeout. `absoluteTimeout` is required when `lifetimeType` is `Permanent`.                      |
| no matching spec field                 | _no `domain` attribute_    | Cookies are host-only for both `HTTPRoute` and `GRPCRoute`.                                                                         |
| no matching spec field                 | `path`                     | Behavior is described separately for `HTTPRoute` below.                                                                             |
| no matching spec field                 | `secure`                   | Enabled by default for all routes.                                                                                                  |
| no matching spec field                 | `httpOnly`                 | Enabled by default for all routes.                                                                                                  |

Note: The default `secure` and `httpOnly` settings may change in the future if we need to align with the Gateway API specification.

#### Domain and Path selection for Routes

Cookies use the [domain](https://datatracker.ietf.org/doc/html/rfc6265?#section-5.1.3) and [path](https://datatracker.ietf.org/doc/html/rfc6265?#section-5.1.4) attributes to control when the browser sends them back to the server. Domain limits the cookie to a host (and its subdomains, if set), while path limits it to URLs under a specific path prefix. Together they control where the browser sends the cookie, and therefore where session persistence actually applies.

For **HTTPRoutes**, we do not set the `domain` attribute. Deriving a broader domain (for example, a common suffix across hostnames or a parent domain) would widen the cookie scope to sibling subdomains and increase the risk of cross-host leakage. Since users cannot explicitly configure this field, inferring a shared domain would also be vulnerable to abuse. Leaving domain unset ensures each cookie is scoped to the exact host that issued it.

To determine the cookie `path` for HTTPRoutes, we handle the simple case where there is a single path match as follows:

| Path Value                          | Path Match Type | Cookie `Path` Value | Cookie Match Expectations                                                                                                                         |
|-------------------------------------|-----------------|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| `/hello-exact`                      | Exact           | `/hello-exact`      | Cookie header is sent for `/hello-exact` path only.                                                                                                      |
| `/hello-prefix`                     | Prefix          | `/hello-prefix`     | Cookie header is sent for `/hello-prefix` and any subpath starting with `/hello-prefix` (e.g. `/hello-prefix/foo`).                                      |
| `/hello-regex/[a-zA-Z0-9_-]+$`      | Regex           | `/hello-regex`      | Cookie header is sent for any request whose path starts with `/hello-regex` and matches the regex in the location block (e.g. `/hello-regex/a`, `/hello-regex/abc123`). The regex still determines which requests match the route on the server side. |

When there are multiple path matches that share the same sessionPersistence configuration, we derive a single cookie path by computing the longest common prefix that ends on a path-segment boundary `/`. If no non-empty common prefix on a segment boundary exists, we fall back to `/` which is allowing all paths.

For **GRPCRoutes**, we do not set explicit cookie `domain` or `path` attributes. Leaving `domain` unset keeps cookies host-only, and omitting `path` means the user agent applies its default path derivation. This avoids guessing a cookie scope from gRPC routing metadata. gRPC routing is driven by a combination of listener hostnames, methods, and header matches, none of which map cleanly onto a single stable cookie scope: methods are too granular, hostnames may be broad or wildcarded, and header-based matches are inherently dynamic. Any attempt to derive a `domain` or `path` from this information would likely be ambiguous or over-scoped.

These decisions let HTTPRoute traffic benefit from path-scoped cookies while keeping cookie domain to host-only for both HTTPRoutes and GRPCRoutes to avoid cross-host leakage.
For GRPCRoutes, we only provide basic sessionPersistence because typical gRPC clients do not implement browser-style cookie storage and replay. Cookies are treated as ordinary headers, so applications must handle them explicitly rather than relying on an automatic client-side cookie store.

## Use Cases

This enhancement targets apps that need straightforward session persistence, such as keeping a user on the same backend across multiple requests or supporting stateful services that keep session data in memory. Session persistence keeps a client pinned to one upstream while it’s healthy instead of re-randomizing on each request.

## Testing

There are no existing conformance tests for session persistence, so we will add functional tests to verify end-to-end behavior for both OSS and Plus. For OSS, tests will confirm that `ip_hash` keeps a client pinned to a single upstream while it is healthy. For Plus, tests will verify that `sessionPersistence` produces the expected `sticky cookie` configuration for both HTTPRoute and GRPCRoute and that requests with a valid session cookie are routed consistently to the same upstream.

## Security Considerations

The main security concern is how far session cookies reach. This design keeps cookies host-only by never setting the domain attribute, and for HTTPRoutes it scopes cookies by route path (or `/` when no safe common prefix exists). That limits both cross-host and cross-path leakage and reduces the impact of a compromised cookie.

We also set `secure` and `httponly` on the session cookie by default as a hardening measure. If Gateway API later adds these as fields with different defaults, we’ll treat any change in behavior as an experimental-API breaking change. This will be documented in the Session Persistence guide, and upgrade guidance will be provided so users can explicitly configure these cookie attributes.

### Edge Cases

- If both Kubernetes Service-level session affinity and Gateway API sessionPersistence are configured for the same traffic, the route MUST be rejected, with a status condition explaining that the two mechanisms are incompatible.
- For traffic-splitting configurations, if cookie-based session persistence is enabled, sessions must remain pinned consistently across the split backends.

### Future work

- Define clear precedence and additional restrictions when SessionPersistence is configured via a separate policy.
- Add support for the `sameSite` cookie attribute in a way that remains compliant with the Gateway API specification.

## Useful Links

- Session Persistence [specification](https://gateway-api.sigs.k8s.io/reference/spec/#sessionpersistence).
- Extended Session Persistence [GEP](https://gateway-api.sigs.k8s.io/geps/gep-1619).
- RFC standard for [Set-Cookie](https://datatracker.ietf.org/doc/html/rfc6265#section-4.1) header.
- [Security risks with subdomain](https://blog.stackademic.com/session-security-risks-with-subdomains-2802c56d681f).
- [Cookie Security](https://www.appsecmonkey.com/blog/cookie-security), read the section `Malicious Subdomains`.
- [gRPC Metadata](https://grpc.io/docs/guides/metadata/)
