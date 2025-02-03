---
title: Get started
toc: true
weight: 200
docs: DOCS-000
---

This is a guide for getting started with NGINX Gateway Fabric. It explains how to:

- Set up a [kind (Kubernetes in Docker)](https://kind.sigs.k8s.io/) cluster
- Install [NGINX Gateway Fabric](https://github.com/nginx/nginx-gateway-fabric) with [NGINX](https://nginx.org/)
- Test NGINX Gateway Fabric with an example application

By following the steps in order, you will finish with a functional NGINX Gateway Fabric cluster.

---

## Before you begin

To complete this guide, you need the following prerequisites installed:

- [Go 1.16](https://go.dev/dl/) or newer, which is used by kind
- [Docker](https://docs.docker.com/get-started/get-docker/), for creating and managing containers
- [kind](https://kind.sigs.k8s.io/#installation-and-usage), which allows for running a local Kubernetes cluster using Docker
- [kubectl](https://kubernetes.io/docs/tasks/tools/), which provides a command line interface (CLI) for interacting with Kubernetes clusters
- [Helm 3.0](https://helm.sh/docs/intro/install/) or newer to install NGINX Gateway Fabric
- [curl](https://curl.se/), to test the example application

## Set up a kind cluster

Create the file _cluster-config.yaml_ with the following contents, noting the highlighted lines:

```yaml {linenos=true, hl_lines=[6, 9]}
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 31437
    hostPort: 8080
    protocol: TCP
  - containerPort: 31438
    hostPort: 8443
    protocol: TCP
```

{{< warning >}}
Take note of the two _containerPort_ values. They are necessary for later configuring a _NodePort_.
{{< /warning >}}

Run the following command:

```shell
kind create cluster --config cluster-config.yaml
```

```text
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.31.0) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Thanks for using kind! 😊
```

{{< note >}}
If you have cloned [the NGINX Gateway Fabric repository](https://github.com/nginx/nginx-gateway-fabric/tree/main), you can also create a kind cluster from the root folder with the following *make* command:

```shell
make create-kind-cluster
```

{{< /note >}}

---

## Install NGINX Gateway Fabric

### Add Gateway API resources

Use `kubectl` to add the API resources for NGINX Gateway Fabric with the following command:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=v1.6.0" | kubectl apply -f -
```

```text
customresourcedefinition.apiextensions.k8s.io/gatewayclasses.gateway.networking.k8s.io created
customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io created
customresourcedefinition.apiextensions.k8s.io/grpcroutes.gateway.networking.k8s.io created
customresourcedefinition.apiextensions.k8s.io/httproutes.gateway.networking.k8s.io created
customresourcedefinition.apiextensions.k8s.io/referencegrants.gateway.networking.k8s.io created
```

{{< note >}}
To use experimental features, you'll need to install the API resources from the experimental channel instead.

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/experimental?ref=v1.6.0" | kubectl apply -f -
```

{{< /note >}}

---

### Install the Helm chart

Use `helm` to install NGINX Gateway Fabric with the following command:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway --set service.create=false
```

```text
Pulled: ghcr.io/nginx/charts/nginx-gateway-fabric:1.6.0
Digest: sha256:9bbd1a2fcbfd5407ad6be39f796f582e6263512f1f3a8969b427d39063cc6fee
NAME: ngf
LAST DEPLOYED: Mon Oct 21 14:45:14 2024
NAMESPACE: nginx-gateway
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

{{< note >}}
If you installed the API resources from the experimental channel during the last step, you will need to enable the _nginxGateway.gwAPIExperimentalFeatures_ option:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --create-namespace -n nginx-gateway --set service.create=false --set nginxGateway.gwAPIExperimentalFeatures.enable=true
```

{{< /note >}}

---

### Set up a NodePort

Create the file _nodeport-config.yaml_ with the following contents:

{{< note >}}
The highlighted _nodePort_ values should equal the _containerPort_ values from _cluster-config.yaml_ [when you created the kind cluster](#set-up-a-kind-cluster).
{{< /note >}}

```yaml {linenos=true, hl_lines=[20, 25]}
apiVersion: v1
kind: Service
metadata:
  name: nginx-gateway
  namespace: nginx-gateway
  labels:
    app.kubernetes.io/name: nginx-gateway-fabric
    app.kubernetes.io/instance: ngf
    app.kubernetes.io/version: "1.6.0"
spec:
  type: NodePort
  selector:
    app.kubernetes.io/name: nginx-gateway-fabric
    app.kubernetes.io/instance: ngf
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
    nodePort: 31437
  - name: https
    port: 443
    protocol: TCP
    targetPort: 443
    nodePort: 31438
```

Apply it using `kubectl`:

```shell
kubectl apply -f nodeport-config.yaml
```

```text
service/nginx-gateway created
```

{{< warning >}}
The NodePort resource must be deployed in the same namespace as NGINX Gateway Fabric.

If you are making customizations, ensure your `labels:` and `selectors:` also match the labels of the NGINX Gateway Fabric Deployment.
{{< /warning >}}

---

## Create an example application

In the previous section, you deployed NGINX Gateway Fabric to a local cluster. This section shows you how to deploy a simple web application to test that NGINX Gateway Fabric works.

{{< note >}}
The YAML code in the following sections can be found in the [cafe-example folder](https://github.com/nginx/nginx-gateway-fabric/tree/main/examples/cafe-example) of the GitHub repository.
{{< /note >}}

---

### Create the application resources

Create the file _cafe.yaml_ with the following contents:

{{< ghcode `https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/refs/heads/main/examples/cafe-example/cafe.yaml`>}}

Apply it using `kubectl`:

```shell
kubectl apply -f cafe.yaml
```

```text
deployment.apps/coffee created
service/coffee created
deployment.apps/tea created
service/tea created
```

Verify that the new pods are in the `default` namespace:

```shell
kubectl -n default get pods
```

```text
NAME                      READY   STATUS    RESTARTS   AGE
coffee-6db967495b-wk2mm   1/1     Running   0          10s
tea-7b7d6c947d-d4qcf      1/1     Running   0          10s
```

---

### Create Gateway and HTTPRoute resources

Create the file _gateway.yaml_ with the following contents:

{{< ghcode `https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/refs/heads/main/examples/cafe-example/gateway.yaml`>}}

Apply it using `kubectl`:

```shell
kubectl apply -f gateway.yaml
```

```text
gateway.gateway.networking.k8s.io/gateway created
```

Create the file _cafe-routes.yaml_ with the following contents:

{{< ghcode `https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/refs/heads/main/examples/cafe-example/cafe-routes.yaml`>}}

Apply it using `kubectl`:

```shell
kubectl apply -f cafe-routes.yaml
```

```text
httproute.gateway.networking.k8s.io/coffee created
httproute.gateway.networking.k8s.io/tea created
```

---

### Verify the configuration

You can check that all of the expected services are available using `kubectl get`:

```shell
kubectl get service --all-namespaces
```

```text
NAMESPACE       NAME            TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
default         coffee          ClusterIP   10.96.18.163    <none>        80/TCP                       2m51s
default         kubernetes      ClusterIP   10.96.0.1       <none>        443/TCP                      4m41s
default         tea             ClusterIP   10.96.169.132   <none>        80/TCP                       2m51s
kube-system     kube-dns        ClusterIP   10.96.0.10      <none>        53/UDP,53/TCP,9153/TCP       4m40s
nginx-gateway   nginx-gateway   NodePort    10.96.186.45    <none>        80:31437/TCP,443:31438/TCP   3m6s
```

You can also use `kubectl describe` on the new resources to check their status:

```shell
kubectl describe httproutes
```

```text
Name:         coffee
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  gateway.networking.k8s.io/v1
Kind:         HTTPRoute
Metadata:
  Creation Timestamp:  2024-10-21T13:46:51Z
  Generation:          1
  Resource Version:    821
  UID:                 cc591089-d3aa-44d3-a851-e2bbfa285029
Spec:
  Hostnames:
    cafe.example.com
  Parent Refs:
    Group:         gateway.networking.k8s.io
    Kind:          Gateway
    Name:          gateway
    Section Name:  http
  Rules:
    Backend Refs:
      Group:
      Kind:    Service
      Name:    coffee
      Port:    80
      Weight:  1
    Matches:
      Path:
        Type:   PathPrefix
        Value:  /coffee
Status:
  Parents:
    Conditions:
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               The route is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               All references are resolved
      Observed Generation:   1
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
    Parent Ref:
      Group:         gateway.networking.k8s.io
      Kind:          Gateway
      Name:          gateway
      Namespace:     default
      Section Name:  http
Events:              <none>


Name:         tea
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  gateway.networking.k8s.io/v1
Kind:         HTTPRoute
Metadata:
  Creation Timestamp:  2024-10-21T13:46:51Z
  Generation:          1
  Resource Version:    823
  UID:                 d72d2a19-1c4d-48c4-9808-5678cff6c331
Spec:
  Hostnames:
    cafe.example.com
  Parent Refs:
    Group:         gateway.networking.k8s.io
    Kind:          Gateway
    Name:          gateway
    Section Name:  http
  Rules:
    Backend Refs:
      Group:
      Kind:    Service
      Name:    tea
      Port:    80
      Weight:  1
    Matches:
      Path:
        Type:   Exact
        Value:  /tea
Status:
  Parents:
    Conditions:
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               The route is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               All references are resolved
      Observed Generation:   1
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
    Parent Ref:
      Group:         gateway.networking.k8s.io
      Kind:          Gateway
      Name:          gateway
      Namespace:     default
      Section Name:  http
Events:              <none>
```

```shell
kubectl describe gateways
```

```text
Name:         gateway
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  gateway.networking.k8s.io/v1
Kind:         Gateway
Metadata:
  Creation Timestamp:  2024-10-21T13:46:36Z
  Generation:          1
  Resource Version:    824
  UID:                 2ae8ec42-70eb-41a4-b249-3e47177aea48
Spec:
  Gateway Class Name:  nginx
  Listeners:
    Allowed Routes:
      Namespaces:
        From:  Same
    Hostname:  *.example.com
    Name:      http
    Port:      80
    Protocol:  HTTP
Status:
  Addresses:
    Type:   IPAddress
    Value:  10.244.0.5
  Conditions:
    Last Transition Time:  2024-10-21T13:46:51Z
    Message:               Gateway is accepted
    Observed Generation:   1
    Reason:                Accepted
    Status:                True
    Type:                  Accepted
    Last Transition Time:  2024-10-21T13:46:51Z
    Message:               Gateway is programmed
    Observed Generation:   1
    Reason:                Programmed
    Status:                True
    Type:                  Programmed
  Listeners:
    Attached Routes:  2
    Conditions:
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               Listener is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               Listener is programmed
      Observed Generation:   1
      Reason:                Programmed
      Status:                True
      Type:                  Programmed
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               All references are resolved
      Observed Generation:   1
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
      Last Transition Time:  2024-10-21T13:46:51Z
      Message:               No conflicts
      Observed Generation:   1
      Reason:                NoConflicts
      Status:                False
      Type:                  Conflicted
    Name:                    http
    Supported Kinds:
      Group:  gateway.networking.k8s.io
      Kind:   HTTPRoute
      Group:  gateway.networking.k8s.io
      Kind:   GRPCRoute
Events:       <none>
```

---

## Test NGINX Gateway Fabric

By configuring the cluster with the ports `31437` and `31438`, there is implicit port forwarding from your local machine to NodePort, allowing for direct communication to the NGINX Gateway Fabric service.

You can use `curl` to test the new services by targeting the hostname (_cafe.example.com_) with the _/coffee_ and _/tea_ paths:

```shell
curl --resolve cafe.example.com:8080:127.0.0.1 http://cafe.example.com:8080/coffee
```

```text
Server address: 10.244.0.6:8080
Server name: coffee-6db967495b-wk2mm
Date: 21/Oct/2024:13:52:13 +0000
URI: /coffee
Request ID: fb226a54fd94f927b484dd31fb30e747
```

```shell
curl --resolve cafe.example.com:8080:127.0.0.1 http://cafe.example.com:8080/tea
```

```text
Server address: 10.244.0.7:8080
Server name: tea-7b7d6c947d-d4qcf
Date: 21/Oct/2024:13:52:17 +0000
URI: /tea
Request ID: 43882f2f5794a1ee05d2ea017a035ce3
```

---

## See also

- [Install NGINX Gateway Fabric]({{< ref "/installation/installing-ngf/" >}}), for additional ways to install NGINX Gateway Fabric
- [How-to guides]({{< ref "/how-to/" >}}), for configuring your cluster
- [Traffic management]({{< ref "/how-to/traffic-management/" >}}), for more in-depth traffic management configuration
