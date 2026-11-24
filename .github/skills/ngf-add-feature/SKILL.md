---
name: ngf-add-feature
description: 'Checklists for adding NginxProxy fields, HTTPRoute/GRPCRoute filters or other route features, and Helm chart values to NGF. Use when adding new configuration options, new NGINX directives, new filters, new CRD fields, or new Helm values.'
---

# Adding Features to NGF

Process: discussion -> issue -> proposal if a maintainer asks for one (`docs/proposals/README.md`, `template.md`) -> implementation (`docs/developer/implementing-a-feature.md`). New fields follow `docs/developer/crd-versioning.md`: adding an optional field needs no version bump; renaming, removing, adding a required field or changing validation does.

---

## Adding an NginxProxy Field

1. `apis/v1alpha2/nginxproxy_types.go`: add an optional field with kubebuilder and CEL markers. Run `make generate-crds`.
2. CEL tests: `tests/cel/nginxproxy_test.go`, with expected error strings in `tests/cel/common.go`.
3. Go validation: `validateNginxProxy` and helpers in `internal/controller/state/graph/nginxproxy.go`. Plus-only fields take the `plus` flag and report a condition when Plus is off (see `validateLogging`, `validateServerTokens`).
4. Merge behaviour: `buildEffectiveNginxProxy` JSON-merges the GatewayClass and Gateway NginxProxy. Slices and mutually exclusive fields need a case in `cleanupEffectiveNginxProxy`.
5. Dataplane: add a `build*` function in `internal/controller/state/dataplane/configuration.go` (e.g. `buildWorkerProcesses`) and the field in `dataplane/types.go`.
6. Template: `internal/controller/nginx/config/*_template.go` and its `.go` file (`main_config`, `base_http_config`, `servers`, ...). If the value is user-supplied, route it through a validator.
7. If it changes bootstrap config or Kubernetes objects: `internal/controller/provisioner/objects.go`, `templates.go`, `setter.go`.
8. Helm: mirror the field under `nginx.config` in `charts/nginx-gateway-fabric/values.yaml` (hand-maintained, `# @schema` block), then `make generate-helm-schema generate-helm-docs`. `templates/nginxproxy.yaml` passes `nginx.config` through with `toYaml`.
9. Tests: validation (valid and invalid), dataplane, and a config test that renders and asserts the directive.
10. `make generate-all`, `make unit-test`, `make lint`.

## Adding an HTTPRoute / GRPCRoute Filter or Route Feature

1. `internal/controller/state/graph/common_filter.go`: add a `Filter` field, a `FilterType` constant, a case in `convertHTTPRouteFilters`, an entry in `supportedHTTPFilterTypes`, and a case in `validateFilter`.
2. `internal/controller/state/dataplane/convert.go`: add a `convert*Filter` function; add to `HTTPFilters` in `dataplane/types.go`; wire it in `createHTTPFilters` in `configuration.go`.
3. `internal/controller/nginx/config/servers.go`: `createLocations` / `updateLocation*`; the directive goes in `servers_template.go` (and `maps.go`, `variable_names.go` if needed).
4. If the feature is a Gateway API extended feature, advertise it in `supportedFeatures` in `internal/controller/status/gatewayclass.go` (experimental features are gated). For OpenShift conformance also update `SUPPORTED_EXTENDED_FEATURES_OPENSHIFT` in `tests/Makefile`.
5. Validate every user string with the validators in `internal/controller/nginx/config/validation`, and add negative tests.
6. Tests: graph, dataplane, `servers_test.go` asserting the rendered directive, functional test with an example under `examples/`.

## Adding a Helm Chart Value

1. `charts/nginx-gateway-fabric/values.yaml`: add the value with a `# --` description comment (read by helm-docs) and a `# @schema` block when the type cannot be inferred or needs enum, required, etc.
2. `make generate-helm-schema` (regenerates `values.schema.json`) and `make generate-helm-docs` (regenerates the chart `README.md`).
3. Use it in `charts/nginx-gateway-fabric/templates/*.yaml`. A CLI flag also needs the flag in `cmd/gateway/commands.go` and the arg in `deployment.yaml`.
4. Add a helm-unittest case in `charts/nginx-gateway-fabric/tests/<template>_test.yaml` (values under `tests/values/`). Run `make helm-unit-test`, and `make lint-helm` if Docker is available.
5. `make generate-manifests` regenerates `deploy/*/deploy.yaml`. A new variant needs `examples/helm/<variant>/values.yaml`.
6. Operator: it is helm-based and takes chart values as-is. Only RBAC changes need a sync (`make verify-operator-rbac`).

## OSS vs Plus

- Plus is `cfg.Plus` from `--nginx-plus`. Reject Plus-only input in the validator with a condition when `!plus`, and gate rendering with `{{- if $.Plus }}`.
- Verify directive context and availability at <https://nginx.org/en/docs/> and <https://docs.nginx.com/nginx/admin-guide/> before wiring a template.

## Snippets

Content is raw NGINX config by design, enabled by `--snippets` (Helm `nginxGateway.snippets.enable`) and RBAC. Do not add new raw-config fields.

## Gotchas

- Never hand-edit generated files (see `ngf-structure`).
- CRD validation can be bypassed: always validate in Go too.
- `docs/api/content.md` is generated and gitignored; it is published elsewhere.
- Update examples and the Gateway API compatibility docs (kept outside this repo) when behaviour changes.
