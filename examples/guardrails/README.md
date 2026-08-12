# AI Guardrails example

This example stands up a mock LLM backend behind NGINX Gateway Fabric and attaches a
`PayloadProcessor` policy that routes request/response payloads through an external **Guardrails
API** for inspection. Disallowed content is blocked before it reaches the model (requests) or the
client (responses).

The payload inspection is performed by the `ai-guardrails` NGINX module. See
[`internal/controller/nginx/modules/rust/ai-guardrails/README.md`](../../internal/controller/nginx/modules/rust/ai-guardrails/README.md)
for how that module works.

> **Prerequisite:** NGF must be installed with the `--payload-processor` flag enabled and an image
> that includes the `ai-guardrails` module. Without it, the `PayloadProcessor` policy has no effect.

## Files

| File | Purpose |
| ------ | --------- |
| `gateway.yaml` | The `inference-gateway` Gateway (HTTP listener on port 80). |
| `llm.yaml` | The mock LLM backend (`vllm-qwen3-32b` Deployment + Service on port 8000). |
| `llm-route.yaml` | HTTPRoute sending all traffic to the mock LLM Service. |
| `payload-processor.yaml` | The `PayloadProcessor` policy attaching Guardrails to the route. |
| `guardrails-service.yaml` | The Guardrails backend Service (external, `ExternalName`). |
| `guardrails-secret.yaml` | Secret holding the Guardrails API bearer token. |
| `test-data.json` | ShareGPT-style **source** dataset (input to `ds-tool`); each `human` prompt maps to a canned `gpt` response. Edit this to change what the mock LLM returns. |
| `inference-sim-dataset.sqlite3` | **Generated artifact** — the canned dataset served by the mock LLM, built from `test-data.json` (see [Regenerating the dataset](#regenerating-the-dataset)). |
| `inference-sim-dataset.json` | **Generated artifact** — human-readable debug dump of the built dataset (tokenized, with `prompt_hash`). Not an input; do not hand-edit. |

## Setup

### 1. Seed the mock LLM dataset

The mock LLM (`ghcr.io/llm-d/llm-d-inference-sim`) serves canned responses from a SQLite dataset
rather than running a real model. `llm.yaml` mounts that dataset from a ConfigMap named
`inference-sim-dataset` at `/data/inference-sim-dataset.sqlite3`. Because the dataset is a binary
file (awkward to embed in YAML), create the ConfigMap imperatively from the local file:

```shell
kubectl create configmap inference-sim-dataset \
  --from-file=inference-sim-dataset.sqlite3=./inference-sim-dataset.sqlite3
```

The `--from-file=<key>=<path>` form sets the ConfigMap key to `inference-sim-dataset.sqlite3`, so it
lands at exactly the path the container's `--dataset-path` flag expects. Without this ConfigMap the
Pod stays in `ContainerCreating` because the referenced volume does not exist.

> **Note:** ConfigMaps are limited to ~1 MiB. If the dataset grows beyond that, deliver it another
> way (e.g. an initContainer download or a PersistentVolume).

### 2. Configure the Guardrails backend

Choose one of the two backend styles described in
[Guardrails backend addressing](#guardrails-backend-addressing) below, then apply the matching
`guardrails-service.yaml`.

Set the token in `guardrails-secret.yaml` (the value must live under the `token` key) with your F5 AI Guardrails API token.

### 3. Apply the manifests

> **Using the default `ExternalName` backend?** First add the `NginxProxy` DNS resolver and wire it
> to the Gateway via `parametersRef`, otherwise NGINX cannot resolve the external hostname at request
> time — see
> [Configuring the DNS resolver](#configuring-the-dns-resolver-required-for-externalname-backends).

```shell
kubectl apply -f gateway.yaml
kubectl apply -f llm.yaml
kubectl apply -f llm-route.yaml
kubectl apply -f guardrails-service.yaml
kubectl apply -f guardrails-secret.yaml
kubectl apply -f payload-processor.yaml
```

### 4. Verify

Confirm the policy was accepted:

```shell
kubectl get payloadprocessor llm-guardrails -o yaml
```

A rejected policy reports `Accepted=False` in its status conditions. Common causes are listed under
[Troubleshooting](#troubleshooting).

## Testing guardrails

The `ai-guardrails` module inspects traffic on **two independent paths** (see the module's
[status/type matrix](../../internal/controller/nginx/modules/rust/ai-guardrails/README.md#http-status-vs-error-type)):

- **Request path** — the client's *input* is inspected before it reaches the LLM. A block returns
  `403` with `error.type: invalid_request_error`. This is driven entirely by the request text, so it
  works regardless of the mock LLM dataset.
- **Response path** — the model's *output* is inspected before it reaches the client. A block returns
  `403` with `error.type: api_error` (non-SSE). This requires the mock LLM to actually *return*
  disallowed content, which is what the seeded dataset below arranges.

> The examples below use synthetic, well-known **test** values (documentation IP ranges, reserved
> `555-01xx` phone numbers, the `4111…` Visa test card, `example.com`, etc.) — not real PII. Whether
> each value is actually blocked depends on your Guardrails backend's policy configuration; enable
> the relevant detectors (passport, phone, IP, SSN, date of birth, credit card, email) to see them
> trip. All commands target `/v1/completions`.

### Request blocking (bad input)

Each request embeds one PII type in the `prompt`. Expected result: `HTTP 403`, body
`{"error":{"type":"invalid_request_error","code":"content_policy_violation", ...}}`. The request
never reaches the LLM.

```shell
# Passport
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"My passport number is X12345678"}'

# Phone number
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Call me at +1-202-555-0173"}'

# IP address
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"My server IP is 192.0.2.14"}'

# SSN
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"My SSN is 123-45-6789"}'

# Date of birth
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"My date of birth is 1985-07-22"}'

# Credit card number
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"My card number is 4111 1111 1111 1111"}'

# Email address
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"My email is jane.doe@example.com"}'
```

### Response blocking (bad output)

Each request sends a **benign** prompt that the seeded dataset maps to a response containing one PII
type. The prompt must be sent **verbatim** — the mock LLM matches responses by tokenizing the exact
prompt (see [Regenerating the dataset](#regenerating-the-dataset)). Expected result: `HTTP 403`,
body `{"error":{"type":"api_error","code":"content_policy_violation", ...}}`.

```shell
# Passport            -> "Here is a test passport number: X12345678"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test passport number"}'

# Phone number        -> "Here is a test phone number: +1-202-555-0173"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test phone number"}'

# IP address          -> "Here is a test IP address: 192.0.2.14"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test IP address"}'

# SSN                 -> "Here is a test SSN: 123-45-6789"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test SSN"}'

# Date of birth       -> "Here is a test date of birth: 1985-07-22"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test date of birth"}'

# Credit card number  -> "Here is a test credit card number: 4111 1111 1111 1111"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test credit card number"}'

# Email address       -> "Here is a test email address: jane.doe@example.com"
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"Give me a test email address"}'
```

> **SSE note:** if you send `"stream": true`, the upstream `200` headers are flushed before the
> response can be inspected, so a blocked streaming response arrives as `HTTP 200` carrying an
> `api_error` payload inside an SSE `data:` frame rather than a `403`. This is expected — see the
> module README's status/type matrix.

### Clean pass-through

A benign prompt whose seeded response contains no PII returns a normal `HTTP 200` completion:

```shell
curl -s http://localhost:8080/v1/completions -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","stream":false,"max_tokens":128,"prompt":"What is NGINX?"}'
```

### Regenerating the dataset

The mock LLM (`llm-d-inference-sim`) does **not** hash the prompt string directly — it tokenizes the
prompt and hashes the resulting token IDs, then looks that hash up in the dataset. So the dataset
cannot be edited by hand: after changing `test-data.json` you must rebuild
`inference-sim-dataset.sqlite3` **with the same tokenizer the running simulator uses**, or the hashes
won't match and every prompt falls back to a random response (you'll see the same prompt return
rotating, unrelated answers).

#### Which tokenizer does the simulator actually use?

This is the subtle part. On startup the simulator probes
`https://huggingface.co/api/models/<--model>`. Only if that succeeds does it download and use the
**real** model tokenizer (`Qwen/Qwen3-32B`). In an offline / air-gapped / proxied cluster the probe
fails and the simulator **silently falls back to its built-in `SimpleTokenizer`** (a regex splitter +
FNV-32a hash), logging:

```text
"Model is not a real HF model, using simulated tokenizer" model="Qwen/Qwen3-32B"
```

Check which path your pod took:

```shell
kubectl logs deploy/vllm-qwen3-32b | grep -i tokenizer
```

The dataset **must** be built with whichever tokenizer the pod logs report.

#### Rebuilding for the offline (`SimpleTokenizer`) case

The committed `inference-sim-dataset.sqlite3` is built for the `SimpleTokenizer` (the offline default)
using the bundled `regen-dataset.py`, which reproduces the simulator's exact tokenization + hashing —
no HuggingFace access or `ds-tool` render server required:

```shell
python3 regen-dataset.py
kubectl create configmap inference-sim-dataset \
  --from-file=inference-sim-dataset.sqlite3=./inference-sim-dataset.sqlite3 \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deploy/vllm-qwen3-32b
```

Edit `test-data.json` (prompt → response pairs), rerun the three commands, and the mock LLM will
deterministically return your new responses.

#### Rebuilding for the online (real Qwen tokenizer) case

If your pod *can* reach HuggingFace and logs the real tokenizer, build the dataset with the upstream
`ds-tool` converter instead so the token IDs match. Follow the
[llm-d-inference-sim ds-tool docs](https://github.com/llm-d/llm-d-inference-sim/blob/main/docs/dataset_tool.md),
running the render server and converter:

```shell
make run-render MODEL_NAME=Qwen/Qwen3-32B RENDER_PORT=8082
make ds-tool-build
```

#### Why the requests set `max_tokens`

Matching the prompt hash only selects the *candidate* seeded response — the simulator then decides how
much of it to return based on a **target length**:

- `target = max_tokens` when the request sets it, otherwise `target = max-model-len - input_length`.
- On a hash hit, a seeded response whose length is `<= target` is returned **untrimmed**; a response
  **longer** than `target` is trimmed down to `target`.

The seeded responses here are short (≤64 tokens), so this rarely bites, but to keep the behavior
explicit and robust the example pins `--max-model-len 2048` in `llm.yaml` and sends `"max_tokens":128`
on each request — both comfortably above every seeded response, so a matched row is returned **in full**
and the embedded PII is never trimmed off the end (that PII is exactly what the response-blocking demo
must detect). If you add longer responses to `test-data.json`, raise these accordingly.

## Guardrails backend addressing

The Guardrails backend can live **outside** or **inside** the cluster. NGF picks the URL scheme from
the referenced Service's type and, **per Gateway**, from whether a
[`BackendTLSPolicy`](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/) **targets that
Service** and is **effective for that Gateway**; see the [per-Gateway note](#in-cluster-https-via-backendtlspolicy) below):

| Backend location | Service type | `BackendTLSPolicy` effective for this Gateway? | Resolved URL (per Gateway) |
| ------------------ | ------------- | ------------------- | -------------- |
| External | `ExternalName` | ignored (always system-trust https) | `https://<externalName>:<backendRef.port>` |
| In-cluster (plaintext) | `ClusterIP` (or any non-`ExternalName`) | no | `http://<name>.<namespace>.svc.<cluster-domain>:<backendRef.port>` |
| In-cluster (TLS) | `ClusterIP` (or any non-`ExternalName`) | yes | `https://<name>.<namespace>.svc.<cluster-domain>:<backendRef.port>` |

Two important rules regardless of location:

- **The port comes from `backendRef.port` in `payload-processor.yaml`**, not from the Service's own
  `.spec.ports`. Set them to the same value or the module will call a dead port.
- **Scheme selection is per Gateway:** external (`ExternalName`) backends are always called over
  **https**; in-cluster backends default to **http**, and are upgraded to **https** only for the
  Gateways for which a `BackendTLSPolicy` is *effective*. A `BackendTLSPolicy` **targets the backend
  Service**; it becomes effective for a Gateway only when that Gateway routes to the
  target Service and the policy is valid for it. So the same `PayloadProcessor` can be `https`
  (verified) on one Gateway and plaintext `http` on another for which the policy is not effective.
- **`BackendTLSPolicy` applies only to in-cluster (`ClusterIP`) backends.** A `BackendTLSPolicy` that
  targets an **`ExternalName`** guardrails Service is **ignored**: an ExternalName backend is always
  verified against the image's **system trust store** and uses a **variable `proxy_pass` + `resolver`**
  so a rotating external IP is re-resolved on every request. It is never pinned to a fixed IP or
  switched to a private CA by a `BackendTLSPolicy`. To verify a guardrails backend against a private or
  self-signed CA, use an in-cluster `ClusterIP` Service with a `BackendTLSPolicy` targeting it.

### In-cluster HTTPS via `BackendTLSPolicy`

An in-cluster ClusterIP backend is called over **https** when a `BackendTLSPolicy` targets its
Service. In that case NGF:

- verifies the backend certificate against the **CA bundle** referenced by the policy's
  `validation.caCertificateRefs` (a ConfigMap holding `ca.crt`), so **custom or self-signed CAs are
  supported** — there is no requirement that the backend cert chain to a public root;
- uses the policy's `validation.hostname` for `proxy_ssl_name` (SNI), the `Host` header, and
  certificate hostname verification (`proxy_ssl_verify on`);
- emits a **fixed `proxy_pass`** to the in-cluster Service FQDN
  (`https://<name>.<namespace>.svc.<cluster-domain>:<port>`) and therefore needs **no DNS resolver**
  (ClusterIP is stable; NGINX resolves it at worker startup).

This differs from the `ExternalName` path, which is verified against the **system trust store** and
uses a **variable `proxy_pass` + `resolver`** so a rotating external IP is re-resolved on every
request (see [below](#configuring-the-dns-resolver-required-for-externalname-backends)).

An in-cluster HTTPS example manifest is shown in [In-cluster HTTPS backend](#in-cluster-https-backend).

If a `BackendTLSPolicy` targets the backend Service but is invalid (or the referenced CA bundle is
missing), NGF fails **closed** (the `PayloadProcessor` is not programmed for that backend) rather than
silently falling back to plaintext. (If no `BackendTLSPolicy` targets the Service at all, the
in-cluster backend is simply called over plaintext **http**.)

> **Per-Gateway note:** a `BackendTLSPolicy` **attaches to the backend Service** (via
> `spec.targetRefs`, `kind: Service`). Its https upgrade is
> nonetheless scoped **per Gateway**: NGF derives the set of Gateways the policy is *effective* for
> from which Gateways route to the target Service, and applies the upgrade for a Gateway only when the
> policy is valid and accepted for it (subject to the usual ancestor limits). So if a
> `PayloadProcessor` is used by routes bound to multiple Gateways but the policy cannot be applied to
> some of them (e.g. ancestor-limit trimming or per-Gateway invalidity), the backend is reached over
> **https (verified)** on the effective Gateways and over plaintext **http** on the others — the scheme
> never leaks from one Gateway to another. To get https on every Gateway, ensure every such Gateway
> routes to the target Service and the policy is valid and accepted (within ancestor limits) for each.

Both inspection paths (request and response) reach the Guardrails backend the **same** way (see the
module README's [request/response architecture](../../internal/controller/nginx/modules/rust/ai-guardrails/README.md#request-path-vs-response-path)):
the module issues an NGINX **subrequest** through the internal NGINX location (`proxy_pass`). This
goes through NGINX's own connection and DNS machinery, so an `ExternalName` backend is resolved the
same way a normal proxied upstream would be — on both paths.

When using an `ExternalName` Guardrails backend, you **must** configure a `resolver` (via the
Gateway's NginxProxy) so the internal location can resolve the external hostname at request time.
This applies to all inspection traffic. See
[Configuring the DNS resolver](#configuring-the-dns-resolver-required-for-externalname-backends) below.

### Configuring the DNS resolver (required for `ExternalName` backends)

The guardrails inspection subrequest `proxy_pass`es to the `ExternalName` backend's hostname on both
the request and response paths. NGINX must be able to resolve that hostname **at request time**, so a
DNS `resolver` is required. When a resolver is configured, NGF emits a per-request (re-resolving)
`proxy_pass` so a rotating backend IP is picked up on every request; when it is missing, NGINX fails
to load the config (`no resolver defined to resolve <host>`) or pins a stale IP at worker startup.

Configure the resolver on an `NginxProxy` resource and attach it to the Gateway via
`spec.infrastructure.parametersRef`. This mirrors the
[`examples/externalname-service`](../externalname-service/gateway.yaml) pattern.

Add the `NginxProxy` (in the same namespace as the Gateway):

```yaml
apiVersion: gateway.nginx.org/v1alpha2
kind: NginxProxy
metadata:
  name: guardrails-nginx-config
spec:
  dnsResolver:
    addresses:
    - type: IPAddress
      value: "10.96.0.10"   # in-cluster kube-dns/CoreDNS ClusterIP (cluster-dependent)
    # timeout: "30s"        # optional
    # cacheTTL: "60s"       # optional
    # disableIPv6: false    # optional
```

> **Find your cluster's DNS ClusterIP:** `kubectl -n kube-system get svc kube-dns` (or `coredns`).
> `10.96.0.10` is a common default but is cluster-dependent. Use a public resolver (e.g. `8.8.8.8`)
> instead only if your provider's hostname is not resolvable through cluster DNS.

### External backend (default in this example)

`guardrails-service.yaml` ships as an `ExternalName` Service pointing at a hosted Guardrails API.
Replace the `<GUARDRAILS_API_HOSTNAME>` placeholder with your provider's hostname:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: guardrails-api
spec:
  type: ExternalName
  externalName: <GUARDRAILS_API_HOSTNAME>
  ports:
  - name: https
    port: 443
    protocol: TCP
```

With `backendRef.port: 443` in `payload-processor.yaml`, this resolves to
`https://<GUARDRAILS_API_HOSTNAME>:443`.

### In-cluster backend

To point at a Guardrails backend running inside the cluster, replace `guardrails-service.yaml` with
a normal Service that selects your backend Pods:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: guardrails-api          # keep the name referenced by payload-processor.yaml
spec:                           # no `type:` (defaults to ClusterIP), no `externalName:`
  selector:
    app: my-guardrails-backend  # must match your backend Pods' labels
  ports:
  - name: http
    port: 8080                  # whatever port your backend listens on
    targetPort: 8080
    protocol: TCP
```

Then update `payload-processor.yaml` so `backendRef.port` matches (e.g. `8080`). This resolves to
`http://guardrails-api.<namespace>.svc.cluster.local:8080`. Keep the Service in the same namespace
as the `PayloadProcessor`, or set `backendRef.namespace` explicitly.

### In-cluster HTTPS backend

To call an in-cluster Guardrails backend over **https** (including a backend that presents a
self-signed or private-CA certificate), keep the normal ClusterIP Service and add a
`BackendTLSPolicy` targeting it.

First, make the CA that signed your backend's serving certificate available as a ConfigMap holding
`ca.crt` (in the same namespace as the backend Service):

```shell
kubectl create configmap guardrails-ca --from-file=ca.crt=./ca.crt
```

Then apply the Service and `BackendTLSPolicy`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: guardrails-api          # keep the name referenced by payload-processor.yaml
spec:                           # ClusterIP (default); no `externalName:`
  selector:
    app: my-guardrails-backend  # must match your backend Pods' labels
  ports:
  - name: https
    port: 8443                  # TLS port your backend listens on
    targetPort: 8443
    protocol: TCP
---
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: guardrails-api-tls
spec:
  targetRefs:
  - group: ""
    kind: Service
    name: guardrails-api
  validation:
    caCertificateRefs:
    - group: ""
      kind: ConfigMap
      name: guardrails-ca
    # hostname must match a SAN on the backend's serving certificate; it is
    # used for SNI, the Host header, and certificate hostname verification.
    hostname: guardrails-api.default.svc.cluster.local
```

Set `backendRef.port: 8443` in `payload-processor.yaml` to match the Service port. This resolves to
`https://guardrails-api.<namespace>.svc.<cluster-domain>:8443`, verified against `guardrails-ca`. No
DNS resolver is required for the in-cluster HTTPS path.

## Troubleshooting

The `PayloadProcessor` is marked `Accepted=False` when its references cannot be resolved:

| Condition | Cause | Fix |
| ----------- | ------- | ----- |
| `backend Service ... not found` | `backendRef.name`/`namespace` does not match a Service. | Apply `guardrails-service.yaml`; check name and namespace. |
| `ExternalName service has empty ... externalName` | `ExternalName` Service with a blank `externalName`. | Set `spec.externalName`. |
| `auth token Secret ... not found` | `authTokenRef` set but Secret missing. | Apply `guardrails-secret.yaml`, or remove `authTokenRef`. |
| `auth token Secret ... missing "token" key` | Secret has no `token` key. | Add the token under `stringData.token`. |
| `auth token Secret ... has empty "token" key` | `token` key present but empty. | Populate the token value. |
| NGINX error `no resolver defined to resolve <host>`, or guardrails requests fail against an `ExternalName` backend | No `dnsResolver` configured on the NginxProxy. | Add the `dnsResolver` block and wire it via `parametersRef` (see [Configuring the DNS resolver](#configuring-the-dns-resolver-required-for-externalname-backends)). |

Other checks:

- Mock LLM Pod stuck in `ContainerCreating` → the `inference-sim-dataset` ConfigMap is missing
  (see [step 1](#1-seed-the-mock-llm-dataset)).
- Guardrails not taking effect → confirm NGF was installed with `--payload-processor` and an image
  that bundles the `ai-guardrails` module.
