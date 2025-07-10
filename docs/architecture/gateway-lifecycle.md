# Gateway Lifecycle

This shows what happens when you create, update, or delete a Gateway.

## Gateway Creation Flow

```mermaid
%%{init: {'theme':'dark', 'themeVariables': {'fontSize': '14px', 'darkMode': true, 'primaryColor': '#4f46e5', 'primaryTextColor': '#e5e7eb', 'primaryBorderColor': '#6b7280', 'lineColor': '#9ca3af', 'secondaryColor': '#1f2937', 'tertiaryColor': '#374151', 'background': '#111827', 'actorBkg': '#374151', 'actorBorder': '#6b7280', 'actorTextColor': '#e5e7eb', 'activationBkgColor': '#4f46e5', 'activationBorderColor': '#3730a3', 'signalColor': '#9ca3af', 'signalTextColor': '#e5e7eb', 'labelBoxBkgColor': '#1f2937', 'labelBoxBorderColor': '#6b7280', 'labelTextColor': '#e5e7eb', 'loopTextColor': '#e5e7eb', 'noteBkgColor': '#374151', 'noteBorderColor': '#6b7280', 'noteTextColor': '#e5e7eb'}}}%%
sequenceDiagram
    participant User
    participant API as K8s API Server
    participant NGF as NGF Controller
    participant K8s as Kubernetes
    participant Agent as NGINX Agent
    participant NGINX

    User->>API: Create Gateway resource
    API->>NGF: Watch event: Gateway created
    NGF->>NGF: Validate Gateway config
    NGF->>K8s: Create NGINX Deployment
    NGF->>K8s: Create LoadBalancer Service
    K8s->>Agent: Start NGINX pod
    Agent->>NGF: gRPC connect & register
    NGF->>Agent: Send initial config
    Agent->>NGINX: Write nginx.conf
    Agent->>NGINX: Start NGINX process
    Agent->>NGF: Report ready status
    NGF->>API: Update Gateway status: Ready
    API->>User: Gateway is ready
```

## What Happens During Creation

### 1. User Creates Gateway

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  gatewayClassName: nginx
  listeners:
  - name: http
    port: 80
    protocol: HTTP
```

### 2. NGF Controller Validates

```text
Validation Checks:
├── GatewayClass exists and is valid
├── Listener configuration is correct
├── Required permissions are available
├── Resource limits are acceptable
└── No conflicting Gateways exist
```

### 3. Kubernetes Resources Created

```text
Resources Created (not exhaustive):
├── Deployment: nginx-gateway-{gateway-name}
├── Service: nginx-gateway-{gateway-name}
├── ConfigMap: nginx-config-{gateway-name}
├── ServiceAccount: nginx-gateway-{gateway-name}
└── Secrets: TLS certificates (if needed)
```

### 4. NGINX Pod Starts

```text
Pod Startup:
├── Pull NGINX + Agent image
├── Mount configuration files
├── Start NGINX Agent process
├── Agent connects to control plane
└── Download initial configuration
```

### 5. Gateway Becomes Ready

```text
Ready Conditions:
├── Deployment is available
├── Service has endpoints
├── NGINX is serving traffic
├── Health checks pass
└── Status updated in Kubernetes
```

## HTTPRoute Attachment Flow

```mermaid
%%{init: {'theme':'dark', 'themeVariables': {'fontSize': '14px', 'darkMode': true, 'primaryColor': '#4f46e5', 'primaryTextColor': '#e5e7eb', 'primaryBorderColor': '#6b7280', 'lineColor': '#9ca3af', 'secondaryColor': '#1f2937', 'tertiaryColor': '#374151', 'background': '#111827', 'actorBkg': '#374151', 'actorBorder': '#6b7280', 'actorTextColor': '#e5e7eb', 'activationBkgColor': '#4f46e5', 'activationBorderColor': '#3730a3', 'signalColor': '#9ca3af', 'signalTextColor': '#e5e7eb', 'labelBoxBkgColor': '#1f2937', 'labelBoxBorderColor': '#6b7280', 'labelTextColor': '#e5e7eb', 'loopTextColor': '#e5e7eb', 'noteBkgColor': '#374151', 'noteBorderColor': '#6b7280', 'noteTextColor': '#e5e7eb'}}}%%
sequenceDiagram
    participant User
    participant API as K8s API Server
    participant NGF as NGF Controller
    participant Agent as NGINX Agent
    participant NGINX

    User->>API: Create HTTPRoute
    API->>NGF: Watch event: HTTPRoute created
    NGF->>NGF: Validate route rules
    NGF->>NGF: Check Gateway compatibility
    NGF->>NGF: Generate updated config
    NGF->>Agent: Send new nginx.conf
    Agent->>NGINX: Test configuration
    Agent->>NGINX: Reload if valid
    Agent->>NGF: Report configuration status
    NGF->>API: Update HTTPRoute status
    NGF->>API: Update Gateway status
```

### Route Configuration Process

```text
HTTPRoute Processing:
├── Parse route rules and matches
├── Validate backend references
├── Check Gateway listener compatibility
├── Generate NGINX location blocks
├── Update upstream definitions
├── Apply filters and policies
└── Send complete config to data plane
```

## Gateway Update Flow

### Configuration Changes

```mermaid
%%{init: {'theme':'dark', 'themeVariables': {'fontSize': '14px', 'darkMode': true, 'primaryColor': '#4f46e5', 'primaryTextColor': '#e5e7eb', 'primaryBorderColor': '#6b7280', 'lineColor': '#9ca3af', 'secondaryColor': '#1f2937', 'tertiaryColor': '#374151', 'background': '#111827', 'actorBkg': '#374151', 'actorBorder': '#6b7280', 'actorTextColor': '#e5e7eb', 'activationBkgColor': '#4f46e5', 'activationBorderColor': '#3730a3', 'signalColor': '#9ca3af', 'signalTextColor': '#e5e7eb', 'labelBoxBkgColor': '#1f2937', 'labelBoxBorderColor': '#6b7280', 'labelTextColor': '#e5e7eb', 'loopTextColor': '#e5e7eb', 'noteBkgColor': '#374151', 'noteBorderColor': '#6b7280', 'noteTextColor': '#e5e7eb'}}}%%
sequenceDiagram
    participant User
    participant API as K8s API Server
    participant NGF as NGF Controller
    participant Agent as NGINX Agent
    participant NGINX

    User->>API: Update Gateway/Route
    API->>NGF: Watch event: Resource updated
    NGF->>NGF: Calculate configuration diff
    NGF->>NGF: Generate new config
    NGF->>Agent: Send updated config
    Agent->>NGINX: Test new configuration

    alt Configuration Valid
        Agent->>NGINX: Reload NGINX
        Agent->>NGF: Report success
        NGF->>API: Update status: Accepted
    else Configuration Invalid
        Agent->>NGF: Report validation error
        NGF->>API: Update status: Invalid
        Note over NGINX: Keep running old config
    end
```

### Scaling Changes

```text
Scaling Operations:
├── Update Deployment replica count
├── New pods start and register
├── Load balancer adds new endpoints
├── Traffic distributes to all pods
└── Old pods drain gracefully
```

## Gateway Deletion Flow

```mermaid
%%{init: {'theme':'dark', 'themeVariables': {'fontSize': '14px', 'darkMode': true, 'primaryColor': '#4f46e5', 'primaryTextColor': '#e5e7eb', 'primaryBorderColor': '#6b7280', 'lineColor': '#9ca3af', 'secondaryColor': '#1f2937', 'tertiaryColor': '#374151', 'background': '#111827', 'actorBkg': '#374151', 'actorBorder': '#6b7280', 'actorTextColor': '#e5e7eb', 'activationBkgColor': '#4f46e5', 'activationBorderColor': '#3730a3', 'signalColor': '#9ca3af', 'signalTextColor': '#e5e7eb', 'labelBoxBkgColor': '#1f2937', 'labelBoxBorderColor': '#6b7280', 'labelTextColor': '#e5e7eb', 'loopTextColor': '#e5e7eb', 'noteBkgColor': '#374151', 'noteBorderColor': '#6b7280', 'noteTextColor': '#e5e7eb'}}}%%
sequenceDiagram
    participant User
    participant API as K8s API Server
    participant NGF as NGF Controller
    participant K8s as Kubernetes
    participant Agent as NGINX Agent

    User->>API: Delete Gateway
    API->>NGF: Watch event: Gateway deleted
    NGF->>NGF: Check for attached routes

    alt Routes Still Attached
        NGF->>API: Update route status: Gateway not found
    end

    NGF->>Agent: Send shutdown signal
    Agent->>Agent: Graceful shutdown
    NGF->>K8s: Delete LoadBalancer Service
    NGF->>K8s: Delete Deployment
    NGF->>K8s: Delete ConfigMaps
    NGF->>K8s: Delete ServiceAccount
    K8s->>API: Resources deleted
    API->>User: Gateway deletion complete
```

### Graceful Shutdown Process

```text
Shutdown Steps:
├── Stop accepting new connections
├── Finish processing existing requests
├── Close upstream connections
├── Terminate NGINX process
└── Remove pod from endpoints
```
