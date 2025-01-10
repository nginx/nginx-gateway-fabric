---
title: "Installation with Helm"
weight: 100
toc: true
docs: "DOCS-1430"
---

## Overview

Learn how to install, upgrade, and uninstall NGINX Gateway Fabric in a Kubernetes cluster using Helm.

{{< important >}} NGINX Plus users that are upgrading from version 1.4.0 to 1.5.x need to install an NGINX Plus JWT
Secret before upgrading. Follow the steps in the [Before you begin](#before-you-begin) section to create the Secret. If you use a different name than the default `nplus-license` name, specify the Secret name by setting `--set nginx.usage.secretName=<secret-name>` when running `helm upgrade`. {{< /important >}}

## Before you begin

To complete this guide, you'll need to install:

- [kubectl](https://kubernetes.io/docs/tasks/tools/), a command-line tool for managing Kubernetes clusters.
- [Helm 3.0 or later](https://helm.sh/docs/intro/install/), for deploying and managing applications on Kubernetes.

{{< important >}} If you’d like to use NGINX Plus, some additional setup is also required: {{</ important >}}
<details closed>
<summary>NGINX Plus JWT setup</summary>

{{<include "installation/jwt-password-note.md" >}}

### 1. Download the JWT from MyF5

{{<include "installation/nginx-plus/download-jwt.md" >}}

### 2. Create the Docker Registry Secret

{{<include "installation/nginx-plus/docker-registry-secret.md" >}}

### 3. Create the NGINX Plus Secret

{{<include "installation/nginx-plus/nginx-plus-secret.md" >}}

{{< note >}} For more information on why this is needed and additional configuration options, including how to report to NGINX Instance Manager instead, see the [NGINX Plus Image and JWT Requirement]({{< relref "installation/nginx-plus-jwt.md" >}}) document. {{< /note >}}

</details>

## Deploy NGINX Gateway Fabric

### Installing the Gateway API resources

{{<include "installation/install-gateway-api-resources.md" >}}

### Install from the OCI registry

The following steps install NGINX Gateway Fabric directly from the OCI helm registry. If you prefer, you can [install from sources](#install-from-sources) instead.

{{<tabs name="install-helm-oci">}}

{{%tab name="NGINX"%}}

To install the latest stable release of NGINX Gateway Fabric in the **nginx-gateway** namespace, run the following command:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway
```

{{% /tab %}}

{{%tab name="NGINX Plus"%}}

{{< note >}} If applicable, replace the F5 Container registry `private-registry.nginx.com` with your internal registry for your NGINX Plus image, and replace `nginx-plus-registry-secret` with your Secret name containing the registry credentials. If your NGINX Plus JWT Secret has a different name than the default `nplus-license`, then define that name using the `nginx.usage.secretName` flag. {{< /note >}}

To install the latest stable release of NGINX Gateway Fabric in the **nginx-gateway** namespace, run the following command:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric  --set nginx.image.repository=private-registry.nginx.com/nginx-gateway-fabric/nginx-plus --set nginx.plus=true --set serviceAccount.imagePullSecret=nginx-plus-registry-secret -n nginx-gateway
```

{{% /tab %}}

{{</tabs>}}

`ngf` is the name of the release, and can be changed to any name you want. This name is added as a prefix to the Deployment name.

If you want the latest version from the **main** branch, add `--version 0.0.0-edge` to your install command.

To wait for the Deployment to be ready, you can either add the `--wait` flag to the `helm install` command, or run the following after installing:

```shell
kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
```

### Install from sources {#install-from-sources}

If you prefer to install directly from sources, instead of through the OCI helm registry, use the following steps.

{{<include "installation/helm/pulling-the-chart.md" >}}

{{<tabs name="install-helm-src">}}

{{%tab name="NGINX"%}}

To install the chart into the **nginx-gateway** namespace, run the following command:

```shell
helm install ngf . --create-namespace -n nginx-gateway
```

{{% /tab %}}

{{%tab name="NGINX Plus"%}}

{{< note >}} If applicable, replace the F5 Container registry `private-registry.nginx.com` with your internal registry for your NGINX Plus image, and replace `nginx-plus-registry-secret` with your Secret name containing the registry credentials. If your NGINX Plus JWT Secret has a different name than the default `nplus-license`, then define that name using the `nginx.usage.secretName` flag. {{< /note >}}

To install the chart into the **nginx-gateway** namespace, run the following command:

```shell
helm install ngf . --set nginx.image.repository=private-registry.nginx.com/nginx-gateway-fabric/nginx-plus --set nginx.plus=true --set serviceAccount.imagePullSecret=nginx-plus-registry-secret -n nginx-gateway
```

{{% /tab %}}

{{</tabs>}}

   `ngf` is the name of the release, and can be changed to any name you want. This name is added as a prefix to the Deployment name.

   To wait for the Deployment to be ready, you can either add the `--wait` flag to the `helm install` command, or run the following after installing:

   ```shell
   kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
   ```

### Custom installation options

#### Service type

By default, the NGINX Gateway Fabric helm chart deploys a LoadBalancer Service.

To use a NodePort Service instead:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway --set service.type=NodePort
```

To disable the creation of a Service:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway --set service.create=false
```

#### Experimental features

We support a subset of the additional features provided by the Gateway API experimental channel. To enable the
experimental features of Gateway API which are supported by NGINX Gateway Fabric:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway --set nginxGateway.gwAPIExperimentalFeatures.enable=true
```

{{<note>}}Requires the Gateway APIs installed from the experimental channel.{{</note>}}

#### Examples

You can find several examples of configuration options of the `values.yaml` file in the [helm examples](https://github.com/nginx/nginx-gateway-fabric/tree/v1.6.0/examples/helm) directory.

### Access NGINX Gateway Fabric

{{<include "installation/expose-nginx-gateway-fabric.md" >}}

## Upgrade NGINX Gateway Fabric

{{<tip>}}For guidance on zero downtime upgrades, see the [Delay Pod Termination](#configure-delayed-pod-termination-for-zero-downtime-upgrades) section below.{{</tip>}}

To upgrade NGINX Gateway Fabric and get the latest features and improvements, take the following steps:

### Upgrade Gateway resources

To upgrade your Gateway API resources, take the following steps:

- Verify the Gateway API resources are compatible with your NGINX Gateway Fabric version. Refer to the [Technical Specifications]({{< relref "reference/technical-specifications.md" >}}) for details.
- Review the [release notes](https://github.com/kubernetes-sigs/gateway-api/releases) for any important upgrade-specific information.
- To upgrade the Gateway API resources, run:

  ```shell
  kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=v1.6.0" | kubectl apply -f -
  ```

  or, if you installed the from the experimental channel:

  ```shell
  kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/experimental?ref=v1.6.0" | kubectl apply -f -
  ```

### Upgrade NGINX Gateway Fabric CRDs

Helm's upgrade process does not automatically upgrade the NGINX Gateway Fabric CRDs (Custom Resource Definitions).

To upgrade the CRDs, take the following steps:

1. {{<include "installation/helm/pulling-the-chart.md" >}}

2. Upgrade the CRDs:

   ```shell
   kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/deploy/crds.yaml
   ```

   {{<note>}}Ignore the following warning, as it is expected.{{</note>}}

   ```text
   Warning: kubectl apply should be used on resource created by either kubectl create --save-config or kubectl apply.
   ```

### Upgrade NGINX Gateway Fabric release

{{< important >}} NGINX Plus users that are upgrading from version 1.4.0 to 1.5.x need to install an NGINX Plus JWT
Secret before upgrading. Follow the steps in the [Before you begin](#before-you-begin) section to create the Secret. If you use a different name than the default `nplus-license` name, specify the Secret name by setting `--set nginx.usage.secretName=<secret-name>` when running `helm upgrade`. {{</ important >}}

There are two possible ways to upgrade NGINX Gateway Fabric. You can either upgrade from the OCI registry, or download the chart and upgrade from the source.

#### Upgrade from the OCI registry

- To upgrade to the latest stable release of NGINX Gateway Fabric, run:

  ```shell
  helm upgrade ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric -n nginx-gateway
  ```

  If needed, replace `ngf` with your chosen release name.

#### Upgrade from sources

1. {{<include "installation/helm/pulling-the-chart.md" >}}

1. To upgrade, run: the following command:

   ```shell
   helm upgrade ngf . -n nginx-gateway
   ```

   If needed, replace `ngf` with your chosen release name.

## How to upgrade from NGINX OSS to NGINX Plus

- To upgrade from NGINX OSS to NGINX Plus, update the Helm command to include the necessary values for Plus:

  {{< note >}}If applicable, replace the F5 Container registry `private-registry.nginx.com` with your internal registry for your NGINX Plus image, and replace `nginx-plus-registry-secret` with your Secret name containing the registry credentials.{{< /note >}}

  {{< important >}}Ensure that you [Create the required JWT Secrets]({{< relref "installation/nginx-plus-jwt.md" >}}) before installing.{{< /important >}}

  ```shell
  helm upgrade ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric  --set nginx.image.repository=private-registry.nginx.com/nginx-gateway-fabric/nginx-plus --set nginx.plus=true --set serviceAccount.imagePullSecret=nginx-plus-registry-secret -n nginx-gateway
  ```

  If needed, replace `ngf` with your chosen release name.

## Delay pod termination for zero downtime upgrades {#configure-delayed-pod-termination-for-zero-downtime-upgrades}

{{< include "installation/delay-pod-termination/delay-pod-termination-overview.md" >}}

Follow these steps to configure delayed pod termination:

1. Open the `values.yaml` for editing.

1. **Add delayed shutdown hooks**:

   - In the `values.yaml` file, add `lifecycle: preStop` hooks to both the `nginx` and `nginx-gateway` container definitions. These hooks instruct the containers to delay their shutdown process, allowing time for connections to close gracefully. Update the `sleep` value to what works for your environment.

     ```yaml
      nginxGateway:
      <...>
      lifecycle:
          preStop:
          exec:
              command:
              - /usr/bin/gateway
              - sleep
              - --duration=40s # This flag is optional, the default is 30s

      nginx:
      <...>
      lifecycle:
          preStop:
          exec:
              command:
              - /bin/sleep
              - "40"
     ```

1. **Set the termination grace period**:

   - {{<include "installation/delay-pod-termination/termination-grace-period.md">}}

1. Save the changes.

{{<see-also>}}
For additional information on configuring and understanding the behavior of containers and pods during their lifecycle, refer to the following Kubernetes documentation:

- [Container Lifecycle Hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks)
- [Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/Pods/Pod-lifecycle/#Pod-termination)

{{</see-also>}}

## Uninstall NGINX Gateway Fabric

Follow these steps to uninstall NGINX Gateway Fabric and Gateway API from your Kubernetes cluster:

1. **Uninstall NGINX Gateway Fabric:**

   - To uninstall NGINX Gateway Fabric, run:

     ```shell
     helm uninstall ngf -n nginx-gateway
     ```

     If needed, replace `ngf` with your chosen release name.

2. **Remove namespace and CRDs:**

   - To remove the **nginx-gateway** namespace and its custom resource definitions (CRDs), run:

     ```shell
     kubectl delete ns nginx-gateway
     kubectl delete -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/deploy/crds.yaml
     ```

3. **Remove the Gateway API resources:**

   - {{<include "installation/uninstall-gateway-api-resources.md" >}}

## Additional configuration

For a full list of the Helm Chart configuration parameters, read [the NGINX Gateway Fabric Helm Chart](https://github.com/nginx/nginx-gateway-fabric/blob/v1.6.0/charts/nginx-gateway-fabric/README.md#configuration).
