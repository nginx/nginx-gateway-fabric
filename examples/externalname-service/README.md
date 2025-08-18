# ExternalName Services Example

This example demonstrates how to use NGINX Gateway Fabric to route traffic to external services using [Kubernetes ExternalName Services](https://kubernetes.io/docs/concepts/services-networking/service/#externalname). This enables you to proxy traffic to external APIs or services that are not running in your cluster.

## Overview

In this example, we will:

1. Deploy NGINX Gateway Fabric with DNS resolver configuration
2. Create an ExternalName service pointing to an external API
3. Configure HTTP and TLS routes to route traffic to this external service
4. Test both HTTP and TLS routing functionality

## Running the Example

## 1. Deploy NGINX Gateway Fabric

1. Follow the [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/install/) to deploy NGINX Gateway Fabric.

## 2. Deploy the Gateway with DNS Resolver

Create the Gateway and NginxProxy configuration that enables DNS resolution for ExternalName services:

```shell
kubectl apply -f gateway.yaml
```

This creates:

- A Gateway with HTTP and TLS listeners
- An NginxProxy resource with DNS resolver configuration

## 3. Deploy ExternalName Services

Create the ExternalName service that points to external APIs:

```shell
kubectl apply -f external-service.yaml
```

This creates an ExternalName service which points to `httpbin.org` with both HTTP and HTTPS ports

## 4. Configure Routing

Create the HTTPRoute and TLSRoute that route traffic to the external service:

```shell
kubectl apply -f route.yaml
```

This creates:

- An HTTPRoute that routes HTTP traffic to the httpbin service
- A TLSRoute that routes TLS traffic to the httpbin service

## 5. Test the Configuration

Wait for the Gateway to be ready:

```shell
kubectl wait --for=condition=Programmed gateway/external-gateway --timeout=60s
```

Save the public IP address and ports of the NGINX Service into shell variables:

```text
GW_IP=XXX.YYY.ZZZ.III
GW_PORT_HTTP=<port number>
GW_PORT_HTTPS=<port number>
```

Test the httpbin service via HTTP:

```shell
curl --resolve httpbin.example.com:$GW_PORT_HTTP:$GW_IP http://httpbin.example.com:$GW_PORT_HTTP/get
```

Test the httpbin service via TLS passthrough:

```shell
curl -k --resolve httpbin.example.com:$GW_PORT_HTTPS:$GW_IP https://httpbin.example.com:$GW_PORT_HTTPS/get
```

You should see a JSON response from httpbin.org showing request details e.g.

```json
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Host": "httpbin.example.com",
    "User-Agent": "curl/8.7.1",
    "X-Amzn-Trace-Id": "Root=1-68a49086-1e1dabb51155e05c1ebc1f63"
  },
  "origin": "xxx.xxx.xxx.xx",
  "url": "https://httpbin.example.com/get"
}
```

## Cleanup

Remove all resources created in this example:

```shell
kubectl delete -f route.yaml
kubectl delete -f external-service.yaml
kubectl delete -f gateway.yaml
```
