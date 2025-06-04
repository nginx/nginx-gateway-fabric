# Deployment manifests

This directory contains the Kubernetes manifests for deploying NGINX Gateway Fabric in a Kubernetes cluster. They are generated from the Helm Chart [examples](../examples/helm/).

They are a single file deployment manifest that can be applied to a Kubernetes cluster using `kubectl apply -f <file>`. You should have the Gateway API CRDs and the NGINX Gateway Fabric CRDs deployed before applying these manifests.
The NGINX Gateway Fabric CRDs can be found in this directory as a single file deployment manifest [crds.yaml](./crds.yaml).

To deploy the manifests using a different registry or tag, you can modify the `kustomization.yaml` file with the desired values and
use the following command to apply the manifests:

```shell
kubectl kustomize | kubectl apply -f -
```

For more information on how to deploy NGINX Gateway Fabric and the Gateway API CRDs see the [installation guide](https://docs.nginx.com/nginx-gateway-fabric/install/manifests/).
