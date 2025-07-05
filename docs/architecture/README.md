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

## Key Concepts

### Separation of Concerns

- Control and data planes run in **separate pods**
- They communicate over **secure gRPC**
- Each can **scale independently**
- **Different security permissions** for each

### Gateway API Integration

NGF implements these Kubernetes Gateway API resources:

- **Gateway**: Defines entry points for traffic
- **HTTPRoute**: Defines HTTP routing rules
- **GRPCRoute**: Defines gRPC routing rules
- **TLSRoute**: Defines TLS routing rules

### NGINX Agent

- **NGINX Agent v3** connects control and data planes
- Runs inside each NGINX pod
- Downloads configs from control plane
- Manages NGINX lifecycle (start, reload, monitor)

## Security Model

### Control Plane Security

- **Full Kubernetes API access** (to watch resources)
- **gRPC server** for data plane connections
- **Certificate management** for secure communication

### Data Plane Security

- **No Kubernetes API access** (security isolation)
- **gRPC client** connects to control plane
- **Minimal permissions** (principle of least privilege)
