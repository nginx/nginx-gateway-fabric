# Architecture

NGINX Gateway Fabric (NGF) follows a control plane / data plane separation. The control plane watches Kubernetes resources, builds a validated resource graph, generates NGINX configuration, and pushes it to data plane pods over mTLS gRPC. The data plane runs NGINX with an agent that applies configuration and handles traffic.

## High-Level Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                    Control Plane Pod                          │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐ │
│  │  Controller   │  │  gRPC Server │  │    Provisioner     │ │
│  │  Manager      │  │  (mTLS:8443) │  │                    │ │
│  │              │  │              │  │  Creates per-Gateway│ │
│  │  Watches K8s  │  │  Sends config│  │  Deployments,      │ │
│  │  resources    │  │  to agents   │  │  Services, etc.    │ │
│  └──────┬───────┘  └──────┬───────┘  └────────┬───────────┘ │
│         │                 │                    │             │
│         ▼                 │                    │             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                  Event Handler                        │   │
│  │  1. Batch events from controllers                     │   │
│  │  2. Build validated resource graph                    │   │
│  │  3. Generate NGINX config via templates               │   │
│  │  4. Broadcast to connected agents                     │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐                         │
│  │ Health :8081  │  │ Metrics:9113 │                         │
│  └──────────────┘  └──────────────┘                         │
└──────────────────────────────────────────────────────────────┘
                          │ mTLS gRPC
                          ▼
┌──────────────────────────────────────────────────────────────┐
│                   Data Plane Pod(s)                           │
│                                                              │
│  ┌────────────────┐    ┌─────────────────────────────────┐  │
│  │  NGINX Agent    │    │           NGINX                 │  │
│  │                │    │                                 │  │
│  │  Receives config│    │  HTTP/HTTPS/TCP/UDP listeners   │  │
│  │  via gRPC       │    │  Reverse proxy to app pods      │  │
│  │  Runs nginx -t  │    │  TLS termination                │  │
│  │  Triggers reload│    │  NJS modules for advanced logic │  │
│  └────────────────┘    └─────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## Major Components

### Controller Manager

**Location:** `internal/controller/manager.go`

Sets up and runs all Kubernetes controllers using controller-runtime. Registers watches for Gateway API resources (Gateway, HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, UDPRoute, ReferenceGrant), NGF custom resources (NginxProxy, ClientSettingsPolicy, RateLimitPolicy, ObservabilityPolicy, etc.), and core Kubernetes resources (Secrets, Services, EndpointSlices, Namespaces). All watch events are funneled into a single event channel for batch processing.

Supports leader election for HA deployments. The event handler, provisioner, and status updater only run on the leader.

### Event Handler

**Location:** `internal/controller/handler.go`

Consumes batched events from the controller manager. For each batch it:

1. Upserts or deletes resources in the graph processor
2. Builds a validated resource graph (relationships + validation)
3. Generates dataplane configuration from the graph
4. Renders NGINX config files via Go `text/template`
5. Broadcasts config to all connected agents via gRPC
6. Updates resource statuses in the Kubernetes API

### Graph Processor

**Location:** `internal/controller/state/graph/`

Builds a validated in-memory graph of all Gateway API resources and their relationships. The graph models: GatewayClass -> Gateways -> Listeners -> Routes -> Rules -> BackendRefs -> Services -> EndpointSlices, with policies attached at each level. Validation happens in three layers: CRD-level (kubebuilder + CEL), graph-level (Go validation), and NGINX-level (`nginx -t`).

### NGINX Config Generator

**Location:** `internal/controller/nginx/config/`

Transforms the dataplane configuration into NGINX config files using Go `text/template`. Generates: `nginx.conf`, `http.conf`, `stream.conf`, per-listener server blocks, upstream definitions, snippet includes, TLS certificates, and htpasswd files.

### gRPC Agent Server

**Location:** `internal/controller/nginx/agent/grpc/`

Listens on port 8443 with mTLS (TLS 1.3 minimum, `RequireAndVerifyClientCert`). Provides two gRPC services:

- **CommandService** -- agent registration, bidirectional config streaming, health reporting
- **FileService** -- file download (single and streaming) for config and secret files

Every RPC is authenticated via a Kubernetes TokenReview: the agent presents its ServiceAccount token, the server validates it against the K8s API and verifies a running pod exists for that service account.

### Provisioner

**Location:** `internal/controller/provisioner/`

Dynamically creates Kubernetes resources for each valid Gateway: Deployment (or DaemonSet), Service, ServiceAccount, ConfigMaps (agent config + NGINX bootstrap), and Secrets. All resources are owned by the Gateway via `ownerReferences`, so deletion is handled by Kubernetes garbage collection. The NginxProxy CRD is the single source of truth for customizing these resources; direct edits are reverted on reconciliation.

### NJS Modules

**Location:** `internal/controller/nginx/modules/src/`

NGINX JavaScript modules for request processing that cannot be expressed in pure NGINX config:

- `httpmatches.js` -- complex HTTP matching (method, headers, query params)
- `epp.js` -- Gateway API Inference Extension integration for AI endpoint selection

## Reconciliation Flow

1. User applies a Gateway API resource (e.g., `kubectl apply -f gateway.yaml`)
2. Kubernetes API server stores the resource and notifies watchers
3. Controller-runtime informers filter and forward events to the event channel
4. The event loop batches events over a short window to prevent thrashing
5. The event handler processes the batch: builds the resource graph, generates NGINX config
6. Config files are stored in the deployment store and broadcast to all subscribed agents
7. Each agent writes files, runs `nginx -t`, and reloads NGINX if valid
8. The agent reports success/failure back to the control plane
9. The control plane updates resource statuses in the Kubernetes API

## Security Model

- Control plane and data plane run in separate pods with different RBAC permissions
- Data plane pods have no Kubernetes API access (security isolation)
- All control-data plane communication uses mTLS gRPC with TLS 1.3
- Agent identity is verified via Kubernetes TokenReview with audience-scoped, short-lived ServiceAccount tokens
- Secrets are transmitted over gRPC and stored in emptyDir volumes (never on shared PVCs)

## Key References

- [ARCHITECTURE.md](/ARCHITECTURE.md) -- comprehensive architecture with detailed diagrams and code references
- [docs/architecture/](/docs/architecture/) -- simplified architecture docs (configuration flow, traffic flow, gateway lifecycle, provisioning)
- [Design Principles](/docs/developer/design-principles.md) -- security, availability, performance, resilience, observability, ease of use
