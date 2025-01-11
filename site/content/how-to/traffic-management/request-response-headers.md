---
title: "Modify HTTP request and response headers"
toc: true
weight: 600
type: how-to
product: NGF
docs: DOCS-000
---

Learn how to modify the request and response headers of your application using NGINX Gateway Fabric.

## Overview

[HTTP Header Modifiers](https://gateway-api.sigs.k8s.io/guides/http-header-modifier/?h=request#http-header-modifiers) can be used to add, modify or remove headers during the request-response lifecycle. The [RequestHeaderModifier](https://gateway-api.sigs.k8s.io/guides/http-header-modifier/#http-request-header-modifier) is used to alter headers in a request sent by client and [ResponseHeaderModifier](https://gateway-api.sigs.k8s.io/guides/http-header-modifier/#http-response-header-modifier) is used to alter headers in a response to the client.

This guide describes how to configure the headers application to modify the headers in the request. Another version of the headers application is then used to modify response headers when client requests are made. For an introduction to exposing your application, we recommend that you follow the [basic guide]({{< relref "/how-to/traffic-management/routing-traffic-to-your-app.md" >}}) first.

---

## Before you begin

- [Install]({{< relref "/installation/" >}}) NGINX Gateway Fabric.
- Save the public IP address and port of NGINX Gateway Fabric into shell variables:

   ```text
   GW_IP=XXX.YYY.ZZZ.III
   GW_PORT=<port number>
   ```

{{< note >}} In a production environment, you should have a DNS record for the external IP address that is exposed, and it should refer to the hostname that the gateway will forward for .{{< /note >}}

---

## HTTP Header Modifiers examples

We will configure a common gateway for the `RequestHeaderModifier` and `ResponseHeaderModifier` examples mentioned below.

---

### Deploy the Gateway API resources for the Header application

The [Gateway](https://gateway-api.sigs.k8s.io/api-types/gateway/) resource is typically deployed by the [Cluster Operator](https://gateway-api.sigs.k8s.io/concepts/roles-and-personas/#roles-and-personas_1). This Gateway defines a single listener on port 80. Since no hostname is specified, this listener matches on all hostnames. To deploy the Gateway:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
spec:
  gatewayClassName: nginx
  listeners:
  - name: http
    port: 80
    protocol: HTTP
EOF
```

---

## RequestHeaderModifier example

This examples demonstrates how to configure traffic routing for a simple echo server. A HTTPRoute resource is used to route traffic to the headers application, using the `RequestHeaderModifier` filter to modify headers in the request. You can then verify that the server responds with the modified request headers.

### Deploy the Headers application

Begin by deploying the example application `headers`. It is a simple application that returns the request headers which will be modified later.

```shell
kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.5.1/examples/http-request-header-filter/headers.yaml
```

This will create the headers Service and a Deployment with one Pod. Run the following command to verify the resources were created:

```shell
kubectl get pods,svc
```

```text
pod/headers-545698447b-z52kj   1/1     Running   0          23s

NAME                 TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE
service/headers      ClusterIP   10.96.26.161   <none>        80/TCP    23s
```

---

### Configure the HTTPRoute with RequestHeaderModifier filter

Create a HTTPRoute that exposes the header application outside the cluster using the listener created in the previous section. Use the following command:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: headers
spec:
  parentRefs:
  - name: gateway
    sectionName: http
  hostnames:
  - "echo.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /headers
    filters:
    - type: RequestHeaderModifier
      requestHeaderModifier:
        set:
        - name: My-Overwrite-Header
          value: this-is-the-only-value
        add:
        - name: Accept-Encoding
          value: compress
        - name: My-Cool-header
          value: this-is-an-appended-value
        remove:
        - User-Agent
    backendRefs:
    - name: headers
      port: 80
EOF
```

This HTTPRoute has a few important properties:

- The `parentRefs` references the Gateway resource that we created, and specifically defines the `http` listener to attach to, via the `sectionName` field.
- `echo.example.com` is the hostname that is matched for all requests to the backends defined in this HTTPRoute.
- The `match` rule defines that all requests with the path prefix `/headers` are sent to the `headers` Service.
- It has a `RequestHeaderModifier` filter defined for the path prefix `/headers`. This filter:

    1. Sets the value for header `My-Overwrite-Header` to `this-is-the-only-value`.
    1. Appends the value `compress` to the `Accept-Encoding` header and `this-is-an-appended-value` to the `My-Cool-header`.
    1. Removes `User-Agent` header.

---

### Send traffic to the Headers application

To access the application, use `curl` to send requests to the `headers` Service, which includes headers within the request.

```shell
curl -s --resolve echo.example.com:$GW_PORT:$GW_IP http://echo.example.com:$GW_PORT/headers -H "My-Cool-Header:my-client-value" -H "My-Overwrite-Header:dont-see-this"
```

```text
  Headers:
  header 'Accept-Encoding' is 'compress'
  header 'My-Cool-header' is 'my-client-value,this-is-an-appended-value'
  header 'My-Overwrite-Header' is 'this-is-the-only-value'
  header 'Host' is 'echo.example.com:8080'
  header 'X-Forwarded-For' is '127.0.0.1'
  header 'Connection' is 'close'
  header 'X-Real-IP' is '127.0.0.1'
  header 'X-Forwarded-Proto' is 'http'
  header 'X-Forwarded-Host' is 'echo.example.com'
  header 'X-Forwarded-Port' is '80'
  header 'Accept' is '*/*'
```

In the output above, you can see that the headers application modifies the following custom headers:

- `User-Agent` header is absent.
- The header `My-Cool-header` gets appended with the new value `my-client-value`.
- The header `My-Overwrite-Header` gets overwritten from `dont-see-this` to `this-is-the-only-value`.
- The header `Accept-encoding` remains unchanged as we did not modify it in the curl request sent.

---

### Delete the resources

Delete the headers application and HTTPRoute: another instance will be used for the next examples.

```shell
kubectl delete httproutes.gateway.networking.k8s.io headers
```

```shell
kubectl delete -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.5.1/examples/http-request-header-filter/headers.yaml
```

## ResponseHeaderModifier example

Begin by configuring an application with custom headers and a simple HTTPRoute. The server response can be observed see its headers. The next step is to modify some of the headers using HTTPRoute filters to modify responses. Finally, verify the server responds with the modified headers.

### Deploy the Headers application

Begin by deploying the example application `headers`. It is a simple application that adds response headers that will be modified later.

```shell
kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.5.1/examples/http-response-header-filter/headers.yaml
```

This will create the headers Service and a Deployment with one Pod. Run the following command to verify the resources were created:

```shell
kubectl get pods,svc
```

```text
NAME                          READY   STATUS    RESTARTS   AGE
pod/headers-6f854c478-hd2jr   1/1     Running   0          95s

NAME                 TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
service/headers      ClusterIP   10.96.15.12   <none>        80/TCP    95s
```

### Configure the basic HTTPRoute

Create a HTTPRoute that exposes the header application outside the cluster using the listener created in the previous section. You can do this with the following command:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: headers
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
        value: /headers
    backendRefs:
    - name: headers
      port: 80
EOF
```

This HTTPRoute has a few important properties:

- The `parentRefs` references the Gateway resource that we created, and specifically defines the `http` listener to attach to, via the `sectionName` field.
- `cafe.example.com` is the hostname that is matched for all requests to the backends defined in this HTTPRoute.
- The `match` rule defines that all requests with the path prefix `/headers` are sent to the `headers` Service.

### Send traffic to the Headers application

Use `curl` with the `-i` flag to access the application and include the response headers in the output:

```shell
curl -i --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/headers
```

```text
HTTP/1.1 200 OK
Server: nginx/1.25.5
Date: Mon, 06 May 2024 19:08:39 GMT
Content-Type: text/plain
Content-Length: 2
Connection: keep-alive
X-Header-Unmodified: unmodified
X-Header-Add: add-to
X-Header-Set: overwrite
X-Header-Remove: remove

ok
```

In the output above, you can see that the headers application adds the following custom headers to the response:

- X-Header-Unmodified: unmodified
- X-Header-Add: add-to
- X-Header-Set: overwrite
- X-Header-Remove: remove

The next section will modify these headers by adding a ResponseHeaderModifier filter to the headers HTTPRoute.

### Update the HTTPRoute to modify the Response headers

Update the HTTPRoute by adding a `ResponseHeaderModifier` filter:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: headers
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
        value: /headers
    filters:
    - type: ResponseHeaderModifier
      responseHeaderModifier:
        set:
        - name: X-Header-Set
          value: overwritten-value
        add:
        - name: X-Header-Add
          value: this-is-the-appended-value
        remove:
        - X-Header-Remove
    backendRefs:
    - name: headers
      port: 80
EOF
```

Notice that this HTTPRoute has a `ResponseHeaderModifier` filter defined for the path prefix `/headers`. This filter:

- Sets the value for the header `X-Header-Set` to `overwritten-value`.
- Adds the value `this-is-the-appended-value` to the header `X-Header-Add`.
- Removes `X-Header-Remove` header.


### Send traffic to the modified Headers application

Send a curl request to the modified `headers` application to verify the response headers are modified.

```shell
curl -i --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/headers
```

```text
HTTP/1.1 200 OK
Server: nginx/1.25.5
Date: Mon, 06 May 2024 17:58:33 GMT
Content-Type: text/plain
Content-Length: 2
Connection: keep-alive
X-Header-Unmodified: unmodified
X-Header-Add: add-to
X-Header-Add: this-is-the-appended-value
X-Header-Set: overwritten-value

ok
```

In the output above you can notice the modified response headers as the `X-Header-Unmodified` remains unchanged as we did not include it in the filter and `X-Header-Remove` header is absent. The header `X-Header-Add` gets appended with the new value and `X-Header-Set` gets overwritten to `overwritten-value` as defined in the *HttpRoute*.

---

## Further reading

To learn more about the Gateway API and the resources we created in this guide, check out the following Kubernetes documentation resources:

- [Gateway API Overview](https://gateway-api.sigs.k8s.io/concepts/api-overview/)
- [Deploying a simple Gateway](https://gateway-api.sigs.k8s.io/guides/simple-gateway/)
- [HTTP Routing](https://gateway-api.sigs.k8s.io/guides/http-routing/)
