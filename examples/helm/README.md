# Helm Chart Examples

This directory contains examples of Helm charts that can be used to deploy NGINX Gateway Fabric in a Kubernetes cluster.

## Prerequisites

- Helm 3.x

## Examples

- [Default](./default) - deploys NGINX Gateway Fabric with NGINX OSS with default configuration.
- [NGINX Plus](./nginx-plus) - deploys NGINX Gateway Fabric with NGINX Plus as the data plane. The image is pulled from the
  NGINX Plus Docker registry, and the `imagePullSecretName` is the name of the secret to use to pull the image.
  The secret must be created in the same namespace as the NGINX Gateway Fabric deployment.
- [Experimental](./experimental) - deploys NGINX Gateway Fabric with the Gateway API experimental features enabled and NGINX OSS as the data plane.
- [Experimental with NGINX Plus](./experimental-nginx-plus) - deploys NGINX Gateway Fabric with the Gateway API experimental features enabled and NGINX Plus as the data plane. The image is pulled from the NGINX Plus Docker registry, and the `imagePullSecretName` is the name of the secret to use to pull the image. The secret must be created in the same namespace as the NGINX Gateway Fabric deployment.
- [Azure](./azure) - deploys NGINX Gateway Fabric with NGINX OSS using a nodeSelector to deploy the gateway on Linux nodes in an Azure Kubernetes Service (AKS) cluster.
- [NodePort](./nodeport) - deploys NGINX Gateway Fabric with NGINX OSS using a Service of type `NodePort` to expose the gateway on a specific port on each node.

## Manifests generation

These examples are used to generate the manifests for the NGINX Gateway Fabric located in the deploy directory [here](../../deploy).

If you want to generate manifests for a specific example, or need to customize one of the examples, run the following
command from the root of the project:

```shell
helm template nginx-gateway --namespace nginx-gateway --values examples/helm/<example>/values.yaml charts/nginx-gateway-fabric
```
