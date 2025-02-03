---
title: "Resource validation"
weight: 600
toc: true
docs: "DOCS-1414"
---

## Overview

This document describes how NGINX Gateway Fabric validates Gateway API and NGINX Gateway Fabric Kubernetes resources.

## Gateway API resource validation

NGINX Gateway Fabric validates Gateway API resources for several reasons:

- _Robustness_: to gracefully handle invalid resources.
- _Security_: to prevent malicious input from propagating to the NGINX configuration.
- _Correctness_: to conform to the Gateway API specification for handling invalid resources.

The process involves four different steps, explained in detail in this document, with the goal of making sure that NGINX continues to handle traffic even if invalid Gateway API resources were created.

### Step 1 - OpenAPI Scheme validation by Kubernetes API Server

The Kubernetes API server validates Gateway API resources against the OpenAPI schema embedded in the Gateway API CRDs. For example, if you create an HTTPRoute with an invalid hostname "cafe.!@#$%example.com", the API server will reject it with the following error:

```shell
kubectl apply -f coffee-route.yaml
```

```text
The HTTPRoute "coffee" is invalid: spec.hostnames[0]: Invalid value: "cafe.!@#$%example.com": spec.hostnames[0] in body should match '^(\*\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$'
```

{{< note >}}While unlikely, bypassing this validation step is possible if the Gateway API CRDs are modified to remove the validation. If this happens, Step 4 will reject any invalid values (from NGINX perspective).{{< /note >}}

### Step 2 - CEL validation by Kubernetes API Server

The Kubernetes API server validates Gateway API resources using CEL validation embedded in the Gateway API CRDs. It validates Gateway API resources using advanced rules unavailable in the OpenAPI schema validation. For example, if you create a Gateway resource with a TCP listener that configures a hostname, the CEL validation will reject it with the following error:

```shell
kubectl apply -f some-gateway.yaml
```

```text
The Gateway "some-gateway" is invalid: spec.listeners: Invalid value: "array": hostname must not be specified for protocols ['TCP', 'UDP']
```

More information on CEL in Kubernetes can be found [here](https://kubernetes.io/docs/reference/using-api/cel/).


### Step 3 - Validation by NGINX Gateway Fabric

This step catches the following cases of invalid values:

- Valid values from the Gateway API perspective but not supported by NGINX Gateway Fabric yet. For example, a feature in an HTTPRoute routing rule. For the list of supported features see [Gateway API Compatibility]({{< relref "gateway-api-compatibility.md" >}}) doc.
- Valid values from the Gateway API perspective, but invalid for NGINX, because NGINX has stricter validation requirements for certain fields. These values will cause NGINX to fail to reload or operate erroneously.
- Invalid values (both from the Gateway API and NGINX perspectives) that were not rejected because Step 1 was bypassed. Similar to the previous case, these values will cause NGINX to fail to reload or operate erroneously.
- Malicious values that inject unrestricted NGINX config into the NGINX configuration (similar to an SQL injection attack).

Below is an example of how NGINX Gateway Fabric rejects an invalid resource. The validation error is reported via the status:

```shell
kubectl describe httproutes.gateway.networking.k8s.io coffee
```

```text
. . .
Status:
  Parents:
    Conditions:
      Last Transition Time:  2023-03-30T22:37:53Z
      Message:               All rules are invalid: spec.rules[0].matches[0].method: Unsupported value: "CONNECT": supported values: "DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"
      Observed Generation:   1
      Reason:                UnsupportedValue
      Status:                False
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
    Parent Ref:
      Group:         gateway.networking.k8s.io
      Kind:          Gateway
      Name:          prod-gateway
      Namespace:     default
      Section Name:  http
```

{{< note >}}This validation step always runs and cannot be bypassed.{{< /note >}}

### Confirm validation

To confirm that a resource is valid and accepted by NGINX Gateway Fabric, check that the **Accepted** condition in the resource status has the Status field set to **True**. For example, in a status of a valid HTTPRoute, if NGINX Gateway Fabric accepts a parentRef, the status of that parentRef will look like this:

```text
Status:
  Parents:
    Conditions:
      Last Transition Time:  2023-03-30T23:18:00Z
      Message:               The route is accepted
      Observed Generation:   2
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
    Parent Ref:
      Group:         gateway.networking.k8s.io
      Kind:          Gateway
      Name:          gateway
      Namespace:     default
      Section Name:  http
```

{{< note>}}Make sure the reported observed generation is the same as the resource generation.{{< /note >}}

## NGINX Gateway Fabric Resource validation

### Step 1 - OpenAPI Scheme validation by Kubernetes API Server

The Kubernetes API server validates NGINX Gateway Fabric resources against the OpenAPI schema embedded in the NGINX Gateway Fabric CRDs. For example, if you create an NginxGateway with an invalid logging level, "some-level", the API server will reject it with the following error:

```shell
kubectl apply -f nginx-gateway-config.yaml
```

```text
The NginxGateway "nginx-gateway-config" is invalid: spec.logging.level: Unsupported value: "some-level": supported values: "info", "debug", "error"
```

{{< note >}}While unlikely, bypassing this validation step is possible if the NGINX Gateway Fabric CRDs are modified to remove the validation. If this happens, Step 2 will report an error in the resource's status.{{< /note >}}

### Step 2 - Validation by NGINX Gateway Fabric

This step validates the settings in the NGINX Gateway Fabric CRDs and rejects invalid resources. The validation error is reported via the status and as an Event. For example:

```shell
kubectl describe nginxgateways.gateway.nginx.org nginx-gateway-config
```

Status:

```text
...
Status:
  Conditions:
    Last Transition Time:  2023-12-15T21:02:30Z
    Message:               Failed to update control plane configuration: logging.level: Unsupported value: "some-level": supported values: "info", "debug", "error"
    Observed Generation:   1
    Reason:                Invalid
    Status:                False
    Type:                  Valid
```

Event:

```text
Warning  UpdateFailed  1s (x2 over 1s)  nginx-gateway-fabric-nginx  Failed to update control plane configuration: logging.level: Unsupported value: "some-level": supported values: "info", "debug", "error"
```

### Confirm validation

To confirm that a resource is valid and accepted by NGINX Gateway Fabric, check that the **Valid** condition in the resource status has the Status field set to **True**. For example, the status of a valid NginxGateway will look like this:

```text
Status:
  Conditions:
    Last Transition Time:  2023-12-15T21:04:49Z
    Message:               NginxGateway is valid
    Observed Generation:   1
    Reason:                Valid
    Status:                True
    Type:                  Valid
```
