# Testing

This document describes the test types in NGINX Gateway Fabric, how to run each, and what each is responsible for verifying.

## Test Types Overview

| Type | Location | Framework | What It Verifies |
|------|----------|-----------|------------------|
| Unit tests | `internal/**/*_test.go`, `cmd/**/*_test.go` | Go testing + Ginkgo/Gomega | Individual component behavior in isolation |
| NJS unit tests | `internal/controller/nginx/modules/` | Node.js (Jest) | NGINX JavaScript module correctness |
| Conformance tests | `tests/conformance/` | Gateway API conformance suite | Compliance with the Gateway API specification |
| Functional tests | `tests/suite/` | Ginkgo/Gomega | NGF-specific functionality on a live Kind cluster |
| CEL tests | `tests/cel/` | Go testing | CEL validation rules on CRDs |
| NFR / Scale tests | `tests/suite/` (with GKE) | Ginkgo/Gomega + wrk | Performance, scale, and reconfig latency |
| Longevity tests | `tests/suite/` (with GKE) | Ginkgo/Gomega + wrk | Stability over 72 hours under continuous load |

## Unit Tests

### Go Unit Tests

The project uses a mix of BDD-style tests (Ginkgo/Gomega) and standard Go tests:

- **All exported interfaces** must be covered by BDD-style Ginkgo tests
- **Most testing targets the exported interface layer** to decouple tests from implementation details
- **Table-driven tests** are preferred for testing multiple cases (Ginkgo `DescribeTable` or Go subtests)
- **Tests should run in parallel** -- add `t.Parallel()` to standard Go tests; Ginkgo tests run in parallel by default
- **Mocks** are generated with [Counterfeiter](https://github.com/maxbrunsfeld/counterfeiter)

```shell
make unit-test
```

Runs all Go unit tests with `-race` flag and generates `coverage.out` and `cover.html` in the project root.

### NJS Unit Tests

```shell
make njs-unit-test
```

Runs JavaScript tests for the NJS modules (`httpmatches.js`, `epp.js`) inside a Node.js Docker container.

### Full Development Check

```shell
make dev-all
```

Runs: `deps` -> `fmt` -> `njs-fmt` -> `vet` -> `lint` -> `unit-test` -> `njs-unit-test`.

## Conformance Tests

Verify that NGF conforms to the Gateway API specification. These tests run the upstream Gateway API conformance test suite against a live cluster.

### Prerequisites

- A running Kind cluster with NGF installed (or OpenShift for OpenShift-specific conformance)
- Gateway API CRDs installed

### Steps

```shell
# From the tests/ directory:
make install-ngf-local-build          # Build and install NGF on Kind
make build-test-runner-image          # Build the conformance test runner
make run-conformance-tests            # Run Gateway conformance tests
make run-inference-conformance-tests  # Run Inference conformance tests (optional)
make cleanup-conformance-tests        # Clean up test fixtures
```

To test experimental features, set `ENABLE_EXPERIMENTAL=true` before deploying. To test against the Gateway API `main` branch, set `GW_API_VERSION=main` and run `make update-go-modules`.

## Functional Tests

Test NGF-specific functionality on a live Kind cluster. Each parallel process gets its own NGF deployment, namespace, and port range.

```shell
# From the tests/ directory:
make test TAG=$(whoami)                                    # Full functional suite (4 parallel processes)
make test TAG=$(whoami) GINKGO_LABEL=graceful-recovery GINKGO_PROCS=2   # Specific label
make test TAG=$(whoami) GINKGO_LABEL=telemetry GINKGO_PROCS=1           # Single test
```

`GINKGO_PROCS` controls parallelism. Match it roughly to the number of specs being run to avoid unnecessary per-process NGF installs.

For NGINX Plus:

```shell
make test TAG=$(whoami) PLUS_ENABLED=true
```

## CEL Tests

Validate Custom Expression Language (CEL) rules defined on CRDs.

Located in `tests/cel/`. These are standard Go tests that verify CRD validation rules reject invalid input.

## NFR / Scale Tests

Non-Functional Requirements tests run on GKE clusters (not Kind). They measure performance, scale, and reconfiguration latency.

```shell
# Requires a GKE cluster and GCP VM -- see tests/README.md
make setup-gcp-and-run-nfr-tests
```

## Longevity Tests

Run NGF under continuous load for 72 hours on GKE. Started and stopped via GitHub Actions workflows or manually:

```shell
make start-longevity-test
# ... 72 hours later ...
make stop-longevity-test
```

Results are collected from GCP Monitoring dashboards and submitted via PR.

## Manual Testing

After deploying changes to a Kind cluster:

1. Check control plane logs: `kubectl -n nginx-gateway logs <ngf-pod>`
2. Check NGINX logs: `kubectl -n <ns> logs <nginx-pod>`
3. Inspect generated config: `kubectl exec -it -n <ns> <nginx-pod> -- nginx -T`
4. Verify resource statuses: `kubectl describe <resource> <name>`
5. Confirm traffic proxying works
6. Run the `examples/` to check for regressions

## CI Integration

Tests run automatically in GitHub Actions:

| Workflow | Tests Run |
|----------|-----------|
| `ci.yml` | Builds, unit tests, linting |
| `lint.yml` | golangci-lint, markdownlint, yamllint |
| `conformance.yml` | Gateway API conformance suite |
| `functional.yml` | Functional tests on Kind |
| `nfr.yml` | Scale and performance tests on GKE |
| `helm.yml` | Helm chart validation |

## Key References

- [docs/developer/testing.md](/docs/developer/testing.md) -- unit test guidelines, manual testing checklist
- [tests/README.md](/tests/README.md) -- full instructions for conformance, functional, and NFR tests
- [tests/Makefile](/tests/Makefile) -- all test-related Make targets
