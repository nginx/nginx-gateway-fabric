# Gateway API Compatibility

This document describes which Gateway API resources NGINX Kubernetes Gateway supports and the extent of that support.

## Summary

| Resource | Support Status |
|-|-|
| [GatewayClass](#gatewayclass) | Partially supported |
| [Gateway](#gateway) | Partially supported |
| [HTTPRoute](#httproute) | Partially supported |
| [TLSRoute](#tlsroute) | Not supported |
| [TCPRoute](#tcproute) | Not supported |
| [UDPRoute](#udproute) | Not supported |
| [ReferenceGrant](#referencegrant) |  Not supported |
| [Custom policies](#custom-policies) | Not supported |

## Terminology

We use the following words to describe support status:
- *Supported*. The resource or field is fully supported and conformant to the Gateway API specification.
- *Partially supported*. The resource or field is supported partially or with limitations. It will become fully supported in future releases.
- *Not supported*. The resource or field is not yet supported. It will become partially or fully supported in future releases.

Note: it might be possible that NGINX Kubernetes Gateway will never support some resources and/or fields of the Gateway API. We will document these decisions on a case by case basis.

## Resources

Below we list the resources and the support status of their corresponding fields. 

For a description of each field, visit the [Gateway API documentation](https://gateway-api.sigs.k8s.io/references/spec/). 

### GatewayClass 

> Status: Partially supported. 

NGINX Kubernetes Gateway supports only a single GatewayClass resource configured via `--gatewayclass` flag
of the [static-mode](./cli-help.md#static-mode) command.

Fields:
* `spec`
	* `controllerName` - supported.
	* `parametersRef` - not supported.
	* `description` - supported.
* `status`
	* `conditions` - partially supported.

### Gateway

> Status: Partially supported.

NGINX Kubernetes Gateway supports only a single Gateway resource. The Gateway resource must reference NGINX Kubernetes Gateway's corresponding GatewayClass.
See [static-mode](./cli-help.md#static-mode) command for more info.

Fields:
* `spec`
	* `gatewayClassName` - supported.
	* `listeners`
		* `name` - supported.
		* `hostname` - partially supported. Wildcard hostnames like `*.example.com` are not yet supported.
		* `port` - partially supported. Allowed values: `80` for HTTP listeners and `443` for HTTPS listeners.
		* `protocol` - partially supported. Allowed values: `HTTP`, `HTTPS`.
		* `tls`
		  * `mode` - partially supported. Allowed value: `Terminate`.
		  * `certificateRefs` - partially supported. The TLS certificate and key must be stored in a Secret resource of type `kubernetes.io/tls` in the same namespace as the Gateway resource. Only a single reference is supported. You must deploy the Secret before the Gateway resource. Secret rotation (watching for updates) is not supported.
		  * `options` - not supported.
		* `allowedRoutes` - not supported. 
	* `addresses` - not supported.
* `status`
  * `addresses` - Pod IPAddress supported.
  * `conditions` - not supported.
  * `listeners`
	* `name` - supported.
	* `supportedKinds` - not supported.
	* `attachedRoutes` - supported.
	* `conditions` - Supported (Condition/Status/Reason):
      * `Accepted/True/Accepted`
      * `Accepted/True/ListenersNotValid`
      * `Accepted/False/Invalid`
      * `Accepted/False/ListenersNotValid`
      * `Accepted/False/UnsupportedValue`: Custom reason for when a value of a field in a Gateway is invalid or not supported.
      * `Accepted/False/GatewayConflict`: Custom reason for when the Gateway is ignored due to a conflicting Gateway. NKG only supports a single Gateway.

### HTTPRoute

> Status: Partially supported.

Fields:
* `spec`
  * `parentRefs` - partially supported. Port not supported.
  * `hostnames` - partially supported. Wildcard binding is not supported: a hostname like `example.com` will not bind to a listener with the hostname `*.example.com`. However, `example.com` will bind to a listener with the empty hostname.
  * `rules`
	* `matches`
	  * `path` - partially supported. Only `PathPrefix` and `Exact` types.
	  * `headers` - partially supported. Only `Exact` type.
	  * `queryParams` - partially supported. Only `Exact` type. 
	  * `method` -  supported.
	* `filters`
		* `type` - supported.
		* `requestRedirect` - supported except for the experimental `path` field. If multiple filters with `requestRedirect` are configured, NGINX Kubernetes Gateway will choose the first one and ignore the rest. 
		* `requestHeaderModifier`, `requestMirror`, `urlRewrite`, `extensionRef` - not supported.
	* `backendRefs` - partially supported. Backend ref `filters` are not supported.
* `status`
  * `parents`
	* `parentRef` - supported.
	* `controllerName` - supported.
	* `conditions` - partially supported. Supported (Condition/Status/Reason):
    	*  `Accepted/True/Accepted`
    	*  `Accepted/False/NoMatchingListenerHostname`
        *  `Accepted/False/UnsupportedValue`: Custom reason for when the HTTPRoute includes an invalid or unsupported value.
        *  `Accepted/False/InvalidListener`: Custom reason for when the HTTPRoute references an invalid listener.
        *  `ResolvedRefs/True/ResolvedRefs`
        *  `ResolvedRefs/False/InvalidKind`
        *  `ResolvedRefs/False/RefNotPermitted`
        *  `ResolvedRefs/False/BackendNotFound`
        *  `ResolvedRefs/False/UnsupportedValue`: Custom reason for when one of the HTTPRoute rules has a backendRef with an unsupported value.

### TLSRoute

> Status: Not supported.

### TCPRoute

> Status: Not supported.

### UDPRoute

> Status: Not supported.

### ReferenceGrant

> Status: Not supported.

### Custom Policies

> Status: Not supported.

Custom policies will be NGINX Kubernetes Gateway-specific CRDs that will allow supporting features like timeouts, load-balancing methods, authentication, etc. - important data-plane features that are not part of the Gateway API spec.

While those CRDs are not part of the Gateway API, the mechanism of attaching them to Gateway API resources is part of the Gateway API. See the [Policy Attachment doc](https://gateway-api.sigs.k8s.io/references/policy-attachment/).
