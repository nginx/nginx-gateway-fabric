---
docs: "DOCS-1438"
---

{{<note>}}The [Gateway API resources](https://github.com/kubernetes-sigs/gateway-api) from the standard channel must be installed before deploying NGINX Gateway Fabric. If they are already installed in your cluster, please ensure they are the correct version as supported by the NGINX Gateway Fabric - [see the Technical Specifications](https://github.com/nginxinc/nginx-gateway-fabric/blob/v1.2.0/README.md#technical-specifications).{{</note>}}

To install the Gateway API resources, run the following:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml
```

Alternatively, you can install the Gateway API resources from the experimental channel. We support a subset of the
additional features provided by the experimental channel. To install from the experimental channel, run the following:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/experimental-install.yaml
```

If you are running on Kubernetes 1.23 or 1.24, you also need to install the validating webhook. To do so, run:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/webhook-install.yaml
```

{{< important >}}The validating webhook is not needed if you are running Kubernetes 1.25+. Validation is done using CEL on the CRDs. See the [resource validation doc]({{< relref "/overview/resource-validation.md" >}}) for more information.{{< /important >}}
