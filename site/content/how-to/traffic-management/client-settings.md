---
title: "Client Settings Policy API"
weight: 800
toc: true
docs: "DOCS-000"
---

Learn how to use the `ClientSettingsPolicy` API.

## Overview

The `ClientSettingsPolicy` API allows Cluster Operators and Application Developers to configure the connection behavior between the client and NGINX.

The settings in `ClientSettingsPolicy` correspond to the following NGINX directives:

- [`client_max_body_size`](<https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size>)
- [`client_body_timeout`](<https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_timeout>)
- [`keepalive_requests`](<https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_requests>)
- [`keepalive_time`](<https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_time>)
- [`keepalive_timeout`](<https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_timeout>)

`ClientSettingsPolicy` is an [Inherited Policy Attachment](https://gateway-api.sigs.k8s.io/reference/policy-attachment/) that can be applied to a Gateway, HTTPRoute, or GRPCRoute in the same namespace as the `ClientSettingsPolicy`.

When applied to a Gateway, the settings specified in the `ClientSettingsPolicy` affect all HTTPRoutes and GRPCRoutes attached to the Gateway. This allows Cluster Operators to set defaults for all applications using the Gateway.

When applied to an HTTPRoute or GRPCRoute, the settings in the `ClientSettingsPolicy` affect only the route they are applied to. This allows Application Developers to set values for their applications based on their application's behavior or requirements.
Settings applied to an HTTPRoute or GRPCRoute take precedence over settings applied to a Gateway. See the [custom policies]({{< relref "overview/custom-policies.md" >}}) document for more information on policies.

This guide will show you how to use the `ClientSettingsPolicy` API to configure the client max body size for your applications.

For all the possible configuration options for `ClientSettingsPolicy`, see the [API reference]({{< relref "reference/api.md" >}}).

## Setup

- [Install]({{< relref "/installation/" >}}) NGINX Gateway Fabric.
- Save the public IP address and port of NGINX Gateway Fabric into shell variables:

   ```text
  GW_IP=XXX.YYY.ZZZ.III
  GW_PORT=<port number>
  ```

  {{< note >}}In a production environment, you should have a DNS record for the external IP address that is exposed, and it should refer to the hostname that the gateway will forward for.{{< /note >}}

- Create the coffee and tea example applications:

  ```yaml
  kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/examples/client-settings-policy/app.yaml
  ```

- Create a Gateway:

  ```yaml
  kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/examples/client-settings-policy/gateway.yaml
   ```

- Create HTTPRoutes for the coffee and tea applications:

  ```yaml
  kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/examples/client-settings-policy/httproutes.yaml
   ```

- Test the configuration:

  You can send traffic to the coffee and tea applications using the external IP address and port for NGINX Gateway Fabric.

  Send a request to coffee:

  ```shell
  curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
  ```

  This request should receive a response from the coffee Pod:

  ```text
  Server address: 10.244.0.9:8080
  Server name: coffee-76c7c85bbd-cf8nz
  ```

  Send a request to tea:

  ```shell
  curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
   ```

  This request should receive a response from the tea Pod:

  ```text
  Server address: 10.244.0.9:8080
  Server name: tea-76c7c85bbd-cf8nz
  ```

## Configure client max body size

### Set a default client max body size for the Gateway

To set a default client max body size for the Gateway created during setup, add the following `ClientSettingsPolicy`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: ClientSettingsPolicy
metadata:
  name: gateway-client-settings
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway
  body:
    maxSize: "50" # sizes without a unit are bytes.
EOF
```

This `ClientSettingsPolicy` targets the Gateway we created in the setup by specifying it in the `targetRef` field. It limits the max client body size to 50 bytes.
Since this policy is applied to the Gateway, it will affect all HTTPRoutes and GRPCRoutes attached to the Gateway. All requests to the coffee and tea applications must have a request body of less than or equal to 50 bytes.

Verify that the `ClientSettingsPolicy` is Accepted:

```shell
kubectl describe clientsettingspolicies.gateway.nginx.org gateway-client-settings
```

You should see the following status:

```text
Status:
  Ancestors:
    Ancestor Ref:
      Group:      gateway.networking.k8s.io
      Kind:       Gateway
      Name:       gateway
      Namespace:  default
    Conditions:
      Last Transition Time:  2024-05-30T19:57:18Z
      Message:               Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

Next, test that the policy is configured by sending a POST request to the coffee and tea applications exceeding the client's max body size of 50 bytes.

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee -X POST --data "this payload is greater than fifty bytes by four bytes"
```

You should receive the following error:

```text
<html>
<head><title>413 Request Entity Too Large</title></head>
<body>
<center><h1>413 Request Entity Too Large</h1></center>
<hr><center>nginx/1.25.5</center>
</body>
</html>
```

Try again with a payload that's less than the 50 byte limit:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee -X POST --data "this payload is under fifty bytes"
```

This time, you should receive a response from coffee:

```text
Server address: 10.244.0.6:8080
Server name: coffee-56b44d4c55-7ldjc
```

You can repeat this test with the tea application to confirm that the policy affects both HTTPRoutes.

### Set a different client max body size for a route

To set a different client max body size for a particular route, you can create another `ClientSettingsPolicy` that targets the route:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: ClientSettingsPolicy
metadata:
  name: tea-client-settings
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: tea
  body:
    maxSize: "75" # sizes without a unit are bytes.
EOF
```

This `ClientSettingsPolicy` targets the tea HTTPRoute we created in the setup by specifying it in the `targetRef` field. It sets the max client body size to 75 bytes.
Since this policy is applied to the tea HTTPRoute, it will only affect the tea HTTPRoute, and the `ClientSettingsPolicy` we created in the previous step will affect all other routes attached to the Gateway. This means that the coffee app still has a client max body size of 50 bytes, and the tea app has a max body size of 75.

Verify that the `ClientSettingsPolicy` is Accepted:

```shell
kubectl describe clientsettingspolicies.gateway.nginx.org tea-client-settings
```

You should see the following status:

```text
Status:
  Ancestors:
    Ancestor Ref:
      Group:      gateway.networking.k8s.io
      Kind:       HTTPRoute
      Name:       tea
      Namespace:  default
    Conditions:
      Last Transition Time:  2024-05-30T19:57:18Z
      Message:               Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

Notice that the Ancestor Ref in the status is the tea HTTPRoute instead of the Gateway.

Next, test that the policy is configured by sending a POST request to the tea application with a request body size greater than 50 bytes.

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea -X POST --data "this payload is greater than fifty bytes but less than seventy five"
```

You should receive a response from tea:

```text
Server address: 10.244.0.7:8080
Server name: tea-596697966f-bf6tw
```

However, since the coffee app is still affected by the `ClientSettingsPolicy` attached to the Gateway, the same request to coffee should fail:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee -X POST --data "this payload is greater than fifty bytes but less than seventy five"
```

```text
<html>
<head><title>413 Request Entity Too Large</title></head>
<body>
<center><h1>413 Request Entity Too Large</h1></center>
<hr><center>nginx/1.25.5</center>
</body>
</html>
```

To configure a `ClientSettingsPolicy` for a GRPCRoute, you can specify the GRPCRoute in the `spec.targetRef`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: ClientSettingsPolicy
metadata:
  name: grpc-client-settings
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: GRPCRoute
    name: my-grpc-route
  body:
    maxSize: "75" # sizes without a unit are bytes.
EOF
```

## Further reading

- [Custom policies]({{< relref "overview/custom-policies.md" >}}): learn about how NGINX Gateway Fabric custom policies work.
- [API reference]({{< relref "reference/api.md" >}}): all configuration fields for the `ClientSettingsPolicy` API.
