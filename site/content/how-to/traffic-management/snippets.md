---
title: "Use the SnippetsFilter API"
weight: 800
toc: true
docs: "DOCS-000"
---

This topic introduces Snippets, how to implement them using the `SnippetsFilter` API, and provides an example of how to use `SnippetsFilter` for rate limiting.

---

## Overview

Snippets allow users to insert NGINX configuration into different contexts of the
NGINX configurations that NGINX Gateway Fabric generates.

Snippets should only be used by advanced NGINX users who need more control over the generated NGINX configuration,
and only in cases where Gateway API resources or NGINX extension policies don't apply.

Users can configure Snippets through the `SnippetsFilter` API. `SnippetsFilter` can be an [HTTPRouteFilter](https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.HTTPRouteFilter) or [GRPCRouteFilter](https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.GRPCRouteFilter),
that can be defined in an HTTPRoute/GRPCRoute rule and is intended to modify NGINX configuration specifically for that Route rule. `SnippetsFilter` is an `extensionRef` type filter.

---

## Disadvantages of Snippets

{{< warning >}} We recommend managing NGINX configuration through Gateway API resources, [first-class policies]({{< relref "overview/custom-policies.md" >}}), and other existing [NGINX extensions]({{< relref "data-plane-configuration.md" >}})
before using Snippets. {{< /warning >}}

Snippets are configured using the `SnippetsFilter` API, but are disabled by default due to their complexity and security implications.

Snippets have the following disadvantages:

- _Complexity_. Snippets require you to:
  - Understand NGINX configuration primitives to implement correct NGINX configuration.
  - Understand how NGINX Gateway Fabric generates NGINX configuration so that a Snippet doesn’t interfere with the other features in the configuration.
- _Decreased robustness_. An incorrect Snippet can invalidate NGINX configuration, causing reload failures. Until the snippet is fixed, it will prevent any new configuration updates, including updates for the other Gateway resources.
- _Security implications_. Snippets give access to NGINX configuration primitives, which are not validated by NGINX Gateway Fabric. For example, a Snippet can configure NGINX to serve the TLS certificates and keys used for TLS termination for Gateway resources.

{{< note >}} If the NGINX configuration includes an invalid Snippet, NGINX will continue to operate with the last valid configuration. No new configuration will be applied until the invalid Snippet is fixed. {{< /note >}}

---

## Best Practices when using SnippetsFilters

There are endless ways to use `SnippetsFilters` to modify NGINX configuration, and thus there are many ways to generate invalid or undesired NGINX configuration.
We have outlined a few best practices to keep in mind when using `SnippetsFilters` to keep NGINX Gateway Fabric functioning correctly:

1. Using the [Roles and Personas](https://gateway-api.sigs.k8s.io/concepts/roles-and-personas/) defined in the Gateway API, `SnippetsFilter` access
   should be limited to Cluster operators. Application developers should not be able to create, modify, or delete `SnippetsFilters` as they affect other applications.
   `SnippetsFilter` creates a natural split of responsibilities between the Cluster operator and the Application developer: the Cluster operator creates a `SnippetsFilter`; the Application developer references the `SnippetsFilter` in an HTTPRoute/GRPCRoute to enable it.
2. In a `SnippetsFilter`, only one Snippet per NGINX context is allowed, however multiple `SnippetsFilters` can be referenced in the same routing rule. As such, `SnippetsFilters` should not conflict with each other. If `SnippetsFilters` do conflict, they should not be referenced on the same routing rule.
3. `SnippetsFilters` that define Snippets targeting NGINX contexts `main`, `http`, or `http.server`, can potentially affect more than the routing rule they are referenced by. Proceed with caution and verify the behavior of the NGINX configuration before creating those `SnippetsFilters` in a production scenario.

---

## Setup

- To enable Snippets, [install]({{< relref "/installation/" >}}) NGINX Gateway Fabric with these modifications:
  - Using Helm: set the `nginxGateway.snippetsFilters.enable=true` Helm value.
  - Using Kubernetes manifests: set the `--snippets-filters` flag in the nginx-gateway container argument, add `snippetsfilters` to the RBAC
    rules with verbs `list` and `watch`, and add `snippetsfilters/status` to the RBAC rules with verb `update`. See this [example manifest](https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/main/deploy/snippets-filters/deploy.yaml) for clarification.

- Save the public IP address and port of NGINX Gateway Fabric into shell variables:

   ```text
  GW_IP=<ip address>
  GW_PORT=<port number>
  ```

  {{< note >}} In a production environment, you should have a DNS record for the external IP address that is exposed, and it should refer to the hostname that the gateway will forward for. {{< /note >}}

- Create the coffee and tea example applications:

  ```yaml
  kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/examples/snippets-filter/app.yaml
  ```

- Create a Gateway:

  ```yaml
  kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/examples/snippets-filter/gateway.yaml
   ```

- Create HTTPRoutes for the coffee and tea applications:

  ```yaml
  kubectl apply -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/v1.6.0/examples/snippets-filter/httproutes.yaml
   ```

- Test the configuration:

  You can send traffic to the coffee and tea applications using the external IP address and port for NGINX Gateway Fabric.

  Send a request to coffee:

  ```shell
  curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
  ```

  This request should receive a response from the coffee Pod:

  ```text
  Server address: 10.244.0.7:8080
  Server name: coffee-76c7c85bbd-cf8nz
  ```

  Send a request to tea:

  ```shell
  curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
   ```

  This request should receive a response from the tea Pod:

  ```text
  Server address: 10.244.0.6:8080
  Server name: tea-76c7c85bbd-cf8nz
  ```

  Before we enable rate limiting, try sending multiple requests to coffee:

  ```shell
  for i in `seq 1 10`; do curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee; done
  ```

  You should see all successful responses in quick succession as we have not configured any rate limiting rules yet.

---

## Create Rate Limiting SnippetsFilters

Configure a rate limiting `SnippetsFilter` named `rate-limiting-sf` by adding the following `SnippetsFilter`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsFilter
metadata:
  name: rate-limiting-sf
spec:
  snippets:
    - context: http
      value: limit_req_zone \$binary_remote_addr zone=rate-limiting-sf:10m rate=1r/s;
    - context: http.server.location
      value: limit_req zone=rate-limiting-sf burst=3;
EOF
```

This `SnippetsFilter` defines two Snippets to configure rate limiting. The first Snippet injects the value: `limit_req_zone \$binary_remote_addr zone=rate-limiting-sf:10m rate=1r/s;`
into the `http` context. The second Snippet injects the value: `limit_req zone=rate-limiting-sf burst=3;` into the location(s) generated for whichever route(s) reference this `SnippetsFilter`.
This `SnippetsFilter` will limit the request processing rate to 1 request per second.

Verify that the `SnippetsFilter` is Accepted:

```shell
kubectl describe snippetsfilters.gateway.nginx.org rate-limiting-sf
```

You should see the following status:

```text
Status:
  Controllers:
    Conditions:
      Last Transition Time:  2024-10-21T22:20:22Z
      Message:               SnippetsFilter is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

Configure another rate limiting `SnippetsFilter` named `no-delay-rate-limiting-sf` by adding the following `SnippetsFilter`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: SnippetsFilter
metadata:
  name: no-delay-rate-limiting-sf
spec:
  snippets:
    - context: http
      value: limit_req_zone \$binary_remote_addr zone=no-delay-rate-limiting-sf:10m rate=1r/s;
    - context: http.server.location
      value: limit_req zone=no-delay-rate-limiting-sf burst=3 nodelay;
EOF
```

This `SnippetsFilter` is the same as the `rate-limiting-sf` `SnippetsFilter`, however it adds the `nodelay` setting to the
`limit_req` directive in the Snippet targeting the `http.server.location` context. This will limit the request processing rate
to 1 request per second, and if there are more than 3 requests in queue, it will throw a 503 error.

Verify that the `SnippetsFilter` is Accepted:

```shell
kubectl describe snippetsfilters.gateway.nginx.org no-delay-rate-limiting-sf
```

You should see the following status:

```text
Status:
  Controllers:
    Conditions:
      Last Transition Time:  2024-10-21T22:20:22Z
      Message:               SnippetsFilter is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```

## Configure coffee to reference rate-limiting-sf SnippetsFilter

To use the `rate-limiting-sf` `SnippetsFilter`, update the coffee HTTPRoute to reference it:

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
            type: PathPrefix
            value: /coffee
      filters:
        - type: ExtensionRef
          extensionRef:
            group: gateway.nginx.org
            kind: SnippetsFilter
            name: rate-limiting-sf
      backendRefs:
        - name: coffee
          port: 80
EOF
```

Verify that the coffee HTTPRoute has been configured correctly:

```shell
kubectl describe httproutes.gateway.networking.k8s.io coffee
```

You should see the following conditions:

```text
Conditions:
      Last Transition Time:  2024-10-28T00:33:08Z
      Message:               The route is accepted
      Observed Generation:   2
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2024-10-28T00:33:08Z
      Message:               All references are resolved
      Observed Generation:   2
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
```

Test that the `rate-limiting-sf` `SnippetsFilter` is configured and has successfully applied the rate limiting NGINX configuration changes.

Send a request to coffee:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
```

This request should receive a response from the coffee Pod:

```text
Server address: 10.244.0.7:8080
Server name: coffee-76c7c85bbd-cf8nz
```

When processing a single request, the rate limiting configuration has no noticeable effect. Try to exceed the
set rate limit with a script that sends multiple requests.

```shell
for i in `seq 1 10`; do curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee; done
```

You should see all successful responses from the coffee Pod, but they should be spaced apart roughly one second each as
expected through the rate limiting configuration.

## Configure tea to reference no-delay-rate-limiting-sf SnippetsFilter

Update the tea HTTPRoute to reference the `no-delay-rate-limting-sf` `SnippetsFilter`:

```yaml
kubectl apply -f - <<EOF
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
            type: PathPrefix
            value: /tea
      filters:
        - type: ExtensionRef
          extensionRef:
            group: gateway.nginx.org
            kind: SnippetsFilter
            name: no-delay-rate-limiting-sf
      backendRefs:
        - name: tea
          port: 80
EOF
```

Verify that the tea HTTPRoute has been configured correctly:

```shell
kubectl describe httproutes.gateway.networking.k8s.io tea
```

You should see the following conditions:

```text
Conditions:
      Last Transition Time:  2024-10-28T00:33:08Z
      Message:               The route is accepted
      Observed Generation:   2
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2024-10-28T00:33:08Z
      Message:               All references are resolved
      Observed Generation:   2
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
```

Test that the `SnippetsFilter` is configured and has successfully applied the rate limiting NGINX configuration changes.

Send a request to tea:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
```

This request should receive a response from the tea Pod:

```text
Server address: 10.244.0.6:8080
Server name: tea-76c7c85bbd-cf8nz
```

When processing a single request, the rate limiting configuration has no noticeable effect. Try sending
multiple requests.

```shell
for i in `seq 1 10`; do curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea; done
```

You should see some successful responses from the tea Pod, however there should be multiple `503` responses such as:

```text
Request ID: 890c17df930ef1ef573feed3c6e81290
<html>
<head><title>503 Service Temporarily Unavailable</title></head>
<body>
<center><h1>503 Service Temporarily Unavailable</h1></center>
<hr><center>nginx</center>
</body>
</html>
```

This is the default error response given by NGINX when the rate limit burst is exceeded, meaning our `SnippetsFilter`
correctly applied our rate limiting NGINX configuration changes.

---

## Conclusion

You've successfully used Snippets with the `SnippetsFilter` resource to configure two distinct rate limiting rules to different backend applications.

In this example guide, the Cluster Operator would have played the role in creating and applying the `SnippetsFilter` resources shown in [Create Rate Limiting SnippetsFilters](#create-rate-limiting-snippetsfilters)
while the Application Developers for coffee and tea would have played the role in modifying their application to reference whichever `SnippetsFilter` they want shown in
[Configure coffee to reference rate-limiting-sf SnippetsFilter](#configure-coffee-to-reference-rate-limiting-sf-snippetsfilter) and [Configure tea to reference no-delay-rate-limiting-sf SnippetsFilter](#configure-tea-to-reference-no-delay-rate-limiting-sf-snippetsfilter).
This follows our recommended Role and Persona separation described in the [Best Practices when using SnippetsFilters](#best-practices-when-using-snippetsfilters).

For an alternative method of modifying the NGINX configuration NGINX Gateway Fabric generates through Gateway API resources, check out
our supported [first-class policies]({{< relref "overview/custom-policies.md" >}}) which don't carry many of the aforementioned disadvantages of Snippets.

---

## Troubleshooting

If a `SnippetsFilter` is defined in a Route and contains a Snippet which includes an invalid NGINX configuration, NGINX will continue to operate
with the last valid configuration and an event with the error will be outputted. No new configuration will be applied until the invalid Snippet is fixed.

An example of an error from the NGINX Gateway Fabric `nginx-gateway` container logs:

```text
{"level":"error","ts":"2024-10-29T22:19:41Z","logger":"eventLoop.eventHandler","msg":"Failed to update NGINX configuration","batchID":156,"error":"failed to reload NGINX: reload unsuccessful: no new NGINX worker processes started for config version 141. Please check the NGINX container logs for possible configuration issues: context deadline exceeded","stacktrace":"github.com/nginx/nginx-gateway-fabric/internal/mode/static.(*eventHandlerImpl).HandleEventBatch\n\tgithub.com/nginx/nginx-gateway-fabric/internal/mode/static/handler.go:219\ngithub.com/nginx/nginx-gateway-fabric/internal/framework/events.(*EventLoop).Start.func1.1\n\tgithub.com/nginx/nginx-gateway-fabric/internal/framework/events/loop.go:74"}
```

An example of an error from the NGINX Gateway Fabric `nginx` container logs:

```text
2024/10/29 22:18:41 [emerg] 40#40: invalid number of arguments in "limit_req_zone" directive in /etc/nginx/includes/SnippetsFilter_http_default_rate-limiting-sf.conf:1
```

The Route which references the `SnippetsFilter` may also contain information in its conditions describing the error:

```text
 Conditions:
      Last Transition Time:  2024-10-29T22:19:41Z
      Message:               All references are resolved
      Observed Generation:   2
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
      Last Transition Time:  2024-10-29T22:19:41Z
      Message:               The Gateway is not programmed due to a failure to reload nginx with the configuration. Please see the nginx container logs for any possible configuration issues. NGINX may still be configured for this Route. However, future updates to this resource will not be configured until the Gateway is programmed again
      Observed Generation:   2
      Reason:                GatewayNotProgrammed
      Status:                False
      Type:                  Accepted
```

If a Route references a `SnippetsFilter` which cannot be resolved, the route will return a 500 HTTP error response on all requests.
The Route conditions will contain information describing the error:

```text
Conditions:
      Last Transition Time:  2024-10-29T22:26:01Z
      Message:               The route is accepted
      Observed Generation:   2
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2024-10-29T22:26:01Z
      Message:               spec.rules[0].filters[0].extensionRef: Not found: v1.LocalObjectReference{Group:"gateway.nginx.org", Kind:"SnippetsFilter", Name:"rate-limiting-sf"}
      Observed Generation:   2
      Reason:                InvalidFilter
      Status:                False
      Type:                  ResolvedRefs
```

{{< note >}} If you run into situations where an NGINX directive fails to be applied and the troubleshooting information here isn't sufficient, please create an issue in the
[NGINX Gateway Fabric Github repository](https://github.com/nginx/nginx-gateway-fabric). {{< /note >}}

---

## See also

- [API reference]({{< relref "reference/api.md" >}}): all configuration fields for the `SnippetsFilter` API.
