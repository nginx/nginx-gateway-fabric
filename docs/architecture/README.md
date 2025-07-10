# NGINX Gateway Fabric Architecture

This guide explains how NGINX Gateway Fabric works in simple terms.

## What is NGINX Gateway Fabric?

NGINX Gateway Fabric (NGF) turns Kubernetes Gateway API resources into working traffic routing. It has two main parts:

- **Control Plane**: Watches Kubernetes and creates NGINX configs
- **Data Plane**: NGINX servers that handle your traffic

## Control Plane vs Data Plane

### Control Plane

The **Control Plane** is the brain:

- Watches for Gateway and Route changes in Kubernetes
- Converts Gateway API configs into NGINX configs
- Manages NGINX instances
- Handles certificates and security

### Data Plane

The **Data Plane** does the work:

- Receives incoming traffic from users
- Routes traffic to your apps
- Handles SSL/TLS termination
- Applies load balancing and security rules

## Architecture Diagrams

### [Configuration Flow](./configuration-flow.md)

How Gateway API resources become NGINX configurations

### [Traffic Flow](./traffic-flow.md)

How user requests travel through the system

### [Gateway Lifecycle](./gateway-lifecycle.md)

What happens when you create or update a Gateway

For more detailed architectural information, see the [Gateway Architecture Overview](https://docs.nginx.com/nginx-gateway-fabric/overview/gateway-architecture/).

## Key Concepts

### Separation of Concerns

- Control and data planes run in **separate pods**
- They communicate over **secure gRPC**
- Each can **scale independently**
- **Different security permissions** for each

### Gateway API Integration

NGF implements Kubernetes Gateway API resources. For supported resources and their feature compatibility, see [Gateway API Compatibility](https://docs.nginx.com/nginx-gateway-fabric/overview/gateway-api-compatibility/).

### NGINX Agent

- **NGINX Agent v3** connects control and data planes
- Runs inside each NGINX pod
- Downloads configs from control plane
- Manages NGINX lifecycle (start, reload, monitor)

## Security Model

### Control Plane Security

- **Limited Kubernetes API access** (RBAC-controlled permissions to watch resources)
- **gRPC server** for data plane connections
- **Certificate management** for secure communication

### Data Plane Security

- **No Kubernetes API access** (security isolation)
- **gRPC client** connects to control plane
- **Minimal permissions** (principle of least privilege)
