---
name: ngf-structure
description: 'NGF architecture, resource processing pipeline, control/data plane split, validation layers and generated artifacts. Use when exploring the codebase, tracing data flow, debugging config generation, or working on controller logic.'
---

# NGF Architecture and Structure

## Repository Layout

```text
cmd/gateway/                         Binary: controller, generate-certs, initialize, sleep (flags in commands.go)
apis/v1alpha1, apis/v1alpha2/        gateway.nginx.org CRD types (source of truth)
  policy_methods.go                  Generated (make generate-policies)
  zz_generated.deepcopy.go           Generated (make generate-crds)
apis/waf/v1/                         Status parsers for F5 APPolicy/APLogConf (NGF does not own those CRDs)
internal/controller/
  manager.go                         Wires controllers, policy manager, validators, feature flags
  handler.go, handler_configapply.go Event batch handling, config apply with retry
  state/change_processor.go          Applies events to the store, builds the Graph
  state/graph/                       Validate resources and link them into the Graph
  state/dataplane/                   Graph -> intermediate Configuration
  state/validation/                  Validator interfaces (implementations live in nginx/config/validation)
  state/resolver/, mirror/, conditions/
  nginx/config/                      Configuration -> nginx files (text/template in *_template.go)
  nginx/config/policies/<name>/      Per-policy validator.go + generator.go
  nginx/config/validation/           Concrete validators (injection guards)
  nginx/agent/                       gRPC Command/File services, NginxUpdater, broadcaster
  nginx/conf/, nginx/modules/        Static nginx.conf files, NJS and Rust modules
  provisioner/                       Creates and owns each Gateway's NGINX Deployment/Service/ConfigMaps
  status/                            Status queue and leader-aware updaters
  telemetry/, licensing/, crd/, metrics/
internal/framework/                  controller (Register/Reconciler), events (EventLoop), kinds, helpers, waf
charts/nginx-gateway-fabric/         Helm chart (crds is a symlink to config/crd/bases)
config/crd/                          bases (generated), gateway-api and inference-extension (kustomize refs)
deploy/                              Generated manifests
operators/                           Helm-based operator and bundle
tests/                               Separate Go module: suite, cel, conformance, framework
build/                               Dockerfile, Dockerfile.nginx, Dockerfile.nginxplus, Dockerfile.rustmodules, ubi/
```

---

## Control Plane and Data Plane

- The control plane runs a gRPC server (`internal/controller/nginx/agent/grpc`). Agent connections use TLS and are authenticated by a TokenReview interceptor.
- Each Gateway gets its own NGINX Deployment, created and owned by the provisioner. Manual edits to those objects are reverted; the NginxProxy resource is the source of truth (`docs/architecture/provisioning.md`).
- The data plane has no Kubernetes API access (`docs/architecture/README.md`).
- NGINX config templates are Go string constants in `internal/controller/nginx/config/*_template.go`, parsed with `text/template` and run through `helpers.MustExecuteTemplate`. Bootstrap templates (main, mgmt, agent, events) are in `provisioner/templates.go`.
- **Plus** is selected only by `--nginx-plus` (`cfg.Plus`). It reaches `NewGeneratorImpl`, `BuildConfiguration`, `graph.FeatureFlags.Plus` and the UpstreamSettingsPolicy validator.

---

## Resource Processing Pipeline

```text
Kubernetes change
  -> controller-runtime Reconciler  [framework/controller/reconciler.go]
  -> UpsertEvent/DeleteEvent on eventCh  [framework/events]
  -> EventLoop batches events  [framework/events/loop.go]
  -> eventHandlerImpl.HandleEventBatch  [handler.go]
  -> ChangeProcessorImpl.Process  [state/change_processor.go]
       store update; returns nil if nothing relevant changed
       -> graph.BuildGraph  [state/graph/graph.go]
  -> sendNginxConfig, per Gateway  [handler.go]
       -> dataplane.BuildConfiguration  [state/dataplane/configuration.go]
       -> applyConfigWithRetry  [handler_configapply.go]
            -> GeneratorImpl.Generate  [nginx/config/generator.go]  -> GenerateResult{ConfigApplyFiles, ContentUpdateFiles}
            -> NginxUpdaterImpl.UpdateConfig  [nginx/agent/agent.go]  -> Broadcaster.Send
                 agent receives ConfigApplyRequest, pulls files via fileService.GetFile
                 Plus only: UpdateUpstreamServers through the Plus API
  -> status queue  -> status.Prepare*Requests -> statusUpdater.UpdateGroup (only when leader)
```

---

## Validation Layers

| Layer | Where | Notes |
| --- | --- | --- |
| CRD | kubebuilder markers and CEL in `apis/**`; tested in `tests/cel` | Can be bypassed |
| Graph | e.g. `validateNginxProxy`, `validateFilter` in `state/graph` | Data-plane-agnostic checks; reports conditions |
| Data-plane validators | `nginx/config/validation` behind the `state/validation` interfaces | Tied to the templates; guards against config injection |
| Policy validators | `nginx/config/policies/<name>/validator.go` | Return `[]conditions.Condition` |

`design/resource-validation.md` is the policy: re-validate in Go whatever could break or hijack nginx config, report problems in status, no webhook. The generator does not validate (`generator.go`); callers must validate first. `validation.SkipValidator` exists only for internally created mirror routes.

Injection guards (`nginx/config/validation/common.go`): `validateEscapedString`, `validateEscapedStringNoVarExpansion` (no `$`), `validateSingleQuotedString`, `validateNoLineBreaks`, `validatePath` (`pathFmt` rejects whitespace, `{`, `}`, `;`, and `$`). `GenericValidator`, `HTTPValidator` and `AuthFieldValidator` expose them. Escaping helpers: `helpers.SanitizeNginxVar`, `escapeForNginxVar` in `nginx/config/maps.go`.

---

## Generated Artifacts -- never hand-edit

`make generate-all` runs `generate generate-crds generate-helm-schema generate-manifests generate-api-docs generate-helm-docs generate-policies verify-operator-rbac`. CI re-runs it and diffs the whole tree.

| Artifact | Source | Target |
| --- | --- | --- |
| `apis/*/zz_generated.deepcopy.go`, `config/crd/bases/*.yaml`, `deploy/crds.yaml` | Types and kubebuilder markers | `make generate-crds` |
| `apis/v1alpha{1,2}/policy_methods.go` | Types labelled `gateway.networking.k8s.io/policy` (`apis/policygen.go`) | `make generate-policies` |
| `deploy/<variant>/deploy.yaml` | `helm template` of `examples/helm/*/values.yaml` (`scripts/generate-manifests.sh`) | `make generate-manifests` |
| `docs/api/content.md` (gitignored) | CRD types | `make generate-api-docs` |
| chart `README.md`, `values.schema.json` | `values.yaml` comments and `@schema` blocks | `make generate-helm-docs`, `make generate-helm-schema` |
| `*fakes/fake_*.go` | `//go:generate go tool moq ...` | `make generate` |
| `internal/controller/telemetry/*_generated.go`, `data.avdl` | `//go:generate` in `collector.go` | `make generate` |

Notes:

- `charts/nginx-gateway-fabric/crds` is a symlink to `config/crd/bases`. A new CRD must be added by hand to `config/crd/kustomization.yaml`.
- Fakes use moq, not counterfeiter. Two older counterfeiter fakes remain (`policiesfakes/fake_validator.go`, `pollerfakes/fake_poller_manager.go`) and are not regenerated.
- `make verify-operator-rbac` only checks; it writes nothing.

---

## Layering

Intended dependency direction: a layer may import the layers above it (the arrows point at what it depends on), not below it. Nothing enforces it yet (no `depguard`), so follow it by hand:

```text
apis/, internal/framework/            leaf: never import internal/controller
  ^
state/conditions, state/resolver,     shared vocabulary
nginx/config/{http,shared,policies}
  ^
state/validation (interfaces)  <-  nginx/config/validation (implementations)
  ^
state/graph        validate and link resources; data-plane agnostic
  ^
state/dataplane    Graph + one Gateway -> Configuration; the only thing the generator should need
  ^
nginx/config       Configuration -> files (imports nginx/agent only for the File type)
nginx/agent        files -> gRPC push (imports dataplane for Plus upstream updates and status for errors)
  ^
handler.go, provisioner/, manager.go, status/    orchestration; may import everything above
```

Rules for new code:

- `state/graph` does not import `nginx/config/*`, except the shared `policies` contract. Put new shared types or constants in a neutral package, not under `nginx/config`.
- `nginx/config` (templates, generators) takes its input from `dataplane.Configuration` only. Do not import `state/graph`; add the field to `dataplane` instead.
- Validation lives in `state/graph` or behind the `state/validation` interfaces, never in a generator or template.
- Only `ChangeProcessorImpl` changes cluster state, and a built `Graph` is read-only afterwards.
- Only the provisioner creates or updates the NGINX Deployment, Service and ConfigMaps.
- Do not add new responsibilities to `handler.go`; put new behaviour in its own file or collaborator.
- Use `helpers.GetPointer`, `helpers.MustCastObject` and `helpers.Diff` instead of local copies.

Where new logic goes:

| You are adding... | Put it in |
| --- | --- |
| A field and its CRD/CEL validation | `apis/**` |
| Data-plane-agnostic validation, linking, conditions | `state/graph` |
| An NGINX-specific value check (injection guard) | `nginx/config/validation`, reached through `state/validation` |
| Data the templates need | `state/dataplane` types and `BuildConfiguration` / `convert.go` |
| A directive | `nginx/config/*_template.go` plus its `execute*` function |
| A per-policy validator or generator | `nginx/config/policies/<name>/` |

### Known exceptions (do not copy them)

| Exception | Where | Status |
| --- | --- | --- |
| `state/graph` imports `nginx/config/http` and `shared` for constants and `ParseSize` | `httproute.go`, `grpcroute.go`, `nginxproxy.go` | Legacy. Remove when touched. |
| `nginx/config` imports `state/graph` for Plus secret keys, `AppProtocolTypeH2C` and WAF bundle-key helpers | `servers.go`, `main_config.go`, `policies/waf/generator.go` | Legacy. Carry the value in `dataplane` instead. |
| `dataplane.Configuration` carries raw CRD objects as `[]policies.Policy`; generators cast back to `ngfAPI` types | `state/dataplane/types.go`, `policies/*/generator.go` | Deliberate: each policy is a self-contained slice. Do not extend the pattern to non-policy kinds. |
| `BuildGraph` does network I/O (WAF and PLM bundle fetches) | `graph/policies.go` | Deliberate for now. Do not add new I/O to graph building. |
| The handler edits the built Graph (`mergeWAFPollErrors`, `mergeWAFBundleUpdates`) | `handler.go` | Legacy. Do not add more post-build mutation. |
| `BuildConfiguration` reads EndpointSlices from the cache, and the resolver records `EndpointSliceOwnership`, which the ChangeProcessor uses to detect rebuilds | `state/resolver` | Deliberate (delete events carry no labels). Do not add similar feedback loops. |
| `nginx/agent` writes apply errors to the status queue | `nginx/agent/command.go` | Deliberate (apply results arrive asynchronously). |
| `BuildGraph` fills `plusSecrets` in place and panics on a missing field | `graph.go` `setPlusSecretContent` | Legacy. |

Kubernetes reads happen through the controller cache (`ServiceResolver` in `BuildConfiguration`) and the WAF fetchers passed to `BuildGraph`, not through reconcilers alone.

## Custom Kinds and Watches

- Kinds in `gateway.nginx.org`: see `.github/copilot-instructions.md`. There is no v1alpha1 NginxProxy or ObservabilityPolicy any more.
- Always watched: GatewayClass, Gateway, HTTPRoute, GRPCRoute, ConfigMap, Service, Secret (label-filtered), EndpointSlice, Namespace, and the NGF policy/filter kinds. NginxGateway only when `ConfigName` is set.
- Only if the CRD exists: TCPRoute, UDPRoute, TLSRoute, ListenerSet, BackendTLSPolicy, ReferenceGrant.
- Behind flags (`featureFlagControllerCfgs`): InferencePool, SnippetsFilter/SnippetsPolicy (`--snippets`), ExternalLoadBalancer, PayloadProcessor, APPolicy/APLogConf.
- Gateway API and inference extension versions: `go.mod` and `config/crd/gateway-api/*/kustomization.yaml`. `validateCRDVersions` (`graph/gatewayclass.go`) compares installed CRDs with the bundle version.

## Error Handling and Conditions

- Invalid input is reported as status conditions from `state/conditions/conditions.go` (e.g. `NewPolicyInvalid`, `NewRoutePartiallyInvalid`), written by `status/prepare_requests.go`.
- Handle an error once: log or return, not both (`docs/developer/go-style-guide.md`). Logging rules: `docs/developer/logging-guidelines.md`.
