---
title: "Control plane configuration"
toc: true
weight: 400
type: how-to
product: NGF
docs: DOCS-1416
---

Learn how to dynamically update the NGINX Gateway Fabric control plane configuration.

## Overview

NGINX Gateway Fabric can dynamically update the control plane configuration without restarting. The control plane configuration is stored in the NginxGateway custom resource, created during the installation of NGINX Gateway Fabric.

NginxGateway is deployed in the same namespace as the controller (Default: `nginx-gateway`). The resource's default name is based on your [installation method]({{< relref "/installation/installing-ngf" >}}):

- Helm: `<release-name>-config`
- Manifests: `nginx-gateway-config`

The control plane only watches this single instance of the custom resource.

If the resource is invalid to the OpenAPI schema, the Kubernetes API server will reject the changes. If the resource is deleted or deemed invalid by NGINX Gateway Fabric, a warning event is created in the `nginx-gateway` namespace, and the default values will be used by the control plane for its configuration.

Additionally, the control plane updates the status of the resource (if it exists) to reflect whether it is valid or not.

**For a full list of configuration options that can be set, see the `NginxGateway spec` in the [API reference]({{< relref "reference/api.md" >}}).**

## Viewing and Updating the Configuration

{{< note >}} For the following examples, the name `ngf-config` should be updated to the name of the resource created for your installation.{{< /note >}}

To view the current configuration and its status:

```shell
kubectl -n nginx-gateway describe nginxgateways ngf-config
```

```text
...
Status:
  Conditions:
    Last Transition Time:  2024-08-13T19:22:14Z
    Message:               NginxGateway is valid
    Observed Generation:   1
    Reason:                Valid
    Status:                True
    Type:                  Valid
```

To update the configuration:

```shell
kubectl -n nginx-gateway edit nginxgateways ngf-config
```

This will open the configuration in your default editor. You can then update and save the configuration, which is applied automatically to the control plane.
