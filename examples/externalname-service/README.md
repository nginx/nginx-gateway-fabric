# ExternalName Services Example

This example demonstrates how to use NGINX Gateway Fabric to route traffic to external services using [Kubernetes ExternalName Services](https://kubernetes.io/docs/concepts/services-networking/service/#externalname). This enables you to proxy traffic to external APIs or services that are not running in your cluster.

## Overview

In this example, we will:

1. Deploy NGINX Gateway Fabric with DNS resolver configuration
2. Create an ExternalName service pointing to an external API (httpbin.org)
3. Deploy an internal service (coffee) to demonstrate mixed routing
4. Configure an HTTPRoute that routes to both external and internal services
5. (Optional) Configure BackendTLSPolicy for HTTPS connections to external services
6. Test HTTP routing with both external and internal services
7. (Optional) Test HTTPS backend connections and TLS passthrough

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

## 3. Deploy Services

Create the ExternalName service and internal service:

```shell
kubectl apply -f cafe.yaml
```

This creates:

- An ExternalName service which points to `httpbin.org` with both HTTP and HTTPS ports
- A coffee deployment and service for internal routing

## 4. Configure Routing

Create the HTTPRoute and TLSRoute that route traffic to the services:

```shell
kubectl apply -f route.yaml
```

This creates:

- An HTTPRoute with two paths:
  - `/external/*` - Routes to the ExternalName service (httpbin.org) with a URLRewrite filter that strips the `/external` prefix
  - `/coffee` - Routes to the internal coffee service
- A TLSRoute for TLS passthrough to the external httpbin service (commented out by default)

## 5. (Optional) Configure HTTPS Backend

If your external service requires HTTPS connections (like `https://httpbin.org`), you need to:

1. Update the HTTPRoute to use port 443 for the httpbin service in `route.yaml`:

   ```yaml
   backendRefs:
   - name: httpbin
     port: 443  # Change from 80 to 443
   ```

2. Create a BackendTLSPolicy:

   ```shell
   kubectl apply -f backendtlspolicy.yaml
   ```

This configures NGINX Gateway Fabric to:

- Establish TLS connections to the backend service
- Verify the backend's TLS certificate using system CA certificates
- Match the certificate's hostname against the external service hostname

**Note**: BackendTLSPolicy is different from TLSRoute:

- **BackendTLSPolicy**: NGF terminates client TLS/HTTP and establishes HTTPS to the backend. Allows HTTP-level routing (paths, headers, etc.)
- **TLSRoute**: TLS passthrough where the client establishes TLS directly with the backend. No HTTP-level routing possible.

## 6. Test the Configuration

Wait for the Gateway to be ready:

```shell
kubectl wait --for=condition=Programmed gateway/external-gateway --timeout=60s
```

Save the public IP address and ports of the NGINX Service into shell variables:

```text
GW_IP=XXX.YYY.ZZZ.III
GW_PORT_HTTP=<port number>
```

Test the ExternalName service (httpbin.org) via HTTP:

```shell
curl --resolve cafe.example.com:$GW_PORT_HTTP:$GW_IP http://cafe.example.com:$GW_PORT_HTTP/external/get
```

You should see a JSON response from httpbin.org. Notice the `Host` header is correctly set to `httpbin.org`:

```json
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Host": "httpbin.org",
    "User-Agent": "curl/8.7.1",
    "X-Amzn-Trace-Id": "Root=1-6901e342-2ce43cf518ee439d4e4d4867",
    "X-Forwarded-Host": "cafe.example.com"
  },
  "origin": "xxx.xxx.xxx.xx",
  "url": "http://cafe.example.com/get"
}
```

Test the internal coffee service:

```shell
curl --resolve cafe.example.com:$GW_PORT_HTTP:$GW_IP http://cafe.example.com:$GW_PORT_HTTP/coffee
```

You should see a response from the coffee service:

```text
Server address: 10.244.0.7:8080
Server name: coffee-<pod-id>
Date: ...
URI: /coffee
Request ID: ...
```

You can also test other httpbin.org endpoints:

```shell
# Test /anything endpoint
curl --resolve cafe.example.com:$GW_PORT_HTTP:$GW_IP http://cafe.example.com:$GW_PORT_HTTP/external/anything

# Test /headers endpoint
curl --resolve cafe.example.com:$GW_PORT_HTTP:$GW_IP http://cafe.example.com:$GW_PORT_HTTP/external/headers
```

### Testing HTTPS Backend (Optional)

If you configured BackendTLSPolicy in step 5, NGF will establish HTTPS connections to the external service. The client connection remains HTTP, but the backend connection uses HTTPS:

```shell
curl --resolve cafe.example.com:$GW_PORT_HTTP:$GW_IP http://cafe.example.com:$GW_PORT_HTTP/external/get
```

The response will show the request was successfully proxied to `https://httpbin.org` with proper TLS verification.

### Testing TLS Passthrough (Optional)

To test TLS passthrough, first uncomment the TLSRoute section in `route.yaml` and reapply:

```shell
kubectl apply -f route.yaml
```

Then save the HTTPS port:

```text
GW_PORT_HTTPS=<port number>
```

Test the httpbin service via TLS passthrough:

```shell
curl -k --resolve httpbin.example.com:$GW_PORT_HTTPS:$GW_IP https://httpbin.example.com:$GW_PORT_HTTPS/get
```

You should see a JSON response from httpbin.org via HTTPS.

## How It Works

This example demonstrates key features for routing to external services:

1. **DNS Resolution**: The NginxProxy resource configures DNS resolvers (8.8.8.8, 1.1.1.1) so NGINX can resolve external hostnames
2. **Host Header Handling**: NGF automatically detects ExternalName services and sets the `Host` header to the external hostname (`httpbin.org`) instead of the Gateway hostname (`cafe.example.com`), ensuring external services receive the correct Host header
3. **URL Rewriting**: The URLRewrite filter strips the `/external` prefix before proxying to httpbin.org, so `/external/get` becomes `/get` on the external service
4. **Mixed Routing**: The same HTTPRoute can route to both ExternalName services and internal Kubernetes services seamlessly
5. **HTTPS Backends**: BackendTLSPolicy enables secure HTTPS connections to external services while allowing HTTP-level routing based on paths, headers, etc.
6. **TLS Passthrough**: The TLSRoute allows direct TLS connections to external services without termination at the Gateway (no HTTP-level routing)

## Cleanup

Remove all resources created in this example:

```shell
kubectl delete -f route.yaml
kubectl delete -f cafe.yaml
kubectl delete -f gateway.yaml
```
