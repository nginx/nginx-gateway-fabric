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
| `inference-sim-dataset.sqlite3` | Canned dataset served by the mock LLM. |
| `inference-sim-dataset.json` | Human-readable source for the dataset. |
| `test-data.json` | Sample request payloads for testing. |

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

If your backend requires authentication, set the token in `guardrails-secret.yaml` (the value must
live under the `token` key) and keep the `authTokenRef` in `payload-processor.yaml`. If it does not,
remove the `authTokenRef` block from `payload-processor.yaml` and skip the Secret.

### 3. Apply the manifests

```shell
kubectl apply -f gateway.yaml
kubectl apply -f llm.yaml
kubectl apply -f llm-route.yaml
kubectl apply -f guardrails-service.yaml
kubectl apply -f guardrails-secret.yaml       # skip if no auth token is needed
kubectl apply -f payload-processor.yaml
```

### 4. Verify

Confirm the policy was accepted:

```shell
kubectl get payloadprocessor llm-guardrails -o yaml
```

A rejected policy reports `Accepted=False` in its status conditions. Common causes are listed under
[Troubleshooting](#troubleshooting).

## Guardrails backend addressing

The Guardrails backend can live **outside** or **inside** the cluster. NGF picks the URL scheme from
the referenced Service's type (resolved in `resolveExtProcessURL`, `payloadprocessor.go`):

| Backend location | Service type | Resolved URL |
| ------------------ | ------------- | -------------- |
| External | `ExternalName` | `https://<externalName>:<backendRef.port>` |
| In-cluster | `ClusterIP` (or any non-`ExternalName`) | `http://<name>.<namespace>.svc.cluster.local:<backendRef.port>` |

Two important rules regardless of location:

- **The port comes from `backendRef.port` in `payload-processor.yaml`**, not from the Service's own
  `.spec.ports`. Set them to the same value or the module will call a dead port.
- **Current scheme limitation:** external backends are always called over **https**, and in-cluster
  backends always over **http**. An in-cluster HTTPS backend or an external HTTP backend cannot be
  expressed today.

The `ai-guardrails` module makes its own outbound HTTP call (it does not proxy through NGINX), so an
`ExternalName` Guardrails backend does **not** require a DNS resolver in the Gateway's NginxProxy —
unlike `ExternalName` Services used as HTTPRoute backends.

### External backend (default in this example)

`guardrails-service.yaml` ships as an `ExternalName` Service pointing at a hosted Guardrails API:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: guardrails-api
spec:
  type: ExternalName
  externalName: us1.calypsoai.app
  ports:
  - name: https
    port: 443
    protocol: TCP
```

With `backendRef.port: 443` in `payload-processor.yaml`, this resolves to
`https://us1.calypsoai.app:443`.

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

## Troubleshooting

The `PayloadProcessor` is marked `Accepted=False` when its references cannot be resolved:

| Condition | Cause | Fix |
| ----------- | ------- | ----- |
| `backend Service ... not found` | `backendRef.name`/`namespace` does not match a Service. | Apply `guardrails-service.yaml`; check name and namespace. |
| `ExternalName service has empty ... externalName` | `ExternalName` Service with a blank `externalName`. | Set `spec.externalName`. |
| `auth token Secret ... not found` | `authTokenRef` set but Secret missing. | Apply `guardrails-secret.yaml`, or remove `authTokenRef`. |
| `auth token Secret ... missing "token" key` | Secret has no `token` key. | Add the token under `stringData.token`. |
| `auth token Secret ... has empty "token" key` | `token` key present but empty. | Populate the token value. |

Other checks:

- Mock LLM Pod stuck in `ContainerCreating` → the `inference-sim-dataset` ConfigMap is missing
  (see [step 1](#1-seed-the-mock-llm-dataset)).
- Guardrails not taking effect → confirm NGF was installed with `--payload-processor` and an image
  that bundles the `ai-guardrails` module.
