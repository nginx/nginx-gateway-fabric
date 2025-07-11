# NGINX Gateway Fabric Permissions

This document outlines the permissions required by NGINX Gateway Fabric components.

## Overview

NGINX Gateway Fabric uses a **split-plane architecture** where control and data functions are separated for security and scalability:

### Architecture Split

- **Control Plane**: A single deployment that watches Kubernetes APIs, validates configurations, and manages NGINX data plane deployments. Requires broad Kubernetes API access but handles no user traffic.

- **Data Plane**: Multiple NGINX pods (created by the control plane) that process actual traffic. Requires minimal permissions since they only serve traffic and receive configuration from the control plane via secure gRPC.

- **Certificate Generator**: A one-time job that creates TLS certificates for secure communication between planes.

### Why Different Permissions?

This split requires different security contexts because:

- **Control plane** needs Kubernetes API access to manage data plane deployments but never handles user traffic
- **Data plane** processes user traffic but never accesses Kubernetes APIs directly
- **Secure communication**: Control plane sends NGINX configuration to data plane via gRPC over mTLS (port 8443)
- **Isolated deployment**: Each plane runs in separate pods with independent security contexts

## Control Plane

The control plane runs as a single container in the `nginx-gateway` deployment.

### Security Context

- **User ID**: 101 (non-root)
- **Group ID**: 1001
- **Capabilities**: All dropped (`drop: ALL`) - no capabilities are added
- **Privilege Escalation**: Disabled (may need to be enabled in some environments for NGINX reload)
- **Root Filesystem**: Read-only
- **Seccomp**: Runtime default profile

### Required Volumes

| Volume | Type | Purpose |
|--------|------|---------|
| `nginx-agent-tls` | Secret | TLS certificates for communicating with NGINX Agent |

### RBAC Permissions

The control plane requires these Kubernetes API permissions:

- **Secrets, ConfigMaps, Services**: Create, update, delete, list, get, watch
- **Deployments, DaemonSets**: Create, update, delete, list, get, watch
- **ServiceAccounts**: Create, update, delete, list, get, watch
- **Namespaces, Pods**: Get, list, watch
- **Events**: Create, patch
- **EndpointSlices**: List, watch
- **Gateway API resources**: List, watch (read-only) + update status subresources only
- **NGF Custom resources**: Get, list, watch (read-only) + update status subresources only
- **Leases**: Create, get, update (for leader election)
- **CustomResourceDefinitions**: List, watch
- **TokenReviews**: Create (for authentication)

## Data Plane

The data plane consists of NGINX containers dynamically created and managed by the control plane. These containers focus solely on traffic processing and require **fewer privileges** than the control plane since they:

- Don't access Kubernetes APIs (no RBAC needed)
- Receive configuration via secure gRPC from control plane
- Only need to bind to network ports and write to specific directories

### Security Context

- **User ID**: 101 (non-root)
- **Group ID**: 1001
- **Capabilities**: All dropped (`drop: ALL`)
- **Privilege Escalation**: Disabled
- **Root Filesystem**: Read-only
- **Seccomp**: Runtime default profile
- **Sysctl**: `net.ipv4.ip_unprivileged_port_start=0` (allows binding to ports < 1024)

### Required Volumes

| Volume | Type | Purpose |
|--------|------|---------|
| `nginx-agent` | EmptyDir | NGINX Agent configuration |
| `nginx-agent-tls` | Secret | TLS certificates for control plane communication |
| `token` | Projected | Service account token |
| `nginx-agent-log` | EmptyDir | NGINX Agent logs |
| `nginx-agent-lib` | EmptyDir | NGINX Agent runtime data |
| `nginx-conf` | EmptyDir | Main NGINX configuration |
| `nginx-stream-conf` | EmptyDir | Stream configuration |
| `nginx-main-includes` | EmptyDir | Main context includes |
| `nginx-secrets` | EmptyDir | TLS secrets for NGINX |
| `nginx-run` | EmptyDir | Runtime files (PID, sockets) |
| `nginx-cache` | EmptyDir | NGINX cache directory |
| `nginx-includes` | EmptyDir | HTTP context includes |

### Volume Mounts

All volumes are mounted with appropriate read/write permissions. Critical runtime directories like `/var/run/nginx` and `/var/cache/nginx` use ephemeral storage.

## Certificate Generator

Runs as a Kubernetes Job to create initial TLS certificates.

### Security Context

- **User ID**: 101 (non-root)
- **Group ID**: 1001
- **Capabilities**: All dropped (`drop: ALL`)
- **Root Filesystem**: Read-only
- **Seccomp**: Runtime default profile

### RBAC Permissions

Limited to secret management in the NGINX Gateway Fabric namespace:

- **Secrets**: Create, update, get

## OpenShift Compatibility

NGINX Gateway Fabric includes Security Context Constraints (SCCs) for OpenShift:

### Control Plane SCC

- **Privilege Escalation**: Disabled
- **Host Access**: Disabled (network, IPC, PID, ports)
- **User ID Range**: 101-101 (fixed)
- **Group ID Range**: 1001-1001 (fixed)
- **Volumes**: Secret only

### Data Plane SCC

Same restrictions as control plane, plus additional volume types:

- **Additional Volumes**: EmptyDir, ConfigMap, Projected

## Linux Capabilities

**Current Requirements**: NGINX Gateway Fabric drops ALL Linux capabilities and adds none. This follows security best practices for containerized applications.

**Why No Capabilities Needed**:

- **NGINX Process Management**: Uses standard Unix signals that don't require elevated capabilities
- **Port Binding**: Uses sysctl `net.ipv4.ip_unprivileged_port_start=0` to bind to ports < 1024 without privileges
- **File Operations**: All necessary files are writable via volume mounts

**Historical Context**: Earlier versions or specific environments may have required the KILL capability for process management. The current implementation has been redesigned to eliminate this requirement through:

- Proper signal handling without elevated privileges
- NGINX Agent managing process lifecycle
- Secure communication between control and data planes

**Troubleshooting**: If you encounter "operation not permitted" errors during NGINX reload, you may need to temporarily enable `allowPrivilegeEscalation: true` in your environment while investigating the root cause.

## Security Features

The split-plane architecture enables a defense-in-depth security model:

- **Separation of concerns**: Control plane has API access but no traffic exposure; data plane has traffic exposure but no API access
- **Non-root execution**: All components run as unprivileged user (UID 101)
- **Zero capabilities**: All Linux capabilities dropped, none added
- **Read-only root**: Prevents runtime modifications to container filesystems
- **Ephemeral storage**: Writable data uses temporary volumes, not persistent storage
- **Least privilege RBAC**: Control plane gets only required Kubernetes permissions; data plane needs no RBAC
- **Secure inter-plane communication**: mTLS-encrypted gRPC (TLS 1.3+) between control and data planes
