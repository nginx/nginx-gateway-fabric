# GatewayLink with Global Configuration

This example demonstrates how to use a global NginxProxy configuration that applies GatewayLink settings to all Gateways attached to the GatewayClass.

## Overview

Instead of attaching an NginxProxy to each individual Gateway via `infrastructure.parametersRef`, this example configures GatewayLink at the GatewayClass level. Any Gateway that references the `nginx` GatewayClass automatically inherits the GatewayLink configuration without needing a per-Gateway NginxProxy reference.

This approach is useful when you want a consistent GatewayLink configuration across multiple Gateways and namespaces, such as production and test environments sharing the same BIG-IP integration settings.

## How It Works

1. A global NginxProxy resource is configured on the GatewayClass (via Helm or `kubectl edit`)
2. Two separate Gateways are deployed in different namespaces (`gateway-prod` and `gateway-test`)
3. Both Gateways inherit the global GatewayLink, Proxy Protocol, and service configuration
4. Each Gateway gets its own IngressLink resource and BIG-IP virtual server, but shares common settings like iRules, TLS profiles, and multi-cluster configuration

## Files

```tree
gateway-global-config/
├── nginxproxy-global.yaml          # Global NginxProxy with GatewayLink config
├── gateway1/
│   ├── gateway.yaml                # Production Gateway (no parametersRef needed)
│   ├── cafe.yaml                   # Backend coffee/tea deployments and services
│   └── cafe-routes.yaml            # HTTPRoutes for cafe1.example.com
└── gateway2/
    ├── gateway.yaml                # Test Gateway (no parametersRef needed)
    ├── cafe.yaml                   # Backend coffee/tea deployments and services
    └── cafe-routes.yaml            # HTTPRoutes for cafe2.example.com
```

## Prerequisites

- F5 BIG-IP with Container Ingress Services (CIS) installed
- (Optional) F5 IPAM Controller for dynamic IP allocation
- NGINX Gateway Fabric installed with `--bigip-gateway-link` flag enabled

## Setup

### 1. Configure the global NginxProxy

The global NginxProxy can be configured during Helm installation or by editing the existing resource:

```bash
kubectl edit nginxproxies -n nginx-gateway nginx-gateway-proxy-config
```

See [nginxproxy-global.yaml](nginxproxy-global.yaml) for the full configuration, which includes:

- **GatewayLink**: IPAM label, iRules, TLS client/server SSL profiles, multi-cluster settings
- **Proxy Protocol**: Preserves client IPs from BIG-IP
- **Service type**: NodePort for multi-cluster compatibility with CIS
- **Readiness probe**: Exposed for BIG-IP health checking

### 2. Deploy the production Gateway

```bash
kubectl create namespace gateway-prod
kubectl apply -n gateway-prod -f gateway-prod/
```

### 3. Deploy the test Gateway

```bash
kubectl create namespace gateway-test
kubectl apply -n gateway-test -f gateway-test/
```

### 4. Verify

Both Gateways should receive BIG-IP virtual server addresses in their status:

```bash
kubectl get gateways -A
```

## Key Differences from Per-Gateway Configuration

| Aspect                    | Global Config (this example)                   | Per-Gateway Config                                          |
|---------------------------|------------------------------------------------|-------------------------------------------------------------|
| NginxProxy reference      | Attached to GatewayClass                       | Attached to each Gateway via `infrastructure.parametersRef` |
| Gateway manifests         | Simple, no parametersRef needed                | Must reference an NginxProxy                                |
| Configuration consistency | All Gateways share the same settings           | Each Gateway can have different settings                    |
| Use case                  | Uniform BIG-IP integration across environments | Fine-grained control per Gateway                            |
