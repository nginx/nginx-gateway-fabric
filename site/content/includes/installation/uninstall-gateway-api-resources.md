---
docs: "DOCS-1436"
---

{{<warning>}}This will remove all corresponding custom resources in your entire cluster, across all namespaces. Double-check to make sure you don't have any custom resources you need to keep, and confirm that there are no other Gateway API implementations active in your cluster.{{</warning>}}

To uninstall the Gateway API resources, run the following:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=v1.6.0" | kubectl delete -f -
```

Alternatively, if you installed the Gateway APIs from the experimental channel, run the following:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/experimental?ref=v1.6.0" | kubectl delete -f -
```
