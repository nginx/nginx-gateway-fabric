---
title: "Gateway API Compatibility"
description: "Learn which Gateway API resources NGINX Gateway Fabric supports and the extent of that support."
weight: 700
toc: true
docs: "DOCS-000"
---

## Summary

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| [Gateway](#gateway)                 | Supported          | Not supported          | Not supported                         | v1          |
| [GatewayClass](#gatewayclass)       | Supported          | Not supported          | Not supported                         | v1          |
| [HTTPRoute](#httproute)             | Supported          | Partially supported    | Not supported                         | v1          |
| [ReferenceGrant](#referencegrant)   | Supported          | N/A                    | Not supported                         | v1beta1     |
| [TLSRoute](#tlsroute)               | Not supported      | Not supported          | Not supported                         | N/A         |
| [TCPRoute](#tcproute)               | Not supported      | Not supported          | Not supported                         | N/A         |
| [UDPRoute](#udproute)               | Not supported      | Not supported          | Not supported                         | N/A         |
| [Custom policies](#custom-policies) | Not supported      | N/A                    | Not supported                         | N/A         |
{{< /bootstrap-table >}}

---

## Terminology

Gateway API features has three [support levels](https://gateway-api.sigs.k8s.io/concepts/conformance/#2-support-levels): Core, Extended and Implementation-specific. We use the following terms to describe the support status for each level and resource field:

- _Supported_. The resource or field is fully supported.
- _Partially supported_. The resource or field is supported partially, with limitations. It will become fully
  supported in future releases.
- _Not supported_. The resource or field is not yet supported. It will become partially or fully supported in future
  releases.

{{< note >}} It's possible that NGINX Gateway Fabric will never support some resources or fields of the Gateway API. They will be documented on a case by case basis. NGINX Gateway Fabric doesn't support any features from the experimental release channel. {{< /note >}}

---

## Resources

Each resource below includes the support status of their corresponding fields.

For a description of each field, visit the [Gateway API documentation](https://gateway-api.sigs.k8s.io/references/spec/).

- `spec`
  - `controllerName` - supported.
  - `parametersRef` - not supported.
  - `description` - supported.
- `status`
  - `conditions` - supported (Condition/Status/Reason):
    - `Accepted/True/Accepted`
    - `Accepted/False/InvalidParameters`
    - `Accepted/False/UnsupportedVersion`
    - `Accepted/False/GatewayClassConflict`: Custom reason for when the GatewayClass references this controller, but
          a different GatewayClass name is provided to the controller via the command-line argument.
    - `SupportedVersion/True/SupportedVersion`
    - `SupportedVersion/False/UnsupportedVersion`

---

### Gateway

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| Gateway              | Supported          | Not supported          | Not supported                         | v1          |
{{< /bootstrap-table >}}

NGINX Gateway Fabric supports a single Gateway resource. The Gateway resource must reference NGINX Gateway Fabric's corresponding GatewayClass.

See the [static-mode]({{< relref "/reference/cli-help.md#static-mode">}}) command for more information.

**Fields**:

- `spec`
  - `gatewayClassName`: Supported.
  - `listeners`
    - `name`: Supported.
    - `hostname`: Supported.
    - `port`: Supported.
    - `protocol`: Partially supported. Allowed values: `HTTP`, `HTTPS`.
    - `tls`
      - `mode`: Partially supported. Allowed value: `Terminate`.
      - `certificateRefs` - The TLS certificate and key must be stored in a Secret resource of type `kubernetes.io/tls`. Only a single reference is supported.
      - `options`: Not supported.
    - `allowedRoutes`: Supported.
  - `addresses`: Not supported.
- `status`
  - `addresses`: Partially supported (LoadBalancer and Pod IP).
  - `conditions`: Supported (Condition/Status/Reason):
    - `Accepted/True/Accepted`
    - `Accepted/True/ListenersNotValid`
    - `Accepted/False/ListenersNotValid`
    - `Accepted/False/Invalid`
    - `Accepted/False/UnsupportedValue`: Custom reason for when a value of a field in a Gateway is invalid or not supported.
    - `Accepted/False/GatewayConflict`: Custom reason for when the Gateway is ignored due to a conflicting Gateway.
          NGF only supports a single Gateway.
    - `Programmed/True/Programmed`
    - `Programmed/False/Invalid`
    - `Programmed/False/GatewayConflict`: Custom reason for when the Gateway is ignored due to a conflicting Gateway. NGF only supports a single Gateway.
  - `listeners`
    - `name`: Supported.
    - `supportedKinds`: Supported.
    - `attachedRoutes`: Supported.
    - `conditions`: Supported (Condition/Status/Reason):
      - `Accepted/True/Accepted`
      - `Accepted/False/UnsupportedProtocol`
      - `Accepted/False/InvalidCertificateRef`
      - `Accepted/False/ProtocolConflict`
      - `Accepted/False/UnsupportedValue`: Custom reason for when a value of a field in a Listener is invalid or not supported.
      - `Accepted/False/GatewayConflict`: Custom reason for when the Gateway is ignored due to a conflicting Gateway. NGF only supports a single Gateway.
      - `Programmed/True/Programmed`
      - `Programmed/False/Invalid`
      - `ResolvedRefs/True/ResolvedRefs`
      - `ResolvedRefs/False/InvalidCertificateRef`
      - `ResolvedRefs/False/InvalidRouteKinds`
      - `Conflicted/True/ProtocolConflict`
      - `Conflicted/False/NoConflicts`

---

### GatewayClass

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| GatewayClass   | Supported          | Not supported       | Not supported          | v1          |
{{< /bootstrap-table >}}

NGINX Gateway Fabric supports a single GatewayClass resource configured with the `--gatewayclass` flag of the [static-mode]({{< relref "/reference/cli-help.md#static-mode">}}) command.

**Fields**:

- `spec`
  - `controllerName`: Supported.
  - `parametersRef`: Not supported.
  - `description`: Supported.
- `status`
  - `conditions` - Supported (Condition/Status/Reason):
    - `Accepted/True/Accepted`
    - `Accepted/False/InvalidParameters`
    - `Accepted/False/GatewayClassConflict`: Custom status for when GatewayClass references this controller, but a different GatewayClass name is provided to the controller via the command-line argument.

---

### HTTPRoute

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| HTTPRoute             | Supported          | Partially supported    | Not supported                         | v1          |
{{< /bootstrap-table >}}

**Fields**:

- `spec`
  - `parentRefs`: Partially supported. Port not supported.
  - `hostnames`: Supported.
  - `rules`
    - `matches`
      - `path`: Partially supported. Only `PathPrefix` and `Exact` types.
      - `headers`: Partially supported. Only `Exact` type.
      - `queryParams`: Partially supported. Only `Exact` type.
      - `method`: Supported.
    - `filters`
      - `type`: Supported.
      - `requestRedirect`: Supported except for the experimental `path` field. If multiple filters are configured, NGINX Gateway Fabric will choose the first and ignore the rest.
      - `requestHeaderModifier`: Supported. If multiple filters are configured, NGINX Gateway Fabric will choose the first and ignore the rest.
      - `responseHeaderModifier`, `requestMirror`, `urlRewrite`, `extensionRef`: Not supported.
    - `backendRefs`: Partially supported. Backend ref `filters` are not supported.
- `status`
  - `parents`
    - `parentRef`: Supported.
    - `controllerName`: Supported.
    - `conditions`: Partially supported. Supported (Condition/Status/Reason):
      - `Accepted/True/Accepted`
      - `Accepted/False/NoMatchingListenerHostname`
      - `Accepted/False/NoMatchingParent`
      - `Accepted/False/NotAllowedByListeners`
      - `Accepted/False/UnsupportedValue`: Custom reason for when the HTTPRoute includes an invalid or unsupported value.
      - `Accepted/False/InvalidListener`: Custom reason for when the HTTPRoute references an invalid listener.
      - `Accepted/False/GatewayNotProgrammed`: Custom reason for when the Gateway is not Programmed. HTTPRoute can be valid and configured, but will maintain this status as long as the Gateway is not Programmed.
      - `ResolvedRefs/True/ResolvedRefs`
      - `ResolvedRefs/False/InvalidKind`
      - `ResolvedRefs/False/RefNotPermitted`
      - `ResolvedRefs/False/BackendNotFound`
      - `ResolvedRefs/False/UnsupportedValue`: Custom reason for when one of the HTTPRoute rules has a backendRef with an unsupported value.
      - `PartiallyInvalid/True/UnsupportedValue`

---

### ReferenceGrant

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| ReferenceGrant  | Supported          | N/A                    | Not supported                         | v1beta1     |
{{< /bootstrap-table >}}

Fields:

- `spec`
  - `to`
    - `group` - supported.
    - `kind` - supports `Secret` and `Service`.
    - `name`- supported.
  - `from`
    - `group` - supported.
    - `kind` - supports `Gateway` and `HTTPRoute`.
    - `namespace`- supported.

---

### TLSRoute

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| TLSRoute             | Not supported      | Not supported          | Not supported                         | N/A         |
{{< /bootstrap-table >}}

---

### TCPRoute

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| TCPRoute              | Not supported      | Not supported          | Not supported                         | N/A         |
{{< /bootstrap-table >}}

---

### UDPRoute

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| UDPRoute            | Not supported      | Not supported          | Not supported                         | N/A         |
{{< /bootstrap-table >}}

---

### Custom Policies

{{< bootstrap-table "table table-striped table-bordered" >}}
| Resource                            | Core Support Level | Extended Support Level | Implementation-Specific Support Level | API Version |
|-------------------------------------|--------------------|------------------------|---------------------------------------|-------------|
| Custom policies | Not supported      | N/A                    | Not supported                         | N/A         |
{{< /bootstrap-table >}}

Custom policies will be NGINX Gateway Fabric-specific CRDs (Custom Resource Definitions) that will support features such as timeouts, load-balancing methods, authentication, etc. These important data-plane features are not part of the Gateway API specifications.

While these CRDs are not part of the Gateway API, the mechanism to attach them to Gateway API resources is part of the Gateway API. See the [Policy Attachment documentation](https://gateway-api.sigs.k8s.io/references/policy-attachment/).
