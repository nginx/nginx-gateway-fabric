# Glossary

Concise definitions of project-specific and Gateway API terminology used in NGINX Gateway Fabric.

## Gateway API Resources

| Term | Definition |
|------|------------|
| **Gateway API** | A Kubernetes SIG-Network project that defines a standard, portable API for managing ingress and service mesh traffic. Successor to the Ingress API. |
| **GatewayClass** | A cluster-scoped resource that defines a class of Gateways. Specifies which controller implements Gateways of this class. Similar to IngressClass. |
| **Gateway** | A namespaced resource that requests a point where traffic can enter the cluster. Defines listeners (ports, protocols, hostnames) and references a GatewayClass. |
| **Listener** | A logical endpoint on a Gateway that accepts traffic on a specific port/protocol/hostname combination. |
| **HTTPRoute** | A route resource for HTTP traffic. Defines hostnames, path matches, header matches, and backend references. Attaches to Gateway listeners. |
| **GRPCRoute** | A route resource for gRPC traffic. Similar to HTTPRoute but with gRPC-specific matching. |
| **TLSRoute** | A route for TLS passthrough traffic (SNI-based routing without termination). |
| **TCPRoute** | A route for raw TCP traffic (L4). Experimental. |
| **UDPRoute** | A route for UDP traffic (L4). Experimental. |
| **ReferenceGrant** | A resource that grants permission for cross-namespace references (e.g., a Route in namespace A referencing a Service in namespace B). |
| **BackendTLSPolicy** | A policy for configuring TLS settings when connecting to backends. |

## NGF-Specific Terms

| Term | Definition |
|------|------------|
| **NGF** | NGINX Gateway Fabric -- this project. |
| **Control Plane** | The NGF controller that watches Kubernetes resources, builds a resource graph, generates NGINX configuration, and pushes it to data plane pods. Runs as a Deployment. |
| **Data Plane** | NGINX pods that handle actual traffic. Run NGINX plus the nginx-agent. Created dynamically per Gateway by the provisioner. |
| **NGINX Agent** | A sidecar process in data plane pods (v3) that receives configuration from the control plane over gRPC, writes files, and manages NGINX lifecycle (reload, health). |
| **Provisioner** | The control plane component that dynamically creates Kubernetes resources (Deployment, Service, ConfigMaps, etc.) for each valid Gateway. |
| **Graph Processor** | The component that builds an in-memory validated graph of all Gateway API resources and their relationships before configuration generation. |
| **NginxProxy** | An NGF CRD (v1alpha2) for configuring NGINX data plane settings: IP family, telemetry, logging, rewriteClientIP, and Kubernetes resource patches. The single source of truth for data plane customization. |
| **NginxGateway** | An NGF CRD (v1alpha1) for controller-level configuration: logging level and telemetry settings. |

## NGF Policy CRDs

| Term | Definition |
|------|------------|
| **ClientSettingsPolicy** | Policy for client connection settings (body size, timeouts). |
| **ProxySettingsPolicy** | Policy for proxy behavior (buffering, timeouts, headers). |
| **RateLimitPolicy** | Policy for request rate limiting. |
| **UpstreamSettingsPolicy** | Policy for upstream connection tuning (keepalives, load balancing). |
| **ObservabilityPolicy** | Policy for OpenTelemetry tracing and custom logging (v1alpha2). |
| **SnippetsFilter** | Filter for injecting raw NGINX config into specific routes. Requires `--snippets-filters` flag. |
| **SnippetsPolicy** | Policy for injecting raw NGINX config at the Gateway level. Requires `--snippets` flag. |
| **AuthenticationFilter** | Filter for HTTP Basic authentication. |

## Architecture Terms

| Term | Definition |
|------|------------|
| **controller-runtime** | A Go library (from Kubernetes SIG) for building controllers. NGF uses it for watches, caching, leader election, and health endpoints. |
| **Reconciliation** | The process of comparing desired state (Gateway API resources) with actual state and taking action to converge. In NGF, reconciliation generates NGINX config and pushes it to agents. |
| **Event Handler** | The NGF component that batches watch events and triggers the reconciliation loop. |
| **mTLS** | Mutual TLS -- both client and server authenticate via certificates. Used for control-plane-to-data-plane gRPC communication. |
| **TokenReview** | A Kubernetes API for validating ServiceAccount tokens. NGF uses it to authenticate agent connections. |
| **Owner Reference** | A Kubernetes mechanism for declarative resource ownership. NGF sets Gateway as owner of all provisioned resources, enabling garbage collection on Gateway deletion. |

## Testing Terms

| Term | Definition |
|------|------------|
| **Conformance Tests** | The official Gateway API test suite that verifies an implementation complies with the specification. |
| **Functional Tests** | NGF-specific integration tests that run on a live Kind cluster. |
| **NFR Tests** | Non-Functional Requirements tests for performance, scale, and reconfiguration latency. Run on GKE. |
| **Longevity Tests** | Tests that run NGF under continuous load for 72 hours to verify stability. |
| **CEL** | Common Expression Language -- used for validation rules in Kubernetes CRDs. |
| **Ginkgo** | A BDD-style Go testing framework used by NGF. |
| **Gomega** | A matcher/assertion library used with Ginkgo. |
| **Counterfeiter** | A tool for generating mock implementations of Go interfaces. |

## NGINX Terms

| Term | Definition |
|------|------------|
| **NGINX OSS** | The open-source version of NGINX. |
| **NGINX Plus** | The commercial version of NGINX with additional features (dynamic upstreams, advanced metrics, etc.). |
| **NJS** | NGINX JavaScript -- a module that allows running JavaScript code within NGINX for custom request processing. |
| **nginx -t** | NGINX config test command. The agent runs this before applying new configuration. |
| **nginx -s reload** | NGINX reload signal. Applies new configuration without dropping connections. |

## Key References

- [Gateway API Concepts](https://gateway-api.sigs.k8s.io/concepts/api-overview/)
- [ARCHITECTURE.md](/ARCHITECTURE.md) -- defines NGF-specific terms in context
- [apis/](/apis/) -- CRD type definitions
