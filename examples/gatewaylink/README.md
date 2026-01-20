# GatewayLink Integration with F5 BIG-IP

This example demonstrates how to integrate NGINX Gateway Fabric with F5 BIG-IP using the GatewayLink functionality in NGINX Gateway Fabric.

## Overview

GatewayLink allows F5 BIG-IP to act as an external load balancer in front of NGINX Gateway Fabric. When enabled:

1. NGINX Gateway Fabric creates an IngressLink resource for each Gateway
2. F5 CIS configures BIG-IP to route traffic to the NGINX Gateway Service
3. The Gateway's status.addresses is populated with the BIG-IP virtual server IP

## Prerequisites

- F5 BIG-IP with Container Ingress Services (CIS) installed
- (Optional) F5 IPAM Controller for dynamic IP allocation
- NGINX Gateway Fabric with `--bigip-gateway-link` flag enabled

## Configuration Options

### Option 1: Manual IP Address

Specify a static IP address on BIG-IP. See [nginxproxy-manual-ip.yaml](nginxproxy-manual-ip.yaml) for a complete example:

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: nginx-proxy-gatewaylink
spec:
  gatewayLink:
    enabled: true
    virtualServerAddress: "10.8.3.101"
    host: cafe.example.com
    iRules:
      - "/Common/Proxy_Protocol_iRule"
```

### Option 2: IPAM Integration

Use F5 IPAM Controller for dynamic IP allocation. See [nginxproxy-ipam.yaml](nginxproxy-ipam.yaml) for a complete example:

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: nginx-proxy-gatewaylink-ipam
spec:
  gatewayLink:
    enabled: true
    ipamLabel: "production"
    partition: "Common"
```

## Proxy Protocol Configuration

When BIG-IP forwards traffic to NGINX, client IP information is preserved via Proxy Protocol. Configure NGINX Gateway Fabric to accept Proxy Protocol:

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: nginx-proxy-gatewaylink
spec:
  gatewayLink:
    enabled: true
    virtualServerAddress: "10.8.3.101"
  rewriteClientIP:
    mode: ProxyProtocol
    trustedAddresses:
      - type: CIDR
        value: "10.8.0.0/14"  # BIG-IP subnet
```

## Service Configuration

When using GatewayLink, configure the Gateway service as ClusterIP since BIG-IP handles external traffic routing. You can also expose a health check port for BIG-IP:

```yaml
spec:
  kubernetes:
    service:
      type: ClusterIP
      patches:
        - type: JSONPatch
          value:
            - op: add
              path: /spec/ports/-
              value:
                name: health
                port: 8081
                targetPort: 8081
    deployment:
      container:
        readinessProbe:
          path: "/nginx-ready"
          port: 8081
```

## Helm Installation

Enable GatewayLink via Helm:

```bash
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric \
  --set nginxGateway.gatewayLink.enable=true
```

## How It Works

1. When a Gateway is created, NGF provisions an IngressLink CR in the same namespace
2. F5 CIS watches for IngressLink resources and configures BIG-IP accordingly
3. If using IPAM, F5 IPAM Controller allocates an IP and updates IngressLink status
4. NGF watches for IngressLink status changes and updates the Gateway's status.addresses

## Example Files

- [nginxproxy-manual-ip.yaml](nginxproxy-manual-ip.yaml) - NginxProxy with static IP and full configuration
- [nginxproxy-ipam.yaml](nginxproxy-ipam.yaml) - NginxProxy with IPAM integration
- [gateway.yaml](gateway.yaml) - Example Gateway referencing the NginxProxy
- [cafe.yaml](cafe.yaml) - Sample backend application (coffee and tea services)
- [cafe-routes.yaml](cafe-routes.yaml) - HTTPRoute definitions for the cafe application
