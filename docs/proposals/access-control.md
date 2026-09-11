# Enhancement Proposal-5848: Access Control Policy

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5848
- Status: Implementable

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
- Attachment to TLSRoute, TCPRoute, or UDPRoute. IP-based access control for L4 (stream) routes would use the same
  NGINX directives but would warrant a separate, purpose-built CRD to avoid exposing L7-only fields
  (authorization rules, HTTP methods, paths, headers) in a context where they do not apply.
- Geographic (GeoIP) based access control. May be added as a future extension to the same API.

## Introduction

### IP-Based Access Control

IP-based access control allows administrators to restrict which clients can access services based on the client's IP
address. This is a fundamental security mechanism used for:

- Restricting administrative endpoints to trusted corporate networks.
- Blocking known malicious IP ranges.
- Enforcing compliance by limiting access to specific geographic regions (by IP range).
- Creating a default-deny posture where only explicitly allowlisted clients can connect.

NGINX provides IP-based access control through the
[`ngx_http_access_module`](https://nginx.org/en/docs/http/ngx_http_access_module.html), which supports `allow` and
`deny` directives in the `http`, `server`, `location`, and `limit_except` contexts:

```nginx
location /admin {
    allow 10.0.0.0/8;
    allow 192.168.1.0/24;
    deny  all;
}
```

Rules are evaluated in order until the first match is found. This sequential evaluation model is well-suited for
expressing allowlist (default-deny) and denylist (default-allow) patterns.

### Upstream Alignment with XAccessPolicy

The [kube-agentic-networking](https://github.com/kubernetes-sigs/kube-agentic-networking) community project defines
an `XAccessPolicy` CRD for Gateway API-native access control. The upstream API uses a rule-based model with named
rules, typed sources, and an optional authorization block. Its core design supports Gateway as a target, with an
extension model that allows implementations to support additional target kinds and source types.

This proposal aligns with the XAccessPolicy structure by:

- Using the same top-level `action` field with `Allow` and `Deny` semantics (extending the upstream enum, which
  defines `Allow` and `ExternalAuth`, with `Deny`).
- Using named `AccessRule` objects with an optional, typed `source` field (omitting `source` matches any source).
- Adding an `IP` source type alongside the upstream `ServiceAccount` and `SPIFFE` types. The upstream spec's
  discriminated union on `source.type` explicitly supports implementation-defined extensions.
- Supporting `targetRefs` with Route kinds (HTTPRoute, GRPCRoute) in addition to the upstream's core Gateway support,
  as permitted by the spec's extension model.

This structural alignment means that when the upstream XAccessPolicy matures, users can migrate by changing the
`apiVersion` and `kind` without restructuring their rule definitions.

### Client IP Detection

For access control to work correctly, NGINX must know the true client IP address. When NGINX Gateway Fabric is
deployed behind a load balancer or proxy, the `$remote_addr` variable contains the address of the last hop, not the
original client.

The existing [`NginxProxy`](rewrite-client-ip.md) `RewriteClientIP` configuration handles this concern. When
`RewriteClientIP` is configured (via X-Forwarded-For or PROXY protocol), NGINX's `ngx_http_realip_module` rewrites
`$remote_addr` to the original client IP before the access module evaluates `allow`/`deny` rules. The `AccessPolicy`
relies on this existing mechanism and does not introduce its own client IP detection configuration.

Users deploying `AccessPolicy` behind a proxy **must** configure `RewriteClientIP` on their `NginxProxy` resource
to ensure accurate IP-based decisions. Without it, rules will match against the proxy's IP address rather than the
original client.

## Use Cases

- As a Cluster Operator, I want to set a default IP denylist on a Gateway so that known malicious IP ranges are
  blocked for all applications without requiring each Application Developer to configure their own access rules.
- As a Cluster Operator, I want to restrict all traffic through a Gateway to a set of trusted corporate CIDR ranges
  (default-deny posture) so that only authorized networks can reach internal services.
- As an Application Developer, I want to restrict access to my application's administrative endpoints to a specific
  set of IP addresses so that only authorized users can access sensitive functionality.
- As an Application Developer, I want to override the Cluster Operator's Gateway-level access rules for my specific
  Route because my application has different access requirements than the default.
- As a Cluster Operator, I want to apply an IP denylist at the Gateway and have Application Developers apply
  additional allowlist rules at the Route level, with the denylist always taking precedence regardless of the
  Route-level policy.

## API

The `AccessPolicy` API is a CRD that is part of the `gateway.nginx.org` Group. It adheres to the guidelines and
requirements of an Inherited Policy as defined in the
[Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/).

The policy uses `targetRefs` (plural) to support targeting multiple resources with a single policy instance. This
follows the current GEP-713 guidance and provides better user experience by:

- Avoiding policy duplication when applying the same settings to multiple targets.
- Reducing maintenance burden and risk of configuration inconsistencies.
- Preventing future migration challenges from singular to plural forms.

### Go

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// AccessPolicy is an Inherited Attached Policy. It provides a way to configure IP-based access control
// (allowlists and denylists) for traffic flowing through NGINX Gateway Fabric.
type AccessPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the AccessPolicy.
	Spec AccessPolicySpec `json:"spec"`

	// Status defines the state of the AccessPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccessPolicyList contains a list of AccessPolicies.
type AccessPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessPolicy `json:"items"`
}

// AccessPolicySpec defines the desired state of the AccessPolicy.
type AccessPolicySpec struct {
	// Action specifies the action to take when rules match.
	//
	// - Allow: Traffic is allowed only if it matches at least one rule. All non-matching traffic is denied.
	// - Deny: Traffic is denied if it matches any rule. All non-matching traffic is allowed.
	//
	// When multiple AccessPolicies apply to the same target (through inheritance or direct attachment),
	// Deny policies are evaluated first. If any Deny policy matches, the request is rejected.
	// Allow policies are then evaluated; the request must match at least one Allow rule from every
	// attached Allow policy.
	Action AccessPolicyActionType `json:"action"`

	// Rules defines the access control rules.
	// For Allow policies, a request is allowed if it matches any rule (OR semantics).
	// For Deny policies, a request is denied if it matches any rule (OR semantics).
	// If omitted, the policy applies to all traffic.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=64
	// +listType=atomic
	Rules []AccessRule `json:"rules,omitempty"`

	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	//
	// Support: Gateway, HTTPRoute, GRPCRoute
	//
	// Note: A single policy cannot target both Gateway and Route kinds simultaneously.
	// Use separate policies: one targeting Gateway (for inherited settings) and others
	// targeting specific Routes (for overrides).
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind == 'Gateway' || t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group == 'gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind and Name combination must be unique",rule="self.all(t1, self.exists_one(t2, t1.group == t2.group && t1.kind == t2.kind && t1.name == t2.name))"
	// +kubebuilder:validation:XValidation:message="Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs",rule="!(self.exists(t, t.kind == 'Gateway') && self.exists(t, t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute'))"
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`
}

// AccessPolicyActionType identifies the action to take when rules match.
//
// +kubebuilder:validation:Enum=Allow;Deny
type AccessPolicyActionType string

const (
	// AccessPolicyActionAllow allows traffic that matches any rule.
	AccessPolicyActionAllow AccessPolicyActionType = "Allow"

	// AccessPolicyActionDeny denies traffic that matches any rule.
	AccessPolicyActionDeny AccessPolicyActionType = "Deny"
)

// AccessRule defines an access control rule.
type AccessRule struct {
	// Name specifies a unique name for this rule.
	// This follows the DNS Subdomain naming convention.
	//
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Source specifies the source of the request to match against.
	// If omitted, the rule matches requests from any source.
	//
	// +optional
	Source *AccessRuleSource `json:"source,omitempty"`
}

// AccessRuleSource specifies the source of a request.
//
// +kubebuilder:validation:XValidation:message="ipAddress must be set when type is IPAddress",rule="(self.type == 'IPAddress') == has(self.ipAddress)"
//
//nolint:lll
type AccessRuleSource struct {
	// Type identifies the source type.
	//
	// +unionDiscriminator
	Type AccessRuleSourceType `json:"type"`

	// IPAddress specifies an IP address or CIDR range.
	// Required when type is IPAddress; must not be set otherwise.
	//
	// +optional
	IPAddress *AccessRuleSourceIPAddress `json:"ipAddress,omitempty"`
}

// AccessRuleSourceType identifies a type of source for access control.
//
// +kubebuilder:validation:Enum=IPAddress
type AccessRuleSourceType string

const (
	// AccessRuleSourceTypeIP matches requests by client IP address.
	AccessRuleSourceTypeIP AccessRuleSourceType = "IPAddress"
)

// AccessRuleSourceIPAddress specifies an IP address or CIDR range to match against.
type AccessRuleSourceIPAddress struct {
	// Address is an IP address or CIDR range.
	// Examples: "192.168.1.1", "10.0.0.0/8", "2001:db8::/32".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=49
	Address string `json:"address"`
}
```

### Versioning and Installation

The version of the `AccessPolicy` API will be `v1alpha1`.

The `AccessPolicy` CRD will be installed by the Cluster Operator via Helm or with manifests. It will be required,
and if the `AccessPolicy` CRD does not exist in the cluster, NGINX Gateway Fabric will log errors until it is
installed.

### Status

#### CRD Label

According to the [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/), the
`AccessPolicy` CRD must have the `gateway.networking.k8s.io/policy: inherited` label to specify that it is an
inherited policy. This label will help with discoverability and will be used by Gateway API tooling.

#### Conditions

According to the [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/), the
`AccessPolicy` CRD must include a `status` stanza with a slice of Conditions.

The following Conditions must be populated on the `AccessPolicy` CRD:

- `Accepted`: Indicates whether the policy has been accepted by the controller. This condition uses the reasons
  defined in the [PolicyCondition API](https://github.com/kubernetes-sigs/gateway-api/blob/main/apis/v1alpha2/policy_types.go).
- `Programmed`: Indicates whether the policy configuration has been propagated to the data plane.

When multiple AccessPolicies of the same action type target the same resource, all are accepted and their rules are
merged. This differs from policies like `RateLimitPolicy` where only one wins; for access control, it is valid and
expected to compose multiple deny or allow lists from different policy objects.

#### Setting Status on Objects Affected by a Policy

NGINX Gateway Fabric must set a Condition on all objects affected by an `AccessPolicy` to provide discoverability
for object owners. This involves defining a Condition type and reason:

```go
package conditions

import (
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	AccessPolicyAffected v1.PolicyConditionType = "gateway.nginx.org/AccessPolicyAffected"
	PolicyAffectedReason v1.PolicyConditionReason = "PolicyAffected"
)
```

NGINX Gateway Fabric must set this Condition on all HTTPRoutes, GRPCRoutes, and Gateways affected by an
`AccessPolicy`. Below is an example of what this Condition may look like:

```yaml
Conditions:
  Type:                  gateway.nginx.org/AccessPolicyAffected
  Message:               Object affected by an AccessPolicy.
  Observed Generation:   1
  Reason:                PolicyAffected
  Status:                True
```

Some additional rules:

- This Condition should be added when the affected object starts being affected by an `AccessPolicy`.
- If an object is affected by multiple `AccessPolicy` instances, only one Condition should exist.
- When the last `AccessPolicy` affecting that object is removed, the Condition should be removed.
- The Observed Generation is the generation of the affected object, not the generation of the `AccessPolicy`.

### YAML

Below is an example of an `AccessPolicy` that creates an allowlist (default-deny) at the Gateway level:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AccessPolicy
metadata:
  name: corporate-allowlist
  namespace: default
spec:
  action: Allow
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: example-gateway
  rules:
  - name: corporate-network
    source:
      type: IPAddress
      ipAddress:
        address: "10.0.0.0/8"
  - name: vpn-range
    source:
      type: IPAddress
      ipAddress:
        address: "172.16.0.0/12"
status:
  ancestors:
  - ancestorRef:
      group: gateway.networking.k8s.io
      kind: Gateway
      name: example-gateway
      namespace: default
    conditions:
    - type: Accepted
      status: "True"
      reason: Accepted
      message: Policy is accepted
    - type: Programmed
      status: "True"
      reason: Programmed
      message: Policy is programmed
```

Below is an example of an `AccessPolicy` that creates a denylist (default-allow) at a Route level:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: AccessPolicy
metadata:
  name: block-bad-actors
  namespace: default
spec:
  action: Deny
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: my-app
  rules:
  - name: known-scanner
    source:
      type: IPAddress
      ipAddress:
        address: "203.0.113.50"
  - name: abuse-range
    source:
      type: IPAddress
      ipAddress:
        address: "198.51.100.0/24"
```

## Attachment and Inheritance

The `AccessPolicy` may be attached to Gateways, HTTPRoutes, and GRPCRoutes.

**Important Constraint**: A single `AccessPolicy` instance cannot target both Gateway and Route kinds simultaneously.
This prevents configuration conflicts and ensures clear policy boundaries. To configure both Gateway-level defaults
and Route-level overrides, use separate policy instances.

### How Inheritance Works

Unlike most other inherited policies in NGINX Gateway Fabric (e.g., `ClientSettingsPolicy`, `ProxySettingsPolicy`),
access control policies do not follow a simple field-by-field merge where the "lowest" (most specific) value wins.
Instead, access control uses an **additive evaluation model** where policies at different levels are composed
together:

1. **Deny policies are evaluated first.** All Deny policies that apply to a request (from both Gateway and Route
   levels) are checked. If the request matches any Deny rule from any attached Deny policy, the request is
   rejected immediately.

2. **Allow policies are evaluated next.** If no Deny policy matched, all Allow policies that apply to a request are
   checked. The request must satisfy at least one rule from every attached Allow policy. If there are no Allow
   policies, traffic is allowed by default.

This model means that:

- A Deny policy at the Gateway level **cannot** be overridden by a Route-level Allow policy. If the Gateway says
  "deny 198.51.100.0/24", that range is blocked for all Routes regardless of their own Allow policies.
- A Route-level Allow policy **narrows** access further, it does not widen it. If the Gateway allows `10.0.0.0/8`
  and the Route allows only `10.1.0.0/16`, only `10.1.0.0/16` can reach that Route.

### Attachment Scenarios

**1. Gateway Attachment Only**

When an `AccessPolicy` is attached to a Gateway, the access rules apply to all HTTPRoutes and GRPCRoutes attached
to that Gateway.

**2. Route Attachment Only**

When an `AccessPolicy` is attached to an HTTPRoute or GRPCRoute, the rules apply to that Route only. Other Routes
on the same Gateway are unaffected and use NGINX's default open access.

**3. Gateway and Route Attachment (Separate Policies)**

When separate `AccessPolicy` instances are attached to a Gateway and one or more of its Routes:

- **Deny + Allow at Gateway**: A Gateway has both a Deny policy and an Allow policy attached. The Deny rules are
  checked first; any matching traffic is rejected. Remaining traffic must then match the Allow rules. Routes
  without their own policies inherit this combined behavior.
- **Gateway Allow + Route Allow**: The Gateway allows `10.0.0.0/8`. A Route has an additional Allow policy for
  `10.1.0.0/16`. Because all Allow policies must pass, the effective access for that Route is narrowed to
  `10.1.0.0/16`.
- **Gateway Deny + Route Allow**: The Gateway denies `198.51.100.0/24`. A Route allows `198.51.100.0/24`.
  The Deny takes precedence — the range remains blocked.

### NGINX Inheritance Behavior

The `allow` and `deny` NGINX directives have a specific inheritance behavior that differs from most other NGINX
directives. Per the
[NGINX documentation](https://nginx.org/en/docs/http/ngx_http_access_module.html):

> These directives are inherited from the previous configuration level if and only if there are no `allow` and
> `deny` directives defined on the current level.

This means that if a `location` block defines **any** `allow` or `deny` directive, it **completely replaces** the
rules from the enclosing `server` block rather than merging with them. This is a full replacement, not an additive
merge.

### Creating the Effective Policy in NGINX Config

To implement the additive evaluation model described above while respecting NGINX's replacement inheritance, the
controller must compute the **complete effective ruleset** at each NGINX context level rather than relying on NGINX's
built-in inheritance.

The strategy is:

- **When an `AccessPolicy` is attached to a Gateway only**, add the corresponding `allow`/`deny` directives to the
  `server` blocks generated from that Gateway. NGINX's inheritance will propagate these rules to all `location`
  blocks that do not define their own access rules.

- **When an `AccessPolicy` is attached to a Route**, the controller must emit the **full combined ruleset** (Gateway
  policies + Route policies) in each of the Route's `location` blocks. Because NGINX's access directives use
  replacement inheritance, if we only emitted the Route-level rules in the `location`, the Gateway-level rules
  would be lost.

- **Deny rules are always emitted before Allow rules**, followed by a final `deny all` (for Allow-action policies)
  or `allow all` (for Deny-action policies). The ordering ensures NGINX's sequential evaluation produces the
  correct result.

For example, given a Gateway-level Deny policy for `198.51.100.0/24` and a Route-level Allow policy for
`10.0.0.0/8`, the generated `location` block would contain:

```nginx
location /my-app {
    deny  198.51.100.0/24;  # from Gateway Deny policy
    allow 10.0.0.0/8;       # from Route Allow policy
    deny  all;              # default deny (from Allow policy)
    ...
}
```

## Testing

- Unit tests
- Functional tests that test attachment and inheritance behavior, including:
  - Policy attached to Gateway only — all Routes inherit access rules
  - Policy attached to Route only — only that Route is affected
  - Deny policy at Gateway + Allow policy at Route — Deny takes precedence
  - Allow policy at Gateway + Allow policy at Route — Route narrows access
  - Multiple policies of the same action type on the same target — rules are merged
  - Policy removal — access rules are cleaned up and affected conditions are removed
  - IPv4 and IPv6 address support
  - Integration with `RewriteClientIP` configuration

## Security Considerations

### Validation

Validating all fields in the `AccessPolicy` is critical to ensuring that the NGINX config generated by NGINX Gateway
Fabric is correct and secure.

All fields in the `AccessPolicy` will be validated with OpenAPI Schema validation. If the OpenAPI Schema validation
rules are not sufficient, we will use
[CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

Key validation rules:

- `Address` fields must be valid IPv4 or IPv6 addresses or CIDR ranges.
- TargetRef must reference Gateway, HTTPRoute, or GRPCRoute only.
- TargetRefs cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in the same policy.
- Rule names must be unique within a policy.
- When `source.type` is `IPAddress`, the `ipAddress` field must be set.
- The `action` field must be either `Allow` or `Deny`.

### Client IP Spoofing

If `RewriteClientIP` is not configured or is misconfigured, the `$remote_addr` variable may contain the address of a
proxy rather than the true client. In this case, access rules would be evaluated against the wrong IP, potentially
allowing unauthorized access or blocking legitimate traffic. The documentation should clearly state that
`RewriteClientIP` must be configured when NGINX Gateway Fabric is behind a proxy.

### RBAC

RBAC via the Kubernetes API server will ensure that only authorized users can create, update, or delete `AccessPolicy`
resources. Cluster Operators should restrict who can create policies targeting Gateways, as Gateway-level Deny policies
affect all attached Routes.

## Alternatives

- **Direct Policy instead of Inherited Policy**: A Direct Policy would not support the Gateway-to-Route inheritance
  model that is central to this design. Cluster Operators need to set organization-wide access rules on Gateways
  that Routes inherit, making an Inherited Policy the appropriate choice.
- **SnippetsPolicy / SnippetsFilter**: The existing `SnippetsPolicy` can already inject `allow`/`deny` directives
  (as shown in the [advanced NGINX extensions proposal](advanced-nginx-extensions.md#access-control-snippetpolicy)).
  However, this requires users to write raw NGINX configuration, provides no validation of IP addresses, no status
  reporting, and no structured composability of multiple policies. A purpose-built CRD provides a safer and more
  user-friendly experience.

## Future Work

- **Identity-based sources (ServiceAccount, SPIFFE)**: Adding new `AccessRuleSourceType` values and corresponding
  source fields to support identity-based access control alongside IP-based rules.
- **MCP authorization attributes**: Adding the `Authorization` field from the upstream `AuthorizationRule` to
  support method-level, path-level, and MCP-specific matching criteria.
- **CEL-based expressions**: Adding a CEL expression type to the authorization rule for complex matching logic.
- **ExternalAuth action**: Adding the upstream `ExternalAuth` action type to delegate authorization decisions to an
  external service.
- **Stream (L4) access control**: A separate, purpose-built CRD for IP-based access control on TLSRoute, TCPRoute,
  and UDPRoute. The IP source type definitions could be shared at the Go level, but the CRDs should be distinct to
  avoid exposing L7-only fields in an L4 context.
- **GeoIP-based rules**: Adding a geographic source type that matches by country or region code using the NGINX
  GeoIP module.

## References

- [NGINX `ngx_http_access_module` documentation](https://nginx.org/en/docs/http/ngx_http_access_module.html)
- [kube-agentic-networking XAccessPolicy](https://github.com/kubernetes-sigs/kube-agentic-networking/blob/main/api/v1alpha1/accesspolicy_types.go)
- [Policy Attachment GEP (GEP-713)](https://gateway-api.sigs.k8s.io/geps/gep-713/)
- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Rewrite Client IP Enhancement Proposal](rewrite-client-ip.md)
- [Advanced NGINX Extensions Enhancement Proposal](advanced-nginx-extensions.md)
