# Repository Structure

This file describes each significant top-level folder in the NGINX Gateway Fabric repository. Generated output directories (e.g., `build/out/`) and the `.git/` directory are omitted.

## Source Code

| Folder | Description |
|--------|-------------|
| `cmd/gateway/` | Single entry point for the NGF binary. Contains `main.go`, CLI subcommands (controller, init container, cert generation, endpoint picker), and flag validation. |
| `internal/` | Core implementation, split into two packages. `internal/controller/` holds the main controller logic: the controller-runtime manager, event handler, graph processor, NGINX config generator, gRPC agent server, provisioner, status updater, and telemetry. `internal/framework/` holds shared utilities for controller helpers, event handling, file operations, and common helpers. |
| `apis/` | Custom Resource Definition (CRD) Go types. `v1alpha1/` defines NginxGateway, policy types (ClientSettings, ProxySettings, RateLimit, UpstreamSettings), filter types (Authentication, Snippets), and SnippetsPolicy. `v1alpha2/` defines NginxProxy and ObservabilityPolicy. |

## Configuration and Deployment

| Folder | Description |
|--------|-------------|
| `charts/nginx-gateway-fabric/` | Helm chart for installing NGF. Includes `Chart.yaml`, `values.yaml`, `values.schema.json`, and templates for the control plane Deployment, RBAC, Services, cert generation Job, GatewayClass, and optional NginxProxy/NginxGateway resources. |
| `config/` | Kubernetes manifests for CRD definitions (`config/crd/bases/`), kustomize overlays for Gateway API CRDs (standard and experimental channels), inference extension CRDs, and test fixtures. |
| `deploy/` | Pre-rendered deployment manifests generated from the Helm chart. Variants include `default/`, `nginx-plus/`, `experimental/`, `snippets/`, and `openshift/`. |
| `build/` | Dockerfiles for container images. `Dockerfile` builds the control plane (multi-stage to scratch). `Dockerfile.nginx` and `Dockerfile.nginxplus` build the NGINX OSS and Plus data plane images. The `ubi/` subdirectory holds Red Hat UBI variants. Also contains the data plane `entrypoint.sh`. |

## Documentation

| Folder | Description |
|--------|-------------|
| `docs/` | Developer-facing documentation. `docs/developer/` contains the quickstart guide, branching workflow, pull request guidelines, Go style guide, logging guidelines, testing guide, release process, and design principles. `docs/architecture/` has simplified architecture diagrams. `docs/proposals/` holds enhancement proposals. `docs/api/` has API doc generation templates. |
| `design/` | Historical architecture design documents. The `archive/` subfolder contains older designs that informed the current architecture. |
| `examples/` | Example Gateway API configurations for users to deploy and test common use cases (routing, TLS, policies). |

## Testing

| Folder | Description |
|--------|-------------|
| `tests/` | All non-unit test suites. Has its own `go.mod` for test dependencies. `tests/conformance/` runs the Gateway API conformance suite. `tests/suite/` contains functional test cases. `tests/cel/` has CEL validation tests. `tests/ipv6/` has IPv6-specific tests. `tests/framework/` provides test utilities. `tests/scripts/` holds test helper scripts. `tests/results/` stores NFR test results. |

## CI, Build, and Operations

| Folder | Description |
|--------|-------------|
| `.github/` | GitHub configuration. `workflows/` contains 21+ GitHub Actions workflows for CI, builds, linting, conformance testing, functional testing, NFR testing, releases, security scanning, and dependency management. Also includes issue/PR templates, CODEOWNERS, and release config. |
| `scripts/` | Build helper scripts for manifest generation (`generate-manifests.sh`) and CRD processing (`strip-crd-excludes.sh`). |
| `operators/` | Operator Lifecycle Manager (OLM) operator bundle for Red Hat OpenShift certification. Contains the operator Dockerfile, bundle metadata, and RBAC sync verification scripts. |
| `debug/` | Dockerfile for building a `dlv` (Delve) debugger image, used for attaching a debugger to the NGF binary during development. |

## Key Top-Level Files

| File | Description |
|------|-------------|
| `Makefile` | Primary build automation (~297 lines, 40+ targets). Covers building, linting, testing, image creation, Kind cluster management, Helm installs, code generation, and debugging. |
| `go.mod` / `go.sum` | Go module definition for `github.com/nginx/nginx-gateway-fabric/v2`. Uses Go 1.25. |
| `ARCHITECTURE.md` | Comprehensive architecture document (943 lines) covering all components, data flows, and design decisions. |
| `.golangci.yml` | golangci-lint v2 configuration with ~50 linters and formatters enabled. |
| `.pre-commit-config.yaml` | Pre-commit hooks (12 hooks): trailing whitespace, YAML check, gitleaks, golangci-lint, prettier, markdownlint, yamllint, doctoc, helm-docs, shfmt, helm-schema. |
| `.goreleaser.yml` | GoReleaser configuration for building release binaries and generating SBOMs. |

## Key References

- [ARCHITECTURE.md](/ARCHITECTURE.md) -- includes a detailed directory tree with file-level descriptions
- [CONTRIBUTING.md](/CONTRIBUTING.md) -- project structure summary in the "Getting Started" section
