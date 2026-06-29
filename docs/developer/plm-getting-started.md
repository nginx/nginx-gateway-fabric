# Getting Started with PLM (Policy Lifecycle Manager)

This is a getting-started guide for the **NGINX Ingress Controller (NIC)** team on integrating the
F5 **Policy Lifecycle Manager (PLM)** — the `f5-waf-policy-controller` — as a source for compiled
NGINX App Protect (NAP) WAF bundles. It is distilled from the working end-to-end integration in
NGINX Gateway Fabric (NGF).

In NIC, the resource that consumes a WAF policy is the **`Policy`** CRD (`k8s.nginx.org`, kind
`Policy`), referenced from a `VirtualServer`/`VirtualServerRoute` (or via Ingress annotations). This
guide uses NGF's `WAFPolicy` as the reference implementation and maps the **PLM-specific contract**
onto NIC's `Policy`. The reference NGF implementation lives in `tests/suite/waf_policy_test.go`,
`tests/framework/plm.go`, and `tests/suite/manifests/waf-policy/`.

For the full design background, see the NGF NAP WAF design proposal:
[docs/proposals/nap-waf.md](../proposals/nap-waf.md).

<!-- TOC -->
- [What PLM Is](#what-plm-is)
- [Architecture at a Glance](#architecture-at-a-glance)
- [Prerequisites](#prerequisites)
- [Step 1 — Install PLM](#step-1--install-plm)
- [Step 2 — Author and Compile a Policy (APPolicy / APLogConf)](#step-2--author-and-compile-a-policy-appolicy--aplogconf)
- [Step 3 — Wire NIC to PLM Storage](#step-3--wire-nic-to-plm-storage)
- [Step 4 — Reference the Policy from NIC's Policy Resource](#step-4--reference-the-policy-from-nics-policy-resource)
- [Step 5 — Verify the Integration](#step-5--verify-the-integration)
- [Cross-Namespace References](#cross-namespace-references)
- [Status & Failure Modes](#status--failure-modes)
- [Reference: Constants and Conventions](#reference-constants-and-conventions)
- [Troubleshooting](#troubleshooting)
<!-- /TOC -->

## What PLM Is

PLM (the `f5-waf-policy-controller`) is a Kubernetes controller that decouples **policy authoring**
from **policy consumption**:

1. You author WAF policy as Kubernetes-native CRDs: `APPolicy` (the security policy) and
   `APLogConf` (the security-logging profile), both in the `appprotect.f5.com/v1` API group.
2. PLM **watches** those CRDs, **compiles** each one into a NAP `.tgz` bundle (compilation runs as a
   Job and can take a while on a cold cluster), **uploads** the bundle to an in-cluster
   S3-compatible store (**SeaweedFS**, deployed by the PLM chart), and **writes the result back to
   the resource's `status.bundle`** field (`state`, `location`, `sha256`).
3. NIC (the consumer) watches those same CRDs, waits for `status.bundle.state: ready`,
   then **fetches the compiled bundle from SeaweedFS via its S3 endpoint** and hands it to the
   NGINX data plane.

The key difference over the HTTP/NIM/N1C bundle sources is that **the entire policy lifecycle stays
inside the cluster** — no external NGINX Instance Manager / NGINX One Console dependency, no manual
`.tgz` compilation step, and policy is expressed as first-class Kubernetes objects.

## Architecture at a Glance

```text
   Author (cluster operator)
        │  kubectl apply APPolicy / APLogConf  (appprotect.f5.com/v1)
        ▼
 ┌───────────────────────────────────────────────────────────────┐
 │ PLM namespace ("plm")                                         │
 │                                                               │
 │  f5-waf-policy-controller ──compiles──▶  NAP .tgz bundle      │
 │        │                                      │               │
 │        │ writes status.bundle                 ▼               │
 │        │ (state/location/sha256)        SeaweedFS (S3)        │
 │        ▼                               filer :8333            │
 │   APPolicy.status.bundle.state: ready        ▲                │
 └──────────────────────────────────────────────┼───────────-────┘
        │ watch                                  │ S3 GET (creds Secret)
        ▼                                        │
 ┌───────────────────────────────────────────────────────────────┐
 │ NGINX Ingress Controller (NIC)                                │
 │   • watches APPolicy/APLogConf status                         │
 │   • on state: ready → fetch bundle from SeaweedFS S3 URL      │
 │   • validate sha256, write bundle to data-plane pod           │
 │   • emit app_protect_* directives in NGINX config             │
 └───────────────────────────────────────────────────────────────┘
        │
        ▼
   NGINX + NAP data plane enforces the policy
```

## Prerequisites

- A Kubernetes cluster on **amd64** (NAP WAF does not support arm64).
- **NGINX Plus with NAP** in the NIC data plane (WAF requires Plus).
- Access to `private-registry.nginx.com` (the PLM controller and SeaweedFS images live there). You
  will need a registry pull secret (the JWT-based NGINX Plus registry credential works).
- Helm 3 and `kubectl`.

## Step 1 — Install PLM

PLM is distributed as a public Helm chart from the `nginx-stable` repo.

```bash
# 1. Create the namespace and the registry image pull secret PLM needs.
kubectl create namespace plm
kubectl create secret docker-registry regcred \
  --namespace plm \
  --docker-server=private-registry.nginx.com \
  --docker-username=<JWT-token> \
  --docker-password=none

# 2. Add the chart repo.
helm repo add nginx-stable https://helm.nginx.com/stable --force-update
helm repo update

# 3. Install the controller + SeaweedFS storage.
helm install policy-controller nginx-stable/f5-waf-policy-controller \
  --create-namespace \
  --namespace plm \
  --set imagePullSecrets[0].name=regcred \
  --set seaweedfs-operator.image.pullSecrets=regcred \
  --set seaweedfsOperatorConfig.seaweedfs.certificates.enabled=false \
  --set policyController.s3.skipTlsVerify=true \
  --wait
```

> The two `seaweedfs*` overrides disable TLS on the in-cluster S3 filer, which keeps the consumer
> wiring simple (plain HTTP, no CA/client-cert plumbing). For production you would leave TLS enabled
> and configure the corresponding CA / client-SSL secrets on the consumer side (see Step 3).

This installs three things you care about:

| Resource | Default name | Purpose |
| --- | --- | --- |
| The PLM controller Deployment | `policy-controller-...` | Watches/compiles CRDs |
| SeaweedFS S3 filer Service | `policy-controller-f5-waf-seaweed-filer` | Serves bundles over S3 on port **8333** |
| S3 credentials Secret | `policy-controller-f5-waf-seaweedfs-auth` | Holds the S3 secret key under `seaweedfs_admin_secret` |

The names follow the pattern `<release>-f5-waf-*` (the chart's `f5-waf.fullname` helper), so with the
release name `policy-controller` the in-cluster S3 endpoint is:

```text
http://policy-controller-f5-waf-seaweed-filer.plm.svc.cluster.local:8333
```

> **Uninstall note:** PLM puts a finalizer on its `APSignatures` resources. After
> `helm uninstall policy-controller -n plm`, those finalizers will block namespace deletion because
> the controller is gone. Clear them before deleting the namespace:
>
> ```bash
> kubectl get apsignatures.appprotect.f5.com -n plm -o name | \
>   xargs -I{} kubectl patch {} -n plm --type=merge -p '{"metadata":{"finalizers":[]}}'
> ```

## Step 2 — Author and Compile a Policy (APPolicy / APLogConf)

Apply an `APPolicy` (and, optionally, an `APLogConf` for security logging). PLM compiles them
automatically.

`appolicy.yaml`:

```yaml
apiVersion: appprotect.f5.com/v1
kind: APPolicy
metadata:
  name: attack-signatures
spec:
  policy:
    name: attack-signatures-blocking
    template:
      name: POLICY_TEMPLATE_NGINX_BASE
    applicationLanguage: utf-8
    enforcementMode: blocking
    signature-sets:
    - name: All Signatures
      block: true
      alarm: true
    cookies:
    - name: "*"
      attackSignaturesCheck: true
      enforcementType: enforce
      maskValueInLogs: false
```

`aplogconf.yaml` (optional, for security logging):

```yaml
apiVersion: appprotect.f5.com/v1
kind: APLogConf
metadata:
  name: log-illegal
spec:
  filter:
    request_type: illegal
  content:
    format: default
    max_request_size: any
    max_message_size: 15k
```

Apply and wait for compilation to finish. The contract NIC relies on is
`status.bundle.state`:

```bash
kubectl apply -f appolicy.yaml -f aplogconf.yaml

# Poll until compilation succeeds. Allow several minutes on a cold cluster.
kubectl get appolicy attack-signatures \
  -o jsonpath='{.status.bundle.state}{"\n"}'   # → "ready" when done
```

`status.bundle.state` values to handle:

| State | Meaning |
| --- | --- |
| (absent) | Not yet picked up by PLM |
| `ready` | Compiled successfully; `status.bundle.location` + `status.bundle.sha256` are populated |
| `invalid` | Compilation failed (e.g. a malformed policy spec) — never becomes consumable |

> In the NGF test harness, `waitForAPBundleState()` polls this field as an *unstructured* object
> (the `appprotect.f5.com` CRDs are not in the consumer's scheme). NIC can read the same field via a
> dynamic/unstructured client without vendoring PLM's types.

## Step 3 — Wire NIC to PLM Storage

NIC needs three pieces of cluster-wide configuration to reach SeaweedFS. In NGF these are exposed as
CLI flags on the control plane and surfaced as Helm values; NIC must expose equivalent flags/Helm
values. The NGF shape below is the reference contract to replicate.

| NGF CLI flag | NGF Helm value | Required? | Purpose |
| --- | --- | --- | --- |
| `--plm-storage-url` | `nginxGateway.plmStorage.url` | **Yes** (enables PLM) | SeaweedFS S3 endpoint URL |
| `--plm-storage-credentials-secret` | `nginxGateway.plmStorage.credentialsSecretName` | Yes (for auth) | Secret holding the S3 secret key (`seaweedfs_admin_secret`) |
| `--plm-storage-ca-secret` | `nginxGateway.plmStorage.tls.caSecretName` | Only with TLS | CA cert (`ca.crt`) to verify the filer |
| `--plm-storage-client-ssl-secret` | `nginxGateway.plmStorage.tls.clientSSLSecretName` | Only with mTLS | Client cert/key for mutual TLS |
| `--plm-storage-skip-verify` | `nginxGateway.plmStorage.tls.insecureSkipVerify` | No (dev/test only) | Disable TLS verification |

Design notes worth copying:

- **PLM support is gated on the URL.** If `--plm-storage-url` is empty, PLM is disabled and any
  `type: PLM` policy is rejected with a clear status condition (NGF: *"PLM storage not configured;
  set --plm-storage-url on the controller"*). Treat the URL as the on/off switch.
- **Secret references support a cross-namespace `namespace/name` form.** Because the credentials
  Secret lives in the `plm` namespace (not NIC's namespace), the flag accepts
  `plm/policy-controller-f5-waf-seaweedfs-auth`. A plain `name` resolves in NIC's own namespace.
- **Watch namespaces must cover the CRDs.** NIC must watch the namespaces where `APPolicy`/`APLogConf`
  resources live, *and* the namespace holding the credentials Secret. The simplest setup (used by the
  NGF tests) is to watch all namespaces.

Example Helm install args (matching the NGF test harness, with the TLS-disabled SeaweedFS from
Step 1):

```bash
--set nginxGateway.plmStorage.url=http://policy-controller-f5-waf-seaweed-filer.plm.svc.cluster.local:8333
--set nginxGateway.plmStorage.credentialsSecretName=plm/policy-controller-f5-waf-seaweedfs-auth
```

The credentials Secret is read for the S3 secret access key under the **`seaweedfs_admin_secret`**
key — the same key the PLM chart writes.

## Step 4 — Reference the Policy from NIC's Policy Resource

In NIC, WAF is configured through the **`Policy`** CRD (`k8s.nginx.org`, kind `Policy`) via its
`spec.waf` section, and that `Policy` is attached to a `VirtualServer`/`VirtualServerRoute` (or to
Ingress resources via annotation). NIC's `waf` section already has an **`apPolicy`** field that
references an `APPolicy` resource.

For PLM, **NIC reuses the existing `apPolicy` reference** — no new field or source discriminator is
needed. The `APPolicy` it points at is simply one that PLM has compiled (its `status.bundle.state`
is `ready`), and NIC fetches the compiled bundle from SeaweedFS. The one enhancement is that
**`apPolicy` can now accept cross-namespace references** (`namespace/name`), so an `APPolicy` managed
centrally (e.g. in the `plm` namespace, or a dedicated policy namespace) can be referenced from a
`Policy` in an application namespace.

```yaml
apiVersion: k8s.nginx.org/v1
kind: Policy
metadata:
  name: waf-policy
spec:
  waf:
    enable: true
    # Existing apPolicy field — now also accepts a cross-namespace "namespace/name" reference
    # to an APPolicy that PLM has compiled.
    apPolicy: "plm/attack-signatures"
    securityLog:
      apLogConf: "plm/log-illegal"
      logDest: "stderr"
```

NGF models the same contract with its `WAFPolicy` (`type: PLM` + `policyRef.apPolicyRef`); it is the
reference implementation for the resolve/fetch/program flow:

```yaml
# NGF reference implementation.
apiVersion: gateway.nginx.org/v1alpha1
kind: WAFPolicy
metadata:
  name: gateway-waf-plm
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway
  type: PLM
  policyRef:
    apPolicyRef:
      name: attack-signatures
  securityLogs:
  - logRef:
      apLogConfRef:
        name: log-illegal
    destination:
      type: stderr
```

Behavior to replicate from the NGF implementation:

- The referenced `APPolicy` must have `status.bundle.state: ready` before the bundle is fetched and
  the `Policy` programs; otherwise surface a not-resolved status (see [Status & Failure
  Modes](#status--failure-modes)).
- A cross-namespace `apPolicy` reference must be authorized (see [Cross-Namespace
  References](#cross-namespace-references)).

The data-plane side is unchanged from NIC's existing App Protect support: once the bundle is fetched
and written to the data-plane pod, NIC emits the standard NAP directives (`app_protect_enable on;`,
`app_protect_policy_file <path>;`, and the `app_protect_security_log*` directives) in the relevant
server/location context.

## Step 5 — Verify the Integration

End-to-end smoke test (mirrors the NGF E2E spec):

```bash
# 1. APPolicy compiled and ready.
kubectl get appolicy attack-signatures -o jsonpath='{.status.bundle.state}'   # ready

# 2. The NIC Policy resource is valid/accepted and the bundle is programmed.
#    (NGF surfaces this as Accepted=True/Accepted and Programmed=True/Programmed.)

# 3. The NGINX data plane has the app_protect directives.
kubectl exec -n <data-plane-ns> <nginx-pod> -- nginx -T | grep app_protect

# 4. An attack is blocked. </script> is a classic XSS payload the
#    "All Signatures" set blocks; expect a "Request Rejected" body.
curl -H 'Host: cafe.example.com' \
  'http://<addr>/coffee?x=%3C%2Fscript%3E'
```

## Cross-Namespace References

If an `APPolicy` lives in a different namespace than the consuming `Policy`, cross-namespace access
must be explicitly authorized. NGF, being a Gateway API implementation, uses a Gateway API
**`ReferenceGrant`** placed in the `APPolicy`'s namespace:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-waf-policy-to-appolicy
  namespace: <appolicy-namespace>     # grant lives WHERE the APPolicy is
spec:
  from:
  - group: gateway.nginx.org
    kind: WAFPolicy                    # NGF's consuming kind
    namespace: <policy-namespace>
  to:
  - group: appprotect.f5.com
    kind: APPolicy
```

NIC does **not** use Gateway API, so `ReferenceGrant` does not apply directly. NIC should decide its
own cross-namespace authorization model — for example, requiring the `apPolicyRef.namespace` to be
in NIC's watched namespaces, gating it behind the existing `Policy` cross-namespace rules, or
electing to only support same-namespace references initially.

Behavior NGF implements, regardless of the authorization mechanism (recommended to mirror):

- When the cross-namespace reference is **not** authorized: the policy is denied with
  `ResolvedRefs=False`, reason `RefNotPermitted`, and **no** WAF directives are emitted.
- Once authorized: the reference resolves, the bundle is fetched, and the policy programs.

## Status & Failure Modes

NGF exposes three condition types; NIC should surface equivalent status on the `Policy` resource so
operators can diagnose issues without reading controller logs.

| Condition | Status / Reason | When |
| --- | --- | --- |
| `Accepted` | True / `Accepted` | Spec is valid and accepted |
| `Accepted` | False / `Invalid` | Bad target/spec, or PLM not configured on the controller |
| `ResolvedRefs` | True / `ResolvedRefs` | `APPolicy`/`APLogConf` found and (if cross-namespace) authorized |
| `ResolvedRefs` | False / `InvalidRef` | Referenced `APPolicy` does not exist, or its bundle state is not `ready`/is `invalid` |
| `ResolvedRefs` | False / `RefNotPermitted` | Cross-namespace ref not authorized |
| `Programmed` | True / `Programmed` | Bundle fetched from S3 and pushed to the data plane |
| `Programmed` | False / `Pending` | Bundle not yet available (see fail-open vs fail-closed) |

**Fail-open vs fail-closed** (a control-plane policy NGF makes configurable via the data-plane
config, defaulting to fail-closed):

- **Fail-closed (default):** if a referenced bundle is not yet available, *withhold the entire config
  push* for the affected listener — unrelated route changes do not take effect either. This prevents
  serving traffic that was supposed to be protected but isn't.
- **Fail-open:** push config without the WAF directives so unrelated changes still apply; traffic
  flows unprotected until the bundle arrives. Make this an explicit opt-in.

## Reference: Constants and Conventions

From `tests/framework/plm.go` — the static, derive-able values NIC tooling can compute from the PLM
release name and namespace:

| Thing | Value (release `policy-controller`, ns `plm`) |
| --- | --- |
| Helm repo | `nginx-stable` → `https://helm.nginx.com/stable` |
| Chart | `nginx-stable/f5-waf-policy-controller` |
| Namespace | `plm` |
| Release name | `policy-controller` |
| S3 endpoint | `http://policy-controller-f5-waf-seaweed-filer.plm.svc.cluster.local:8333` |
| Credentials Secret | `policy-controller-f5-waf-seaweedfs-auth` |
| Credentials Secret key | `seaweedfs_admin_secret` |
| CRD group/version | `appprotect.f5.com/v1` (`APPolicy`, `APLogConf`, `APSignatures`) |
| Status field watched | `status.bundle.state` (`ready` / `invalid`) |

All names follow `<release>-f5-waf-*`; the SeaweedFS short-name does **not** include the release name
twice, so plug the chosen release name into the `<release>-f5-waf-...` pattern.

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `APPolicy` never leaves `(no state)` | PLM controller not running / not watching the namespace | Check the `policy-controller` pod and its `watchNamespace`; check it can pull images |
| `APPolicy` is `invalid` | Malformed policy spec | `kubectl describe appolicy <name>` and the controller logs for the compile error |
| Policy `Programmed=False/Pending` forever | NIC can't reach SeaweedFS or read the creds Secret | Verify the PLM storage URL is reachable in-cluster and the creds Secret name/namespace are correct and watched |
| `ResolvedRefs=False/RefNotPermitted` | Cross-namespace ref not authorized | Authorize the reference per NIC's cross-namespace model (or move the `APPolicy` into the same namespace) |
| `Accepted=False/Invalid` "PLM storage not configured" | `--plm-storage-url` not set | Set the URL flag/Helm value on the controller |
| PLM namespace stuck `Terminating` after uninstall | `APSignatures` finalizers | Clear finalizers (see the uninstall note in Step 1) |
| Pull errors on PLM/SeaweedFS pods | Missing/incorrect registry secret | Recreate the `regcred` secret in the `plm` namespace and reference it in both `imagePullSecrets` overrides |

## Source Material

The reference NGF implementation this guide is based on:

- [`docs/proposals/nap-waf.md`](../proposals/nap-waf.md) — the NAP WAF design proposal (architecture,
  bundle sources including PLM, status conditions, fail-open/fail-closed).
- `tests/suite/waf_policy_test.go` — the `"when using the PLM source"` context: gateway- and
  route-targeted policies, missing/malformed `APPolicy`, and cross-namespace + `ReferenceGrant`.
- `tests/framework/plm.go` — install/uninstall helpers and all the naming constants.
- `tests/suite/system_suite_test.go` — `installPLM()` / `plmNGFInstallArgs()` showing the
  install-once-per-cluster pattern and the consumer-side Helm wiring.
- `tests/suite/manifests/waf-policy/` — `appolicy.yaml`, `aplogconf.yaml`, `wafpolicy-plm*.yaml`,
  `referencegrant-appolicy.yaml`.
- `apis/v1alpha1/wafpolicy_types.go` — the `type: PLM` / `policyRef.apPolicyRef` API contract and
  validation rules.
