---
title: "Gateway architecture"
weight: 100
toc: true
docs: "DOCS-1413"
---

Learn about the architecture and design principles of NGINX Gateway Fabric.

The intended audience for this information is primarily the two following groups:


- _Cluster Operators_ who would like to know how the software works and understand how it can fail.
- _Developers_ who would like to [contribute](https://github.com/nginx/nginx-gateway-fabric/blob/main/CONTRIBUTING.md) to the project.

The reader needs to be familiar with core Kubernetes concepts, such as pods, deployments, services, and endpoints. For an understanding of how NGINX itself works, you can read the ["Inside NGINX: How We Designed for Performance & Scale"](https://www.nginx.com/blog/inside-nginx-how-we-designed-for-performance-scale/) blog post.

## Overview

NGINX Gateway Fabric is an open source project that provides an implementation of the [Gateway API](https://gateway-api.sigs.k8s.io/) using [NGINX](https://nginx.org/) as the data plane. The goal of this project is to implement the core Gateway APIs -- _Gateway_, _GatewayClass_, _HTTPRoute_, _GRPCRoute_, _TCPRoute_, _TLSRoute_, and _UDPRoute_ -- to configure an HTTP or TCP/UDP load balancer, reverse proxy, or API gateway for applications running on Kubernetes. NGINX Gateway Fabric supports a subset of the Gateway API.

For a list of supported Gateway API resources and features, see the [Gateway API Compatibility]({{< relref "/overview/gateway-api-compatibility.md" >}}) documentation.

We have more information regarding our [design principles](https://github.com/nginx/nginx-gateway-fabric/blob/v1.6.0/docs/developer/design-principles.md) in the project's GitHub repository.

## NGINX Gateway Fabric at a high level

This figure depicts an example of NGINX Gateway Fabric exposing two web applications within a Kubernetes cluster to clients on the internet:

{{<img src="img/ngf-high-level.png" alt="">}}

{{< note >}} The figure does not show many of the necessary Kubernetes resources the Cluster Operators and Application Developers need to create, like deployment and services. {{< /note >}}

The figure shows:

- A _Kubernetes cluster_.
- Users _Cluster Operator_, _Application Developer A_ and _Application Developer B_. These users interact with the cluster through the Kubernetes API by creating Kubernetes objects.
- _Clients A_ and _Clients B_ connect to _Applications A_ and _B_, respectively, which they have deployed.
- The _NGF Pod_, [deployed by _Cluster Operator_]({{< relref "installation">}}) in the namespace _nginx-gateway_. For scalability and availability, you can have multiple replicas. This pod consists of two containers: `NGINX` and `NGF`. The _NGF_ container interacts with the Kubernetes API to retrieve the most up-to-date Gateway API resources created within the cluster. It then dynamically configures the _NGINX_ container based on these resources, ensuring proper alignment between the cluster state and the NGINX configuration.
- _Gateway AB_, created by _Cluster Operator_, requests a point where traffic can be translated to Services within the cluster. This Gateway includes a listener with a hostname `*.example.com`. Application Developers have the ability to attach their application's routes to this Gateway if their application's hostname matches `*.example.com`.
- _Application A_ with two pods deployed in the _applications_ namespace by _Application Developer A_. To expose the application to its clients (_Clients A_) via the host `a.example.com`, _Application Developer A_ creates _HTTPRoute A_ and attaches it to `Gateway AB`.
- _Application B_ with one pod deployed in the _applications_ namespace by _Application Developer B_. To expose the application to its clients (_Clients B_) via the host `b.example.com`, _Application Developer B_ creates _HTTPRoute B_ and attaches it to `Gateway AB`.
- _Public Endpoint_, which fronts the _NGF_ pod. This is typically a TCP load balancer (cloud, software, or hardware) or a combination of such load balancer with a NodePort service. _Clients A_ and _B_ connect to their applications via the _Public Endpoint_.

The yellow and purple arrows represent connections related to the client traffic, and the black arrows represent access to the Kubernetes API. The resources within the cluster are color-coded based on the user responsible for their creation.

For example, the Cluster Operator is denoted by the color green, indicating they create and manage all the green resources.

## The NGINX Gateway Fabric pod

NGINX Gateway Fabric consists of two containers:

1. `nginx`: the data plane. Consists of an NGINX master process and NGINX worker processes. The master process controls the worker processes. The worker processes handle the client traffic and load balance traffic to the backend applications.
1. `nginx-gateway`: the control plane. Watches Kubernetes objects and configures NGINX.

These containers are deployed in a single pod as a Kubernetes Deployment.

The `nginx-gateway`, or the control plane, is a [Kubernetes controller](https://kubernetes.io/docs/concepts/architecture/controller/), written with the [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) library. It watches Kubernetes objects (services, endpoints, secrets, and Gateway API CRDs), translates them to NGINX configuration, and configures NGINX.

This configuration happens in two stages:

1. NGINX configuration files are written to the NGINX configuration volume shared by the `nginx-gateway` and `nginx` containers.
1. The control plane reloads the NGINX process.

This is possible because the two containers [share a process namespace](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/), allowing the NGINX Gateway Fabric process to send signals to the NGINX main process.

The following diagram represents the connections, relationships and interactions between process with the `nginx` and `nginx-gateway` containers, as well as external processes/entities.

{{<img src="img/ngf-pod.png" alt="">}}

The following list describes the connections, preceeded by their types in parentheses. For brevity, the suffix "process" has been omitted from the process descriptions.

1. (HTTPS)
   - Read: _NGF_ reads the _Kubernetes API_ to get the latest versions of the resources in the cluster.
   - Write: _NGF_ writes to the _Kubernetes API_ to update the handled resources' statuses and emit events. If there's more than one replica of _NGF_ and [leader election](https://github.com/nginx/nginx-gateway-fabric/tree/v1.6.0/charts/nginx-gateway-fabric#configuration) is enabled, only the _NGF_ pod that is leading will write statuses to the _Kubernetes API_.
1. (HTTP, HTTPS) _Prometheus_ fetches the `controller-runtime` and NGINX metrics via an HTTP endpoint that _NGF_ exposes (`:9113/metrics` by default). Prometheus is **not** required by NGINX Gateway Fabric, and its endpoint can be turned off.
1. (File I/O)
   - Write: _NGF_ generates NGINX _configuration_ based on the cluster resources and writes them as `.conf` files to the mounted `nginx-conf` volume, located at `/etc/nginx/conf.d`. It also writes _TLS certificates_ and _keys_ from [TLS secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets) referenced in the accepted Gateway resource to the `nginx-secrets` volume at the path `/etc/nginx/secrets`.
   - Read: _NGF_ reads the PID file `nginx.pid` from the `nginx-run` volume, located at `/var/run/nginx`. _NGF_ extracts the PID of the nginx process from this file in order to send reload signals to _NGINX master_.
1. (File I/O) _NGF_ writes logs to its _stdout_ and _stderr_, which are collected by the container runtime.
1. (HTTP) _NGF_ fetches the NGINX metrics via the unix:/var/run/nginx/nginx-status.sock UNIX socket and converts it to _Prometheus_ format used in #2.
1. (Signal) To reload NGINX, _NGF_ sends the [reload signal](https://nginx.org/en/docs/control.html) to the **NGINX master**.
1. (File I/O)
   - Write: The _NGINX master_ writes its PID to the `nginx.pid` file stored in the `nginx-run` volume.
   - Read: The _NGINX master_ reads _configuration files_  and the _TLS cert and keys_ referenced in the configuration when it starts or during a reload. These files, certificates, and keys are stored in the `nginx-conf` and `nginx-secrets` volumes that are mounted to both the `nginx-gateway` and `nginx` containers.
1. (File I/O)
   - Write: The _NGINX master_ writes to the auxiliary Unix sockets folder, which is located in the `/var/run/nginx`
     directory.
   - Read: The _NGINX master_ reads the `nginx.conf` file from the `/etc/nginx` directory. This [file](https://github.com/nginx/nginx-gateway-fabric/blob/v1.6.0/internal/mode/static/nginx/conf/nginx.conf) contains the global and http configuration settings for NGINX. In addition, _NGINX master_ reads the NJS modules referenced in the configuration when it starts or during a reload. NJS modules are stored in the `/usr/lib/nginx/modules` directory.
1. (File I/O) The _NGINX master_ sends logs to its _stdout_ and _stderr_, which are collected by the container runtime.
1. (File I/O) An _NGINX worker_ writes logs to its _stdout_ and _stderr_, which are collected by the container runtime.
1. (Signal) The _NGINX master_ controls the [lifecycle of _NGINX workers_](https://nginx.org/en/docs/control.html#reconfiguration) it creates workers with the new configuration and shutdowns workers with the old configuration.
1. (HTTP) To consider a configuration reload a success, _NGF_ ensures that at least one NGINX worker has the new configuration. To do that, _NGF_ checks a particular endpoint via the unix:/var/run/nginx/nginx-config-version.sock UNIX socket.
1. (HTTP, HTTPS) A _client_ sends traffic to and receives traffic from any of the _NGINX workers_ on ports 80 and 443.
1. (HTTP, HTTPS) An _NGINX worker_ sends traffic to and receives traffic from the _backends_.

Below are additional connections not depcited on the diagram:

- (HTTPS) NGF sends [product telemetry data]({{< relref "/overview/product-telemetry.md" >}}) to the F5 telemetry service.

### Differences with NGINX Plus

The previous diagram depicts NGINX Gateway Fabric using NGINX Open Source. NGINX Gateway Fabric with NGINX Plus has the following difference:

- An _admin_ can connect to the NGINX Plus API using port 8765. NGINX only allows connections from localhost.

## Updating upstream servers

The normal process to update any changes to NGINX is to write the configuration files and reload NGINX. However, when using NGINX Plus, we can take advantage of the [NGINX Plus API](http://nginx.org/en/docs/http/ngx_http_api_module.html) to limit the amount of reloads triggered when making changes to NGINX. Specifically, when the endpoints of an application in Kubernetes change (Such as scaling up or down), the NGINX Plus API is used to update the upstream servers in NGINX with the new endpoints without a reload. This reduces the potential for a disruption that could occur when reloading.

## Pod readiness

The `nginx-gateway` container includes a readiness endpoint available through the path `/readyz`. A [readiness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-readiness-probes) periodically checks the endpoint on startup, returning a `200 OK` response when the pod can accept traffic for the data plane. Once the control plane successfully starts, the pod becomes ready.

If there are relevant Gateway API resources in the cluster, the control plane will generate the first NGINX configuration and successfully reload NGINX before the pod is considered ready.
