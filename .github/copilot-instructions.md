# Agent Instructions

NGINX Gateway Fabric (NGF) -- an implementation of the Kubernetes Gateway API with NGINX as the data plane. The control plane (controller-runtime) watches Gateway API and `gateway.nginx.org` resources, builds a validated graph, generates NGINX config, and pushes it over gRPC to NGINX Agent running in separate, controller-provisioned NGINX pods. The data plane has no Kubernetes API access.

## References

| Topic | Link |
| --- | --- |
| NGF docs | <https://docs.nginx.com/nginx-gateway-fabric/> |
| Gateway API | <https://gateway-api.sigs.k8s.io/> |
| NGINX directives | <https://nginx.org/en/docs/> |
| NGINX Plus admin guide | <https://docs.nginx.com/nginx/admin-guide/> |
| controller-runtime | <https://pkg.go.dev/sigs.k8s.io/controller-runtime> |
| Kubebuilder markers | <https://book.kubebuilder.io/reference/markers> |
| CRD validation (CEL) | <https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules> |
| NGINX Agent | <https://github.com/nginx/agent> |
| Developer docs | `docs/developer/`, `docs/architecture/`, `design/resource-validation.md` |

## API Groups

| API Group | Kinds |
| --- | --- |
| `gateway.nginx.org/v1alpha1` | AccessPolicy, AuthenticationFilter, ClientSettingsPolicy, ExternalLoadBalancer, NginxGateway, PayloadProcessor, ProxySettingsPolicy, RateLimitPolicy, SnippetsFilter, SnippetsPolicy, UpstreamSettingsPolicy, WAFPolicy |
| `gateway.nginx.org/v1alpha2` | NginxProxy, ObservabilityPolicy |

Types live in `apis/v1alpha1` and `apis/v1alpha2`. The controller also watches `gateway.networking.k8s.io` GatewayClass, Gateway, HTTPRoute, GRPCRoute, and (only when the CRD is installed) TCPRoute, UDPRoute, TLSRoute, ListenerSet, BackendTLSPolicy, ReferenceGrant, plus Inference Extension InferencePool behind a flag. Some kinds are registered only behind feature flags (see `featureFlagControllerCfgs` in `internal/controller/manager.go`). The Gateway API version is pinned in `go.mod` and `config/crd/gateway-api/`.

Use the codebase as the authoritative reference for patterns and style. Plan before implementing. Read files before modifying them. Where `docs/developer/*.md` and the code disagree, trust the code.

---

## Build, Test, Validate

| Command | Purpose |
| --- | --- |
| `make unit-test` | Go unit tests: `go test ./cmd/... ./internal/... -buildvcs -race -shuffle=on` with coverage |
| `make generate-all` | Run every generator (see Key Invariants); CI diffs the whole tree after it |
| `make generate` | `go generate ./...` only: moq fakes and telemetry schema |
| `make generate-crds` | controller-gen CRDs + deepcopy, writes `config/crd/bases` and `deploy/crds.yaml` |
| `make generate-policies` | Regenerate `apis/v1alpha{1,2}/policy_methods.go` |
| `make generate-helm-schema` / `generate-helm-docs` | `values.schema.json` / chart `README.md` |
| `make generate-manifests` | Regenerate `deploy/*/deploy.yaml` from `examples/helm/*/values.yaml` |
| `make verify-operator-rbac` | Check operator RBAC covers the Helm chart's RBAC |
| `make lint` | golangci-lint with `--fix` (edits files) then `logcheck` |
| `make fmt`, `make vet` | `go fmt`, `go vet` |
| `make helm-unit-test` | helm-unittest on `charts/nginx-gateway-fabric/tests` |
| `make lint-helm` | chart-testing (needs Docker) |
| `make njs-unit-test`, `make rust-unit-test` | NJS (vitest) and Rust module tests (need Docker) |
| `make -C tests test-cel-validation` | CRD CEL tests against a real API server (kind) |
| `make -C tests test GINKGO_LABEL=<label>` | Functional tests (kind + Docker) |
| `make lint-job-gating`, `make lint-workflow-gating` | Workflow gating validators |

Always use `make unit-test` over raw `go test`. After editing anything under `apis/`, run `make generate-all`.

CI (`vars` job in `ci.yml`) runs `go mod tidy` (root and `tests/`) and `make generate-all`, then `git diff --exit-code`. Commit every regenerated file.

---

## Project Layout

| Path | Purpose |
| --- | --- |
| `cmd/gateway/` | Binary: `controller`, `generate-certs`, `initialize`, `sleep` subcommands; CLI flags in `commands.go` |
| `apis/` | `gateway.nginx.org` CRD Go types (source of truth), generated `policy_methods.go`, `zz_generated.deepcopy.go` |
| `internal/controller/manager.go` | Wires controllers, policy manager, validators |
| `internal/controller/handler.go`, `handler_configapply.go` | Event batch handling, config apply with retry |
| `internal/controller/state/` | Change processor and cluster state store |
| `internal/controller/state/graph/` | Validates resources and links them into the `Graph` |
| `internal/controller/state/dataplane/` | Graph -> intermediate `Configuration` |
| `internal/controller/state/validation/` | Validator interfaces |
| `internal/controller/nginx/config/` | Configuration -> NGINX files (Go `text/template` in `*_template.go`), `policies/*`, `validation/` (concrete validators) |
| `internal/controller/nginx/agent/` | gRPC server and file/command services for NGINX Agent |
| `internal/controller/provisioner/` | Creates and owns each Gateway's NGINX Deployment, Service, ConfigMaps |
| `internal/controller/status/` | Status queue and updaters |
| `internal/controller/telemetry/` | Product telemetry (has generated files) |
| `internal/framework/` | Generic plumbing: controller registration, events, kinds, helpers |
| `charts/nginx-gateway-fabric/` | Helm chart (`crds` is a symlink to `config/crd/bases`) |
| `operators/` | Helm-based operator and bundle |
| `deploy/` | Generated manifests |
| `tests/` | Separate Go module: functional suite, CEL, conformance, framework |
| `build/` | Dockerfiles (NGF, NGINX OSS, NGINX Plus, Rust modules; `ubi/` variants) |
| `.github/workflows/` | CI/CD with a public repo + internal mirror split |

---

## Skills

| Skill | SDLC Stage | When to load |
| --- | --- | --- |
| `ngf-planning` | Plan | Starting any non-trivial task, creating implementation plans |
| `ngf-structure` | Plan + Dev | Exploring the codebase, tracing data flow, understanding architecture |
| `ngf-add-feature` | Dev | Adding NginxProxy fields, HTTPRoute filters or other route features, or Helm values |
| `ngf-add-policy` | Dev | Adding or extending a policy CRD |
| `ngf-testing` | Test | Writing unit, CEL, functional, Helm, NJS, or Rust tests |
| `ngf-debugging` | Bugfix | Diagnosing failures, config apply errors, status conditions |
| `ngf-ci-pipelines` | Review | Working on workflows, release flow, Dockerfiles, image builds |
| `ngf-code-review` | Review | Reviewing PRs (local chat, `pr-review` prompt, GitHub Copilot Code Review bot) |

---

## Key Invariants

- **Layering**: graph validates, dataplane carries data, `nginx/config` renders, the provisioner owns NGINX objects. `state/graph` must not import `nginx/config/*` (except `policies`), and `nginx/config` must not import `state/graph`. Rules, where new logic goes, and legacy exceptions not to copy are in `ngf-structure` ("Layering").
- **NGINX config injection**: Every user-supplied string that reaches NGINX config must be checked by a validator in `internal/controller/nginx/config/validation` (reached through the interfaces in `internal/controller/state/validation`), or be escaped by an existing helper. A template change can require a validation change. `design/resource-validation.md` is the policy: re-validate in Go whatever could break or hijack the config, because CRD validation can be bypassed.
- **Generated files**: Never hand-edit `zz_generated.deepcopy.go`, `apis/*/policy_methods.go`, `config/crd/bases/**`, `deploy/**`, `*fakes/` (moq), `internal/controller/telemetry/*_generated.go`, `data.avdl`, `docs/api/content.md`, chart `README.md` or `values.schema.json`. Change the source and run `make generate-all`. `charts/nginx-gateway-fabric/crds` is a symlink to `config/crd/bases`. A new CRD file must also be added by hand to `config/crd/kustomization.yaml`.
- **NGINX Plus**: Plus is selected only by the `--nginx-plus` flag (`cfg.Plus`). Validators reject Plus-only input with a status condition when Plus is off, and templates gate Plus-only output on `$.Plus`.
- **NGINX semantics**: Verify directive syntax, context and availability against <https://nginx.org/en/docs/> before adding it to a template. A wrong-context directive only fails when NGINX reloads.
- **Template tests**: There are no golden files or snapshots. Config output is tested by calling the `execute*` functions and asserting on the output. Every template change needs a test whose input renders the new directive and asserts it.
- **Validation parity**: Every new API field needs kubebuilder markers or CEL, Go validation in the graph or validator layer, a CEL test in `tests/cel`, unit tests (positive and negative), and a status condition for invalid input.
- **Snippets**: SnippetsFilter and SnippetsPolicy content is raw NGINX config by design. It is protected by the opt-in `--snippets` flag and RBAC, not by sanitising. Do not add new raw-config escape hatches.
- **RBAC**: A new watched kind needs rules in the chart (`rbac.namespacedRules` in `templates/_helpers.tpl`) and in `operators/config/rbac/role.yaml`; `make verify-operator-rbac` checks they match.
- **Go style**: `t.Parallel()` on every test and subtest, Gomega assertions, moq for fakes, handle an error once (do not log and return it), never log secrets. See `docs/developer/go-style-guide.md` and `docs/developer/logging-guidelines.md`.
- **Credentials**: NGINX Plus credentials are BuildKit `--secret` mounts, never `COPY`. CI secrets come from Azure Key Vault via OIDC, not repository secrets.
- **CI repo split**: Every job `if:` starts with `github.repository == 'nginx/nginx-gateway-fabric'` or `github.repository == vars.INTERNAL_REPOSITORY`, with extra conditions in one `( ... )` group. Release images are built in the internal mirror; the public repo only publishes. Never add a build step to a publish-stage workflow. Actions are pinned to a commit SHA with a version comment.

---

## Code Review

For any PR review -- local or via GitHub Copilot Code Review -- load and follow the [`ngf-code-review`](skills/ngf-code-review/SKILL.md) skill. It owns the review workflow, guardrails, dimension coverage, output format, and the "do not comment" list. It delegates codebase-specific rules to the domain skills (`ngf-structure`, `ngf-add-feature`, `ngf-add-policy`, `ngf-testing`, `ngf-ci-pipelines`).

Absolute minimum reviewer discipline:

- Comment only at >80% confidence.
- Be concise, actionable, and file+line specific.
- Verify NGINX directive semantics against <https://nginx.org/en/docs/> and NGF behaviour against <https://docs.nginx.com/nginx-gateway-fabric/> before flagging or approving config-generation changes; cite the page you used.
- Rate likelihood on runtime Blocking findings and report Remote ones separately, as `ngf-code-review` describes.
- Never post secrets, tokens, or license contents in review output.
