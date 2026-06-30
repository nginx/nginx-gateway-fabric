# Worker Processes

This example shows how to control the number of NGINX worker processes using the
`workerProcesses` field on the `NginxProxy` resource.

By default, NGINX uses `worker_processes auto;`, which starts one worker per CPU
core on the node. On large nodes where the data plane Pod is given a small CPU
limit, this can spawn far more workers than intended, increasing memory usage.
Setting `workerProcesses` lets you pin a fixed worker count.

The field accepts an integer between `1` and `1024`. Omit the field to use the
default `auto`. It applies to both OSS NGINX and NGINX Plus.

## Apply the NginxProxy

Reference the `NginxProxy` from either a `GatewayClass` (`parametersRef`) to apply
the setting to all attached Gateways, or from a single `Gateway`
(`infrastructure.parametersRef`) to apply it to that Gateway alone.

Attach to a `Gateway`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
spec:
  gatewayClassName: nginx
  infrastructure:
    parametersRef:
      group: gateway.nginx.org
      kind: NginxProxy
      name: worker-processes-config
  listeners:
  - name: http
    port: 80
    protocol: HTTP
```

Then apply the `NginxProxy`:

```shell
kubectl apply -f nginx-proxy.yaml
```

You can verify the result by checking that the generated NGINX configuration
contains `worker_processes 2;`.
