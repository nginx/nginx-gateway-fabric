# NGF + BIG-IP IngressLink Integration Demo Walkthrough

## Overview

Today I'm going to do a quick demo showing how NGINX Gateway Fabric can integrate with F5 BIG-IP using the IngressLink pattern.

I have CIS installed on our cluster so let’s take a look at what we configured there

```shell
k describe deployment -n kube-system f5-cis-f5-bigip-ctlr
```

I have the controller mode set to custom resources so that CIS will pick up my IngressLink resources, and I have the pool member type set to cluster so that Big IP will route directly to my Gateway dataplane pods without the need for kube-proxy, but it’s important to note that if your environment does not support this configuration, setting the pool member type to NodePort is also supported, in which case the virtual server will be populated with the node IPs and the port exposed for the Gateway dataplane instead.

I also have NGF installed here in my cluster

```shell
k describe deployment -n nginx-gateway nginx-gateway-nginx-gateway-fabric
```

You can see we have this flag configured here which enables our gateway link integration

## Part 1: Deploy the Application Stack

So next we will go ahead and deploy our backend applications and our Gateway API resources to configure the traffic to these applications, and then I’ll talk through the configuration

```shell
kubectl config set-context --current --namespace=ngf-test && k apply -f gateway-test/
```

First let’s take a look at our Gateway resource.

```shell
cat gateway-test/gateway.yaml
```

The Gateway has a parametersRef that points to an NginxProxy resource. The NginxProxy is an NGF CRD and is NGF's way of configuring global data plane settings and infrastructure.

So let’s take a look next at that nginx proxy

```shell
cat gateway-test/nginxproxy.yaml
```

NginxProxy has three critical sections for our BIG-IP integration

First we have our GatewayLink Configuration:

- "enabled: true tells NGF to create IngressLink resources for Gateways using this NginxProxy"
- "virtualServerAddress: 10.8.3.101 specifies the IP address BIG-IP will use for the Virtual Server"
- We can also support IPAM labels if an IPAM service is configured for your Big IP, in which case an IP will be dynamically allocated and reported back in both the IngressLink status and the Gateway address field
- We can provide a list of iRules that should be applied to the Virtual Server, here I am applying the proxy protocol iRule which tells Big IP to send the Proxy Protocol headers to preserve our client IPs
- So, when we create a Gateway that references this NginxProxy, NGF will automatically create a corresponding IngressLink resource
- And then this IngressLink is what F5 CIS watches and uses to configure BIG-IP

Next we have our NGINX PROXY Protocol configuration:

- We have our proxy protocol iRule configured telling Big IP it should send the PROXY protocol headers, but we also need to configure NGINX to accept these PROXY protocol headers
- To do this, we configure our rewriteClientIP section to use ProxyProtocol
- trustedAddresses specifies which IPs we trust to send PROXY headers - in this case, BIG-IP's network range.
- Without this configuration, NGF would reject BIG-IP's connections with the proxy protocol headers as malformed HTTP requests

Finally we have kubernetes section here where we define the infrastructure related configuration
- You can see we are detting our service type to Cluster to match our CIS settings and
- We have our readiness probe configuration so that our Big IP healthchecks will be correctly served from our instance

So NginxProxy serves as the bridge configuration that tells NGF: create IngressLink resources, use this IP for the Virtual Server, and accept PROXY protocol from BIG-IP.

## Part 2: Explore Kubernetes Resources

So let’s take a look at some of these provisioned resources

```shell
kubectl describe ingresslink my-gateway
```

Let me highlight what's in the IngressLink spec:

- virtualServerAddress: 10.8.3.101 - the IP from our NginxProxy configuration
- selector - matches the NGF data plane service by labels
- This tells CIS: create a BIG-IP Virtual Server at this IP, routing to the service that matches these labels

This IngressLink resource here is owned by the Gateway - NGF manages its lifecycle automatically. If we delete the Gateway, the IngressLink goes with it.

Now let’s take a look at the Gateway Service

```shell
kubectl describe svc my-gateway
```

This is the service that IngressLink points to.
Notice that these labels here correspond to the selector in the IngressLink

Next let’s take a look at the gateway itself

```shell
kubectl get gateway my-gateway
```

The Gateway is accepted and ready. This is our entry point resource, and you can see here the address has been updated to the Virtual Server address configured in the IngressLink status.

Finally let’s take a look at our running pods

```shell
kubectl get pods -o wide
```

Here are all our running components - backend pods, and the NGF data plane pods that will receive traffic from BIG-IP.

## Part 3: Show BIG-IP Configuration

**Switch to BIG-IP GUI**

So next let’s take a look at our Big IP UI, and if we click into Virtual Servers here, we can see that CIS has automatically created this Virtual Server based on the IngressLink resource we just looked at. No manual BIG-IP configuration was required.

We can see:

- The Virtual Server is enabled and listening on port 80
- Our health checks are passing so our state is green and available
- Destination is 10.8.3.101:80 - exactly what we specified in our NginxProxy

So next let’s take a look at the pools, we can see here we have our pool that was created by CIS and it contains our NGF data plane pod endpoints.
These are the NGF data plane pod IPs that CIS discovered from the Kubernetes service.
Notice they're all showing green meaning they are available, so the:

- Health monitors are passing and
- BIG-IP can reach these pods directly

The pool members match the pod IPs we saw earlier in Kubernetes.

## Part 4: Test Traffic Flow

Next let’s test our traffic. We’re going to use curl’s resolve flag with our Virtual Server address

```shell
curl --resolve cafe-test.example.com:80:10.8.3.101 http://cafe-test.example.com/tea
curl --resolve cafe-test.example.com:80:10.8.3.101 http://cafe-test.example.com/coffee
```

Traffic flows through the complete path:
- Client → BIG-IP Virtual Server (10.8.3.101)
- BIG-IP → NGF data plane pod (with PROXY protocol header)
- NGINX evaluates the request, and forwards to our tea and coffee service pods
- And our servers respond back through the same path

## Part 5: Integrate production gateway with Big IP

The next thing I’d like to show you is how we can have multiple Gateway deployments, each corresponding to a separate Virtual Server in BIG IP.

NGF has a split control plane/data plane architecture. The control plane is a single, shared component that:

- Watches Gateway API resources across all namespaces
- Translates the Gateway API and related resources into NGINX configuration
- Dynamically provisions data plane deployments and related infrastructure when Gateways are created

The data plane is what actually handles traffic:

- Each Gateway gets its own dedicated data plane deployment and service in the same namespace as the Gateway it serves
- The pods within the deployment run NGINX with the configuration generated by the control plane

This split architecture provides many benefits, but the one I want to highlight here is Multi-tenancy:

- Different teams can have their own Gateways and data planes
- Each data plane is isolated to its Gateway's namespace
- And crucially there is complete traffic isolation

So I actually have another Gateway already deployed in my cluster here, but it does not have GatewayLink enabled, so let's go ahead and configure that now

```shell
kubectl config set-context --current --namespace=ngf && k apply -f gateway-prod/nginxproxy.yaml
```

```shell
k describe nginxproxy nginx-proxy-gatewaylink-prod
```

We can see we have configured this Gateway to use a different Virtual Server IP, but again this also could have been configured to use an IPAM label if configured in your environment.

**Switch to BIG-IP GUI**

When we look at our Big IP UI, we can see that this additional Virtual Server has been created, and the pool populated with IPs of our Gateway deployment.

**Switch back to terminal**

Running our curl again with our new IP and we see the traffic now is passing through our new Virtual Server

```shell
curl --resolve cafe.example.com:80:10.8.3.102 http://cafe.example.com/coffee
```

```shell
curl --resolve cafe.example.com:80:10.8.3.102 http://cafe.example.com/tea
```

## Part 6: Scaling

Another key feature I’d like to show is what happens when we scale our Gateway pods. I’m going to do this manually by editing our nginxproxy resource, but typically in a production environment this would be done using autoscaling, like via a Horizontal Pod Autoscaler which you can configure in your NginxProxy resource based on metrics defined for your environment such as memory or CPU usage. Each Gateway deployment can be scaled independently of both the control plane and each other, meaning high traffic Gateways can have more replicas, and low traffic gateways don’t waste resources. For our example here I’m going to scale my replicas for my prod gateway down, and then we’ll see that the pool in Big IP is automatically updated to reflect the new state.

```shell
k edit nginxproxy nginx-proxy-gatewaylink-prod
```

## Cleanup

To finish off here, I am going to go ahead and delete my test Gateway resources. NGF will cleanup the related infrastructure resources including the IngressLink

```shell
kubectl delete -f gateway-test/ -n ngf-test
```

```shell
k get ingresslink
```

```shell
k get pods
```

**Switch to BIG-IP GUI**

And we can see here that CIS has removed the Virtual Server.

And that's it, thanks for watching!
