---
docs: "DOCS-1438"
---

{{< note >}} The [Gateway API resources](https://github.com/kubernetes-sigs/gateway-api) from the standard channel must be installed before deploying NGINX Gateway Fabric. If they are already installed in your cluster, please ensure they are the correct version as supported by the NGINX Gateway Fabric - [see the Technical Specifications](https://github.com/nginx/nginx-gateway-fabric/blob/v1.6.0/README.md#technical-specifications). {{</ note >}}

To install the Gateway API resources, run the following:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=v1.6.0" | kubectl apply -f -
```

{{< note >}}If you plan to use the `edge` version of NGINX Gateway Fabric, you can replace the version in `ref` with `main`, for example `ref=main`.{{</ note >}}

Alternatively, you can install the Gateway API resources from the experimental channel.
Installing Gateway API resources from the experimental channel includes everything in the standard release channel plus additional experimental resources and fields.
NGINX Gateway Fabric currently supports a subset of the additional features provided by the experimental channel.
To install from the experimental channel, run the following:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/experimental?ref=v1.6.0" | kubectl apply -f -
```

{{< note >}} To learn more about what Gateway API resources NGINX Gateway Fabric currently supports, visit our [Gateway API Compatibility]({{< relref "overview/gateway-api-compatibility.md" >}}) document. {{</ note >}}
