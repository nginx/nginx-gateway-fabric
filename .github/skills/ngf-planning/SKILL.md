---
name: ngf-planning
description: 'Task planning and approach strategy for NGF. Use when starting any non-trivial task, reading issues or specs, planning before implementing, or when asked to create a plan for a change.'
---

# Planning and Task Approach

## Before Writing Code

1. **Read the requirement.** Check the linked issue, discussion, proposal (`docs/proposals/`) and PR. Features that need a proposal follow `docs/proposals/README.md`.
2. **Write acceptance criteria.** State what "done" means in testable terms: what works, what is rejected, what must not break.
3. **Identify security impact.** Does the change:
   - Accept new user input that reaches NGINX config? It needs a validator in `internal/controller/nginx/config/validation` and a negative test.
   - Touch Secrets, TLS, tokens or credentials? Never log them; get a human review.
   - Change RBAC (chart or operator)? Both must change together.
   - Add anything raw (snippets-like)? Do not add new raw-config escape hatches.
4. **Identify affected layers:**
   - API types (`apis/v1alpha1`, `apis/v1alpha2`)
   - Graph (`internal/controller/state/graph`): validation, attachment, conditions
   - Dataplane (`internal/controller/state/dataplane`): intermediate `Configuration`
   - Config generation (`internal/controller/nginx/config`): `*_template.go`, `policies/*`
   - Validators (`internal/controller/nginx/config/validation`)
   - Provisioner (`internal/controller/provisioner`): the NGINX pod's Kubernetes objects
   - Status (`internal/controller/status`), telemetry, Helm chart, operator, deploy manifests
5. **Check the invariants** in `.github/copilot-instructions.md`.
6. **Identify the test surface:** unit (positive and negative), CEL, functional, Helm. See `ngf-testing`.
7. **Produce a plan.** List the files to change in order and what the change must not break.

## Scope Assessment

| Scope | Indicators | Action |
| --- | --- | --- |
| Trivial | Typo, docs, comment | Fix directly |
| Small | One layer, no API change | Brief plan, implement, test |
| Medium | 2-3 layers, new field or filter | Detailed plan, implement layer by layer, test each |
| Large | New policy CRD, new subsystem, cross-cutting | Proposal / plan document first, then stages |

## Layer Impact Checklist

- [ ] Edited `apis/**`? Run `make generate-all` (deepcopy, CRDs, `policy_methods.go`, manifests, API docs).
- [ ] New CRD file? Add it by hand to `config/crd/kustomization.yaml`.
- [ ] New API field? Kubebuilder markers or CEL, Go validation, CEL test in `tests/cel`, unit tests, a status condition for invalid input.
- [ ] Edited a `*_template.go`? Add a test that renders the new directive and asserts it. Check whether validation must change too.
- [ ] Plus-only behaviour? Validator rejects it when `!plus`; template gates it on `$.Plus`.
- [ ] Edited an interface with a moq fake? `make generate`.
- [ ] Added a watched kind? Chart `rbac.namespacedRules` and `operators/config/rbac/role.yaml`, then `make verify-operator-rbac`.
- [ ] Edited chart `values.yaml` or templates? `make generate-helm-schema generate-helm-docs generate-manifests`, plus a helm-unittest case.
- [ ] Changed telemetry counts? `make generate`.
- [ ] Changed imports? `go mod tidy` (root and `tests/`).

## Definition of Done

| Condition | Required action | Verification |
| --- | --- | --- |
| Edited `apis/**` | `make generate-all` | `git status` shows regenerated `config/crd/bases`, `deploy/**`, deepcopy, `policy_methods.go` |
| Edited a template | Test asserting the new directive | The test fails if the template line is removed |
| New or changed validation | Positive and negative unit tests | Injection-style input is rejected |
| New or changed CRD rule | CEL test in `tests/cel` | `make -C tests test-cel-validation` |
| New or changed Helm value | helm-unittest case and regenerated schema, README, manifests | `make helm-unit-test` |
| Edited a workflow | Job gate is correct | `make lint-job-gating lint-workflow-gating` |
| Any Go change | `make unit-test` then `make lint` | Both pass |

## Common Planning Mistakes

- Changing an API type without running `make generate-all`.
- Forgetting the Go validation because the CRD already validates. CRD validation can be bypassed.
- Adding a template directive with no test that asserts it.
- Putting a Plus-only directive in the template without a `$.Plus` gate and a validator check.
- Assuming a directive exists or behaves a certain way without reading <https://nginx.org/en/docs/>.
- Adding a watched kind but forgetting the Helm RBAC or the operator RBAC.
- Not reaching the consumer: the graph accepts a field but the dataplane `Configuration` never carries it.
- Trusting stale developer docs. `implementing-a-feature.md` links paths that no longer exist; use `make generate-all`.

## Ordering Rules for Multi-Layer Changes

1. **API types** with markers; CEL for cross-field rules.
2. **Generate:** `make generate-policies` (policies only), then `make generate-crds`.
3. **Graph:** validation and conditions in `state/graph`.
4. **Validators:** new generic checks go in the `state/validation` interface and `nginx/config/validation`; then `make generate` for the fake.
5. **Dataplane:** carry the data in `state/dataplane/types.go` and `configuration.go`.
6. **Config generation:** template and execute function in `nginx/config`, or a policy generator.
7. **Provisioner, status, telemetry, RBAC** as needed.
8. **Helm and operator:** values, schema, README, manifests, RBAC.
9. **Tests:** unit, CEL, functional, Helm.
10. **Finish:** `make generate-all`, `make unit-test`, `make lint`.
