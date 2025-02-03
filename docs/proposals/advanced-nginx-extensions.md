# Enhancement Proposal-2035: Advanced NGINX Extensions

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/2035
- Status: Implementable

## Summary

NGINX Gateway Fabric (NGF) [exposes](/site/content/overview/gateway-api-compatibility.md) NGINX features via Gateway API
resources (like HTTPRoute) and [NGINX extensions](nginx-extensions.md) (like ClientSettingPolicy). Combined, they
expose a subset of the most common NGINX configuration. As we implement more Gateway API resources and NGINX extensions,
the subset will grow. However, it will take time. Additionally, because the number of NGINX configuration directives
and parameters is huge, not all of them will be supported that way. As a result, users are not able to implement certain
NGINX use cases. To allow them to implement those use cases, we need to bring a new extension mechanism to NGF.

## Goals

- Allow users to insert NGINX configuration not supported via Gateway API resources or NGINX extensions.
- Support configuration from modules not loaded in NGINX by default or third-party modules.
- Most of the configuration complexity should fall onto the Cluster operator persona, not the Application developer.
- Provide security controls to prevent Application developers from injecting arbitrary NGINX configuration.
- Ensure adequate configuration validation to prevent NGINX outages due to invalid configuration.
- Advanced NGINX extensions can be used without source code modification.

## Non-Goals

- Support configuration other than NGINX directives. For example, njs configuration files or TLS certificates.
- Reimplement already supported features through the new extension mechanism.
- Allow users to customize supported NGINX configuration (for example, add a parameter to an NGINX directive).

## Advanced Extensions

This proposal brings two extension mechanisms:

- [Snippets](#snippets) which allow to quickly bring unsupported NGINX configuration to NGF. However,
  because of its implications for reliability and security, Snippets are mostly applicable for the Cluster operator.
- [SnippetsTemplate](#snippetstemplate) which allows to bring unsupported NGINX configuration to NGF by Application
  developers in a safe and uncomplicated manner. However, SnippetsTemplates require some up-front work from a Cluster
  operator, meaning they cannot be implemented quickly, in contrast with Snippets.

### Snippets

Snippets allow inserting NGINX configuration into various NGINX contexts. They come in two flavors:

- SnippetsPolicy
- SnippetsFilter

#### API

> Specific field validation rules like CEL rules are intentionally left out of this proposal. However, important
> values restrictions are documented in the comments of the types.

```go
// SnippetsPolicy is a Direct Attached Policy. It allows inserting NGINX configuration into the generated NGINX
// config that affects resources referenced by the policy.
type SnippetsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SnippetsPolicy.
	Spec SnippetsPolicySpec `json:"spec"`

	// Status defines the state of the SnippetsPolicy.
	Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

// SnippetsPolicyList contains a list of SnippetsPolicies.
type SnippetsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SnippetsPolicy `json:"items"`
}

// SnippetsPolicySpec defines the desired state of the SnippetsPolicy.
type SnippetsPolicySpec struct {
	// Snippets is a list of NGINX configuration snippets.
	// There can only be one snippet per context.
	Snippets []Snippet `json:"snippets"`

	// TargetRefs identifies the API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Objects must be of the same Kinds.
	//
	// Support: Gateway, HTTPRoute, GRPCRoute
	//
	// Supported contexts depend on the targetRef Kind:
	//
	// * HTTPRoute and GRPCRoute: http, http.server and http.server.location.
	// * Gateway: all contexts.
	TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`
}

// NginxContext represents the NGINX configuration context.
type NginxContext string

const (
	// NginxContextMain is the main context of the NGINX configuration.
	NginxContextMain NginxContext = "main"

	// NginxContextHTTP is the http context of the NGINX configuration.
	NginxContextHTTP NginxContext = "http"

	// NginxContextHTTPServer is the server context of the NGINX configuration.
	NginxContextHTTPServer NginxContext = "http.server"

	// NginxContextHTTPServerLocation is the location context of the NGINX configuration.
	NginxContextHTTPServerLocation NginxContext = "http.server.location"

	// NginxContextStream is the stream context of the NGINX configuration.
	NginxContextStream NginxContext = "stream"

	// NginxContextStreamServer is the server context of the NGINX configuration.
	NginxContextStreamServer NginxContext = "stream.server"
)

// Snippet represents an NGINX configuration snippet.
type Snippet struct {
	// Context is the NGINX context to insert the snippet into.
	Context NginxContext `json:"context"`

	// Value is the NGINX configuration snippet.
	Value string `json:"value"`
}

// SnippetsFilter allows inserting NGINX configuration into the generated NGINX config for HTTPRoute and GRPCRoute
// resources.
// To implement authentication-like features, it is recommended to use SnippetsFilter instead of SnippetsPolicy.
// Since a Filter is referenced from a routing rule, it makes it clear to the Application Developer that authentication
// is applied to the route, whereas due to the discoverability issues around Policies, it wouldn't be clear to the
// Application Developer that authentication exists for their routes. Furthermore, if the referenced Filter does not
// exist, NGINX will return a 500 and the protected path will not be exposed. This isn't the case for a Policy, since
// a missing Policy would have no effect on routes and the protected path will be exposed to all users.
type SnippetsFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SnippetsFilter.
	Spec SnippetsFilterSpec `json:"spec"`

	// Status defines the state of the SnippetsFilter.
	Status SnippetsFilterStatus `json:"status,omitempty"`
}

// SnippetsFilterSpec defines the desired state of the SnippetsFilter.
type SnippetsFilterSpec struct {
	// Snippets is a list of NGINX configuration snippets.
	// There can only be one snippet per context.
	// Allowed contexts: http, http.server, http.server.location.
	Snippets []Snippet `json:"snippets"`
}

// SnippetsFilterStatus defines the state of SnippetsFilter.
type SnippetsFilterStatus struct {
	// Conditions describes the state of the SnippetsFilter.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SnippetsFilterConditionType is a type of condition associated with SnippetsFilter
type SnippetsFilterConditionType string

// SnippetsFilterConditionReason is a reason for a SnippetsFilter condition type.
type SnippetsFilterConditionReason string

const (
	// SnippetsFilterConditionTypeAccepted indicates that the SnippetsFilter is accepted.
	//
	// Possible reasons for this condition to be True:
	//
	// * Accepted
	//
	// Possible reasons for this condition to be False:
	//
	// * Invalid
	SnippetsFilterConditionTypeAccepted SnippetsFilterConditionType = "Accepted"

	// SnippetsFilterConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	SnippetsFilterConditionReasonAccepted SnippetsFilterConditionReason = "Accepted"

	// SnippetsFilterConditionTypeInvalid is used with the Accepted condition type when
	// SnippetsFilter is invalid.
	SnippetsFilterConditionTypeInvalid SnippetsFilterConditionType = "Invalid"
)
```

#### Supported NGINX Contexts

SnippetsPolicy supports the following NGINX configuration contexts:

- `main`
- http module: `http`, `server`, `location`
- stream module: `stream`, `server`

`upstream` context for both http and stream modules is not included for the following reasons:

- http module - there are not enough features in https://nginx.org/en/docs/http/ngx_http_upstream_module.html
  - There are a few `keepalive`-related directives. But they also require `proxy_set_header Connection "";`, which
    is possible to configure with location snippets, but it will conflict with the generated Connection header (see
    https://github.com/nginx/nginx-gateway-fabric/blob/5968bc348213a8470f6aaaa1a9bd51f2e90523ac/internal/mode/static/nginx/config/servers.go#L39-L40
    and https://github.com/nginx/nginx-gateway-fabric/blob/5968bc348213a8470f6aaaa1a9bd51f2e90523ac/internal/mode/static/nginx/config/maps_template.go#L21-L26)
    so those keepalive directives won't work.
  - Session persistence ([`sticky`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#sticky)). Those are  valid use cases. But at the same time, they only apply to
    NGINX Plus. Additionally, Gateway API started introducing session persistence.
    See https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1alpha2.BackendLBPolicy
- stream module - there are not any features in https://nginx.org/en/docs/stream/ngx_stream_upstream_module.html

Note: because NGF already inserts the `random` load-balancing method, an `upstream` snippet will not be able to
configure a different method.

If we choose to introduce upstream snippets, the SnippetsPolicy will need to support targeting a Service, because NGF
generates one upstream per Service.

> We don't support routes that correspond to the NGINX stream module -- TLSRoute, TCPRoute, and UDPRoute. However,
> because we're going to support them in the future, this proposal for SnippetsPolicy includes `stream` and `server`
> contexts of the stream module.

SnippetsFilter supports `http`, `server`, and `location` contexts of the http module.

> TLSRoute, TCPRoute, and UPDRoute don't support filters. As a result, SnippetsFilter doesn't support stream-related
> contexts.
>
> Snippets for location might share the same problem as mentioned in the issue https://github.com/nginx/nginx-gateway-fabric/issues/207,
> depending on the NGINX directives being used in the snippets. This proposal doesn't address the problem but
> anticipates the solution to https://github.com/nginx/nginx-gateway-fabric/issues/2079 will also solve the problem
> for Snippets.

#### Examples

Below are a few examples of using snippets to bring unsupported NGINX configuration into NGF.

##### Rate-limiting SnippetPolicy

We use NGINX [limit req module](https://nginx.org/en/docs/http/ngx_http_limit_req_module.html) to configure rate
limiting.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: rate-limit
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: cafe-route
  snippets:
  - context: http
    value: limit_req_zone $binary_remote_addr zone=myzone:10m rate=1r/s;
  - context: http.server.location
    value: limit_req zone=myzone burst=3;
```

As a result, NGF will insert those snippets into the generated configuration for the referenced HTTPRoute:

```text
# this is http context

limit_req_zone $binary_remote_addr zone=myzone:10m rate=1r/s;
. . .

server {
  . . .

  location /coffee {
    . . .
    limit_req zone=myzone burst=3;
    . . .
  }

  location /tea {
    . . .
    limit_req zone=myzone burst=3;
    . . .
  }
```

##### Proxy Buffering SnippetPolicy

We configure [proxy_buffering](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffering) directive
to disable buffering.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: buffering
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: cafe-route
  snippets:
  - context: http.server.location
    value: proxy_buffering off;
```

As a result, NGF will insert the provided config into the generated locations for the cafe-route HTTPRoute.

##### Access Control SnippetPolicy

We use NGINX [access module](https://nginx.org/en/docs/http/ngx_http_access_module.html) to configure access based
on client IPs.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: access-control
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: cafe
  snippets:
  - context: http.server
    value: |
      allow 10.0.0.0/8;
      deny all;
```

As a result, NGF will insert the provided NGINX config into the server context of all generated HTTP servers for
the Gateway cafe.

##### Access Control SnippetsFilter

We use NGINX [access module](https://nginx.org/en/docs/http/ngx_http_access_module.html) to configure access based
on client IPs.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsFilter
metadata:
  name: access-control
spec:
  snippets:
  - context: http.server.location
    value: |
      allow 10.0.0.0/8;
      deny all;
```

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: coffee
spec:
  parentRefs:
  - name: gateway
    sectionName: http
  hostnames:
  - "cafe.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /coffee
    filters:
    - type: ExtensionRef
      extensionRef:
        group: gateway.nginx.org/v1alpha1
        kind: SnippetsFilter
        name: access-control
    backendRefs:
    - name: headers
      port: 80
```

As a result, NGF will insert the provided NGINX config into all generated locations for the
`/coffee` path.

##### Third-Party Module SnippetPolicy

We use the third-party [Brotli module](https://docs.nginx.com/nginx/admin-guide/dynamic-modules/brotli/).

> nginx container image must include that module -- the user will need to extend the Dockerfile to install
> the module.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: brotli
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: cafe
  snippets:
  - context: main
    value: load_module modules/ngx_http_brotli_filter_module.so;
  - context: http.server
    value: brotli on;
```

As a result, NGF will:

- Insert `load_module` into the main context to load the module.
- Configure all `server` blocks belonging to the Gateway cafe to enable the module features.

### Inheritance and Conflicts of SnippetsPolicy

SnippetsPolicy is general-purpose: it can configure different NGINX features as shown in the
examples before. It is expected that several SnippetPolicies will exist in the cluster, with some of them targeting
the same resources. As a result, implementing inheritance rules or performing conflict resolution is not only
unnecessary, but it will reduce SnippetsPolicy applicability.

Although a SnippetsPolicy can target different Kinds (like Gateway and HTTPRoute), considering that
(1) each SnippetPolicy is independent of any other and (2) a single SnippetPolicy can only affect resources of the
same Kind, SnippetsPolicy is a [Direct Attached Policy](https://gateway-api.sigs.k8s.io/geps/gep-2648/).

### Why Both Policy and Filter

We introduce both SnippetsPolicy and SnippetsFilter because:

- The usage and error handling of SnippetsFilter is more explicit, as
  explained [here](nginx-extensions.md#authentication).
- SnippetsFilter creates a natural split of responsibilities between the Cluster operator and the Application developer:
  the Cluster operator creates a SnippetsFilter; the Application developer references the SnippetsFilter to enable it.
  Note that with the SnippetsPolicy, because the targetRef is part of the SnippetsPolicy, it is not possible to have
  such a split of responsibilities.

### Personas

The target persona of snippets is the Cluster operator. They will create and manage snippets.

Snippets are not intended for Application developers, because:

- To create snippets, you need to know about NGINX and how NGF generates NGINX configuration.
- Snippets can easily break NGINX config (if not validated by NGF). See
  [Config Validation section](#nginx-values) below.
- Snippets can be used to exploit NGF. (See [Security Considerations section](#security-considerations) below)

As mentioned in the [Why Both Policy and Filter](#why-both-policy-and-filter), when snippets are used
via SnippetsFilter, Application developers can still control whether they want to enable or disable snippets by
referencing them in an HTTPRoute.

### Validation

#### SnippetsPolicy/SnippetsFilter

NGF will validate the fields of SnippetsPolicy resources based on the restrictions mentioned in the [API section](#api).

We will only allow one snippet per context because it will be easier to comprehend a SnippetsPolicy this way: compare
one `http` snippet with multiple `http` snippets scattered around in the spec.

NGF will not validate the values of snippets. See the next section.

When a SnippetsFilter is specified in a routing rule and one of the following is true:

- the SnippetsFilter does not exist
- the SnippetsFilter referenced is invalid
- the same SnippetsFilter is referenced multiple times

the filter MUST NOT be skipped. Instead, requests that would have been processed by that filter MUST receive a 500 HTTP error code response.
In addition, the `ResolvedRefs` condition should be set to `False` with one of the following reasons:

- `FilterNotFound`
- `InvalidFilter`

These reasons are not defined in the Gateway API library, so they will need to be added and documented.
In addition, the `Accepted` route condition should be set to `True`.

Refer to [HTTPRouteFilter API](https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.HTTPRouteFilter) for more implementation details.

#### NGINX Values

An invalid snippet can break NGINX config. When this happens, NGINX will continue to use the last valid configuration.
However, any subsequent configuration updates (for example, caused by changes to an HTTPRoute) will not be possible
until the invalid snippet is removed.

Before injecting snippets in NGINX config, it is possible to validate snippets
using [crossplane](https://github.com/nginxinc/nginx-go-crossplane),
although some work is necessary to support it -- see https://github.com/nginxinc/nginx-go-crossplane/issues/94. Such
validation will catch cases of using invalid directives or invalid parameters of directives. At the same time, it
will not be a complete validation. For example, it won't catch errors like referencing an undefined variable
in the NGINX config.

### Security Considerations

The snippet below exposes the filesystem of the NGINX container including any TLS Secrets and the ServiceAccount Secret,
which NGF uses to access Kubernetes API:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: expose
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: cafe
  snippets:
  - context: http.server
    value: |
      server {
        listen 80;
        root /;
        autoindex on;
      }
```

As a consequence, creating SnippetsPolicy/SnippetsFilter should only be allowed for the privileged users -- the Cluster
operator. As a further precaution, we will disable SnippetsPolicy/SnippetsFilter by default similarly
to [NGINX Ingress Controller](https://docs.nginx.com/nginx-ingress-controller/configuration/security/#snippets).

It is also possible to add validation of snippets to disallow certain directives (like `root` and `autoindex`)
and certain values of directives. For an example of such extensive NGINX config validation see
[NGINX for Azure](https://docs.nginx.com/nginxaas/azure/overview/overview/) product.

### Upgrades

NGF inserts snippets into various parts of the generated configuration. As a result, the configuration
in the snippets must play well with the surrounding configuration, so that it doesn't break that configuration and vice
versa.

As we develop NGF, that surrounding configuration will expand (because we will implement more NGINX features) and also
might change (for example, due to improvements). As a result, there is a risk that snippets can break after the Cluster
operator upgrades NGF to the next version. Such risk shall be clearly documented in the SnippetsPolicy documentation.

### Prior Arts

- NGINX Ingress Controller supports
  snippets -- https://docs.nginx.com/nginx-ingress-controller/configuration/security/#snippets

### Alternatives

#### Splitting Snippets from SnippetsPolicy

In the example below, SnippetsPolicy reference the snippet, which is defined in a separate resource.

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: NginxSnippet
metadata:
  name: buffering-snippet
spec:
  snippets:
  - context: http.server.location
    value: proxy_buffering off;
---
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: buffering-snippet-policy
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: cafe-route
  snippetRef:
    name: buffering-snippet
```

This way the Cluster operator is still responsible for creating the snippet, and the Application developer can
apply the snippet to the required target. This way, the Application developer cannot create an unsafe snippet but
has full control over the target.

However, the Application developer can still target a Gateway resource, even though it is managed by the Cluster
operator:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsPolicy
metadata:
  name: buffering-snippet-policy
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: my-gateway
  snippetRef:
    name: buffering-snippet
```

As a result, that SnippetPolicy will affect all HTTPRoutes, not only HTTPRoutes of their application. At the same time,
inherited policies like ClientSettingsPolicy have the same issue.

We're not pursuing this approach for two reasons:

- Splitting snippets is already possible with SnippetsFilter.
- Splitting snippets complicates SnippetsPolicy, because now the Cluster operator needs to manage two resources.

### Summary

- Snippets allow the Cluster operator to quickly configure NGINX features not available via NGF.
- SnippetsPolicy and SnippetsFilter should be used only by the Cluster operator, because of reliability and security
  implications.
- SnippetsFilter, it is possible to split the responsibility of creating snippets (the Cluster operator) and enabling
  snippets (the Application Developer).

## SnippetsTemplate

SnippetsTemplate is similar to Snippets: it allows inserting NGINX configuration into various NGINX contexts.
However, in contrast with Snippets, it allows Application developers to safely use them without the risk of
breaking NGINX config. At the same time, that safety comes with a cost: creating a SnippetsTemplate is more involved
for the Cluster operator compared to Snippets.

### API

#### SnippetsTemplate

> Specific field validation rules like CEL rules are intentionally left out of this proposal. However, important
> values restrictions are documented in the comments of the types.

```go
// SnippetsTemplate configures an NGINX Gateway Fabric extension based on the templated NGINX configuration snippets.
type SnippetsTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SnippetsTemplate.
	Spec SnippetsTemplateSpec `json:"spec"`

	// Status defines the state of the SnippetsTemplate.
	Status SnippetsTemplateStatus `json:"status,omitempty"`
}

// SnippetsTemplateList contains a list of SnippetsTemplates.
type SnippetsTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SnippetsTemplate `json:"items"`
}

// SnippetsTemplateSpec defines the desired state of the SnippetsTemplate.
type SnippetsTemplateSpec struct {
	// ValuesCRD describes the CRD which provides values for the templates.
	ValuesCRD ValuesCRD `json:"valuesCRD"`

	// Templates is a list of NGINX configuration templates to insert into the generated NGINX config.
	// There can only be one template per context.
	Templates []Template `json:"templates"`
}

// ValuesCRD describes the CRD which provides values for the templates.
type ValuesCRD struct {
	// Name is the name of the CRD.
	// NGF will watch for resources of the CRD group and kind using the version for which 'storage: true'.
	Name string `json:"name"`

	// Type is the type of the CRD.
	Type ValuesCRDType `json:"type"`

	// AllowedKinds is a list of allowed kinds that can be used with the CRD.
	// If the type is 'DirectAttachedPolicy', allowedKinds are the kinds that the policy can target.
	// If the type is 'Filter', allowedKinds are the kinds that can reference the filter.
	AllowedKinds []AllowedKind `json:"allowedKinds"`
}

// ValuesCRDType is the type of the CRD which provides values for the templates.
type ValuesCRDType string

const (
	// ValuesCRDDirectAttachedPolicy corresponds to the DirectAttachedPolicy CRD.
	ValuesCRDDirectAttachedPolicy ValuesCRDType = "DirectAttachedPolicy"

	// ValuesCRDFilter corresponds to the Filter CRD.
	ValuesCRDFilter ValuesCRDType = "Filter"
)

// AllowedKind represents an allowed kind.
type AllowedKind struct {
	// Group is the group of the target resource.
	Group gatewayv1.Group `json:"group"`

	// Kind is kind of the target resource.
	Kind gatewayv1.Kind `json:"kind"`
}

// Template is an NGINX configuration template.
type Template struct {
	// Context is the NGINX context to insert the template into.
	Context NginxContext `json:"context"`

	// Value is the template. It must be a valid Go template -- https://pkg.go.dev/text/template.
	Value string `json:"value"`
}

// SnippetsTemplateStatus defines the state of the SnippetsTemplate.
type SnippetsTemplateStatus struct {
	// Conditions describes the state of the SnippetsTemplate.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SnippetsTemplateConditionType is a type of condition associated with SnippetsTemplate.
type SnippetsTemplateConditionType string

// SnippetsTemplateConditionReason is a reason for a SnippetsTemplate condition type.
type SnippetsTemplateConditionReason string

const (
	// SnippetsTemplateConditionTypeAccepted indicates that the SnippetsTemplate is accepted.
	//
	// Possible reasons for this condition to be True:
	//
	// * Accepted
	//
	// Possible reasons for this condition to be False:
	//
	// * Invalid
	// * CRDInvalid
	// * CRDNotFound
	SnippetsTemplateConditionTypeAccepted SnippetsTemplateConditionType = "Accepted"

	// SnippetsTemplateConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	SnippetsTemplateConditionReasonAccepted SnippetsTemplateConditionReason = "Accepted"

	// SnippetsTemplateConditionReasonInvalid is used with the Accepted condition type when
	// SnippetsTemplate is invalid.
	SnippetsTemplateConditionReasonInvalid SnippetsTemplateConditionReason = "Invalid"

	// SnippetsTemplateConditionReasonCRDInvalid is used with the Accepted condition type when
	// the referenced CRD is invalid.
	SnippetsTemplateConditionReasonCRDInvalid SnippetsTemplateConditionReason = "CRDInvalid"

	// SnippetsTemplateConditionReasonCRDNotFound is used with the Accepted condition type when
	// the referenced CRD is not found.
	SnippetsTemplateConditionReasonCRDNotFound SnippetsTemplateConditionReason = "CRDNotFound"
)
```

#### Templates

A template must be a [go template](https://pkg.go.dev/text/template).

NGF gives a template access to the data and metadata about a Custom resource (corresponds
to `SnippetsTemplate.spec.valuesCRD`) via `TemplateContext` type:

```go
// TemplateContext provides context for NGINX configuration templates.
type TemplateContext struct {
	// Metadata holds the metadata for NGINX configuration templates.
	Metadata TemplateMetadata
	// Data holds the data for NGINX configuration templates.
	Data TemplateData
}

// TemplateData holds the data for NGINX configuration templates.
type TemplateData struct {
	// Spec is the spec of the Policy/Filter of the Kind that corresponds to SnippetsTemplate.spec.valuesCRD.
	Spec map[string]interface{}
}

// TemplateMetadata holds the metadata for NGINX configuration templates.
type TemplateMetadata struct {
	// TargetMeta is the metadata of the target resource like HTTPRoute.
	TargetMeta metav1.ObjectMeta
	// SourceMeta is the metadata of the source resource (Policy/Filter) of the Kind that corresponds to
	// SnippetsTemplate.spec.valuesCRD.
	SourceMeta metav1.ObjectMeta
}
```

If requested by users, NGF can provide additional context. For example, for a template for the `location` context,
it can pass the data from
the [`Location`](https://github.com/nginx/nginx-gateway-fabric/blob/7bc0b6e6c5131920ac18f41359dd1eba7f53a8ba/internal/mode/static/nginx/config/http/config.go#L16)
struct, which NGF uses to generate location config.

NGF will unmarshal the values CRD spec into the `Spec` field of `TemplateData`. This way the template can access
the spec of the Custom resource of the CRD.

### How to Use SnippetsTemplate

1. A Cluster operator comes up with NGINX configuration that configures an NGINX feature needed by an Application
   developer.
2. The Cluster operator thinks about what values the Application developer might want to customize in that
   configuration.
3. The Cluster operator creates a CRD that defines fields and validation for those values, and chooses whether the CRD
   should be a Policy or a Filter. Then, the operator creates the CRD in the cluster.
4. The Cluster operator creates a go template that generates the NGINX configuration based on the fields of the CRD.
5. The Cluster operator allows the Application developer to use the feature by creating a SnippetsTemplate, which binds
   the CRD with the template(s).
6. The Application developer creates a Custom resource of the CRD to enable that NGINX feature with the desired values.

Essentially, the Cluster operator creates an extension for a particular NGINX feature. The Application developer can
use that extension with the ability to further customize it.

### Examples

#### Rate Limiting

First, a Cluster operator comes up with NGINX configuration for the rate-limiting feature asked by an Application
developer:

```text
# http context
limit_req_zone $binary_remote_addr zone=myzone:10m rate=1r/s;

server {
  . . .
  location /somepath {
    . . .
    limit_req zone=myzone burst=3;
  }
```

Next, the Cluster operator decides that the Application developer might want to customize the rate-limiting `rate`
and `burst`
values.

Next, the Cluster operator defines the following CRD for those values. Note that the values are validated to prevent
invalid or malicious values from being propagated into the NGINX config. The Cluster operator also decided to use
a Policy for this use case.

```go
// RateLimitingPolicy is a Direct Attached Policy to configure rate-limiting of client requests.
type RateLimitingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the RateLimitingPolicy.
	Spec RateLimitingPolicySpec `json:"spec"`

	// Status defines the state of the RateLimitingPolicy.
	Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RateLimitingPolicyList contains a list of RateLimitingPolicies.
type RateLimitingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimitingPolicy `json:"items"`
}

// RateLimitingPolicySpec defines the desired state of RateLimitingPolicy.
type RateLimitingPolicySpec struct {
	// The rate of requests permitted per second.
	//
	// +kubebuilder:validation:Minimum=1
	Rate int32 `json:"rate"`

	// Burst delays excessive requests until their number exceeds the burst value, in which case the request is
	// terminated with an error.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	Burst *int32 `json:"burst,omitempty"`

	// TargetRefs identifies the API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Support: HTTPRoute.
	//
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be: HTTPRoute.",rule="self.exists(t, t.kind=='HTTPRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io.",rule="self.all(t, t.group=='gateway.networking.k8s.io')"
	//nolint:lll
	TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`
}
```

Next, the Cluster operator generates a CRD manifest from that go code and creates the CRD in the cluster.

Next, the Cluster operator creates templates that generate the NGINX configuration based on the fields of the CRD:

- For http context:

  ```text
  {{ $rate := index $.Data "rate" }}
  {{ $zoneName := $.Metadata.UID }}
  limit_req_zone $binary_remote_addr zone={{ $zoneName }}:10m rate={{ $rate }}r/s;
  ```

- For location context:

  ```text
  {{ $zoneName := $.Metadata.UID }}
  {{ $burst := index $.Data "burst }}
  limit_req zone={{ $zoneName }}{{ if $burst }}burst={{ $burst }}{{ end }};
  ```

Next, the Cluster operator allows the Application developer to use the feature by creating a SnippetsTemplate, which
binds the CRD with the templates:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsTemplate
metadata:
  name: rate-limiting-template
spec:
  valuesCRD:
    name: ratelimitingpolicy.example.org
    type: DirectAttachedPolicy
    allowedKinds:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
  templates:
  - context: http
    value: |
      {{ $rate := index $.Data "rate" }}
      {{ $zoneName := $.Metadata.UID }}
      limit_req_zone $binary_remote_addr zone={{ $zoneName }}:10m rate={{ $rate }}r/s;
  - context: http.server.location
    value: |
      {{ $zoneName := $.Metadata.UID }}
      {{ $burst := index $.Data "burst }}
      limit_req zone={{ $zoneName }}{{ if $burst }}burst={{ $burst }}{{ end }};
```

As a result, the Cluster operator created an extension, and now it is ready to be used by the Application developer.

Next, the Application developer creates a Custom resource of the CRD to enable that NGINX feature with the desired
values:

```yaml
apiVersion: example.org/v1alpha1
kind: RateLimitingPolicy
metadata:
  name: rate-limit
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: cafe-route
  rate: 10
  burst: 5
```

As a result, NGF will insert the config generated by templates into the configuration generated for the
referenced HTTPRoute:

```text
# this is http context

limit_req_zone $binary_remote_addr zone=d1fd001c-a784-4964-baa2-100d16aa5540:10m rate=10r/s;
. . .

server {
  # this is server
  . . .

  location /coffee {
    . . .
    limit_req zone=d1fd001c-a784-4964-baa2-100d16aa5540 burst=5;
    . . .
  }

  location /tea {
    . . .
    limit_req zone=d1fd001c-a784-4964-baa2-100d16aa5540 burst=5;
    . . .
  }
```

> For brevity, the proposal doesn't include more examples. However, SnippetsTemplate can also implement all examples
> used for SnippetsPolicy. Additionally, the values CRD can also be a Filter rather than a Policy (see a short
> example below).

### Why Both Policy and Filter

Similarly to SnippetsPolicy, considering that SnippetsTemplate can be used to implement authentication and
authorization, because of the reasons mentioned
[here](nginx-extensions.md#authentication), it makes sense to also support filters as the values CRD.

For the rate-limiting example, such CRD remains the same but without the `targetRefs` field.

```yaml
apiVersion: example.org/v1alpha1
kind: RateLimitingFilter
metadata:
  name: rate-limit
spec:
  rate: 10
  burst: 5
```

HTTPRoutes can include that filter:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app
spec:
  . . .
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    filters:
    - type: ExtensionRef
      extensionRef:
        group: example.org/v1alpha1
        kind: RateLimitingFilter
        name: rate-limit
    backendRefs:
    - name: headers
      port: 80
```

Similarly, such filters can also be used in  GRPCRoute. However, TLSRoute, TCPRoute, and UDPRoute don't support filters,
so it needs to be added to the Gateway API first.

### Inheritance and Conflicts

If the values CRD is a Policy, it must be a [Direct Attached Policy](https://gateway-api.sigs.k8s.io/geps/gep-2648/).
Specifically, it must only allow target the same Kinds.

NGF will perform conflict resolution if the Custom resources of the CRD target the same resources.

If requested by users, this proposal can be extended to allow the CRD to also allow
[Inhereted Policy Attachment](https://gateway-api.sigs.k8s.io/geps/gep-2649/).

### Personas

- The Cluster operator creates a SnippetsTemplate and the values CRD.
- The Application developer uses it by creating a Custom resource of the values CRD of the SnippetsTemplate.

### Validation

#### NGINX Values

When a Cluster operator defines a values CRD, they should define validation rules to prevent invalid or malicious values
propagating into NGINX config. The Kubernetes API server will perform that validation when an Application developer
creates a Custom resource of the CRD.

#### SnippetsTemplate

A template defined in SnippetsTemplate can be invalid (not follow go template or panic when executed). NGF must reject
such templates and also not crash when executing them.

A template can generate invalid NGINX configuration. When this happens, NGINX will continue to use the last valid
configuration. However, any subsequent configuration updates (for example, caused by changes to an HTTPRoute) will not
be possible until the invalid configuration is removed. Thus, a Cluster operator must carefully design the template.

NGF will also validate the rest of SnippetsTemplate fields based on the restrictions mentioned in
the [API section](#api-1).

We will only allow one template per context because it will be easier to comprehend a SnippetsTemplate this way: compare
one `http` template with multiple `http` templates scattered around in the spec.

### Security Considerations

As mentioned in the previous section, the Cluster operator should define validation rules to prevent invalid/malicious
values from Custom resources created by Application developers from propagating into the NGINX config.

However, it is possible that a template generates malicious values. Because of that, only the Cluster operator (a
privileged user) should be able to create SnippetsTemplates. As a further precaution, we will disable SnippetsTemplate
by default similarly to [SnippetsPolicy](#security-considerations).

### Upgrades

NGF inserts configuration generated from the templates of a SnippetsTemplate into various parts of the configuration
that it already generates. As a result, the configuration generated from the templates must play well with
the surrounding configuration, so that it doesn't break that configuration and vice versa.

As we develop NGF, that surrounding configuration will expand (because we will implement more NGINX features) and also
might change (for example, due to improvements). As a result, there is a risk that a SnippetsTemplate breaks after
a Cluster operator upgrades NGF to the next version. Such risk shall be clearly documented in the SnippetsTemplate
documentation.

### Prior Arts

- NGINX Ingress
  Controller [Custom annotations](https://docs.nginx.com/nginx-ingress-controller/configuration/ingress-resources/custom-annotations/)
- NGINX Instance
  Manager [Config Templates](https://docs.nginx.com/nginx-management-suite/nim/about/templates/config-templates/)

### Alternatives

Instead of asking the Cluster operator to design a CRD, we can provide a ready container CRD. For example:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsTemplate
metadata:
  name: rate-limiting-template
spec:
  kind: RateLimitingPolicy
  type: DirectAttachedPolicy
  allowedKinds:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
  templates:
  - context: http
    value: |
      {{ $rate := index $.Data "rate" }}
      {{ $zoneName := $.Metadata.UID }}
      limit_req_zone $binary_remote_addr zone={{ $zoneName }}:10m rate={{ $rate }}r/s;
  - context: http.server.location
    value: |
      {{ $zoneName := $.Metadata.UID }}
      {{ $burst := index $.Data "burst }}
      limit_req zone={{ $zoneName }}{{ if $burst }}burst={{ $burst }}{{ end }};
```

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: Values
metadata:
  name: rate-limit
spec:
  values: # values field is a container
    apiVersion: example.com/v1alpha1
    kind: RateLimitingPolicy # must match kind in SnippetsTemplate.spec.kind
    spec:
      rate: 10
      burst: 5
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: cafe-route
```

Embedding values is possible by embedding a resource.
See https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#rawextension

> Note: The cluster operator doesn't need to register the RateLimitingPolicy CRD. However, when embedding a resource
> into a Custom resource field, it is always necessary to provide the apiVersion and kind.

As a result, NGF will execute the templates from the SnippetsTemplate registered for the RateLimitingPolicy,
providing the values from Values.spec.values.spec to the templates.

Pros:

- The Cluster operator doesn't need to design and create a CRD.
- The Cluster operator doesn't need to change NGF RBAC rules to allow for it to watch for the CRD resource type.

Cons:

- Lack of CRD validation. Kubernetes API will not be able to perform validation of the values.
- More complex validation. Because we still want to support validation, we will need to design a mechanism to allow
  the Cluster operator to define a scheme with the structure and validation rules for the values. Because we will not
  be relying on the already available validation mechanism (CRD validation), this will result in extra complexity.

We're not going to pursue this approach because of its cons.

### Summary

SnippetsTemplate allows Application developers to safely and easily use NGINX configuration created by a Cluster
operator. The Cluster operator retains full control of that NGINX configuration, but at the same time allows Application
developers to provide custom values. Kubernetes API will validate such values, which leads to better security and
reliability compared to SnippetsPolicy/SnippetsFilter.

## NGINX Features Supported by Proposed Extensions

> Note: the list is from [NGINX Extensions](nginx-extensions.md#prioritized-nginx-features).

| Features                                                                                         | Supported by SnippetsPolicy/SnippetsFilter/SnippetsTemplate            | Requires NGINX Plus |
|--------------------------------------------------------------------------------------------------|------------------------------------------------------------------------|---------------------|
| Log level and format                                                                             | X (limited, only extra logs on top of the default one)                 |                     |
| Passive health checks                                                                            |                                                                        |                     |
| Slow start. Prevent a recently recovered server from being overwhelmed on connections by restart |                                                                        | X                   |
| Active health checks                                                                             | X (if Service is allowed as TargetRef)                                 | X                   |
| JSON Web Token (JWT) authentication                                                              | X                                                                      | X                   |
| OpenID Connect (OIDC) authentication. Allow users to authenticate with single-sign-on (SSO)      | X (njs files need to be mounted to nginx container FS separately)      | X                   |
| Customize client settings. For example, `client_max_body_size`                                   | X                                                                      |                     |
| Upstream keepalives                                                                              |                                                                        |                     |
| Client keepalives                                                                                | X                                                                      |                     |
| TLS settings. For example, TLS protocols and server ciphers                                      | X                                                                      |                     |
| OpenTelemetry tracing                                                                            | X                                                                      |                     |
| Connection timeouts                                                                              | X                                                                      |                     |
| Backup server                                                                                    |                                                                        |                     |
| Load-balancing method                                                                            |                                                                        |                     |
| Load-balancing method (least time)                                                               |                                                                        | X                   |
| Limit connections to a server                                                                    |                                                                        |                     |
| Request queuing. Queue requests if servers are unavailable                                       |                                                                        | X                   |
| Basic authentication. User/Password                                                              | X (password files need to be mounted to nginx container FS separately) |                     |
| PROXY protocol                                                                                   |                                                                        |                     |
| Upstream zone size                                                                               |                                                                        |                     |
| Custom response. Return a custom response for a given path                                       |                                                                        |                     |
| External routing. Route to services outside the cluster                                          |                                                                        |                     |
| HTTP Strict Transport Security (HSTS)                                                            | X                                                                      |                     |
| API Key authentication                                                                           | X (API Keys need to be mounted to nginx container FS separately)       |                     |
| Next upstream retries. NGINX retries by trying the _next_ server in the upstream group           | X                                                                      |                     |
| Proxy Buffering                                                                                  | X                                                                      |                     |
| Pass/hide headers to/from the client                                                             | X                                                                      |                     |
| Custom error pages                                                                               | X                                                                      |                     |
| Access control                                                                                   | X                                                                      |                     |
| Windows Technology LAN Manager (NTLM)                                                            |                                                                        | X                   |
| Backend client certificate verification                                                          | X (CA file needs to be mounted to nginx container FS separately)       |                     |
| NGINX worker settings                                                                            | X                                                                      |                     |
| HTTP/2 client requests                                                                           |                                                                        |                     |
| Set server name size                                                                             | X                                                                      |                     |
| JSON Web Token (JWT) content-based routing                                                       |                                                                        | X                   |
| QUIC/HTTP3                                                                                       |                                                                        |                     |
| Remote authentication request                                                                    | X                                                                      |                     |
| Timeout for unresponsive clients                                                                 | X                                                                      |                     |
| Rate-limiting                                                                                    | X                                                                      |                     |
| Session persistence                                                                              |                                                                        | x                   |
| Cross-Origin Resource Sharing (CORS)                                                             | X                                                                      |                     |
