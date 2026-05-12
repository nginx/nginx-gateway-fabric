# Dependencies

This document lists the major external dependencies of NGINX Gateway Fabric, what each is used for, and version constraints worth knowing.

## Go Module Dependencies

The project module is `github.com/nginx/nginx-gateway-fabric/v2` using Go 1.25.

### Core Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `sigs.k8s.io/gateway-api` | v1.5.1 | Gateway API types and CRDs. This defines the API surface that NGF implements. Version must match the Gateway API conformance level claimed. |
| `sigs.k8s.io/controller-runtime` | v0.23.3 | Framework for building Kubernetes controllers. Provides watches, caching, leader election, health endpoints, and the manager pattern. |
| `k8s.io/client-go` | v0.35.4 | Kubernetes API client. Used for direct API calls (TokenReview, dynamic resource creation). Version tracks controller-runtime's Kubernetes version. |
| `k8s.io/api` | v0.35.4 | Kubernetes API types (Deployments, Services, Secrets, etc.). |
| `k8s.io/apimachinery` | v0.35.4 | Kubernetes object metadata, runtime, and scheme utilities. |
| `k8s.io/apiextensions-apiserver` | v0.35.4 | CRD types for watching CustomResourceDefinition objects. |
| `github.com/nginx/agent/v3` | v3.9.0 | NGINX Agent protocol definitions (protobuf/gRPC). Defines the CommandService and FileService interfaces used for control-to-data-plane communication. |
| `google.golang.org/grpc` | v1.80.0 | gRPC framework for the agent server (mTLS, streaming, interceptors). |
| `google.golang.org/protobuf` | v1.36.11 | Protocol Buffers runtime for agent communication messages. |
| `sigs.k8s.io/gateway-api-inference-extension` | v1.5.0 | Gateway API Inference Extension types for AI workload routing. |

### Observability

| Dependency | Version | Purpose |
|------------|---------|---------|
| `go.opentelemetry.io/otel` | v1.43.0 | OpenTelemetry API for distributed tracing. |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.43.0 | OTLP gRPC trace exporter for sending traces to collectors. |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics client for exposing `/metrics` endpoint. |

### Application

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework for the `gateway` binary subcommands (controller, init, certs, endpoint-picker). |
| `go.uber.org/zap` | v1.28.0 | Structured logging backend (used via logr interface through go-logr/zapr). |
| `github.com/go-logr/logr` | v1.4.3 | Logging interface abstraction. All NGF code logs through this. |
| `github.com/fsnotify/fsnotify` | v1.9.0 | Filesystem event notifications for watching configuration file changes. |
| `github.com/google/uuid` | v1.6.0 | UUID generation for agent connection tracking. |
| `golang.org/x/text` | v0.36.0 | Text processing utilities. |
| `gopkg.in/evanphx/json-patch.v4` | v4.13.0 | JSON patch support for NginxProxy resource patching. |
| `github.com/dlclark/regexp2` | v1.12.0 | Full-featured regex engine (RE2 alternative) for advanced matching. |
| `github.com/nginx/telemetry-exporter` | v0.1.5 | NGF telemetry data export for usage reporting. |

### Testing

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/onsi/ginkgo/v2` | v2.28.2 | BDD-style test framework. All exported interface tests use Ginkgo. |
| `github.com/onsi/gomega` | v1.39.1 | Matcher/assertion library used with Ginkgo and standard Go tests. |
| `github.com/maxbrunsfeld/counterfeiter/v6` | v6.12.2 | Mock generator for Go interfaces. Declared as a `tool` directive in `go.mod`. |
| `github.com/google/go-cmp` | v0.7.0 | Deep equality comparison for test assertions. |

## Tooling Dependencies (Managed by Makefile)

These are not in `go.mod` but are pinned in the `Makefile` and managed by Renovate:

| Tool | Version | Purpose |
|------|---------|---------|
| golangci-lint | v2.11.4 | Go linting (~50 linters/formatters) |
| controller-gen | v0.20.1 | CRD generation from Go types (kubebuilder) |
| helm-docs | v1.14.2 | Helm chart documentation generation |
| helm-schema | v0.23.2 | Helm values JSON schema generation |
| gen-crd-api-reference-docs | v0.3.0 | API reference documentation generation |
| Node.js | v24 | NJS module development tooling and test runner |
| chart-testing | v3.14.0 | Helm chart linting |

## Data Plane Dependencies

| Component | Version | Notes |
|-----------|---------|-------|
| NGINX OSS | 1.30.0 (edge) | The NGINX reverse proxy that handles traffic. Versioned independently from NGF. |
| NGINX Plus | R36 | Optional commercial version with dynamic upstreams and advanced metrics. Requires a license. |
| NGINX Agent v3 | v3.9.0 | Runs in data plane pods. Receives config from control plane, manages NGINX lifecycle. |

## Kubernetes Version Constraints

| NGF Version | Minimum Kubernetes | Gateway API |
|-------------|-------------------|-------------|
| Edge | 1.31+ | v1.5.1 |
| v2.5.1 | 1.31+ | v1.5.1 |
| v2.4.x | 1.25+ | v1.4.1 |
| v2.0.x - v2.3.x | 1.25+ | v1.3.0 - v1.4.1 |

See the [Technical Specifications](/README.md#technical-specifications) table in the README for the complete version matrix.

## Dependency Management

- **Go modules** are used for Go dependencies (`go.mod` / `go.sum`)
- **Renovate** automates dependency update PRs (configured in `renovate.json`). Bot-created PRs follow the pattern `Update <type> <package> to <version>`.
- Tests have a **separate `go.mod`** in `tests/` to isolate test-specific dependencies from the main module.
- The main module uses a `tool` directive for Counterfeiter, meaning `go tool counterfeiter` works without a separate install step.

## Key References

- [go.mod](/go.mod) -- full dependency list with versions
- [Makefile](/Makefile) -- tooling version pins (search for `_VERSION` variables)
- [README.md Technical Specifications](/README.md#technical-specifications) -- version compatibility matrix
- [renovate.json](/renovate.json) -- Renovate configuration
