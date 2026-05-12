# Development Setup and Workflow

This document covers prerequisites, building, running locally, and the most commonly used Make targets.

## Prerequisites

Install the following before developing:

| Tool | Purpose |
|------|---------|
| [Go](https://golang.org/doc/install) 1.25+ | Primary language |
| [Docker](https://docs.docker.com/get-docker/) 18.09+ | Container image builds |
| [Kind](https://kind.sigs.k8s.io/) | Local Kubernetes clusters |
| [Kubectl](https://kubernetes.io/docs/tasks/tools/) | Kubernetes CLI |
| [Helm](https://helm.sh/docs/intro/quickstart/#install-helm) | Chart installation |
| [GNU Make](https://www.gnu.org/software/make/) | Build automation |
| [yq](https://github.com/mikefarah/yq/) | YAML processing |
| [git](https://git-scm.com/) | Version control |
| [pre-commit](https://pre-commit.com/) | Git hooks (`brew install pre-commit`) |

After cloning the repo, install pre-commit hooks and download Go dependencies:

```shell
pre-commit install
make deps
```

### GOARCH

The Makefile defaults to `GOARCH=amd64`. If your machine uses a different architecture (e.g., Apple Silicon), export it:

```shell
export GOARCH=arm64
```

## Building

### Build the Binary

```shell
make build
```

Outputs the binary to `build/out/gateway`.

### Build Container Images

```shell
make build-images TAG=$(whoami)
```

Builds both the NGF control plane image (`nginx-gateway-fabric:<tag>`) and the NGINX data plane image (`nginx-gateway-fabric/nginx:<tag>`).

For NGINX Plus (requires `nginx-repo.crt`, `nginx-repo.key`, and `license.jwt` in the repo root):

```shell
make build-images-with-plus TAG=$(whoami)
```

## Running Locally on Kind

The standard local development loop is:

### 1. Create a Kind cluster

```shell
make create-kind-cluster
```

Creates a Kind cluster using Kubernetes v1.35.1 (configurable) with dual-stack networking.

### 2. Build, load, and install in one step

```shell
make install-ngf-local-build
```

This runs `build-images` -> `load-images` -> `helm-install-local` in sequence. It installs Gateway API CRDs and deploys NGF via Helm with `NodePort` service type and `Never` pull policy.

For NGINX Plus:

```shell
make install-ngf-local-build-with-plus
```

### 3. Verify the installation

```shell
kubectl get pods -n nginx-gateway
kubectl get gatewayclasses
```

### 4. Deploy examples

Try the configurations in the `examples/` directory to verify NGF is working.

### 5. Clean up

```shell
make delete-kind-cluster
```

## Common Make Targets

### Build

| Target | Description |
|--------|-------------|
| `make build` | Compile the Go binary |
| `make build-images` | Build NGF + NGINX OSS Docker images |
| `make build-images-with-plus` | Build NGF + NGINX Plus Docker images |

### Development Workflow

| Target | Description |
|--------|-------------|
| `make dev-all` | Run deps, fmt, njs-fmt, vet, lint, unit-test, njs-unit-test (full dev check) |
| `make deps` | Tidy, verify, and download Go modules |
| `make fmt` | Run `go fmt` |
| `make vet` | Run `go vet` |
| `make lint` | Run golangci-lint with ~50 linters/formatters (auto-fixes where possible) |
| `make lint-helm` | Lint the Helm chart via chart-testing |

### Code Generation

| Target | Description |
|--------|-------------|
| `make generate` | Run `go generate ./...` |
| `make generate-crds` | Generate CRDs and Go types via controller-gen |
| `make generate-manifests` | Generate deployment manifests from the Helm chart |
| `make generate-helm-docs` | Generate Helm chart documentation |
| `make generate-helm-schema` | Generate Helm values JSON schema |
| `make generate-all` | Run all generation targets |

### Testing

| Target | Description |
|--------|-------------|
| `make unit-test` | Run Go unit tests with race detection and coverage |
| `make njs-unit-test` | Run NJS module tests via Node.js in Docker |

### Kind Cluster

| Target | Description |
|--------|-------------|
| `make create-kind-cluster` | Create a Kind cluster |
| `make delete-kind-cluster` | Delete the Kind cluster |
| `make load-images` | Load NGF + NGINX images into Kind |
| `make install-ngf-local-build` | Build, load, and Helm install NGF (OSS) |
| `make install-ngf-local-build-with-plus` | Build, load, and Helm install NGF (Plus) |
| `make install-crds` | Install NGF CRDs |
| `make install-gateway-crds` | Install Gateway API CRDs |

### Debugging

| Target | Description |
|--------|-------------|
| `make debug-build` | Build binary with debug symbols (no optimizations) |
| `make debug-build-images` | Build all images for debugging (includes dlv image) |
| `make debug-install-local-build` | Install debug build on Kind |

## Key References

- [docs/developer/quickstart.md](/docs/developer/quickstart.md) -- full development quickstart guide
- [Makefile](/Makefile) -- all targets with inline help (`make help`)
