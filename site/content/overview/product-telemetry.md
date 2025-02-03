---
title: "Product telemetry"
weight: 500
toc: true
---

Learn why, what and how NGINX Gateway Fabric collects telemetry.

## Overview

NGINX Gateway Fabric collects telemetry by default, which allows its developers to understand what features are most popular with its user base. This data is used to triage development work, prioritizing features and functionality that will positively impact the most people.

Telemetry data is collected once every 24 hours and sent to a service managed by F5 over HTTPS. Personally identifiable information (PII) is **not** collected. The list of data points that are collected can be seen below.

**If you would prefer to not have data collected, you can [opt-out](#opt-out) when installing NGINX Gateway Fabric.**

## Collected data

- **Kubernetes:**
  - **Platform:** the Kubernetes platform that NGINX Gateway Fabric is running on
  - **Version:** the Kubernetes version that NGINX Gateway Fabric is running on.
  - **Architecture:** the architecture that the Kubernetes environment is running on.
- **Cluster UID:** the UID of the `kube-system` Namespace in the cluster that NGINX Gateway Fabric is running in.
- **Cluster Node Count:** the number of Nodes in the cluster.
- **Version:** the version of the NGINX Gateway Fabric Deployment.
- **Deployment UID:** the UID of the NGINX Gateway Fabric Deployment.
- **Deployment Replica Count:** the count of NGINX Gateway Fabric Pods.
- **Image Build Source:** whether the image was built by GitHub or locally (values are `gha`, `local`, or `unknown`). The source repository of the images is **not** collected.
- **Deployment Flags:** a list of NGINX Gateway Fabric Deployment flags that are specified by a user. The actual values of non-boolean flags are **not** collected; we only record that they are either `true` or `false` for boolean flags and `default` or `user-defined` for the rest.
- **Count of Resources:** the total count of resources related to NGINX Gateway Fabric. This includes `GatewayClasses`, `Gateways`, `HTTPRoutes`,`GRPCRoutes`, `TLSRoutes`, `Secrets`, `Services`, `BackendTLSPolicies`, `ClientSettingsPolicies`, `NginxProxies`, `ObservabilityPolicies`, `UpstreamSettingsPolicies`, `SnippetsFilters`, and `Endpoints`. The data within these resources is **not** collected.
- **SnippetsFilters Info**a list of directive-context strings from applied SnippetFilters and a total count per strings. The actual value of any NGINX directive is **not** collected.
This data is used to identify the following information:

- The flavors of Kubernetes environments that are most popular among our users.
- The number of unique NGINX Gateway Fabric installations.
- The scale of NGINX Gateway Fabric Deployments.
- The scale of Gateway API resources.
- The used features of NGINX Gateway Fabric.

Our goal is to publicly discuss data trends to drive roadmap discussions in our [Community Meeting](https://github.com/nginx/nginx-gateway-fabric/discussions/1472).

## Opt out

You can disable product telemetry when installing NGINX Gateway Fabric using an option dependent on your installation method:

### Helm

Set the `nginxGateway.productTelemetry.enable=false` flag either in the `values.yaml` file or when installing:

```shell
helm install ... --set nginxGateway.productTelemetry.enable=false
```

### Manifests

Add the `--product-telemetry-disable` flag to the `nginx-gateway` container in your Deployment manifest.
