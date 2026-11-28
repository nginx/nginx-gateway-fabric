---
name: ngf-testing
description: 'Testing patterns for NGF: Go unit tests (Gomega, t.Parallel, moq fakes), nginx config generation tests, CEL tests, functional and conformance tests, Helm, NJS and Rust tests. Use when writing or running any test.'
---

# NGF Testing Patterns

## Commands

| Command | Purpose |
| --- | --- |
| `make unit-test` | `go test ./cmd/... ./internal/... -buildvcs -race -shuffle=on` with coverage (`coverage.out`, `cover.html`) |
| `make generate` | Regenerate moq fakes and telemetry files |
| `make lint` | golangci-lint with `--fix` (edits files), then `logcheck` |
| `make fmt`, `make vet` | `go fmt`, `go vet` |
| `make helm-unit-test` | helm-unittest on the chart (needs helm) |
| `make lint-helm` | chart-testing (Docker) |
| `make njs-unit-test` | vitest in a Node container (Docker) |
| `make rust-unit-test`, `make rust-lint` | Rust crates `ai-guardrails`, `inference` (Docker) |
| `make -C tests test-cel-validation [CEL_TEST_TARGET=TestName]` | CEL tests against a real API server (kind) |
| `make -C tests test GINKGO_LABEL=<label>` | Functional tests (kind + Docker) |
| `make -C tests test-framework-unit` | Unit tests of the test framework (no cluster) |

Always use `make unit-test` over raw `go test`. No build tags are used for unit tests (conformance uses `//go:build conformance`).

---

## Go Unit Tests

Rules (`docs/developer/testing.md`, `go-style-guide.md`):

- `t.Parallel()` on every test and subtest (lint: `tparallel`). If a test must run sequentially, omit it and say why in a comment.
- Gomega assertions. Plain `testing` with `NewWithT(t)` is the dominant style; Ginkgo `Describe`/`DescribeTable` is used for exported-interface suites (`*_suite_test.go` with `RegisterFailHandler(Fail); RunSpecs(t, ...)`). No committed `FIt`/`FDescribe`.
- Table-driven subtests.
- Tests are mostly in-package (white-box), e.g. `package graph`, `package config`.
- Compare structs with `helpers.Diff(want, got)` (wraps `cmp.Diff`); build pointers with `helpers.GetPointer`.
- Do not assert on log output.
- A bug fix starts with a unit test that reproduces the bug. A feature needs positive and negative cases. Check `cover.html`; codecov is informational only.

```go
for _, test := range tests {
	t.Run(test.name, func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		result := functionUnderTest(test.input)
		g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
	})
}
```

Lint rules that bite tests (`.golangci.yml`): `thelper` (call `t.Helper()` in helpers), `lll` 120 columns, `govet fieldalignment` (order test-case struct fields), dot-imports only for gomega and ginkgo/v2, goimports local prefix `github.com/nginx/nginx-gateway-fabric`.

### Fakes (moq)

- Directive: `//go:generate go tool moq -skip-ensure -pkg configfakes -out configfakes/fake_generator.go . Generator`. `go.mod` has `tool github.com/matryer/moq`.
- Regenerate with `make generate`. Never hand-edit `*fakes/`.
- Use: `&telemetryfakes.ExporterMock{ExportFunc: func(...) error {...}}`; assert with `g.Expect(fake.GenerateCalls()).To(HaveLen(1))`.
- Two older counterfeiter fakes remain and are not regenerated: `policiesfakes/fake_validator.go`, `pollerfakes/fake_poller_manager.go`.

---

## Testing NGINX Config Generation

There are no golden files or snapshots. Tests call the unexported `execute*` functions (`executeUpstreams`, `executeMaps`, `executeBaseHTTPConfig`, ...) which return `[]executeResult{dest, data}`, and assert on the output: the destination file, `ContainSubstring`, or `strings.Count` against a map of expected substrings (see `upstreams_test.go`). Files: `internal/controller/nginx/config/*_test.go` and `policies/*/generator_test.go`.

Rules:

- Every template or generator change needs a test whose input sets the new field and asserts the new directive. If you delete the template line and no test fails, the change is untested.
- Test both Plus and OSS output for Plus-gated directives; assert the Plus-only directive is absent when Plus is off.
- Validation changes need negative tests with input that would break or hijack the config (`;`, `{`, `}`, `$`, quotes, newlines).

---

## CEL Tests (`tests/cel`)

They check CRD `XValidation` and kubebuilder rules by creating objects on a real API server.

1. Add the expected error string as a const in `tests/cel/common.go`.
2. Add a table test in `tests/cel/<crd>_test.go` with `{spec, name, wantErrors []string}` and `t.Parallel()`.
3. `getKubernetesClient(t)`, `uniqueResourceName(...)`, then `validateCrd(t, tt.wantErrors, obj, k8sClient)` or `validateCrdUpdate(...)`.
4. Copy `tests/cel/clientsettingspolicy_test.go`.

---

## Functional and NFR Tests (`tests/`, separate Go module)

- Ginkgo specs in `tests/suite/*_test.go` (`package main`); manifests in `tests/suite/manifests/<feature>/`; helpers in `tests/framework/`. Reference spec: `tests/suite/sample_test.go`.
- Every functional spec has `Label("functional", "<feature>")`. Other labels: `nfr` (+ `scale|upgrade|performance|reconfiguration|zero-downtime-scale`), `waf`, `gatewaylink`, `telemetry`, `graceful-recovery`, `node-recovery`.
- Run: `make create-kind-cluster`, `make build-images load-images TAG=$(whoami)`, then `make -C tests test GINKGO_LABEL=<label>`. Use `GINKGO_PROCS=1` for a single label. `test-with-plus` and `test-waf` need Plus credentials. Details in `tests/README.md`.
- Use a per-process namespace (`GinkgoParallelProcess()`), `resourceManager.ApplyFromFiles`, and `framework.AddNginxLogsAndEventsToReport` in `AfterEach`.
- Wait by polling, never sleep: `Eventually(func(g Gomega){...}).WithTimeout(timeoutConfig.RequestTimeout).WithPolling(500*time.Millisecond).Should(Succeed())`, or `rm.WaitFor*`. Timeouts come from `framework.DefaultTimeoutConfig()`.
- Do not add `FlakeAttempts` to hide a flake (only `tracing_test.go` has one). Log through `GinkgoWriter`. Never commit `F`/`X` focus prefixes.

## Conformance

`tests/conformance/conformance_test.go` (`//go:build conformance`): `make build-test-runner-image` then `make run-conformance-tests` from `tests/`. Profiles, skips and OpenShift feature lists are `tests/Makefile` variables.

## Helm

helm-unittest files in `charts/nginx-gateway-fabric/tests/*_test.yaml` (`suite`, `templates`, `tests: - it: ... set: ... asserts: [hasDocuments, isKind, equal, notEqual]`), values in `tests/values/`.

## NJS and Rust

- NJS: `internal/controller/nginx/modules/src/httpmatches.js`, tests in `test/httpmatches.test.js` with vitest (the module README still says Mocha; trust `package.json`).
- Rust: inline `#[cfg(test)]` modules in `internal/controller/nginx/modules/rust/*`; unit tests cannot call nginx functions.

## Gotchas

- Generated files must be regenerated, not edited: run `make generate-all` and commit the result.
- `make lint` edits files; review `git diff` afterwards.
- The `tests/` module is linted by pre-commit; run `go mod tidy` there too.
