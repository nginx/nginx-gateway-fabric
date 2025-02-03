---
title: "Upstream Settings Policy API"
weight: 900
toc: true
docs: "DOCS-000"
---

Learn how to use the `UpstreamSettingsPolicy` API.

## Overview

The `UpstreamSettingsPolicy` API allows Application Developers to configure the behavior of a connection between NGINX and the upstream applications.

The settings in `UpstreamSettingsPolicy` correspond to the following NGINX directives:

- [`zone`](<https://nginx.org/en/docs/http/ngx_http_upstream_module.html#zone>)
- [`keepalive`](<https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive>)
- [`keepalive_requests`](<https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_requests>)
- [`keepalive_time`](<https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_time>)
- [`keepalive_timeout`](<https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_timeout>)

`UpstreamSettingsPolicy` is a [Direct Policy Attachment](https://gateway-api.sigs.k8s.io/reference/policy-attachment/) that can be applied to one or more services in the same namespace as the policy.
`UpstreamSettingsPolicies` can only be applied to HTTP or gRPC services, in other words, services that are referenced by an HTTPRoute or GRPCRoute.

See the [custom policies]({{< relref "overview/custom-policies.md" >}}) document for more information on policies.

This guide will show you how to use the `UpstreamSettingsPolicy` API to configure the upstream zone size and keepalives for your applications.

For all the possible configuration options for `UpstreamSettingsPolicy`, see the [API reference]({{< relref "reference/api.md" >}}).

---

## Before you begin

- [Install]({{< relref "/installation/" >}}) NGINX Gateway Fabric.
- Save the public IP address and port of NGINX Gateway Fabric into shell variables:

   ```text
  GW_IP=XXX.YYY.ZZZ.III
  GW_PORT=<port number>
  ```

- Lookup the name of the NGINX Gateway Fabric pod and save into shell variable:

  ```text
  NGF_POD_NAME=<NGF Pod>
  ```

  {{< note >}}In a production environment, you should have a DNS record for the external IP address that is exposed, and it should refer to the hostname that the gateway will forward for.{{< /note >}}

---

## Setup

Create the `coffee` and `tea` example applications:

```yaml
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coffee
spec:
  replicas: 1
  selector:
    matchLabels:
      app: coffee
  template:
    metadata:
      labels:
        app: coffee
    spec:
      containers:
      - name: coffee
        image: nginxdemos/nginx-hello:plain-text
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: coffee
spec:
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: coffee
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tea
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tea
  template:
    metadata:
      labels:
        app: tea
    spec:
      containers:
      - name: tea
        image: nginxdemos/nginx-hello:plain-text
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: tea
spec:
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: tea
EOF
```

This will create two services and pods in the default namespace:

```shell
kubectl get svc,pod -n default
```

```text
NAME                 TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
service/coffee       ClusterIP   10.244.0.14     <none>        80/TCP    23h
service/tea          ClusterIP   10.244.0.15     <none>        80/TCP    23h

NAME                          READY   STATUS    RESTARTS   AGE
pod/coffee-676c9f8944-n9g6n   1/1     Running   0          23h
pod/tea-6fbfdcb95d-cf84d      1/1     Running   0          23h
```

Create a Gateway:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
spec:
  gatewayClassName: nginx
  listeners:
    - name: http
      port: 80
      protocol: HTTP
      hostname: "*.example.com"
EOF
```

Create HTTPRoutes for the `coffee` and `tea` applications:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: coffee
spec:
  parentRefs:
    - name: gateway
      sectionName: http
  hostnames:
    - "cafe.example.com"
  rules:
    - matches:
        - path:
            type: Exact
            value: /coffee
      backendRefs:
        - name: coffee
          port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: tea
spec:
  parentRefs:
    - name: gateway
      sectionName: http
  hostnames:
    - "cafe.example.com"
  rules:
    - matches:
        - path:
            type: Exact
            value: /tea
      backendRefs:
        - name: tea
          port: 80
EOF
```

Test the configuration:

You can send traffic to the `coffee` and `tea` applications using the external IP address and port for NGINX Gateway Fabric.

Send a request to `coffee`:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
```

This request should receive a response from the `coffee` Pod:

```text
Server address: 10.244.0.9:8080
Server name: coffee-76c7c85bbd-cf8nz
```

Send a request to `tea`:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
 ```

This request should receive a response from the `tea` Pod:

```text
Server address: 10.244.0.9:8080
Server name: tea-76c7c85bbd-cf8nz
```

---

## Configure upstream zone size

To set the upstream zone size to 1 megabyte for both the `coffee` and `tea` services, create the following `UpstreamSettingsPolicy`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: 1m-zone-size
spec:
  targetRefs:
  - group: core
    kind: Service
    name: tea
  - group: core
    kind: Service
    name: coffee
  zoneSize: 1m
EOF
```

This `UpstreamSettingsPolicy` targets both the `coffee` and `tea` services we created in the setup by specifying both services in the `targetRefs` field. It limits the upstream zone size of the `coffee` and `tea` services to 1 megabyte.

Verify that the `UpstreamSettingsPolicy` is Accepted:

```shell
kubectl describe upstreamsettingspolicies.gateway.nginx.org 1m-zone-size
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
      Last Transition Time:  2025-01-07T20:06:55Z
      Message:               Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

Next, verify that the policy has been applied to the `coffee` and `tea` upstreams by inspecting the NGINX configuration:

```shell
kubectl exec -it -n nginx-gateway $NGF_POD_NAME -c nginx -- nginx -T
```

You should see the `zone` directive in the `coffee` and `tea` upstreams both specify the size `1m`:

```text
upstream default_coffee_80 {
    random two least_conn;
    zone default_coffee_80 1m;

    server 10.244.0.14:8080;
}

upstream default_tea_80 {
    random two least_conn;
    zone default_tea_80 1m;

    server 10.244.0.15:8080;
}
```

## Enable keepalive connections

To enable keepalive connections for the `coffee` service, create the following `UpstreamSettingsPolicy`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: upstream-keepalives
spec:
  targetRefs:
  - group: core
    kind: Service
    name: coffee
  keepAlive:
    connections: 32
EOF
```

This `UpstreamSettingsPolicy` targets the `coffee` service in the `targetRefs` field. It sets the number of keepalive connections to 32, which activates the cache for connections to the service's pods and sets the maximum number of idle connections to 32.

Verify that the `UpstreamSettingsPolicy` is Accepted:

```shell
kubectl describe upstreamsettingspolicies.gateway.nginx.org upstream-keepalives
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
      Last Transition Time:  2025-01-07T20:06:55Z
      Message:               Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

Next, verify that the policy has been applied to the `coffee` upstreams, by inspecting the NGINX configuration:

```shell
kubectl exec -it -n nginx-gateway $NGF_POD_NAME -c nginx -- nginx -T
```

You should see that the `coffee` upstream has the `keepalive` directive set to 32:

```text
upstream default_coffee_80 {
    random two least_conn;
    zone default_coffee_80 1m;

    server 10.244.0.14:8080;
    keepalive 32;
}
```

Notice, that the `tea` upstream does not contain the `keepalive` directive, since the `upstream-keepalives` policy does not target the `tea` service:

```text
upstream default_tea_80 {
    random two least_conn;
    zone default_tea_80 1m;

    server 10.244.0.15:8080;
}
```

---

## Further reading

- [Custom policies]({{< relref "overview/custom-policies.md" >}}): learn about how NGINX Gateway Fabric custom policies work.
- [API reference]({{< relref "reference/api.md" >}}): all configuration fields for the `UpstreamSettingsPolicy` API.
