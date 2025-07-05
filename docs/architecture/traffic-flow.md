# Traffic Flow

This diagram shows how user requests travel through NGINX Gateway Fabric.

## Simple Overview

```mermaid
%%{init: {'theme':'dark', 'themeVariables': {'fontSize': '16px', 'darkMode': true, 'primaryColor': '#4f46e5', 'primaryTextColor': '#e5e7eb', 'primaryBorderColor': '#6b7280', 'lineColor': '#9ca3af', 'secondaryColor': '#1f2937', 'tertiaryColor': '#374151', 'background': '#111827', 'mainBkg': '#1f2937', 'secondBkg': '#374151', 'tertiaryTextColor': '#d1d5db'}}}%%
graph LR
    %% Simple traffic flow
    USER[👤 User]

    subgraph "Kubernetes Cluster"
        NGINX[🌐 NGINX]

        subgraph "Your Apps"
            SVC1[🔵 user-service]
            SVC2[🔵 order-service]
            POD1[Pod A]
            POD2[Pod B]
            POD3[Pod C]
        end
    end

    %% Simple flow
    USER --> NGINX
    NGINX --> SVC1
    NGINX --> SVC2
    SVC1 --> POD1
    SVC1 --> POD2
    SVC2 --> POD3

    %% Dark-friendly styling
    style USER fill:#fbbf24,stroke:#f59e0b,stroke-width:2px,color:#1f2937
    style NGINX fill:#8b5cf6,stroke:#7c3aed,stroke-width:2px,color:#ffffff
    style SVC1 fill:#10b981,stroke:#059669,stroke-width:2px,color:#ffffff
    style SVC2 fill:#10b981,stroke:#059669,stroke-width:2px,color:#ffffff
```

## Traffic Processing Steps

### 1. User Sends Request

```text
User Request:
├── GET /users
├── POST /orders
├── Headers: Authorization, Content-Type
└── Body: JSON data (if needed)
```

### 2. NGINX Receives Request

```text
NGINX Gateway:
├── Receives request from user
├── Applies SSL termination
├── Matches routing rules
└── Selects backend service
```

### 3. Service Processes Request

```text
Backend Service:
├── Receives request from NGINX
├── Processes business logic
├── Queries database (if needed)
├── Generates response
└── Returns response to NGINX
```

### 4. Response Returns to User

```text
Response Flow:
├── Service → NGINX
├── NGINX → User
└── Request complete
```

## Detailed Request Flow

```mermaid
%%{init: {'theme':'dark', 'themeVariables': {'fontSize': '14px', 'darkMode': true, 'primaryColor': '#4f46e5', 'primaryTextColor': '#e5e7eb', 'primaryBorderColor': '#6b7280', 'lineColor': '#9ca3af', 'secondaryColor': '#1f2937', 'tertiaryColor': '#374151', 'background': '#111827', 'actorBkg': '#374151', 'actorBorder': '#6b7280', 'actorTextColor': '#e5e7eb', 'activationBkgColor': '#4f46e5', 'activationBorderColor': '#3730a3', 'signalColor': '#9ca3af', 'signalTextColor': '#e5e7eb', 'labelBoxBkgColor': '#1f2937', 'labelBoxBorderColor': '#6b7280', 'labelTextColor': '#e5e7eb', 'loopTextColor': '#e5e7eb', 'noteBkgColor': '#374151', 'noteBorderColor': '#6b7280', 'noteTextColor': '#e5e7eb'}}}%%
sequenceDiagram
    participant User
    participant NGINX
    participant Service as Backend Service
    participant Pod

    User->>NGINX: HTTP Request
    NGINX->>NGINX: Route matching
    NGINX->>Service: Proxy to service
    Service->>Pod: Forward to pod
    Pod->>Pod: Process request
    Pod->>Service: Return response
    Service->>NGINX: Response
    NGINX->>User: HTTP Response
```

## Request Routing Logic

### Path-Based Routing

```nginx
# Generated from HTTPRoute resources
location /users {
    proxy_pass http://user-service;
}

location /orders {
    proxy_pass http://order-service;
}

location /products {
    proxy_pass http://product-service;
}
```


### Host-Based Routing

```nginx
# Different hosts route to different services
server {
    server_name api.example.com;
    location / {
        proxy_pass http://api-service;
    }
}

server {
    server_name admin.example.com;
    location / {
        proxy_pass http://admin-service;
    }
}
```
