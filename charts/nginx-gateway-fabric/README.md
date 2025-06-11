
# NGINX Gateway Fabric Helm Chart

![Version: 2.0.1](https://img.shields.io/badge/Version-2.0.1-informational?style=flat-square) ![AppVersion: 2.0.1](https://img.shields.io/badge/AppVersion-2.0.1-informational?style=flat-square)

- [NGINX Gateway Fabric Helm Chart](#nginx-gateway-fabric-helm-chart)
  - [Introduction](#introduction)
  - [Prerequisites](#prerequisites)
    - [Installing the Gateway API resources](#installing-the-gateway-api-resources)
  - [Requirements](#requirements)
  - [Installing the Chart](#installing-the-chart)
    - [Installing the Chart from the OCI Registry](#installing-the-chart-from-the-oci-registry)
    - [Installing the Chart via Sources](#installing-the-chart-via-sources)
      - [Pulling the Chart](#pulling-the-chart)
      - [Installing the Chart](#installing-the-chart-1)
    - [Custom installation options](#custom-installation-options)
      - [Service type](#service-type)
  - [Upgrading the Chart](#upgrading-the-chart)
    - [Upgrading the Gateway Resources](#upgrading-the-gateway-resources)
    - [Upgrading the CRDs](#upgrading-the-crds)
    - [Upgrading the Chart from the OCI Registry](#upgrading-the-chart-from-the-oci-registry)
    - [Upgrading the Chart from the Sources](#upgrading-the-chart-from-the-sources)
    - [Configure Delayed Termination for Zero Downtime Upgrades](#configure-delayed-termination-for-zero-downtime-upgrades)
  - [Uninstalling the Chart](#uninstalling-the-chart)
    - [Uninstalling the Gateway Resources](#uninstalling-the-gateway-resources)
  - [Configuration](#configuration)

## Introduction

This chart deploys the NGINX Gateway Fabric in your Kubernetes cluster.

## Prerequisites

- [Helm 3.0+](https://helm.sh/docs/intro/install/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Installing the Gateway API resources

> [!NOTE]
>
> The [Gateway API resources](https://github.com/kubernetes-sigs/gateway-api) from the standard channel must be
> installed before deploying NGINX Gateway Fabric. If they are already installed in your cluster, please ensure
> they are the correct version as supported by the NGINX Gateway Fabric -
> [see the Technical Specifications](https://github.com/nginx/nginx-gateway-fabric/blob/main/README.md#technical-specifications).

```shell
kubectl kustomize https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard | kubectl apply -f -
```

## Requirements

Kubernetes: `>= 1.25.0-0`

## Installing the Chart

### Installing the Chart from the OCI Registry

To install the latest stable release of NGINX Gateway Fabric in the `nginx-gateway` namespace, run the following command:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway
```

`ngf` is the name of the release, and can be changed to any name you want. This name is added as a prefix to the Deployment name.

If the namespace already exists, you can omit the optional `--create-namespace` flag. If you want the latest version from the `main` branch, add `--version 0.0.0-edge` to your install command.

To wait for the Deployment to be ready, you can either add the `--wait` flag to the `helm install` command, or run
the following after installing:

```shell
kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
```

### Installing the Chart via Sources

#### Pulling the Chart

```shell
helm pull oci://ghcr.io/nginx/charts/nginx-gateway-fabric --untar
cd nginx-gateway-fabric
```

This will pull the latest stable release. To pull the latest version from the `main` branch, specify the
`--version 0.0.0-edge` flag when pulling.

#### Installing the Chart

To install the chart into the `nginx-gateway` namespace, run the following command.

```shell
helm install ngf . --create-namespace -n nginx-gateway
```

`ngf` is the name of the release, and can be changed to any name you want. This name is added as a prefix to the Deployment name.

If the namespace already exists, you can omit the optional `--create-namespace` flag.

To wait for the Deployment to be ready, you can either add the `--wait` flag to the `helm install` command, or run
the following after installing:

```shell
kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
```

### Custom installation options

#### Service type

By default, the NGINX Gateway Fabric helm chart deploys a LoadBalancer Service.

To use a NodePort Service instead:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway --set nginx.service.type=NodePort
```

## Upgrading the Chart

> [!NOTE]
>
> See [below](#configure-delayed-termination-for-zero-downtime-upgrades) for instructions on how to configure delayed
> termination if required for zero downtime upgrades in your environment.

### Upgrading the Gateway Resources

Before you upgrade a release, ensure the Gateway API resources are the correct version as supported by the NGINX
Gateway Fabric - [see the Technical Specifications](../../README.md#technical-specifications).:

To upgrade the Gateway CRDs from [the Gateway API repo](https://github.com/kubernetes-sigs/gateway-api), run:

```shell
kubectl kustomize https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard | kubectl apply -f -
```

### Upgrading the CRDs

Helm does not upgrade the NGINX Gateway Fabric CRDs during a release upgrade. Before you upgrade a release, you
must [pull the chart](#pulling-the-chart) from GitHub and run the following command to upgrade the CRDs:

```shell
kubectl apply -f crds/
```

The following warning is expected and can be ignored:

```text
Warning: kubectl apply should be used on resource created by either kubectl create --save-config or kubectl apply.
```

### Upgrading the Chart from the OCI Registry

To upgrade the release `ngf`, run:

```shell
helm upgrade ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric -n nginx-gateway
```

This will upgrade to the latest stable release. To upgrade to the latest version from the `main` branch, specify
the `--version 0.0.0-edge` flag when upgrading.

### Upgrading the Chart from the Sources

Pull the chart sources as described in [Pulling the Chart](#pulling-the-chart), if not already present. Then, to upgrade
the release `ngf`, run:

```shell
helm upgrade ngf . -n nginx-gateway
```

### Configure Delayed Termination for Zero Downtime Upgrades

To achieve zero downtime upgrades (meaning clients will not see any interruption in traffic while a rolling upgrade is
being performed on NGF), you may need to configure delayed termination on the NGF Pod, depending on your environment.

> [!NOTE]
>
> When proxying Websocket or any long-lived connections, NGINX will not terminate until that connection is closed
> by either the client or the backend. This means that unless all those connections are closed by clients/backends
> before or during an upgrade, NGINX will not terminate, which means Kubernetes will kill NGINX. As a result, the
> clients will see the connections abruptly closed and thus experience downtime.

1. Add `lifecycle` to both the nginx and the nginx-gateway container definition. To do so, update your `values.yaml`
   file to include the following (update the `sleep` values to what is required in your environment):

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

2. Ensure the `terminationGracePeriodSeconds` matches or exceeds the `sleep` value from the `preStopHook` (the default
   is 30). This is to ensure Kubernetes does not terminate the Pod before the `preStopHook` is complete. To do so,
   update your `values.yaml` file to include the following (update the value to what is required in your environment):

   ```yaml
   terminationGracePeriodSeconds: 50
   ```

> [!NOTE]
>
> More information on container lifecycle hooks can be found in the official
> [kubernetes documentation](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks) and a detailed
> description of Pod termination behavior can be found in
> [Termination of Pods](https://kubernetes.io/docs/concepts/workloads/Pods/Pod-lifecycle/#Pod-termination).

## Uninstalling the Chart

To uninstall/delete the release `ngf`:

```shell
helm uninstall ngf -n nginx-gateway
kubectl delete ns nginx-gateway
kubectl delete -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/main/deploy/crds.yaml
```

These commands remove all the Kubernetes components associated with the release and deletes the release.

### Uninstalling the Gateway Resources

> **Warning: This command will delete all the corresponding custom resources in your cluster across all namespaces!
> Please ensure there are no custom resources that you want to keep and there are no other Gateway API implementations
> running in the cluster!**

To delete the Gateway API CRDs from [the Gateway API repo](https://github.com/kubernetes-sigs/gateway-api), run:

```shell
kubectl kustomize https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard | kubectl delete -f -
```

## Configuration

The following table lists the configurable parameters of the NGINX Gateway Fabric chart and their default values.

> More granular configuration options may not show up in this table.
> Viewing the `values.yaml` file directly can show all available options.

| Key | Description | Type | Default |
|-----|-------------|------|---------|
| `certGenerator` | The certGenerator section contains the configuration for the cert-generator Job. | object | `{"affinity":{},"agentTLSSecretName":"agent-tls","annotations":{},"nodeSelector":{},"overwrite":false,"serverTLSSecretName":"server-tls","tolerations":[],"topologySpreadConstraints":[],"ttlSecondsAfterFinished":30}` |
| `certGenerator.affinity` | The affinity of the cert-generator pod. | object | `{}` |
| `certGenerator.agentTLSSecretName` | The name of the base Secret containing TLS CA, certificate, and key for the NGINX Agent to securely communicate with the NGINX Gateway Fabric control plane. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). | string | `"agent-tls"` |
| `certGenerator.annotations` | The annotations of the cert-generator Job. | object | `{}` |
| `certGenerator.nodeSelector` | The nodeSelector of the cert-generator pod. | object | `{}` |
| `certGenerator.overwrite` | Overwrite existing TLS Secrets on startup. | bool | `false` |
| `certGenerator.serverTLSSecretName` | The name of the Secret containing TLS CA, certificate, and key for the NGINX Gateway Fabric control plane to securely communicate with the NGINX Agent. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). | string | `"server-tls"` |
| `certGenerator.tolerations` | Tolerations for the cert-generator pod. | list | `[]` |
| `certGenerator.topologySpreadConstraints` | The topology spread constraints for the cert-generator pod. | list | `[]` |
| `certGenerator.ttlSecondsAfterFinished` | How long to wait after the cert generator job has finished before it is removed by the job controller. | int | `30` |
| `clusterDomain` | The DNS cluster domain of your Kubernetes cluster. | string | `"cluster.local"` |
| `gateways` | A list of Gateway objects. View https://gateway-api.sigs.k8s.io/reference/spec/#gateway for full Gateway reference. | list | `[]` |
| `nginx` | The nginx section contains the configuration for all NGINX data plane deployments installed by the NGINX Gateway Fabric control plane. | object | `{"config":{},"container":{},"debug":false,"image":{"pullPolicy":"IfNotPresent","repository":"ghcr.io/nginx/nginx-gateway-fabric/nginx","tag":"2.0.1"},"imagePullSecret":"","imagePullSecrets":[],"kind":"deployment","plus":false,"pod":{},"replicas":1,"service":{"externalTrafficPolicy":"Local","loadBalancerClass":"","loadBalancerIP":"","loadBalancerSourceRanges":[],"nodePorts":[],"type":"LoadBalancer"},"usage":{"caSecretName":"","clientSSLSecretName":"","endpoint":"","resolver":"","secretName":"nplus-license","skipVerify":false}}` |
| `nginx.config` | The configuration for the data plane that is contained in the NginxProxy resource. This is applied globally to all Gateways managed by this instance of NGINX Gateway Fabric. | object | `{}` |
| `nginx.container` | The container configuration for the NGINX container. This is applied globally to all Gateways managed by this instance of NGINX Gateway Fabric. | object | `{}` |
| `nginx.debug` | Enable debugging for NGINX. Uses the nginx-debug binary. The NGINX error log level should be set to debug in the NginxProxy resource. | bool | `false` |
| `nginx.image.repository` | The NGINX image to use. | string | `"ghcr.io/nginx/nginx-gateway-fabric/nginx"` |
| `nginx.imagePullSecret` | The name of the secret containing docker registry credentials. Secret must exist in the same namespace as the helm release. The control plane will copy this secret into any namespace where NGINX is deployed. | string | `""` |
| `nginx.imagePullSecrets` | A list of secret names containing docker registry credentials. Secrets must exist in the same namespace as the helm release. The control plane will copy these secrets into any namespace where NGINX is deployed. | list | `[]` |
| `nginx.kind` | The kind of NGINX deployment. | string | `"deployment"` |
| `nginx.plus` | Is NGINX Plus image being used. | bool | `false` |
| `nginx.pod` | The pod configuration for the NGINX data plane pod. This is applied globally to all Gateways managed by this instance of NGINX Gateway Fabric. | object | `{}` |
| `nginx.replicas` | The number of replicas of the NGINX Deployment. | int | `1` |
| `nginx.service` | The service configuration for the NGINX data plane. This is applied globally to all Gateways managed by this instance of NGINX Gateway Fabric. | object | `{"externalTrafficPolicy":"Local","loadBalancerClass":"","loadBalancerIP":"","loadBalancerSourceRanges":[],"nodePorts":[],"type":"LoadBalancer"}` |
| `nginx.service.externalTrafficPolicy` | The externalTrafficPolicy of the service. The value Local preserves the client source IP. | string | `"Local"` |
| `nginx.service.loadBalancerClass` | LoadBalancerClass is the class of the load balancer implementation this Service belongs to. Requires nginx.service.type set to LoadBalancer. | string | `""` |
| `nginx.service.loadBalancerIP` | The static IP address for the load balancer. Requires nginx.service.type set to LoadBalancer. | string | `""` |
| `nginx.service.loadBalancerSourceRanges` | The IP ranges (CIDR) that are allowed to access the load balancer. Requires nginx.service.type set to LoadBalancer. | list | `[]` |
| `nginx.service.nodePorts` | A list of NodePorts to expose on the NGINX data plane service. Each NodePort MUST map to a Gateway listener port, otherwise it will be ignored. The default NodePort range enforced by Kubernetes is 30000-32767. | list | `[]` |
| `nginx.service.type` | The type of service to create for the NGINX data plane. | string | `"LoadBalancer"` |
| `nginx.usage.caSecretName` | The name of the Secret containing the NGINX Instance Manager CA certificate. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). | string | `""` |
| `nginx.usage.clientSSLSecretName` | The name of the Secret containing the client certificate and key for authenticating with NGINX Instance Manager. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). | string | `""` |
| `nginx.usage.endpoint` | The endpoint of the NGINX Plus usage reporting server. Default: product.connect.nginx.com | string | `""` |
| `nginx.usage.resolver` | The nameserver used to resolve the NGINX Plus usage reporting endpoint. Used with NGINX Instance Manager. | string | `""` |
| `nginx.usage.secretName` | The name of the Secret containing the JWT for NGINX Plus usage reporting. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). | string | `"nplus-license"` |
| `nginx.usage.skipVerify` | Disable client verification of the NGINX Plus usage reporting server certificate. | bool | `false` |
| `nginxGateway` | The nginxGateway section contains configuration for the NGINX Gateway Fabric control plane deployment. | object | `{"affinity":{},"config":{"logging":{"level":"info"}},"configAnnotations":{},"extraVolumeMounts":[],"extraVolumes":[],"gatewayClassAnnotations":{},"gatewayClassName":"nginx","gatewayControllerName":"gateway.nginx.org/nginx-gateway-controller","gwAPIExperimentalFeatures":{"enable":false},"image":{"pullPolicy":"IfNotPresent","repository":"ghcr.io/nginx/nginx-gateway-fabric","tag":"2.0.1"},"kind":"deployment","labels":{},"leaderElection":{"enable":true,"lockName":""},"lifecycle":{},"metrics":{"enable":true,"port":9113,"secure":false},"nodeSelector":{},"podAnnotations":{},"productTelemetry":{"enable":true},"readinessProbe":{"enable":true,"initialDelaySeconds":3,"port":8081},"replicas":1,"resources":{},"service":{"annotations":{},"labels":{}},"serviceAccount":{"annotations":{},"imagePullSecret":"","imagePullSecrets":[],"name":""},"snippetsFilters":{"enable":false},"terminationGracePeriodSeconds":30,"tolerations":[],"topologySpreadConstraints":[]}` |
| `nginxGateway.affinity` | The affinity of the NGINX Gateway Fabric control plane pod. | object | `{}` |
| `nginxGateway.config.logging.level` | Log level. | string | `"info"` |
| `nginxGateway.configAnnotations` | Set of custom annotations for NginxGateway objects. | object | `{}` |
| `nginxGateway.extraVolumeMounts` | extraVolumeMounts are the additional volume mounts for the nginx-gateway container. | list | `[]` |
| `nginxGateway.extraVolumes` | extraVolumes for the NGINX Gateway Fabric control plane pod. Use in conjunction with nginxGateway.extraVolumeMounts mount additional volumes to the container. | list | `[]` |
| `nginxGateway.gatewayClassAnnotations` | Set of custom annotations for GatewayClass objects. | object | `{}` |
| `nginxGateway.gatewayClassName` | The name of the GatewayClass that will be created as part of this release. Every NGINX Gateway Fabric must have a unique corresponding GatewayClass resource. NGINX Gateway Fabric only processes resources that belong to its class - i.e. have the "gatewayClassName" field resource equal to the class. | string | `"nginx"` |
| `nginxGateway.gatewayControllerName` | The name of the Gateway controller. The controller name must be of the form: DOMAIN/PATH. The controller's domain is gateway.nginx.org. | string | `"gateway.nginx.org/nginx-gateway-controller"` |
| `nginxGateway.gwAPIExperimentalFeatures.enable` | Enable the experimental features of Gateway API which are supported by NGINX Gateway Fabric. Requires the Gateway APIs installed from the experimental channel. | bool | `false` |
| `nginxGateway.image` | The image configuration for the NGINX Gateway Fabric control plane. | object | `{"pullPolicy":"IfNotPresent","repository":"ghcr.io/nginx/nginx-gateway-fabric","tag":"2.0.1"}` |
| `nginxGateway.image.repository` | The NGINX Gateway Fabric image to use | string | `"ghcr.io/nginx/nginx-gateway-fabric"` |
| `nginxGateway.kind` | The kind of the NGINX Gateway Fabric installation - currently, only deployment is supported. | string | `"deployment"` |
| `nginxGateway.labels` | Set of labels to be added for NGINX Gateway Fabric deployment. | object | `{}` |
| `nginxGateway.leaderElection.enable` | Enable leader election. Leader election is used to avoid multiple replicas of the NGINX Gateway Fabric reporting the status of the Gateway API resources. If not enabled, all replicas of NGINX Gateway Fabric will update the statuses of the Gateway API resources. | bool | `true` |
| `nginxGateway.leaderElection.lockName` | The name of the leader election lock. A Lease object with this name will be created in the same Namespace as the controller. | string | Autogenerated if not set or set to "". |
| `nginxGateway.lifecycle` | The lifecycle of the nginx-gateway container. | object | `{}` |
| `nginxGateway.metrics.enable` | Enable exposing metrics in the Prometheus format. | bool | `true` |
| `nginxGateway.metrics.port` | Set the port where the Prometheus metrics are exposed. | int | `9113` |
| `nginxGateway.metrics.secure` | Enable serving metrics via https. By default metrics are served via http. Please note that this endpoint will be secured with a self-signed certificate. | bool | `false` |
| `nginxGateway.nodeSelector` | The nodeSelector of the NGINX Gateway Fabric control plane pod. | object | `{}` |
| `nginxGateway.podAnnotations` | Set of custom annotations for the NGINX Gateway Fabric pods. | object | `{}` |
| `nginxGateway.productTelemetry.enable` | Enable the collection of product telemetry. | bool | `true` |
| `nginxGateway.readinessProbe.enable` | Enable the /readyz endpoint on the control plane. | bool | `true` |
| `nginxGateway.readinessProbe.initialDelaySeconds` | The number of seconds after the Pod has started before the readiness probes are initiated. | int | `3` |
| `nginxGateway.readinessProbe.port` | Port in which the readiness endpoint is exposed. | int | `8081` |
| `nginxGateway.replicas` | The number of replicas of the NGINX Gateway Fabric Deployment. | int | `1` |
| `nginxGateway.resources` | The resource requests and/or limits of the nginx-gateway container. | object | `{}` |
| `nginxGateway.service` | The service configuration for the NGINX Gateway Fabric control plane. | object | `{"annotations":{},"labels":{}}` |
| `nginxGateway.service.annotations` | The annotations of the NGINX Gateway Fabric control plane service. | object | `{}` |
| `nginxGateway.service.labels` | The labels of the NGINX Gateway Fabric control plane service. | object | `{}` |
| `nginxGateway.serviceAccount` | The serviceaccount configuration for the NGINX Gateway Fabric control plane. | object | `{"annotations":{},"imagePullSecret":"","imagePullSecrets":[],"name":""}` |
| `nginxGateway.serviceAccount.annotations` | Set of custom annotations for the NGINX Gateway Fabric control plane service account. | object | `{}` |
| `nginxGateway.serviceAccount.imagePullSecret` | The name of the secret containing docker registry credentials for the control plane. Secret must exist in the same namespace as the helm release. | string | `""` |
| `nginxGateway.serviceAccount.imagePullSecrets` | A list of secret names containing docker registry credentials for the control plane. Secrets must exist in the same namespace as the helm release. | list | `[]` |
| `nginxGateway.serviceAccount.name` | The name of the service account of the NGINX Gateway Fabric control plane pods. Used for RBAC. | string | Autogenerated if not set or set to "" |
| `nginxGateway.snippetsFilters.enable` | Enable SnippetsFilters feature. SnippetsFilters allow inserting NGINX configuration into the generated NGINX config for HTTPRoute and GRPCRoute resources. | bool | `false` |
| `nginxGateway.terminationGracePeriodSeconds` | The termination grace period of the NGINX Gateway Fabric control plane pod. | int | `30` |
| `nginxGateway.tolerations` | Tolerations for the NGINX Gateway Fabric control plane pod. | list | `[]` |
| `nginxGateway.topologySpreadConstraints` | The topology spread constraints for the NGINX Gateway Fabric control plane pod. | list | `[]` |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs](https://github.com/norwoodj/helm-docs)
