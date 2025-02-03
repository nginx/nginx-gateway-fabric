---
title: "Install NGINX Gateway Fabric with experimental features"
docs: "DOCS-000"
---

To use Gateway API experimental resources, the Gateway API resources from the experimental channel must be installed before deploying NGINX Gateway Fabric. Additionally, NGINX Gateway Fabric must have experimental features enabled.

{{< caution >}}As noted in the [Gateway API documentation](https://gateway-api.sigs.k8s.io/guides/#install-experimental-channel), future releases of the Gateway API can include breaking changes to experimental resources and fields. {{</ caution >}}

To install the Gateway API resources from the experimental channel, run the following:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/experimental?ref=v1.6.0" | kubectl apply -f -
```

{{< note >}}If you plan to use the `edge` version of NGINX Gateway Fabric, you can replace the version in `ref` with `main`, for example `ref=main`. {{</ note >}}

To enable experimental features on NGINX Gateway Fabric:

Using Helm: Set `nginxGateway.gwAPIExperimentalFeatures.enable` to true. An example can be found
in the [Installation with Helm]({{< relref "installation/installing-ngf/helm.md#custom-installation-options" >}}) guide.

Using Kubernetes manifests: Add the `--gateway-api-experimental-features` command-line flag to the deployment manifest args.
An example can be found in the [Installation with Kubernetes manifests]({{< relref "installation/installing-ngf/manifests.md#3-deploy-nginx-gateway-fabric" >}}) guide.
