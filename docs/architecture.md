# Architecture

This document provides an overview of the architecture and design principles of the NGINX Kubernetes Gateway. The target
audience includes the following groups:

- *Cluster Operators* who would like to know how the software works and also better understand how it can fail.
- *Developers* who would like to [contribute][contribute] to the project.

We assume that the reader is familiar with core Kubernetes concepts, such as Pods, Deployments, Services, and Endpoints.
Additionally, we recommend reading [this blog post][blog] for an overview of the NGINX architecture.

[contribute]: https://github.com/nginxinc/nginx-kubernetes-gateway/blob/main/CONTRIBUTING.md

[blog]: https://www.nginx.com/blog/inside-nginx-how-we-designed-for-performance-scale/

## What is NGINX Kubernetes Gateway?

The NGINX Kubernetes Gateway is a component in a Kubernetes cluster that configures an HTTP load balancer according to
Gateway API resources created by Cluster Operators and Application Developers.

> If you’d like to read more about the Gateway API, refer to [Gateway API documentation][sig-gateway].

This document focuses specifically on the NGINX Kubernetes Gateway, also known as NKG, which uses NGINX as its data
plane.

[sig-gateway]: https://gateway-api.sigs.k8s.io/

## NGINX Kubernetes Gateway at a High Level

To start, let's take a high-level look at the NGINX Kubernetes Gateway (NKG). The accompanying diagram illustrates an
example scenario where NKG exposes two web applications hosted within a Kubernetes cluster to external clients on the
internet:

![NKG High Level](/docs/images/nkg-high-level.png)

The figure shows:

- A *Kubernetes cluster*.
- Users *Cluster Operator*, *Application Developer A* and *Application Developer B*. These users interact with the
cluster through the Kubernetes API by creating Kubernetes objects.
- *Clients A* and *Clients B* connect to *Applications A* and *B*, respectively. This applications have been deployed by
the corresponding users.
- The *NKG Pod*, [deployed by *Cluster Operator*](/docs/installation.md) in the Namespace *nginx-gateway*. This Pod
consists of two containers: `NGINX` and `NKG`. The *NKG* container interacts with the Kubernetes API to retrieve the
most up-to-date Gateway API resources created within the cluster. It then dynamically configures the *NGINX*
container based on these resources, ensuring proper alignment between the cluster state and the NGINX configuration.
- *Gateway AB*, created by *Cluster Operator*, requests a point where traffic can be translated to Services within the
cluster. This Gateway includes a listener with a hostname `*.example.com`. Application Developers have the ability to
attach their application's routes to this Gateway if their application's hostname matches `*.example.com`.
- *Application A* with two Pods deployed in the *applications* Namespace by *Application Developer A*. To expose the
application to its clients (*Clients A*) via the host `a.example.com`, *Application Developer A* creates *HTTPRoute A*
and attaches it to `Gateway AB`.
- *Application B* with one Pod deployed in the *applications* Namespace by *Application Developer B*. To expose the
application to its clients (*Clients B*) via the host `b.example.com`, *Application Developer B* creates *HTTPRoute B*
and attaches it to `Gateway AB`.
- *Public Endpoint*, which fronts the *NKG* Pod. This is typically a TCP load balancer (cloud, software, or hardware)
or a combination of such load balancer with a NodePort Service. *Clients A* and *B* connect to their applications via
the *Public Endpoint*.

The connections related to client traffic are depicted by the yellow and purple arrows, while the black arrows represent
access to the Kubernetes API. The resources within the cluster are color-coded based on the user responsible for their
creation. For example, the Cluster Operator is denoted by the color green, indicating that they have created and manage
all the green resources.

> Note: For simplicity, many necessary Kubernetes resources like Deployment and Services aren't shown,
> which the Cluster Operator and the Application Developers also need to create.

Next, let's explore the NKG Pod.

## The NGINX Kubernetes Gateway Pod

The NGINX Kubernetes Gateway consists of three containers:

1. `nginx`: the data plane. Consists of an NGINX master process and NGINX worker processes. The master process controls
the worker processes. The worker processes handle the client traffic and load balance the traffic to the backend
applications.
2. `nginx-gateway`: the control plane. Watches Kubernetes objects and configures NGINX.

These containers are deployed in a single Pod as a Kubernetes Deployment.

The `nginx-gateway`, or the control plane, is a [Kubernetes controller][controller], written with
the [controller-runtime][runtime] library. It watches Kubernetes objects (Services, Endpoints, Secrets, and Gateway API
CRDs), translates them to NGINX configuration, and configures NGINX. This configuration happens in two stages. First,
NGINX configuration files are written to the NGINX configuration volume shared by the `nginx-gateway` and `nginx`
containers. Next, the control plane reloads the NGINX process. This is possible because the two
containers [share a process namespace][share], which allows the NKG process to send signals to the NGINX master process.

The diagram below provides a visual representation of the interactions between processes within the `nginx` and
`nginx-gateway` containers, as well as external processes/entities. It showcases the connections and relationships between
these components.

![NKG pod](/docs/images/nkg-pod.png)

The following list provides a description of each connection, along with its corresponding type indicated in
parentheses. To enhance readability, the suffix "process" has been omitted from the process descriptions below.

1. (HTTPS) *NKG* reads the *Kubernetes API* to get the latest versions of the resources in the cluster and writes to the
API to update the handled resources' statuses and emit events.
2. (File I/O)
   - Write: *NKG* generates NGINX *configuration* based on the cluster resources and writes them as `.conf` files to the
     mounted `nginx-conf` volume, located at `/etc/nginx/conf.d`. It also writes *TLS certificates* and *keys*
     from [TLS Secrets][secrets] referenced in the accepted Gateway resource to the `nginx-secrets` volume at the
     path `/etc/nginx/secrets`.
   - Read: *NKG* reads the PID file `nginx.pid` from the `nginx-run` volume, located at `/var/run/nginx`. *NKG*
     extracts the PID of the nginx process from this file in order to send reload signals to *NGINX master*.
3. (File I/O) *NKG* writes logs to its *stdout* and *stderr*, which are collected by the container runtime.
4. (Signal) To reload NGINX, *NKG* sends the [reload signal][reload] to the **NGINX master**.
5. (File I/O)
   - Write: The *NGINX master* writes its PID to the `nginx.pid` file stored in the `nginx-run` volume.
   - Read: The *NGINX master* reads *configuration files*  and the *TLS cert and keys* referenced in the configuration when
     it starts or during a reload. These files, certificates, and keys are stored in the `nginx-conf` and `nginx-secrets`
     volumes that are mounted to both the `nginx-gateway` and `nginx` containers.
6. (File I/O)
   - Write: The *NGINX master* writes to the auxiliary Unix sockets folder, which is located in the `/var/lib/nginx`
     directory.
   - Read: The *NGINX master* reads the `nginx.conf` file from the `/etc/nginx` directory. This [file][conf-file] contains
     the global and http configuration settings for NGINX. In addition, *NGINX master*
     reads the NJS modules referenced in the configuration when it starts or during a reload. NJS modules are stored in
     the `/usr/lib/nginx/modules` directory.
7. (File I/O) The *NGINX master* sends logs to its *stdout* and *stderr*, which are collected by the container runtime.
8. (File I/O) An *NGINX worker* writes logs to its *stdout* and *stderr*, which are collected by the container runtime.
9. (Signal) The *NGINX master* controls the [lifecycle of *NGINX workers*][lifecycle] it creates workers with the new
configuration and shutdowns workers with the old configuration.
10. (HTTP,HTTPS) A *client* sends traffic to and receives traffic from any of the *NGINX workers* on ports 80 and 443.
11. (HTTP,HTTPS) An *NGINX worker* sends traffic to and receives traffic from the *backends*.

[controller]: https://kubernetes.io/docs/concepts/architecture/controller/

[runtime]: https://github.com/kubernetes-sigs/controller-runtime

[secrets]: https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets

[reload]: https://nginx.org/en/docs/control.html

[lifecycle]: https://nginx.org/en/docs/control.html#reconfiguration

[conf-file]: https://github.com/nginxinc/nginx-kubernetes-gateway/blob/main/internal/mode/static/nginx/conf/nginx.conf

[share]: https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/
