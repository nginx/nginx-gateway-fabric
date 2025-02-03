# Provisioner

Provisioner implements data plane provisioning for NGINX Gateway Fabric (NGF): it creates an NGF static mode
Deployment for each Gateway that belongs to the provisioner GatewayClass.

```text
Usage:
  gateway provisioner-mode [flags]

Flags:
  -h, --help   help for provisioner-mode

Global Flags:
      --gateway-ctlr-name string   The name of the Gateway controller. The controller name must be of the form: DOMAIN/PATH. The controller's domain is 'gateway.nginx.org' (default "")
      --gatewayclass string        The name of the GatewayClass resource. Every NGINX Gateway Fabric must have a unique corresponding GatewayClass resource. (default "")
```

> Note: Provisioner is not ready for production yet (see this issue for more details
https://github.com/nginx/nginx-gateway-fabric/issues/634). However, it can be used in the Gateway API conformance
tests, which expect a Gateway API implementation to provision an independent data plane per Gateway.
>
> Note: Provisioner uses [this manifest](https://github.com/nginx/nginx-gateway-fabric/blob/main/config/tests/static-deployment.yaml)
to create an NGF static mode Deployment.
> This manifest gets included into the NGF binary during the NGF build. To customize the Deployment, modify the
manifest and **re-build** NGF.

How to deploy:

1. Follow the [installation](https://docs.nginx.com/nginx-gateway-fabric/installation/) instructions up until the Deploy the NGINX Gateway Fabric step
   to deploy prerequisites for both the static mode Deployments and the provisioner.
1. Deploy provisioner:

   ```shell
   kubectl apply -f provisioner.yaml
   ```

1. Confirm the provisioner is running in nginx-gateway namespace:

   ```shell
   kubectl get pods -n nginx-gateway
   ```

   ```text

   NAME                                         READY   STATUS    RESTARTS   AGE
   nginx-gateway-provisioner-6c9d9fdcb8-b2pf8   1/1     Running   0          11m
   ```
