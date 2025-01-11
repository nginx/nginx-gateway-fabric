---
title: "Data plane configuration"
toc: true
weight: 500
type: how-to
product: NGF
docs: DOCS-000
---

Learn how to dynamically update the NGINX Gateway Fabric global data plane configuration.

## Overview

NGINX Gateway Fabric can dynamically update the global data plane configuration without restarting. The data plane configuration is a global configuration for NGINX that has options that are not available using the standard Gateway API resources. This includes such things as setting an OpenTelemetry collector config, disabling http2, changing the IP family, or setting the NGINX error log level.

The data plane configuration is stored in the NginxProxy custom resource, which is a cluster-scoped resource that is attached to the `nginx` GatewayClass.

By default, the NginxProxy resource is not created when installing NGINX Gateway Fabric. However, you can set configuration options in the `nginx.config` Helm values, and the resource will be created and attached when NGINX Gateway Fabric is installed using Helm. You can also [manually create and attach](#manually-creating-the-configuration) the resource after NGINX Gateway Fabric is already installed.

When installed using the Helm chart, the NginxProxy resource is named `<release-name>-proxy-config`.

**For a full list of configuration options that can be set, see the `NginxProxy spec` in the [API reference]({{< relref "reference/api.md" >}}).**

{{<note>}}Some global configuration also requires an [associated policy]({{< relref "overview/custom-policies.md" >}}) to fully enable a feature (such as [tracing]({{< relref "/how-to/monitoring/tracing.md" >}}), for example).{{</note>}}

## Viewing and Updating the Configuration

If the `NginxProxy` resource already exists, you can view and edit it.

{{< note >}} For the following examples, the name `ngf-proxy-config` should be updated to the name of the resource created for your installation.{{< /note >}}

To view the current configuration:

```shell
kubectl describe nginxproxies ngf-proxy-config
```

To update the configuration:

```shell
kubectl edit nginxproxies ngf-proxy-config
```

This will open the configuration in your default editor. You can then update and save the configuration, which is applied automatically to the data plane.

To view the status of the configuration, check the GatewayClass that it is attached to:

```shell
kubectl describe gatewayclasses nginx
```

```text
...
Status:
  Conditions:
     ...
    Message:               parametersRef resource is resolved
    Observed Generation:   1
    Reason:                ResolvedRefs
    Status:                True
    Type:                  ResolvedRefs
```

If everything is valid, the `ResolvedRefs` condition should be `True`. Otherwise, you will see an `InvalidParameters` condition in the status.

## Manually Creating the Configuration

If the `NginxProxy` resource doesn't exist, you can create it and attach it to the GatewayClass.

The following command creates a basic `NginxProxy` configuration that sets the IP family to `ipv4` instead of the default value of `dual`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: NginxProxy
metadata:
  name: ngf-proxy-config
spec:
  ipFamily: ipv4
EOF
```

Now we need to attach it to the GatewayClass:

```shell
kubectl edit gatewayclass nginx
```

This will open your default editor, allowing you to add the following to the `spec`:

```yaml
parametersRef:
    group: gateway.nginx.org
    kind: NginxProxy
    name: ngf-proxy-config
```

After updating, you can check the status of the GatewayClass to see if the configuration is valid:

```shell
kubectl describe gatewayclasses nginx
```

```text
...
Status:
  Conditions:
     ...
    Message:               parametersRef resource is resolved
    Observed Generation:   1
    Reason:                ResolvedRefs
    Status:                True
    Type:                  ResolvedRefs
```

If everything is valid, the `ResolvedRefs` condition should be `True`. Otherwise, you will see an `InvalidParameters` condition in the status.

## Configure the Data Plane Log Level

You can use the `NginxProxy` resource to dynamically configure the Data Plane Log Level.

The following command creates a basic `NginxProxy` configuration that sets the log level to `warn` instead of the default value of `info`:

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: NginxProxy
metadata:
  name: ngf-proxy-config
spec:
  logging:
    errorLevel: warn
EOF
```

After attaching the NginxProxy to the GatewayClass, the log level of the data plane will be updated to `warn`.

To view the full list of supported log levels, see the `NginxProxy spec` in the [API reference]({{< relref "reference/api.md" >}}).

{{< note >}}For `debug` logging to work, NGINX needs to be built with `--with-debug` or "in debug mode". NGINX Gateway Fabric can easily
be [run with NGINX in debug mode](#run-nginx-gateway-fabric-with-nginx-in-debug-mode) upon startup through the addition
of a few arguments. {{</ note >}}

### Run NGINX Gateway Fabric with NGINX in debug mode

To run NGINX Gateway Fabric with NGINX in debug mode, follow the [installation document]({{< relref "installation/installing-ngf" >}}) with these additional steps:

Using Helm: Set `nginx.debug` to true.

Using Kubernetes Manifests: Under the `nginx` container of the deployment manifest, add `-c` and `rm -rf /var/run/nginx/*.sock && nginx-debug -g 'daemon off;'`
as arguments and add `/bin/sh` as the command. The deployment manifest should look something like this:

```text
...
- args:
  - -c
  - rm -rf /var/run/nginx/*.sock && nginx-debug -g 'daemon off;'
  command:
  - /bin/sh
...
```

## Configure PROXY protocol and RewriteClientIP settings

When a request is passed through multiple proxies or load balancers, the client IP is set to the IP address of the server that last handled the request. To preserve the original client IP address, you can configure `RewriteClientIP` settings in the `NginxProxy` resource. `RewriteClientIP` has the fields: _mode_, _trustedAddresses_ and _setIPRecursively_.

**Mode** determines how the original client IP is passed through multiple proxies and the way the load balancer is set to receive it. It can have two values:

  1. `ProxyProtocol` is a protocol that carries connection information from the source requesting the connection to the destination for which the connection was requested.
  2. `XForwardedFor` is a multi-value HTTP header that is used by proxies to append IP addresses of the hosts that passed the request.

The choice of mode depends on how the load balancer fronting NGINX Gateway Fabric receives information.

**TrustedAddresses** are used to specify the IP addresses of the trusted proxies that pass the request. These can be in the form of CIDRs, IPs, or hostnames. For example, if a load balancer is forwarding the request to NGINX Gateway Fabric, the IP address of the load balancer should be specified in the `trustedAddresses` list to inform NGINX that the forwarded request is from a known source.

**SetIPRecursively** is a boolean field used to enable recursive search when selecting the client's address from a multi-value header. It is applicable in cases where we have a multi-value header containing client IPs to select from, i.e., when using `XForwardedFor` mode.

The following command creates an `NginxProxy` resource with `RewriteClientIP` settings that set the mode to ProxyProtocol and sets a CIDR in the list of trusted addresses to find the original client IP address.

```yaml
kubectl apply -f - <<EOF
apiVersion: gateway.nginx.org/v1alpha1
kind: NginxProxy
metadata:
  name: ngf-proxy-config
spec:
  config:
    rewriteClientIP:
      mode: ProxyProtocol
      trustedAddresses:
      - type: CIDR
        value "76.89.90.11/24"
EOF
```

For the full configuration API, see the `NginxProxy spec` in the [API reference]({{< relref "reference/api.md" >}}).

{{< note >}} When sending curl requests to a server expecting proxy information, use the flag `--haproxy-protocol` to avoid broken header errors. {{< /note >}}
