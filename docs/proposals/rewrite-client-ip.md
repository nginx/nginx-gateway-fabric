# Enhancement Proposal-2335: Rewrite Client IP

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/2325
- Status: Completed

## Summary

This Enhancement Proposal extends the [NginxProxy API](gateway-settings.md), to allow users to configure a method to rewrite the client's IP address to the original client's IP when NGF is fronted by another load balancer or proxy.

## Goals

- Define the API for rewriting the client's IP address.

## Non-Goals

- Provide implementation details for implementing the new API.

## Introduction

When requests travel through one or more proxies or load balancers before reaching NGINX Gateway Fabric, the client IP address is set to the IP address of the server that last handled the request.

For example, consider this request flow:

```mermaid
flowchart LR
    C(Client 1.1.1.1) --> P1(Proxy1 2.2.2.2) --> P2(Proxy2 3.3.3.3) --> NGF(NGINX Gateway Fabric)
```

When the request reaches NGINX Gateway Fabric, the client's IP address, stored in the NGINX variable `$remote_addr`, is set to `3.3.3.3`. A user may want to preserve the original client's IP address, in this case `1.1.1.1`, and pass that to their backend applications.

### Methods for preserving client IP addresses

- X-Forwarded-For: A multi-value HTTP header that is appended to by each proxy. Each proxy appends the IP address of the host from which it received the request. Resulting header should look like `X-Forwarded-For: client, proxy1, proxy2`. Other headers for passing port, host, and proto information: X-Forwarded-Host, X-Forwarded-Port, X-Forwarded-Proto.
- [Forwarded](https://datatracker.ietf.org/doc/html/rfc7239): A multi-value HTTP header of key-value pairs separated by semicolons. `Forwarded: for=client;port=80;proto=https, for=proxy;port=80;proto=https`. Similar to X-Forwarded-For, each proxy appends to this header.
- X-Real-IP: a single value header that contains just the client IP address.
- [PROXY protocol](http://www.haproxy.org/download/1.8/doc/proxy-protocol.txt): allows TCP proxies to inject data about original source and dest addresses to their upstream servers without knowledge of underlying protocol. Can operate at L4, instead of L7.
- Custom header: determine the client IP address for a request based on a trusted custom HTTP header.

The most popular methods are PROXY protocol and X-Forwarded-For. Choosing a method will depend on how the Load Balancer fronting NGF preserves the client IP address, and what the user wants to do with the client IP. If passing the client IP to the backend, then it's important to consider how the backend expects to receive this information.

Initially, this design will expose these two methods only, but it can be extended in the future to support the additional methods.

### Required NGINX directives and their behavior

#### `proxy_protocol` listen param

The `proxy_protocol`  listen parameter configures NGINX server to accept the PROXY protocol. NGINX will also set the `$proxy_protocol_addr` and `$proxy_protocol_port` variables to the original client address and port.

#### real ip module

The [real-ip module](https://nginx.org/en/docs/http/ngx_http_realip_module.html) rewrites the values in the `$remote_addr` and `$remote_port` variables to the client IP address and port. Without this module, the `$remote_addr` and `$remote_port` variables are set to the IP address and port of the load balancer.

How the real-ip modules determines the client IP address and port depends on how you configure it.

#### `set_real_ip_from`

The [`set_real_ip_from`](https://nginx.org/en/docs/http/ngx_http_realip_module.html#set_real_ip_from) directive tells NGINX to only trust replacement IPs from these addresses.

If not provided, the `$remote_addr` and `$remote_port` variables will never be replaced.

To trust all addresses, set to `0.0.0.0/0`.

This directive is also used by the `real_ip_recursive` directive.

#### `real_ip_header`

The [`real_ip_header`](https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header) directive sets the header whose value will be used to replace the client address. By default, NGINX will use the value of the X-Real-IP header. This directive can be set to X-Forwarded-To, proxy_protocol, or any other header name. If set to proxy_protocol, proxy_protocol must be enabled on the server.

#### `real_ip_recursive`

The [`real_ip_recursive`](https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_recursive) directive configures whether recursive search is used when selecting the client's address from a multi-value header. Only makes sense when the header specified in `real_ip_header` is a multi-value header (e.g. contains a list of addresses). Commonly used with X-Forwarded-For. For example:

Say you have the following setup:

```mermaid
flowchart LR
    C(Client 1.1.1.1) --> P1(Proxy1 2.2.2.2) --> P2(Proxy2 3.3.3.3) --> NGF(NGINX Gateway Fabric)
```

and the following NGINX config:

```nginx configuration
 set_real_ip_from 3.3.3.3;
 real_ip_header X-Forwarded-For;
 real_ip_recursive on;
```

Once the request hits NGF, the `X-Forwarded-For` header contains three IP addresses: `X-Forwarded-For: [1.1.1.1, 2.2.2.2, 3.3.3.3]`

Because `real_ip_recursive` is on, NGINX will set `$remote_addr` to 2.2.2.2. This is because it recurses on the values in X-Forwarded-Header from end of array to start of array and selects the first untrusted ip. If you wanted to set `$remote_addr` to the user's IP address instead, and you trust the Proxy, you could achieve that by also specifying the Proxy's IP using `set_real_ip_from`:

```nginx configuration
 set_real_ip_from 3.3.3.3;
 set_real_ip_from 2.2.2.2;
 real_ip_header X-Forwarded-For;
 real_ip_recursive on;
```

If `real_ip_recursive` is off, NGINX will set `$remote_addr` to 3.3.3.3 because it will select the rightmost address.

## API, Customer Driven Interfaces, and User Experience

This API will be added to the `NginxProxy` CRD that is a part of the `gateway.nginx.org` Group. It will be referenced in the `parametersRef` field of a GatewayClass. It will live at the cluster scope.

This is a dynamic configuration that can be changed by a user at any time, and NGF will propagate those changes to NGINX.

For example, an `NginxProxy` named `proxy-settings` would be referenced as follows:

```yaml
kind: GatewayClass
metadata:
    name: nginx
spec:
    controllerName: gateway.nginx.org/nginx-gateway-controller
    parametersRef:
        group: gateway.nginx.org/v1alpha1
        kind: NginxProxy
        name: proxy-settings
```

Below is the Golang API for the `RewriteClientIP` field on `NginxProxy`. Note, all other `NginxProxy` fields have been omitted to keep the focus on `RewriteClientIP`.

### Go

```go
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type NginxProxy struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`

  // Spec defines the desired state of the NginxProxy.
  Spec NginxProxySpec `json:"spec"`
}

type NginxProxySpec struct {
  // RewriteClientIP contains configuration for rewriting the client IP to the original client's IP.
  // +optional
  RewriteClientIP *RewriteClientIP `json:"rewriteClientIP,omitempty"`
}

// RewriteClientIP specifies the configuration for rewriting the client's IP address.
// The client's IP will be stored in the $remote_addr NGINX variable and passed to the backends in the X-Real-IP and X-Forwarded-For* headers.
type RewriteClientIP struct {
  // Mode defines how NGINX will rewrite the client's IP address.
  // There are two possible modes:
  // - ProxyProtocol: NGINX will rewrite the client's IP using the PROXY protocol header.
  // - XForwardedFor: NGINX will rewrite the client's IP using the X-Forwarded-For header.
  // +optional
  Mode *RewriteClientIPModeType `json:"mode,omitempty"`

  // TrustedAddresses specifies the addresses that are trusted to send correct client IP information.
  // If a request comes from a trusted address, NGINX will rewrite the client IP information, and forward it to the backend in the X-Forwarded-For* and X-Real-IP headers.
  // If the request does not come from a trusted address, NGINX will not rewrite the client IP information.
  // Addresses must be provided as CIDR blocks: 10.0.0.0/32, 192.33.21/0.
  // To trust all addresses (not recommended), set to 0.0.0.0/0.
  // If no addresses are provided, NGINX will not rewrite the client IP information.
  // Sets the set_real_ip_from directive in NGINX: https://nginx.org/en/docs/http/ngx_http_realip_module.html#set_real_ip_from.
  // This field is required if mode is set.
  TrustedAddresses []string `json:"trustedAddresses,omitempty"`

  // SetIPRecursively configures whether recursive search is used when selecting the client's address from the X-Forwarded-For header.
  // It is used in conjunction with TrustedAddresses.
  // If enabled, NGINX will recurse on the values in X-Forwarded-Header from the end of array to start of array and select the first untrusted IP.
  // For example, if X-Forwarded-For is [11.11.11.11, 22.22.22.22, 55.55.55.1], and TrustedAddresses is set to 55.55.55.1/0, NGINX will rewrite the client IP to 22.22.22.22.
  // If disabled, NGINX will select the IP at the end of the array. In the previous example, 55.55.55.1 would be selected.
  //
  // +optional
  SetIPRecursively *bool `json:"setIPRecursively,omitempty"`
}

// RewriteClientIPModeType defines how NGINX Gateway Fabric will determine the client's original IP address.
// +kubebuilder:validation:Enum=ProxyProtocol;XForwardedFor
type RewriteClientIPModeType string

const (
  // RewriteClientIPModeProxyProtocol configures NGINX to accept PROXY protocol and set the client's IP address to the IP address in the PROXY protocol header.
  // Sets the proxy_protocol parameter to the listen directive on all servers, and sets real_ip_header to proxy_protocol: https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header.
  RewriteClientIPModeProxyProtocol RewriteClientIPModeType = "ProxyProtocol"

  // RewriteClientIPModeXForwardedFor configures NGINX to set the client's IP address to the IP address in the X-Forwarded-For HTTP header.
  // Sets real_ip_header to XForwardedFor: https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header.
  RewriteClientIPModeXForwardedFor RewriteClientIPModeType = "XForwardedFor"
)

```

The benefits of this API are:

1. Allow users to easily configure the two most common methods for rewriting the client IP address.
2. Express configuration in terms of use cases.
3. Secure by default. By requiring that TrustedAddresses is set, users will have to explicitly enable addresses to trust.
4. Require minimal knowledge of NGINX configuration while still correlating fields to NGINX directives or behavior.
5. Allow for extension to support other methods of rewriting IP addresses in the future.

### Validation

The Go API above does not contain validation annotation. Annotations should be added to enforce the following rules.

- If `mode` is set, then `trustedAddresses` is required.
- `trustedAddresses` can have up to 16 CIDR blocks.
- `trustedAddresses` must be in CIDR block notation.

### Status

Status is set on the GatewayClass, not the `NginxProxy` resource. If the `NginxProxy` is invalid, set the `Accepted` condition reason on the GatewayClass to `InvalidParameters` but still mark `Accepted` as `True`. See [gateway settings proposal](gateway-settings.md#status) for more details on status.

### Future Work

- If requested by a user, add more `RewriteClientIPModes`, such as custom header or Forwarded.
- Allow users to rate limit or apply security policies using the value of `$remote_addr`.
- The `set_real_ip_from` directive accepts IP addresses, CIDR blocks, hostnames and the special value `unix:;` which trusts all UNIX-domain sockets. For simplicity, we will begin by only allowed CIDR blocks since this will cover most use cases. However, if a user requests it, we can extend the TrustedAddresses field to accept other types of addresses.

## Use Cases

- As a Cluster Operator, I want to configure NGINX to rewrite the client's IP address using PROXY protocol for all applications associated with the GatewayClass.
- As a Cluster Operator, I want to configure NGINX to rewrite the client's IP address using the X-Forwarded-For header for all applications associated with the GatewayClass.

## Testing

- Unit tests
- Functional tests that verify the attachment of the CRD to the GatewayClass, and that NGINX behaves properly based on the configuration. This includes verifying client IP is propagated to the backends.

## Security Considerations

Validating all fields in the `NginxProxy` is critical to ensuring that the NGINX config generated by NGINX Gateway Fabric is correct and secure.

All fields in the `NginxProxy` will be validated with Open API Schema. If the Open API Schema validation rules are not sufficient, we will use [CEL](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules).

RBAC via the Kubernetes API server will ensure that only authorized users can update the CRD.

## Alternatives

### API that maps to NGINX directives

Rather than create an expressive API with use-case driven language, we can simply expose the NGINX directives:

```go
type NginxProxySpec struct {
  // RewriteClientIP contains configuration for rewriting the client IP to the original client's IP.
  // +optional
  RewriteClientIP *RewriteClientIP `json:"rewriteClientIP,omitempty"`
}

// RewriteClientIP specifies
type RewriteClientIP struct {
  EnableProxyProtocol *bool `json:"rewriteClientIP,omitempty"`
  SetRealIPFrom []string `json:"setRealIPFrom,omitempty"`
  RealIPHeader string `json:"realIPHeader,omitempty"`
  RealIPRecursive *bool `json:"realIPRecursive,omitempty"`
}
```

then, we could add user documentation describing how to implement the two most common use cases:

1. PROXY protocol:

    ```yaml
    apiVersion: gateway.nginx.org/v1alpha1
    kind: NginxProxy
    metadata:
      name: proxy-protocol
    spec:
      rewriteClientIP:
        enableProxyProtocol: true
        setRealIPFrom:
          - 0.0.0.0/0
        realIPHeader: proxy_protocol
    ```

2. X-Forwarded-For

    ```yaml
    apiVersion: gateway.nginx.org/v1alpha1
    kind: NginxProxy
    metadata:
      name: x-forwarded-for
    spec:
      rewriteClientIP:
        setRealIPFrom:
          - 0.0.0.0/0
        realIPHeader: x-forwarded-for
        realIPRecursive: true
    ```

The benefit to this approach is that it gives the users the full power of the NGINX real IP module and immediately allows them to configure all methods to rewrite a client's IP address.
This is ideal for users that are familiar with NGINX configuration and know what these NGINX directives are and how they work. However, for non-NGINX users or NGINX newcomers, this API would be more challenging to use.
Without reading the documentation, it would be difficult to figure out how to configure PROXY protocol. Additionally, there's a higher chance of misconfiguration since each use case requires three fields to be in sync with each other.

## References

- [NGINX Extensions Enhancement Proposal](nginx-extensions.md)
- [Attaching Policy to GatewayClass](https://gateway-api.sigs.k8s.io/geps/gep-713/#attaching-policy-to-gatewayclass)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [Gateway Settings (NginxProxy CRD)](gateway-settings.md)
