---
title: "NGINX Plus Image and JWT Requirement"
weight: 300
toc: true
docs: "DOCS-000"
---

## Overview

NGINX Gateway Fabric with NGINX Plus requires a valid JSON Web Token (JWT) to download the container image from the F5 registry. From version 1.5.0, this JWT token is also required to run NGINX Plus.

This requirement is part of F5’s broader licensing program and aligns with industry best practices. The JWT will streamline subscription renewals and usage reporting, helping you manage your NGINX Plus subscription more efficiently. The [telemetry](#telemetry) data we collect helps us improve our products and services to better meet your needs.

The JWT is required for validating your subscription and reporting telemetry data. For environments connected to the internet, telemetry is automatically sent to F5’s licensing endpoint. In offline environments, telemetry is routed through [NGINX Instance Manager](https://docs.nginx.com/nginx-instance-manager/). Usage is reported every hour and on startup whenever NGINX is reloaded.

## Set up the JWT

The JWT needs to be configured before deploying NGINX Gateway Fabric. The JWT will be stored in two Kubernetes Secrets: one for downloading the NGINX Plus container image, and the other for running NGINX Plus.

{{< include "installation/jwt-password-note.md" >}}

### Download the JWT from MyF5

{{< include "installation/nginx-plus/download-jwt.md" >}}

### Docker Registry Secret

{{< include "installation/nginx-plus/docker-registry-secret.md" >}}

Provide the name of this Secret when installing NGINX Gateway Fabric:

{{<tabs name="docker-secret-install">}}

{{%tab name="Helm"%}}

Specify the Secret name using the `serviceAccount.imagePullSecret` or `serviceAccount.imagePullSecrets` helm value.

{{% /tab %}}

{{%tab name="Manifests"%}}

Specify the Secret name in the `imagePullSecrets` field of the `nginx-gateway` ServiceAccount.

{{% /tab %}}

{{</tabs>}}

### NGINX Plus Secret

{{< include "installation/nginx-plus/nginx-plus-secret.md" >}}

If using a name other than the default `nplus-license`, provide the name of this Secret when installing NGINX Gateway Fabric:

{{<tabs name="plus-secret-install">}}

{{%tab name="Helm"%}}

Specify the Secret name using the `nginx.usage.secretName` helm value.

{{% /tab %}}

{{%tab name="Manifests"%}}

Specify the Secret name in the `--usage-report-secret` command-line flag on the `nginx-gateway` container.

You also need to define the proper volume mount to mount the Secret to the nginx container. If it doesn't already exist, add the following volume to the Deployment:

```yaml
- name: nginx-plus-license
  secret:
    secretName: nplus-license
```

and the following volume mount to the `nginx` container:

```yaml
- mountPath: /etc/nginx/license.jwt
  name: nginx-plus-license
  subPath: license.jwt
```

{{% /tab %}}

{{</tabs>}}

**If you are reporting to the default licensing endpoint, then you can now proceed with [installing NGINX Gateway Fabric]({{< relref "installation/installing-ngf" >}}). Otherwise, follow the steps below to configure reporting to NGINX Instance Manager.**

### Reporting to NGINX Instance Manager {#nim}

If you are deploying NGINX Gateway Fabric in an environment where you need to report to NGINX Instance Manager instead of the default licensing endpoint, a few extra steps may be required.

First, you must specify the endpoint of your NGINX Instance Manager:

{{<tabs name="nim-endpoint">}}

{{%tab name="Helm"%}}

Specify the endpoint using the `nginx.usage.endpoint` helm value.

{{% /tab %}}

{{%tab name="Manifests"%}}

Specify the endpoint in the `--usage-report-endpoint` command-line flag on the `nginx-gateway` container. You also need to add the following line to the `mgmt` block of the `nginx-includes-bootstrap` ConfigMap:

```text
usage_report endpoint=<your-endpoint>;
```

{{% /tab %}}

{{</tabs>}}

#### CA and Client certificate/key {#nim-cert}

To configure a CA cert and/or client certificate and key, a few extra steps are needed.

First, you need to create two Secrets in the `nginx-gateway` namespace. The CA must live under the key `ca.crt`:

```shell
kubectl -n nginx-gateway create secret generic nim-ca --from-file ca.crt
```

The client cert and key must be added to a TLS Secret:

```shell
kubectl -n nginx-gateway create secret tls nim-client --cert /path/to/cert --key /path/to/key
```

{{<tabs name="nim-secret-install">}}

{{%tab name="Helm"%}}

Specify the CA Secret name using the `nginx.usage.caSecretName` helm value. Specify the client Secret name using the `nginx.usage.clientSSLSecretName` helm value.

{{% /tab %}}

{{%tab name="Manifests"%}}

Specify the CA Secret name in the `--usage-report-ca-secret` command-line flag on the `nginx-gateway` container. Specify the client Secret name in the `--usage-report-client-ssl-secret` command-line flag on the `nginx-gateway` container.

You also need to define the proper volume mount to mount the Secrets to the nginx container. Add the following volume to the Deployment:

```yaml
- name: nginx-plus-usage-certs
  projected:
    sources:
    - secret:
        name: nim-ca
    - secret:
        name: nim-client
```

and the following volume mounts to the `nginx` container:

```yaml
- mountPath: /etc/nginx/certs-bootstrap/
  name: nginx-plus-usage-certs
```

Finally, in the `nginx-includes-bootstrap` ConfigMap, add the following lines to the `mgmt` block:

```text
ssl_trusted_certificate /etc/nginx/certs-bootstrap/ca.crt;
ssl_certificate        /etc/nginx/certs-bootstrap/tls.crt;
ssl_certificate_key    /etc/nginx/certs-bootstrap/tls.key;
```

{{% /tab %}}

{{</tabs>}}

<br>

**Once these Secrets are created and configuration options are set, you can now [install NGINX Gateway Fabric]({{< relref "installation/installing-ngf" >}}).**

## Installation flags to configure usage reporting {#flags}

When installing NGINX Gateway Fabric, the following flags can be specified to configure usage reporting to fit your needs:

If using Helm, the `nginx.usage` values should be set as necessary:

- `secretName` should be the name of the JWT Secret you created. By default this field is set to `nplus-license`. This field is required.
- `endpoint` is the endpoint to send the telemetry data to. This is optional, and by default is `product.connect.nginx.com`.
- `resolver` is the nameserver used to resolve the NGINX Plus usage reporting endpoint. This is optional and used with NGINX Instance Manager.
- `skipVerify` disables client verification of the NGINX Plus usage reporting server certificate.
- `caSecretName` is the name of the Secret containing the NGINX Instance Manager CA certificate. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).
- `clientSSLSecretName` is the name of the Secret containing the client certificate and key for authenticating with NGINX Instance Manager. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).

If using manifests, the following command-line options should be set as necessary on the `nginx-gateway` container:

- `--usage-report-secret` should be the name of the JWT Secret you created. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). By default this field is set to `nplus-license`. A [volume mount](#nginx-plus-secret) for this Secret is required for installation.
- `--usage-report-endpoint` is the endpoint to send the telemetry data to. This is optional, and by default is `product.connect.nginx.com`. Requires [extra configuration](#nim) if specified.
- `--usage-report-resolver` is the nameserver used to resolve the NGINX Plus usage reporting endpoint. This is optional and used with NGINX Instance Manager.
- `--usage-report-skip-verify` disables client verification of the NGINX Plus usage reporting server certificate.
- `--usage-report-ca-secret` is the name of the Secret containing the NGINX Instance Manager CA certificate. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). Requires [extra configuration](#nim-cert) if specified.
- `--usage-report-client-ssl-secret` is the name of the Secret containing the client certificate and key for authenticating with NGINX Instance Manager. Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway). Requires [extra configuration](#nim-cert) if specified.

## What’s reported and how it’s protected {#telemetry}

NGINX Plus reports the following data every hour by default:

- **NGINX version and status**: The version of NGINX Plus running on the instance.
- **Instance UUID**: A unique identifier for each NGINX Plus instance.
- **Traffic data**:
  - **Bytes received from and sent to clients**: HTTP and stream traffic volume between clients and NGINX Plus.
  - **Bytes received from and sent to upstreams**: HTTP and stream traffic volume between NGINX Plus and upstream servers.
  - **Client connections**: The number of accepted client connections (HTTP and stream traffic).
  - **Requests handled**: The total number of HTTP requests processed.
- **NGINX uptime**: The number of reloads and worker connections during uptime.
- **Usage report timestamps**: Start and end times for each usage report.
- **Kubernetes node details**: Information about Kubernetes nodes.

### Security and privacy of reported data

All communication between your NGINX Plus instances, NGINX Instance Manager, and F5’s licensing endpoint (`product.connect.nginx.com`) is protected using **SSL/TLS** encryption.

Only **operational metrics** are reported — no **personally identifiable information (PII)** or **sensitive customer data** is transmitted.

## Pull an image for local use

To pull an image for local use, use this command:

```shell
docker login private-registry.nginx.com --username=<JWT Token> --password=none
```

Replace the contents of `<JWT Token>` with the contents of the JWT token itself.

You can then pull the image:

```shell
docker pull private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.6.0
```

Once you have successfully pulled the image, you can tag it as needed, then push it to a different container registry.

## Alternative installation options

There are alternative ways to get an NGINX Plus image for NGINX Gateway Fabric:

- [Build the Gateway Fabric image]({{<relref "installation/building-the-images.md">}}) describes how to use the source code with an NGINX Plus subscription certificate and key to build an image.
