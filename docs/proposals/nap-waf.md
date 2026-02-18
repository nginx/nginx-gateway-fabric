# Enhancement Proposal-3341: NGINX App Protect WAF Integration with PLM

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/3341
- Status: Implementable

## Summary

This proposal describes the integration of F5 WAF for NGINX with Policy Lifecycle Management (PLM) into NGINX Gateway Fabric (NGF) to provide comprehensive WAF protection at Gateway and Route levels. The design uses Gateway API inherited policy attachment to provide flexible, hierarchical WAF protection by referencing PLM-managed APPolicy resources that contain compiled policy bundles in remote storage locations accessible via API.

## Goals

- Extend NginxProxy resource to enable NAP WAF for GatewayClass/Gateway with multi-container orchestration
- Design WAFGatewayBindingPolicy custom resource using inherited policy attachment for hierarchical WAF configuration with APPolicy references
- Define deployment workflows that integrate with PLM's policy compilation and storage architecture
- Provide secure policy distribution from PLM in-cluster storage to NGF data plane without shared filesystem requirements
- Deliver enterprise-grade WAF capabilities through Kubernetes-native APIs with intuitive policy inheritance
- Maintain alignment with NGF's existing security and operational patterns
- Support configurable security logging for WAF events and policy violations
- Support both HTTPRoute and GRPCRoute protection

## Non-Goals

- Managing PLM controller deployment and lifecycle (handled by PLM team)
- Compiling WAF policies (handled by PLM Policy Compiler)
- Providing inline policy definition (not supported by NAP v5 architecture)
- Supporting NGINX OSS (NAP v5 does not require NGINX Plus, but OSS support is out of scope at this time)
- Real-time policy editing interfaces
- Persistent storage management for policy files in NGF data plane

## Introduction

### NAP v5 and PLM Architectural Overview

NGINX App Protect WAF v5 with Policy Lifecycle Management imposes specific architectural requirements:

- **Multi-container deployment**: Requires separate `waf-enforcer` and `waf-config-mgr` containers alongside the main NGINX container
- **PLM Policy Management**: Policies are managed through Kubernetes CRDs (APPolicy, ApConfigSet, etc.) and compiled by PLM Policy Compiler
- **Remote Storage Architecture**: PLM stores compiled policies in remote storage accessible via API rather than shared filesystems
- **Per-instance ConfigMgr**: Each NGINX/Enforcer pod includes a ConfigMgr sidecar for local policy management

### Design Philosophy

This proposal provides the best possible Kubernetes-native experience by integrating with PLM's Kubernetes CRD-based policy management while respecting NGF's distributed Gateway architecture. The design uses Gateway API's inherited policy attachment pattern to provide intuitive hierarchical security with the ability to override policies at more specific levels.

### PLM Integration Benefits

- **Kubernetes-native Policy Management**: Define policies using Kubernetes CRDs rather than external workflows
- **Automated Policy Compilation**: PLM handles policy compilation and storage automatically
- **Centralized Policy Lifecycle**: Single source of truth for policy definitions across the cluster
- **Simplified Operations**: Eliminates need for external policy compilation pipelines and storage infrastructure
- **Automatic Policy Updates**: Changes to APPolicy resources automatically trigger recompilation and deployment
- **In-Cluster Storage**: All policy storage managed within the cluster, no external dependencies required

### Policy Attachment Strategy

The design uses **inherited policy attachment** following Gateway API best practices:

- **Multiple targets per policy of the same type**: A WAFGatewayBindingPolicy can target muliple resources, but the resources must be of the same type (either a Gateway or Route types, not both)
- **Gateway-level policies** provide default protection for all routes attached to the Gateway
- **Route-level policies** can override Gateway-level policies for specific routes requiring different protection
- **Policy precedence**: More specific policies (Route-level) override less specific policies (Gateway-level)
- **Automatic inheritance**: New routes automatically receive Gateway-level protection without explicit configuration

### Storage Architecture

#### PLM Storage Model

PLM uses in-cluster storage with API access for policy distribution:

- **Policy Compiler Output**: Compiled policy bundles stored in in-cluster storage (Kubernetes-native storage solution managed by PLM)
- **API-based Access**: NGF retrieves policies via API calls to in-cluster storage service
- **No Shared Volumes Required**: Eliminates shared filesystem dependency for distributed deployments
- **Cluster-local Communication**: All policy distribution occurs within cluster boundaries

#### NGF Data Plane Storage

NGF maintains ephemeral volumes (emptyDir) for NAP v5's required local storage:

- **Security alignment**: No persistent state that could be compromised
- **Operational simplicity**: No persistent volume lifecycle management
- **Clean failure recovery**: Fresh volumes on pod restart with current policies
- **Immutable infrastructure**: Policy files cannot be modified at runtime

### Overall System Architecture

```mermaid
graph TB
    %% Kubernetes Cluster - PLM Components
    subgraph "Kubernetes Cluster - PLM Components"
        PolicyController[PLM Policy Controller<br/>Watches APPolicy & APLogConf CRDs]
        PolicyCompiler[PLM Policy Compiler<br/>Job-based Compilation]
        ClusterStorage[In-Cluster Policy Storage<br/>Kubernetes-native Storage Service]
    end

    %% Kubernetes Cluster - NGF Components
    subgraph "Kubernetes Cluster - NGF Components"

        %% Control Plane
        subgraph "nginx-gateway namespace"
            NGFPod[NGF Pod<br/>Controllers + Policy Fetcher]
        end

        %% Application Namespace
        subgraph "applications namespace"

            %% Gateway Resources
            Gateway[Gateway<br/>*.example.com]
            HTTPRoute[HTTPRoute<br/>Protected Route]
            GRPCRoute[GRPCRoute<br/>Protected gRPC Service]
            Application[Application<br/>Backend Service]

            %% Custom Resources (all in app namespace)
            NginxProxy[NginxProxy<br/>waf.enabled=true]
            GatewayWAFGatewayBindingPolicy[WAFGatewayBindingPolicy<br/>Gateway-level Protection<br/>References APPolicy & APLogConf]
            RouteWAFGatewayBindingPolicy[WAFGatewayBindingPolicy<br/>Route-level Override<br/>References APPolicy & APLogConf]

            %% PLM Resources
            APPolicy[APPolicy CRD<br/>Policy Definition]
            APLogConf[APLogConf CRD<br/>Logging Configuration]

            %% NGINX Data Plane (WAF Enabled)
            subgraph "NGINX Pod (Multi-Container when WAF enabled)"
                direction TB
                NGINXContainer[NGINX Container<br/>+ NAP Module]
                WafEnforcer[WAF Enforcer<br/>Container]
                WafConfigMgr[WAF Config Manager<br/>Sidecar per Pod]

                subgraph "Shared Volumes (Ephemeral)"
                    PolicyVol[Policy Volume<br/>emptyDir]
                    ConfigVol[Config Volume<br/>emptyDir]
                end
            end
        end
    end

    %% External Access
    PublicEndpoint[Public Endpoint<br/>Load Balancer]
    Client[Client Traffic]

    %% PLM Policy Workflow
    APPolicy -->|Watched by| PolicyController
    APLogConf -->|Watched by| PolicyController
    PolicyController -->|Triggers| PolicyCompiler
    PolicyCompiler -->|Stores Bundles| ClusterStorage
    PolicyController -->|Updates Status with<br/>Storage Location| APPolicy
    PolicyController -->|Updates Status with<br/>Storage Location| APLogConf

    %% Policy Attachment Flow
    GatewayWAFGatewayBindingPolicy -.->|References| APPolicy
    GatewayWAFGatewayBindingPolicy -.->|References| APLogConf
    RouteWAFGatewayBindingPolicy -.->|References| APPolicy
    RouteWAFGatewayBindingPolicy -.->|References| APLogConf
    GatewayWAFGatewayBindingPolicy -.->|Targets & Protects| Gateway
    RouteWAFGatewayBindingPolicy -.->|Targets & Overrides| HTTPRoute
    Gateway -->|Inherits Protection| HTTPRoute
    Gateway -->|Inherits Protection| GRPCRoute

    %% Configuration Flow
    NginxProxy -.->|Enables WAF| Gateway

    %% Control Plane Operations
    NGFPod -->|Watches Resources| NginxProxy
    NGFPod -->|Watches Resources| GatewayWAFGatewayBindingPolicy
    NGFPod -->|Watches Resources| RouteWAFGatewayBindingPolicy
    NGFPod -->|Watches APPolicy Status| APPolicy
    NGFPod -->|Watches APLogConf Status| APLogConf
    NGFPod -->|Fetches Policy via In-Cluster API<br/>HTTP or HTTPS| ClusterStorage
    NGFPod ===|gRPC Config| NGINXContainer
    NGFPod -->|Deploy Policy & Log Profiles| PolicyVol

    %% NAP v5 Inter-Container Communication
    NGINXContainer <-->|Shared FS| PolicyVol
    WafEnforcer <-->|Shared FS| PolicyVol
    WafConfigMgr <-->|Shared FS| PolicyVol
    WafConfigMgr <-->|Shared FS| ConfigVol
    NGINXContainer <-->|Shared FS| ConfigVol

    %% Traffic Flow
    Client ==>|HTTP/HTTPS/gRPC| PublicEndpoint
    PublicEndpoint ==>|WAF Protected| NGINXContainer
    NGINXContainer ==>|Filtered Traffic| Application

    %% Resource Relationships
    HTTPRoute -->|Attached to| Gateway
    GRPCRoute -->|Attached to| Gateway

    %% Styling
    classDef plm fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
    classDef control fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    classDef gateway fill:#66CDAA,stroke:#333,stroke-width:2px
    classDef wafRequired fill:#ffebee,stroke:#c62828,stroke-width:3px
    classDef app fill:#fce4ec,stroke:#880e4f,stroke-width:2px
    classDef volume fill:#f1f8e9,stroke:#33691e,stroke-width:2px
    classDef endpoint fill:#FFD700,stroke:#333,stroke-width:2px
    classDef storage fill:#fff3e0,stroke:#e65100,stroke-width:2px
    classDef crd fill:#f0f4c3,stroke:#827717,stroke-width:2px

    class PolicyController,PolicyCompiler plm
    class NGFPod control
    class Gateway,HTTPRoute,GRPCRoute gateway
    class WafEnforcer,WafConfigMgr,PolicyVol,NginxProxy wafRequired
    class GatewayWAFGatewayBindingPolicy,RouteWAFGatewayBindingPolicy wafRequired
    class Application app
    class PolicyVol,ConfigVol volume
    class PublicEndpoint endpoint
    class ClusterStorage storage
    class APPolicy,APLogConf crd
```

This architecture demonstrates:

**PLM Components (Light Blue):** The PLM Policy Controller watches both APPolicy and APLogConf CRDs and triggers the Policy Compiler to generate policy and log profile bundles. Compiled bundles are stored in in-cluster storage accessible via API (HTTP or HTTPS).

**NGF Control Plane (Purple):** The NGF Pod watches WAFGatewayBindingPolicy resources that reference APPolicy and APLogConf CRDs. When status is updated with storage locations, NGF fetches the compiled bundles via in-cluster API (supporting both HTTP and HTTPS) and distributes them to the appropriate Gateway data plane pods.

**Policy and Logging Reference Flow:** WAFGatewayBindingPolicy resources use `APPolicySource` to reference APPolicy CRDs and `securityLogs` to reference APLogConf CRDs by name and namespace. NGF watches both resource types' status to get storage locations. This decouples NGF from direct PLM controller dependencies while leveraging PLM's compilation capabilities.

**In-Cluster Storage:** All policy and log profile storage and retrieval occurs within cluster boundaries. NGF accesses storage via Kubernetes-native API calls with configurable TLS support, eliminating external dependencies and simplifying network access requirements.

**Data Plane Storage:** Each Gateway pod maintains ephemeral volumes for local policy and log profile storage. NGF fetches both policies and log profiles from in-cluster storage and places them in these ephemeral volumes for NAP enforcement.

**Policy Inheritance:** Gateway-level WafGatewayBindingPolicies automatically protect all attached routes with both security policies and logging configurations. Route-level WafGatewayBindingPolicies can override Gateway policies with more specific protection and logging.

**Network Access:** All communication occurs within the cluster with optional TLS for secure storage communication - no external network access required for policy distribution.

### Policy Development Workflow with PLM

1. **Policy Development**: Define WAF policies using APPolicy CRDs in Kubernetes
2. **Automatic Compilation**: PLM Policy Controller detects APPolicy changes and triggers compilation
3. **Storage**: Compiled bundles stored in remote storage with location written to APPolicy status
4. **Policy Attachment**: Create WAFGatewayBindingPolicy CR with `APPolicySource` referencing the APPolicy
5. **Automatic Application**: NGF watches APPolicy status, fetches bundle, and deploys to data plane
6. **Automatic Updates**: APPolicy changes trigger recompilation, status update, and NGF re-fetch

```mermaid
sequenceDiagram
    participant User
    participant APPolicy as APPolicy CRD
    participant PLMController as PLM Policy Controller
    participant Compiler as PLM Policy Compiler
    participant Storage as Remote Storage
    participant WAFGatewayBindingPolicy as WAFGatewayBindingPolicy CRD
    participant NGF as NGF Control Plane
    participant DataPlane as NGINX Data Plane

    User->>APPolicy: Create/Update APPolicy
    PLMController->>APPolicy: Watch for changes
    PLMController->>Compiler: Trigger compilation
    Compiler->>Storage: Store compiled bundle
    Compiler->>APPolicy: Update status with storage location

    User->>WAFGatewayBindingPolicy: Create WAFGatewayBindingPolicy referencing APPolicy
    NGF->>WAFGatewayBindingPolicy: Watch WAFGatewayBindingPolicy
    NGF->>APPolicy: Watch referenced APPolicy status
    APPolicy->>NGF: Status contains storage location
    NGF->>Storage: Fetch compiled bundle via API
    NGF->>DataPlane: Deploy policy to ephemeral volume
    DataPlane->>DataPlane: Apply policy

    Note over User,DataPlane: Policy Updates
    User->>APPolicy: Update APPolicy
    PLMController->>Compiler: Trigger recompilation
    Compiler->>Storage: Store updated bundle
    Compiler->>APPolicy: Update status
    NGF->>APPolicy: Detect status change
    NGF->>Storage: Fetch updated bundle
    NGF->>DataPlane: Deploy updated policy
```

### Security Logging Configuration

The securityLogs section supports multiple logging configurations by referencing PLM-managed APLogConf CRDs. Each APLogConf is compiled by PLM and stored in in-cluster storage, similar to APPolicy resources.

**Logging Configuration Approach:**

- **APLogConf References**: WAFGatewayBindingPolicy references APLogConf CRDs by name and namespace
- **Compiled Log Profiles**: PLM Policy Controller watches APLogConf, triggers compilation, and stores bundles
- **Multiple Configurations**: Support for multiple log configurations per WAFGatewayBindingPolicy
- **Cross-namespace**: APLogConf can be referenced across namespaces with ReferenceGrant

**Destination Types:**

- `type: "Stderr"`: Output to container stderr
- `type: "File"`: Write to specified file path (must be mounted for waf-enforcer access)
- `type: "Syslog"`: Send to syslog server via TCP

**Generated NGINX Configuration:**

Each APLogConf reference generates an `app_protect_security_log` directive in NGINX configuration:

```nginx
# APLogConf compiled log profile to stderr
app_protect_security_log /shared_volume/custom-log-profile.tgz stderr;

# APLogConf compiled log profile to file
app_protect_security_log /shared_volume/admin-log-profile.tgz /var/log/app_protect/security.log;

# APLogConf compiled log profile to syslog
app_protect_security_log /shared_volume/blocked-log-profile.tgz syslog:server=syslog-svc.default:514;
```

### Policy Fetch Failure Handling

**First-Time Policy Fetch Failure:**

- Route configuration is **not applied** - no WAF protection enabled
- Route remains unprotected until policy becomes available
- WAFGatewayBindingPolicy status reflects the failure reason

**Policy Update Failure:**

- **Existing policy remains in effect** - no disruption to current protection
- WAF protection continues with the last successfully deployed policy
- WAFGatewayBindingPolicy status indicates update failure but maintains "Accepted" for existing policy

**Retry Behavior:**

- Configurable retry policy with exponential backoff
- No service disruption during retry attempts
- Detailed error messages in WAFGatewayBindingPolicy status for troubleshooting

### Policy Inheritance and Precedence

**Inheritance Hierarchy:**

- Gateway-level WAFGatewayBindingPolicy → HTTPRoute (inherited)
- Gateway-level WAFGatewayBindingPolicy → GRPCRoute (inherited)

**Override Precedence (most specific wins):**

- Route-level WAFGatewayBindingPolicy > Gateway-level WAFGatewayBindingPolicy

**Conflict Resolution:**

- Multiple policies targeting the same resource at the same level = error/rejected
- More specific policy completely overrides less specific policy
- Clear status reporting indicates which policy is active for each route

## API, Customer Driven Interfaces, and User Experience

### NGF Control Plane Configuration for PLM Storage

NGF control plane requires configuration to communicate with the PLM storage service (S3-compatible). This includes the storage endpoint URL, authentication credentials, and optional TLS settings.

> **Note:** PLM is still under active development. The exact authentication and TLS requirements may evolve as PLM matures. This section will be updated as the PLM storage API is finalized.

#### Configuration Approach

NGF uses Kubernetes Secrets for all sensitive PLM storage configuration:

- **Credentials Secret**: Contains S3 secret access key (`seaweedfs_admin_secret`); access key ID is "admin" by default
- **TLS CA Secret** (optional): Contains CA certificate for server verification
- **TLS Client Secret** (optional): Contains client certificate/key for mutual TLS

Secret names are passed via CLI flags, and NGF watches these Secrets dynamically. This approach:

- Follows Kubernetes best practices for credential management
- Aligns with how NGF handles other secrets (NGINX Plus license, usage reporting)
- Avoids embedding credentials in deployment manifests
- Supports credential and certificate rotation via Secret updates (no pod restart required)

#### CLI Arguments

```bash
# PLM storage service URL (required when WAF enabled)
--plm-storage-url=https://plm-storage-service.plm-system.svc.cluster.local

# Secret containing S3 credentials (optional, for authenticated storage access)
# Secret should have "seaweedfs_admin_secret" field containing the S3 secret access key
# The access key ID is "admin" by default for SeaweedFS
--plm-storage-credentials-secret=plm-storage-credentials

# TLS configuration (optional)
--plm-storage-ca-secret=plm-ca-secret           # Secret with ca.crt for server verification
--plm-storage-client-ssl-secret=plm-client-secret    # Secret with tls.crt/tls.key for mutual TLS
--plm-storage-skip-verify=false      # Skip TLS verification (dev only)
```

#### Secrets Format

**Credentials Secret:**

The credentials secret contains the S3 secret access key for SeaweedFS authentication. The access key ID is "admin" by default.
This Secret is automatically created by the PLM installation.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: plm-storage-credentials
  namespace: nginx-gateway
type: Opaque
data:
  seaweedfs_admin_secret: <base64-encoded-secret-access-key>
```

**TLS CA Certificate Secret (optional):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: plm-ca-secret
  namespace: nginx-gateway
type: Opaque
data:
  ca.crt: <base64-encoded-ca-certificate>       # CA certificate for server verification
```

**TLS Client Certificate Secret (optional, for mutual TLS):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: plm-client-secret
  namespace: nginx-gateway
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-client-certificate>  # Client certificate for mutual TLS
  tls.key: <base64-encoded-client-key>          # Client private key for mutual TLS
```

#### Helm Chart Configuration

```yaml
# values.yaml
nginxGateway:
  # PLM (Policy Lifecycle Manager) storage configuration for WAF bundles.
  # Used when WAF is enabled to fetch APPolicy and APLogConf bundles from S3-compatible storage.
  plmStorage:
    # Storage service URL (required when WAF enabled)
    url: "https://plm-storage-service.plm-system.svc.cluster.local"

    # Secret containing S3 credentials for PLM storage.
    # If the secret is in a different namespace, should be in form "ns/name",
    # otherwise same namespace as NGF will be assumed.
    # Secret should have "seaweedfs_admin_secret" field.
    credentialsSecretName: "plm-storage-credentials"

    # TLS configuration for PLM storage connections.
    tls:
      # Name of the secret for the CA certificate file for TLS verification.
      # Secret should have "ca.crt" field.
      caSecretName: "plm-ca-secret"

      # Name of the secret for the client certificate/key for mutual TLS.
      # Secret should have "tls.crt" and "tls.key" fields.
      clientSSLSecretName: "plm-client-secret"

      # Skip TLS certificate verification. Use only for testing.
      insecureSkipVerify: false
```

#### Configuration Options

| CLI Argument                       | Description                                                         | Default | Required         |
|------------------------------------|---------------------------------------------------------------------|---------|------------------|
| `--plm-storage-url`                | PLM storage service URL (HTTP or HTTPS)                             | -       | When WAF enabled |
| `--plm-storage-credentials-secret` | Name of Secret containing S3 credentials (`seaweedfs_admin_secret`) | -       | No*              |
| `--plm-storage-ca-secret`          | Name of Secret containing CA certificate (`ca.crt`)                 | -       | No               |
| `--plm-storage-client-ssl-secret`  | Name of Secret containing client cert/key (`tls.crt`/`tls.key`)     | -       | No               |
| `--plm-storage-skip-verify`        | Skip TLS certificate verification                                   | false   | No               |

**Note:** Secret names can include a namespace prefix in the form `namespace/name`. If no namespace is specified, the NGF controller's namespace is assumed.

#### Security Recommendations

- **Development**: Use HTTP without TLS for simplicity, or HTTPS with `--plm-storage-skip-verify=true`
- **Production**: Always use HTTPS with proper TLS verification via `--plm-storage-ca-secret`
- **High Security**: Enable mutual TLS by providing `--plm-storage-client-ssl-secret` with client certificate and key
- **Never use** `--plm-storage-skip-verify=true` in production

#### Design Decision: Dynamic Secret Watching

PLM secrets (credentials and TLS certificates) are watched dynamically by NGF, allowing updates without pod restarts:

1. **Secret Watching**: NGF watches the configured PLM secrets using the existing secret watching infrastructure
2. **Dynamic Updates**: When a PLM secret changes, NGF automatically updates the S3 fetcher's credentials and/or TLS configuration
3. **Consistency**: This approach aligns with how NGF handles other secrets (Plus license, usage reporting secrets)

### NginxProxy Resource Extension

Users enable WAF through the NginxProxy resource:

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: nginx-proxy-waf
  namespace: nginx-gateway
spec:
  waf: "enabled"  # "enabled" | "disabled"
```

### WAFGatewayBindingPolicy Custom Resource with PLM Integration

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: WAFGatewayBindingPolicy
metadata:
  name: gateway-protection-policy
  namespace: applications
spec:
  # Policy attachment - targets Gateway for inherited protection
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: secure-gateway
    namespace: applications

  # PLM-managed policy source
  APPolicySource:
    name: "production-web-policy"
    namespace: "security"
    # Cross-namespace references require ReferenceGrant

  # Security logging configuration - references APLogConf CRDs
  securityLogs:
  - name: "blocked-requests-logging"
    # Reference to APLogConf CRD for logging configuration
    APLogConfSource:
      name: "log-blocked-profile"
      namespace: "security"
    destination:
      type: "Stderr"

  - name: "admin-detailed-logging"
    # Another APLogConf reference for different logging profile
    APLogConfSource:
      name: "log-all-detailed-profile"
      namespace: "security"
    destination:
      type: "File"
      file:
        path: "/var/log/app_protect/detailed-security.log"

---
# Route-level override example
apiVersion: gateway.nginx.org/v1alpha1
kind: WAFGatewayBindingPolicy
metadata:
  name: admin-strict-policy
  namespace: applications
spec:
  # Policy attachment - targets specific HTTPRoute to override Gateway policy
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: admin-route
    namespace: applications

  # Reference stricter PLM-managed policy for admin routes
  APPolicySource:
    name: "admin-strict-web-policy"
    namespace: "security"

  securityLogs:
  - name: "admin-all-logging"
    APLogConfSource:
      name: "log-all-verbose-profile"
      namespace: "security"
    destination:
      type: "Stderr"
```

### APPolicy CRD Example (Managed by PLM)

```yaml
# This resource is created and managed by users/security teams
# PLM controllers handle compilation and status updates
apiVersion: waf.f5.com/v1alpha1
kind: APPolicy
metadata:
  name: production-web-policy
  namespace: security
spec:
  policy:
    name: "prod-web-protection"
    template:
      name: "POLICY_TEMPLATE_NGINX_BASE"
    applicationLanguage: "utf-8"
    enforcementMode: "blocking"
    # Additional NAP policy configuration
    signatures:
    - signatureSetRef:
        name: "high-accuracy-signatures"
    blocking-settings:
      violations:
      - name: "VIOL_SQL_INJECTION"
        alarm: true
        block: true

status:
  # PLM updates this after compilation
  bundle:
    state: ready  # other values: pending, processing, invalid
    # Location only present when state is "ready"
    location: "s3://bucket_name/folder1/folder2/bundle.tgz"
    sha256: "abcd1234efgh5678ijkl9012mnop3456qrst7890uvwx5678yzab9012cdef3456"
    compilerVersion: "11.582.0"
    signatures:
      attackSignatures: "2024-12-29T19:01:32"
      botSignatures: "2024-12-13T10:01:02"
      threatCampaigns: "2024-12-21T00:01:02"
  processing:
    isCompiled: true  # false if spec referenced a precompiled bundle
    datetime: "2025-01-17T20:19:43"
    errors: []  # array of messages, only if state was "invalid"
```

### APLogConf CRD Example (Managed by PLM)

```yaml
# This resource is created and managed by users/security teams
# PLM controllers handle compilation and status updates
apiVersion: waf.f5.com/v1alpha1
kind: APLogConf
metadata:
  name: log-blocked-profile
  namespace: security
spec:
  content:
    format: splunk
    max_message_size: "10k"
    max_request_size: "any"
  filter:
    request_type: "blocked"

status:
  # PLM updates this after compilation
  bundle:
    state: ready  # other values: pending, processing, invalid
    location: "s3://bucket_name/log-profiles/log-blocked-profile-v1.0.0.tgz"
    sha256: "def456789012345678901234567890123456789012345678901234567890abcd"
    compilerVersion: "11.582.0"
  processing:
    isCompiled: true
    datetime: "2025-01-17T20:20:00"
    errors: []
```

### Gateway and Route Resources

#### Gateway Configuration

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: secure-gateway
  namespace: applications
spec:
  gatewayClassName: nginx
  infrastructure:
    parametersRef:
      name: nginx-proxy-waf
      group: gateway.nginx.org
      kind: NginxProxy
  listeners:
  - name: http
    port: 80
    protocol: HTTP
  - name: grpc
    port: 9090
    protocol: HTTP
    hostname: "grpc.example.com"
```

#### HTTPRoute Integration

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: protected-application
  namespace: applications
spec:
  parentRefs:
  - name: secure-gateway
  <...>
  # Inherits gateway-protection-policy WAFGatewayBindingPolicy automatically

---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: admin-route
  namespace: applications
spec:
  parentRefs:
  - name: secure-gateway
  <...>
  # Uses admin-strict-policy WAFGatewayBindingPolicy override via targetRefs
```

#### GRPCRoute Integration

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: GRPCRoute
metadata:
  name: protected-grpc-service
  namespace: applications
spec:
  parentRefs:
  - name: secure-gateway
  <...>
  # Inherits gateway-protection-policy WAFGatewayBindingPolicy automatically
```

### Cross-Namespace Policy References

```yaml
# ReferenceGrant to allow WAFGatewayBindingPolicy in applications namespace
# to reference APPolicy in security namespace
apiVersion: gateway.networking.k8s.io/v1
kind: ReferenceGrant
metadata:
  name: allow-WAFGatewayBindingPolicy-APPolicy-ref
  namespace: security
spec:
  from:
  - group: gateway.nginx.org
    kind: WAFGatewayBindingPolicy
    namespace: applications
  to:
  - group: waf.f5.com
    kind: APPolicy
  - group: waf.f5.com
    kind: APLogConf
```

## WAFGatewayBindingPolicy Status Conditions

### CRD Label

The `WAFGatewayBindingPolicy` CRD includes the `gateway.networking.k8s.io/policy: inherited` label to specify it as an inherited policy.

### Status Conditions

The `WAFGatewayBindingPolicy` includes status conditions following Gateway API policy patterns. Each condition has a Type and Reason:

#### Accepted Condition

The `Accepted` condition indicates whether the policy is valid and can be applied.

| **Reason**           | **Status** | **Description**                                                   | **Example Message**                                                    |
|----------------------|------------|-------------------------------------------------------------------|------------------------------------------------------------------------|
| `Accepted`           | True       | Policy is valid and accepted                                      | "The Policy is accepted"                                               |
| `Invalid`            | False      | Policy is syntactically or semantically invalid                   | "spec.APPolicySource.name is required"                                 |
| `TargetNotFound`     | False      | Target Gateway or Route does not exist                            | "Gateway 'secure-gateway' not found in namespace 'applications'"       |
| `Conflicted`         | False      | Another WAFGatewayBindingPolicy targets the same resource         | "WAFGatewayBindingPolicy 'other-policy' already targets this Gateway"  |

#### ResolvedRefs Condition

The `ResolvedRefs` condition indicates whether all referenced resources (APPolicy, APLogConf) are resolved and valid.

| **Reason**           | **Status** | **Description**                                                   | **Example Message**                                                    |
|----------------------|------------|-------------------------------------------------------------------|------------------------------------------------------------------------|
| `ResolvedRefs`       | True       | All APPolicy and APLogConf references are resolved                | "All references are resolved"                                          |
| `InvalidRef`         | False      | Referenced APPolicy or APLogConf does not exist or is not compiled| "APPolicy 'prod-policy' not found in namespace 'security'"             |
| `RefNotPermitted`    | False      | Cross-namespace reference not allowed by ReferenceGrant           | "Cross-namespace APPolicy reference requires ReferenceGrant"           |

#### Programmed Condition (Optional)

The `Programmed` condition indicates whether the policy has been successfully deployed to the data plane.

| **Reason**           | **Status** | **Description**                                                   | **Example Message**                                                    |
|----------------------|------------|-------------------------------------------------------------------|------------------------------------------------------------------------|
| `Programmed`         | True       | Policy and log profiles deployed to data plane                    | "Policy successfully deployed to Gateway"                              |
| `FetchError`         | False      | Failed to fetch bundle from PLM storage                           | "Failed to fetch policy bundle: connection timeout"                    |
| `IntegrityError`     | False      | Checksum verification failed                                      | "Policy integrity check failed: checksum mismatch"                     |
| `DeploymentError`    | False      | Data plane failed to apply policy                                 | "Failed to deploy WAF policy to NGINX Pods"                            |

### Example WAFGatewayBindingPolicy Status

```yaml
status:
  ancestors:
  - ancestorRef:
      group: gateway.networking.k8s.io
      kind: Gateway
      name: secure-gateway
      namespace: applications
    conditions:
    - type: Accepted
      status: "True"
      reason: Accepted
      message: "The Policy is accepted"
      lastTransitionTime: "2025-08-15T10:35:00Z"
    - type: ResolvedRefs
      status: "True"
      reason: ResolvedRefs
      message: "All references are resolved"
      lastTransitionTime: "2025-08-15T10:35:00Z"
    - type: Programmed
      status: "True"
      reason: Programmed
      message: "Policy successfully deployed to Gateway"
      lastTransitionTime: "2025-08-15T10:35:00Z"

  # Additional status information reflecting the referenced APPolicy status
  APPolicyStatus:
    name: "production-web-policy"
    namespace: "security"
    bundle:
      state: ready
      location: "s3://bucket_name/policies/prod-web-policy-v1.2.3.tgz"
      sha256: "abc123def456789012345678901234567890123456789012345678901234abcd"
    lastFetched: "2025-08-15T10:35:00Z"

  # Log profile status information reflecting the referenced APLogConf statuses
  APLogConfStatus:
  - name: "log-blocked-profile"
    namespace: "security"
    bundle:
      state: ready
      location: "s3://bucket_name/log-profiles/log-blocked-profile-v1.0.0.tgz"
      sha256: "def456789012345678901234567890123456789012345678901234567890abcd"
    lastFetched: "2025-08-15T10:35:00Z"
```

### Setting Status on Affected Objects

Following Gateway API policy patterns, NGF sets a condition on objects affected by WAFGatewayBindingPolicy:

```go
const (
    WAFGatewayBindingPolicyAffected gatewayv1alpha2.PolicyConditionType = "gateway.nginx.org/WAFGatewayBindingPolicyAffected"
    PolicyAffectedReason gatewayv1alpha2.PolicyConditionReason = "PolicyAffected"
)
```

Example condition on affected Gateway:

```yaml
conditions:
- type: gateway.nginx.org/WAFGatewayBindingPolicyAffected
  status: "True"
  reason: PolicyAffected
  message: "Gateway protected by WAFGatewayBindingPolicy 'gateway-protection-policy'"
  observedGeneration: 1
  lastTransitionTime: "2025-08-15T10:35:00Z"
```

## Implementation Details

### NGF Control Plane Changes

**APPolicy and APLogConf Watcher:**

- Watch APPolicy resources referenced by WAFGatewayBindingPolicy
- Watch APLogConf resources referenced by WAFGatewayBindingPolicy securityLogs
- Monitor status for compilation state and storage location
- Trigger policy/log profile fetch when status indicates successful compilation
- Handle multiple APLogConf references per WAFGatewayBindingPolicy

**Policy Fetcher:**

- Add support for fetching from PLM in-cluster storage API
- Implement HTTP/HTTPS client for in-cluster service communication
- Support configurable TLS with CA certificate validation
- Support mutual TLS with client certificates
- Handle API-specific error conditions and retries
- Verify bundle checksums against APPolicy/APLogConf status
- Fetch both policy and log profile bundles

**ReferenceGrant Validation:**

- Validate cross-namespace APPolicy references
- Validate cross-namespace APLogConf references
- Check for required ReferenceGrant resources
- Update WAFGatewayBindingPolicy status with permission errors if missing

**TLS Configuration Handling:**

- Watch PLM TLS secrets dynamically for changes
- Load CA certificates from Secret `ca.crt` field for server verification
- Load client certificates from Secret `tls.crt` and `tls.key` fields for mutual TLS
- Automatically update S3 fetcher TLS configuration when secrets change
- Support insecure connections for development scenarios via `--plm-storage-skip-verify`
- Validate TLS configuration at startup and on secret changes

### Data Plane Policy Deployment

**NGF Control Plane Managed**

NGF control plane handles all external communication and policy/log profile distribution:

1. **Watch APPolicy and APLogConf Status**: NGF watches referenced resources for `status.bundle.location` updates
2. **Fetch Bundles**: NGF fetches compiled policy and log profile bundles from in-cluster storage via S3 API
3. **Verify Integrity**: NGF validates bundle checksums against APPolicy/APLogConf `status.bundle.sha256`
4. **Distribute to Data Plane**: NGF writes both policy and log profile bundles to ephemeral volume via gRPC/Agent connection
5. **ConfigMgr Discovery**: ConfigMgr discovers policy and log profiles from local ephemeral volume

**Benefits:**
- Clear separation: control plane handles network, data plane handles local operations
- Centralized TLS configuration and error handling in control plane
- ConfigMgr remains simple with no external API dependencies
- Consistent with NGF's existing architecture patterns

**Implementation Notes:**
- NGF requires network access to PLM storage service (in-cluster service communication)
- ConfigMgr configuration points to local filesystem paths only
- Both policy and log profile bundles deployed to shared ephemeral volumes

### Policy Update Detection

**PLM-Managed Policies and Log Profiles (Automatic):**

- NGF watches APPolicy status changes via Kubernetes watch mechanism
- NGF watches APLogConf status changes via Kubernetes watch mechanism
- Automatic update when APPolicy `status.bundle.location` or `status.processing.datetime` changes
- Automatic update when APLogConf `status.bundle.location` or `status.processing.datetime` changes
- No polling required - event-driven updates via Kubernetes API
- Immediate propagation when PLM recompiles policies or log profiles

**Update Flow:**
1. User updates APPolicy or APLogConf spec
2. PLM Policy Controller triggers recompilation
3. PLM updates resource status with new bundle location and timestamp
4. NGF detects status change via watch
5. NGF fetches new bundle from in-cluster storage
6. NGF deploys updated policy/log profile to affected Gateway data planes

### Multi-Container Pod Orchestration

- NGINX container with NAP module
- WAF Enforcer sidecar
- WAF ConfigMgr sidecar per pod instance
- Ephemeral shared volumes for inter-container communication

## Security Considerations

### PLM Integration Security

**API Access Control:**

- NGF service account requires appropriate RBAC for APPolicy and APLogConf read access across namespaces
- In-cluster HTTP/HTTPS communication to PLM storage service
- Configurable TLS for secure communication (recommended for production)
- Cross-namespace references controlled via ReferenceGrant

**Policy Integrity:**

- Checksum validation of fetched policy and log profile bundles against APPolicy/APLogConf status
- TLS encryption for in-cluster communication when enabled
- Bundle signature verification via resource status metadata

**Network Security:**

- All policy distribution occurs within cluster boundaries
- No external network access required
- Configurable TLS with CA certificate validation
- Optional mutual TLS with client certificates
- Standard Kubernetes NetworkPolicy support for restricting in-cluster traffic
- PLM storage service accessible via Kubernetes service DNS
- Storage service credentials will be provided by PLM and stored in a Kubernetes Secret

**TLS Best Practices:**

- Production environments should enable TLS with CA certificate validation
- Use mutual TLS for enhanced security in sensitive environments
- Development/testing can use insecure HTTP for simplicity
- CA and client certificates provided via Kubernetes Secrets
- Certificate rotation supported through Secret updates (no pod restart required)
- TLS configuration is cluster-wide (all WafGatewayBindingPolicies use same settings)

### Policy Reference Security

**Cross-Namespace Access:**

- ReferenceGrant required for cross-namespace APPolicy references
- Explicit permission model prevents unauthorized policy use
- Clear error messages when references not permitted

**Policy Isolation:**

- Each Gateway deployment maintains independent policy state
- No shared policy storage between deployments
- Ephemeral volumes ensure clean state on pod restart

## Testing

### Unit Testing

- APPolicy reference resolution and validation
- APLogConf reference resolution and validation
- ReferenceGrant validation logic for both APPolicy and APLogConf
- APPolicy and APLogConf status watching and change detection
- In-cluster storage API client interactions (mocked)
- Bundle checksum verification for policies and log profiles
- TLS configuration parsing and validation
- Certificate loading from Secrets
- Multiple APLogConf references per WAFGatewayBindingPolicy

### Integration Testing

- Complete PLM integration flow from APPolicy/APLogConf creation to enforcement
- Cross-namespace policy and log profile references with ReferenceGrant
- Policy inheritance with PLM-managed policies and log profiles
- Policy override scenarios at Route level
- Policy and log profile update propagation from APPolicy/APLogConf changes
- Failure scenarios (resources not found, not compiled, fetch errors)
- In-cluster storage service communication with HTTP
- In-cluster storage service communication with HTTPS and TLS verification
- Mutual TLS with client certificates
- Certificate rotation scenarios
- Multiple log profiles per WAFGatewayBindingPolicy

### Performance Testing

- Policy and log profile fetch performance from in-cluster storage API
- Impact of watching multiple APPolicy and APLogConf resources at scale
- Multi-Gateway deployments with shared APPolicy and APLogConf references
- Policy and log profile update propagation time across multiple Gateways
- TLS handshake overhead for storage communication

## Questions/ Considerations

1. **PLM Storage Service**:
   - Rate limiting considerations for bundle fetches
   - The API will be S3 conformant, so we can use the AWS SDK

2. **Authentication and Credentials**:
   - Will PLM use static S3 credentials, or support other authentication methods (e.g., IAM roles, service account tokens)? -> Static credentials for now
   - How will credentials be provisioned to NGF? Will PLM create the Secret, or will operators need to create it manually? -> PLM creates this Secret
   - **Note**: NGF supports credential rotation via dynamic secret watching - when the credentials Secret is updated, NGF automatically updates the S3 client without pod restart

3. **TLS Configuration**:
   - Will TLS be required in production? -> Yes
   - What certificate chain will PLM storage use? -> User provided with no automatic rotation for now.
   - **Note**: NGF supports CA and client certificate configuration via Secrets, with dynamic rotation when Secrets are updated

4. **Policy Reload Mechanism**: Does NGINX require reload when policies or log profiles change?
   - No: can use [NAP reload functionality](https://docs.nginx.com/waf/configure/apreload/)
   - **Note**: NGF may not implement this in the initial phase

5. **Configuration Location (CLI flags vs NginxGateway CRD)**:
   - PLM storage configuration is currently proposed as CLI flags/Helm values (set at install time)
   - An alternative would be to configure PLM storage in the NginxGateway CRD, which would:
     - Allow dynamic configuration changes without pod restart
     - Consolidate control plane configuration in one Kubernetes-native resource
     - Expand the NginxGateway CRD beyond just dynamic logging configuration
   - Trade-offs to consider:
     - PLM storage config is infrastructure-level and unlikely to change frequently
     - Moving to CRD adds controller complexity for watching configuration changes
     - Consistency concern: if some control plane config is in CRD and some in CLI flags, this may be confusing
   - Decision deferred until PLM requirements are clearer and there's a broader vision for what NginxGateway CRD should contain

## References

- [NGINX App Protect WAF v5 Documentation](https://docs.nginx.com/nginx-app-protect-waf/v5/)
- [Gateway API Policy Attachment](https://gateway-api.sigs.k8s.io/reference/policy-attachment/)

## Appendix: Complete Configuration Example

```yaml
# 1. NGF Deployment with PLM storage configuration
# Note: When using Helm, these args are configured via values.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-gateway
  namespace: nginx-gateway
spec:
  template:
    spec:
      containers:
      - name: nginx-gateway
        image: ghcr.io/nginx/nginx-gateway-fabric:edge
        args:
        - --gateway-ctlr-name=gateway.nginx.org/nginx-gateway-controller
        - --gatewayclass=nginx
        # PLM storage configuration
        - --plm-storage-url=https://plm-storage-service.plm-system.svc.cluster.local
        - --plm-storage-credentials-secret=plm-storage-credentials
        - --plm-storage-ca-secret=plm-ca-secret
        - --plm-storage-client-ssl-secret=plm-client-secret

---
# 2. Enable WAF on NginxProxy
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: waf-enabled-proxy
  namespace: nginx-gateway
spec:
  waf: "enabled"

---
# 3. Define WAF policy using PLM APPolicy CRD
apiVersion: waf.f5.com/v1alpha1
kind: APPolicy
metadata:
  name: production-web-policy
  namespace: security
spec:
  policy:
    name: "prod-web-protection"
    template:
      name: "POLICY_TEMPLATE_NGINX_BASE"
    enforcementMode: "blocking"
    signatures:
    - signatureSetRef:
        name: "high-accuracy-signatures"
# Status updated by PLM after compilation
status:
  bundle:
    state: ready
    location: "s3://bucket_name/policies/prod-policy.tgz"
    sha256: "abc123def456789012345678901234567890123456789012345678901234abcd"
    compilerVersion: "11.582.0"
    signatures:
      attackSignatures: "2024-12-29T19:01:32"
      botSignatures: "2024-12-13T10:01:02"
      threatCampaigns: "2024-12-21T00:01:02"
  processing:
    isCompiled: true
    datetime: "2025-01-17T20:19:43"
    errors: []

---
# 4. Define logging profiles using PLM APLogConf CRDs
apiVersion: waf.f5.com/v1alpha1
kind: APLogConf
metadata:
  name: log-blocked-profile
  namespace: security
spec:
  content:
    format: splunk
    max_message_size: "10k"
  filter:
    request_type: "blocked"
status:
  bundle:
    state: ready
    location: "s3://bucket_name/log-profiles/log-blocked-v1.0.0.tgz"
    sha256: "def456789012345678901234567890123456789012345678901234567890abcd"
    compilerVersion: "11.582.0"
  processing:
    isCompiled: true
    datetime: "2025-01-17T20:20:00"
    errors: []

---
# 5. Create Gateway with WAF enabled
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: secure-gateway
  namespace: applications
spec:
  gatewayClassName: nginx
  infrastructure:
    parametersRef:
      name: waf-enabled-proxy
      group: gateway.nginx.org
      kind: NginxProxy
  listeners:
  - name: http
    port: 80
    protocol: HTTP
  - name: grpc
    port: 9090
    protocol: HTTP
    hostname: "grpc.example.com"

---
# 6. Allow cross-namespace APPolicy and APLogConf references
apiVersion: gateway.networking.k8s.io/v1
kind: ReferenceGrant
metadata:
  name: allow-wgbp-references
  namespace: security
spec:
  from:
  - group: gateway.nginx.org
    kind: WAFGatewayBindingPolicy
    namespace: applications
  to:
  - group: waf.f5.com
    kind: APPolicy
  - group: waf.f5.com
    kind: APLogConf

---
# 7. Gateway-level WAFGatewayBindingPolicy referencing APPolicy and APLogConf
apiVersion: gateway.nginx.org/v1alpha1
kind: WAFGatewayBindingPolicy
metadata:
  name: gateway-base-protection
  namespace: applications
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: secure-gateway
    namespace: applications

  APPolicySource:
    name: "production-web-policy"
    namespace: "security"

  securityLogs:
  - name: "blocked-requests"
    APLogConfSource:
      name: "log-blocked-profile"
      namespace: "security"
    destination:
      type: "Stderr"

---
# 8. HTTPRoute inheriting Gateway protection
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-route
  namespace: applications
spec:
  parentRefs:
  - name: secure-gateway
<...>
```

This complete example demonstrates:

- PLM-based policy definition using APPolicy CRD
- PLM-based logging configuration using APLogConf CRD
- Cross-namespace policy and log profile references with ReferenceGrant
- Gateway-level inherited protection
- Automatic policy and log profile updates when PLM recompiles
- Route protection with seamless policy inheritance
- Kubernetes-native policy and logging lifecycle management
