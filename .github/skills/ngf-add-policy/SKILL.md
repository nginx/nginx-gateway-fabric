---
name: ngf-add-policy
description: 'Step-by-step checklist for adding or extending a policy CRD in NGF. Use when implementing a new policy like AccessPolicy, ClientSettingsPolicy, ObservabilityPolicy, ProxySettingsPolicy, RateLimitPolicy, UpstreamSettingsPolicy or WAFPolicy, or extending the policy system.'
---

# Adding a New Policy Type

Traced from AccessPolicy (`fba8fcf9`, telemetry in `d5f589bd`). Follow the steps in order. Existing policies: `apis/v1alpha1` AccessPolicy, ClientSettingsPolicy, ProxySettingsPolicy, RateLimitPolicy, SnippetsPolicy, UpstreamSettingsPolicy, WAFPolicy, PayloadProcessor; `apis/v1alpha2` ObservabilityPolicy.

## API

1. `apis/v1alpha1/<name>_types.go` with `+kubebuilder:object:root=true`, `storageversion`, `subresource:status`, `resource:categories=nginx-gateway-fabric,scope=Namespaced`, a `printcolumn`, and `+kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited|direct"`. Add CEL `XValidation` on `targetRefs` (allowed kinds and group, uniqueness; see `accesspolicy_types.go`). Use `TargetRefs []LocalPolicyTargetReference` (or one `TargetRef`) and `Status gatewayv1.PolicyStatus`.
2. Add the type and its list to `addKnownTypes` in `apis/v1alpha1/register.go`.
3. `make generate-policies` (`apis/policygen.go` scans for the policy label and writes `policy_methods.go`).
4. `make generate-crds` (deepcopy, `config/crd/bases`, `deploy/crds.yaml`).
5. Add the new `config/crd/bases/gateway.nginx.org_<plural>.yaml` by hand to `config/crd/kustomization.yaml`.

## Control plane

6. `internal/framework/kinds/kinds.go`: add the kind constant.
7. `internal/controller/manager.go`: `createPolicyManager` (add a `policies.ManagerConfig{GVK, Validator}`; the composite validator panics for an unregistered kind), `registerControllers` (add an object with `GenerationChangedPredicate`; feature-flagged kinds go in `featureFlagControllerCfgs`), `prepareFirstEventBatchPreparerArgs` (add the list type). Update `manager_test.go`.
8. `NewChangeProcessorImpl` in `internal/controller/state/change_processor.go`: add an entry with `store: commonPolicyObjectStore` and `predicate: funcPredicate{stateChanged: isNGFPolicyRelevant}`.
9. Validator: `internal/controller/nginx/config/policies/<name>/validator.go` implementing `policies.Validator` (`Validate`, `ValidateGlobalSettings`, `Conflicts`). Return `conditions.NewPolicyInvalid(...)`. Use `policies.ValidateTargetRef` and `statevalidation.GenericValidator`. A new generic check goes in the interface in `internal/controller/state/validation/validator.go`, is implemented in `nginx/config/validation/generic.go`, and `make generate` refreshes the moq fake. User strings reaching config must use the existing escaped-string validators.
10. Generator: `policies/<name>/generator.go`, embedding `policies.UnimplementedGenerator` and implementing the contexts needed (`GenerateForMain`, `GenerateForHTTP`, `GenerateForStream`, `GenerateForServer`, `GenerateForStreamServer`, `GenerateForLocation`, `GenerateForInternalLocation`). Render a package-level `text/template` with `helpers.MustExecuteTemplate` and return `policies.File{Name: "<Kind>_<ns>_<name>.conf"}`. Plus-only output is gated on Plus. UpstreamSettingsPolicy is the exception: it uses `processor.go`.
11. Register the generator in `policies.NewCompositeGenerator(...)` in `internal/controller/nginx/config/generator.go`.
12. Graph: `internal/controller/state/graph/policies.go`: add the kind to `policyDeterminingCondition`. `processPolicies` and `attachPolicies` are generic. Add a graph file (like `access_policy.go`) only for special merge logic.
13. Conditions: `internal/controller/state/conditions/conditions.go`: add the `<Kind>Affected` `PolicyConditionType` and a `New<Kind>Affected()` constructor; add the type to `gatewayConditionTypes` in `internal/controller/status/status_setters.go`.
14. Status: add the kind to `settingsPolicyKinds` in `internal/controller/status/prepare_requests.go` if it reports the GEP-713 `Programmed` condition.
15. Dataplane: usually nothing, `buildPolicies` in `state/dataplane/configuration.go` passes gateway, route and service policies through. Custom merging (AccessPolicy's `buildGeoAccessPolicies`) edits `types.go` and `configuration.go`. Http-context output uses `BaseHTTPConfig.Policies`.

## RBAC and deploy

16. Add `<plural>` and `<plural>/status` to `rbac.namespacedRules` in `charts/nginx-gateway-fabric/templates/_helpers.tpl` (there is no `clusterrole.yaml`; `templates/rbac.yaml` renders from helpers; test in `tests/rbac_test.yaml`).
17. Add the same to `operators/config/rbac/role.yaml`, then `make verify-operator-rbac`.
18. `make generate-manifests`.

## Telemetry

19. `internal/controller/telemetry/collector.go`: add `GatewayAttached<X>Count` and `RouteAttached<X>Count` to `NGFResourceCounts`, and a case in `CountPolicies` using `countPolicyTargetRefs`.
20. `make generate` regenerates `data.avdl` and the `*_attributes_generated.go` files. Update `collector_test.go`, `data_test.go` and `tests/suite/telemetry_test.go`. The `.avdl` schema is pushed to the registry after merge.

## Tests, docs, examples

21. Unit: `validator_test.go`, `generator_test.go` (assert the rendered directive), `graph/policies_test.go`, `dataplane/configuration_test.go`, `servers_test.go`, `change_processor_test.go`.
22. CEL: `tests/cel/<name>_test.go` plus constants in `tests/cel/common.go`; `make -C tests test-cel-validation`.
23. Functional: `tests/suite/<name>_test.go` with `Label("functional", "<name>")`, manifests in `tests/suite/manifests/<name>/`; `make -C tests test GINKGO_LABEL=<name>`.
24. `examples/<name>/`, and a proposal in `docs/proposals/<name>.md` set to Completed.
25. `make generate-all`, `make unit-test`, `make lint`.

## Gotchas

- Forgetting step 5 leaves the CRD out of `deploy/crds.yaml` and the chart.
- Forgetting step 7 or 13 leaves the policy silently ignored or its status never written.
- An unregistered validator panics at startup.
- Never log secrets; never build NGINX config from unvalidated strings.
