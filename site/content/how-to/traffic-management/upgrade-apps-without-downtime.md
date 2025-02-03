---
title: "Upgrade applications without downtime"
toc: true
weight: 300
type: how-to
product: NGF
docs: "DOCS-1420"
---

Learn how to use NGINX Gateway Fabric to upgrade applications without downtime.

---

## Overview

{{< note >}} See the [Architecture document]({{< relref "/overview/gateway-architecture.md" >}}) to learn more about NGINX Gateway Fabric architecture.{{< /note >}}

NGINX Gateway Fabric allows upgrading applications without downtime. To understand the upgrade methods, you need to be familiar with the NGINX features that help prevent application downtime: Graceful configuration reloads and upstream server updates.

---

### Graceful configuration reloads

If a relevant gateway API or built-in Kubernetes resource is changed, NGINX Gateway Fabric will update NGINX by regenerating the NGINX configuration. NGINX Gateway Fabric then sends a reload signal to the master NGINX process to apply the new configuration.

We call such an operation a "reload", during which client requests are not dropped - which defines it as a graceful reload.

This process is further explained in the [NGINX configuration documentation](https://nginx.org/en/docs/control.html?#reconfiguration).

---

### Upstream server updates

Endpoints frequently change during application upgrades: Kubernetes creates pods for the new version of an application and removes the old ones, creating and removing the respective endpoints as well.

NGINX Gateway Fabric detects changes to endpoints by watching their corresponding [EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/).

In an NGINX configuration, a service is represented as an [upstream](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#upstream), and an endpoint as an [upstream server](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#server).

Adding and removing endpoints are two of the most common cases:

- If an endpoint is added, NGINX Gateway Fabric adds an upstream server to NGINX that corresponds to the endpoint, then reloads NGINX. Next, NGINX will start proxying traffic to that endpoint.
- If an endpoint is removed, NGINX Gateway Fabric removes the corresponding upstream server from NGINX. After a reload, NGINX will stop proxying traffic to that server. However, it will finish proxying any pending requests to that server before switching to another endpoint.

As long as you have more than one endpoint ready, clients won't experience downtime during upgrades.

{{< note >}}It is good practice to configure a [Readiness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) in the deployment so that a pod can report when it is ready to receive traffic. Note that NGINX Gateway Fabric will not add any endpoint to NGINX that is not ready.{{< /note >}}

---

## Prerequisites

- You have deployed your application as a [deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
- The pods of the deployment belong to a [service](https://kubernetes.io/docs/concepts/services-networking/service/) so that Kubernetes creates an [endpoint](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/endpoints-v1/) for each pod.
- You have exposed the application to the clients via an [HTTPRoute](https://gateway-api.sigs.k8s.io/api-types/httproute/) resource that references that service.

For example, an application can be exposed using a routing rule like below:

```yaml
- matches:
  - path:
      type: PathPrefix
      value: /
  backendRefs:
  - name: my-app
    port: 80
```

{{< note >}}See the [Cafe example](https://github.com/nginx/nginx-gateway-fabric/tree/v1.6.0/examples/cafe-example) for a basic example.{{< /note >}}

The upgrade methods in the next sections cover:

- Rolling deployment upgrades
- Blue-green deployments
- Canary releases

---

## Rolling deployment upgrade

To start a [rolling deployment upgrade](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment), you update the deployment to use the new version tag of the application. As a result, Kubernetes terminates the pods with the old version and create new ones. By default, Kubernetes also ensures that some number of pods always stay available during the upgrade.

This upgrade will add new upstream servers to NGINX and remove the old ones. As long as the number of pods (ready endpoints) during an upgrade does not reach zero, NGINX will be able to proxy traffic, and therefore prevent any downtime.

This method does not require you to update the **HTTPRoute**.

---

## Blue-green deployments

With this method, you deploy a new version of the application (blue version) as a separate deployment, while the old version (green) keeps running and handling client traffic. Next, you switch the traffic from the green version to the blue. If the blue works as expected, you terminate the green. Otherwise, you switch the traffic back to the green.

There are two ways to switch the traffic:

- Update the service selector to select the pods of the blue version instead of the green. As a result, NGINX Gateway Fabric removes the green upstream servers from NGINX and adds the blue ones. With this approach, it is not necessary to update the **HTTPRoute**.
- Create a separate service for the blue version and update the backend reference in the **HTTPRoute** to reference this service, which leads to the same result as with the previous option.

---

## Canary releases

Canary releases involve gradually introducing a new version of your application to a subset of nodes in a controlled manner, splitting the traffic between the old are new (canary) release. This allows for monitoring and testing the new release's performance and reliability before full deployment, helping to identify and address issues without impacting the entire user base.

To support canary releases, you can implement an approach with two deployments behind the same service (see [Canary deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#canary-deployment) in the Kubernetes documentation). However, this approach lacks precision for defining the traffic split between the old and the canary version. You can greatly influence it by controlling the number of pods (for example, four pods of the old version and one pod of the canary). However, note that NGINX Gateway Fabric uses [`random two least_conn`](https://nginx.org/en/docs/http/ngx_http_upstream_module.html#random) load balancing method, which doesn't guarantee an exact split based on the number of pods (80/20 in the given example).

A more flexible and precise way to implement canary releases is to configure a traffic split in an **HTTPRoute**. In this case, you create a separate deployment for the new version with a separate service. For example, for the rule below, NGINX will proxy 95% of the traffic to the old version endpoints and only 5% to the new ones.

```yaml
- matches:
  - path:
      type: PathPrefix
      value: /
  backendRefs:
  - name: my-app-old
    port: 80
    weight: 95
  - name: my-app-new
    port: 80
    weight: 5
```

{{< note >}}Every request coming from the same client won't necessarily be sent to the same backend. NGINX will independently split each request among the backend references.{{< /note >}}

By updating the rule you can further increase the share of traffic the new version gets and finally completely switch to the new version:

```yaml
- matches:
  - path:
      type: PathPrefix
      value: /
  backendRefs:
  - name: my-app-old
    port: 80
    weight: 0
  - name: my-app-new
    port: 80
    weight: 1
```

See the [Traffic splitting example](https://github.com/nginx/nginx-gateway-fabric/tree/v1.6.0/examples/traffic-splitting) from our repository.
