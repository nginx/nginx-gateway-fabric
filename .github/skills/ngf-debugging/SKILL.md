---
name: ngf-debugging
description: 'Debugging and troubleshooting patterns for NGF. Use when diagnosing failures, tracing why a route or policy is not applied, investigating NGINX config apply errors, agent connection problems, or controller sync issues.'
---

# Debugging and Troubleshooting

`docs/developer/debugging.md` covers attaching dlv to the running control plane. This skill covers diagnosis.

## Common Failure Modes

### Resource accepted but traffic not routed

1. `kubectl describe` the Gateway, Route and policy. NGF reports problems as status conditions (`Accepted`, `ResolvedRefs`, `Programmed`, policy `Accepted`/`<Kind>Affected`).
2. Conditions come from `internal/controller/state/conditions/conditions.go` and are written by `internal/controller/status/prepare_requests.go`. Find the constructor for the reason string to locate the code that set it, usually in `internal/controller/state/graph/`.
3. Common causes: unresolved backend or Secret reference, missing ReferenceGrant, listener hostname or port conflict, unsupported filter type, a CRD that is not installed (TCPRoute, TLSRoute, BackendTLSPolicy, ListenerSet are only watched when their CRD exists), a feature flag that is off (snippets, inference extension).

### Policy not applied

- Check `status.ancestors` conditions. A policy can be `Conflicted`, `Invalid` (validator output) or target a kind it does not support.
- Trace: validator (`nginx/config/policies/<name>/validator.go`) -> graph attach (`state/graph/policies.go`) -> generator (`policies/<name>/generator.go`) -> include in the generated server or location.
- Plus-only input with Plus disabled is rejected with a condition, not ignored.

### NGINX config apply failure

1. Control plane logs show the config apply error and retries (`handler_configapply.go`, `config_retry.go`). Some failures are remediated automatically (e.g. zone size, `zonesize_remediator.go`).
2. The agent logs show the `nginx -t` or reload error with the line number in the generated file.
3. Common causes: a user string that was not validated, a directive in the wrong context, duplicate directives from two policies, a Plus-only directive when running OSS.
4. Fix at the right layer: validation bug -> validator or graph; wrong directive -> template; missing data -> dataplane. Then add a test (see `ngf-testing`).

### Data plane pod not receiving config

- The agent connects to the control plane gRPC server over TLS with a TokenReview-authenticated token (`nginx/agent/grpc`). Check the certs job and secrets, the agent logs, and the control plane logs for connection or auth errors.
- Config reaches the pod by `NginxUpdaterImpl.UpdateConfig` -> `Broadcaster.Send`; the agent pulls files via the file service.
- The provisioner owns the NGINX Deployment/Service/ConfigMaps (`internal/controller/provisioner`). Manual edits are reverted; change NginxProxy or Helm values instead.

### Controller crash or panic

- Read the stack trace. Likely sites: nil dereference on an optional API field, an unregistered policy validator (`createPolicyManager` panics), `helpers.MustCastObject` on an unexpected type, `Process` called with an unsupported event type.

### Unit or CEL test failure

- `make unit-test` runs with `-race -shuffle=on`; reproduce an order-dependent failure with the printed seed (`-shuffle=<seed>`).
- A CEL test needs a cluster with the CRDs installed (`make -C tests test-cel-validation`).
- Stale generated files fail CI: run `make generate-all` and look at `git status`.

## Debugging Workflow

1. **Reproduce.** Get the exact status condition, log line or nginx error.
2. **Assess security impact.** Can a namespaced tenant trigger it? Does it expose data in logs or status? If exploitable, flag it before fixing.
3. **Locate the layer:** API/CEL, graph, dataplane, generator/template, agent push, provisioner, status.
4. **Isolate** the smallest resource set that triggers it.
5. **Fix** in the correct layer; do not patch templates for validation bugs.
6. **Prevent.** Write the failing unit test first, then fix; include negative or malicious input for validation bugs.

## Useful Commands

| Need | Command |
| --- | --- |
| Resource status | `kubectl get gateway,httproute,grpcroute -A`; `kubectl describe <kind> <name>` |
| Policy status | `kubectl get <policy-kind> -A -o yaml` and read `status.ancestors` |
| Control plane logs | `kubectl logs -n nginx-gateway deploy/<ngf-controller>` |
| Data plane logs | `kubectl logs <nginx pod> -c nginx` |
| Generated config | `kubectl exec <nginx pod> -- cat /etc/nginx/conf.d/http.conf` (and `/etc/nginx/stream-conf.d/`) |
| Syntax check | `kubectl exec <nginx pod> -- nginx -t` |

Namespace and container names follow the Helm release and `examples/helm/*/values.yaml`; confirm with `kubectl get pods -A`.

## Gotchas

- NGINX line numbers refer to the generated file, not the Go template.
- A Secret problem often surfaces as a missing file in NGINX logs.
- Never log or paste secrets, tokens or license contents; log key names and paths.
- Debug-log levels: `V(1)` is debug (`docs/developer/logging-guidelines.md`); NginxGateway can change the control plane log level at runtime (`log_level_setters.go`).
