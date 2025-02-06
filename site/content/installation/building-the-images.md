---
title: "Build NGINX Gateway Fabric and NGINX images"
weight: 500
toc: true
docs: "DOCS-1431"
---

## Overview

While most users will install NGINX Gateway Fabric [with Helm]({{< relref "/installation/installing-ngf/helm.md" >}}) or [Kubernetes manifests]({{< relref "/installation/installing-ngf/manifests.md" >}}), manually building the [NGINX Gateway Fabric and NGINX images]({{< relref "/overview/gateway-architecture.md#the-nginx-gateway-fabric-pod" >}}) can be helpful for testing and development purposes. Follow the steps in this document to build the NGINX Gateway Fabric and NGINX images.

## Prerequisites

Before you can build the NGINX Gateway Fabric and NGINX images, make sure you have the following software
installed on your machine:

- [git](https://git-scm.com/)
- [GNU Make](https://www.gnu.org/software/software.html)
- [Docker](https://www.docker.com/) v18.09+
- [Go](https://go.dev/doc/install) v1.20

If building the NGINX Plus image, you will also need a valid NGINX Plus license certificate (`nginx-repo.crt`) and key (`nginx-repo.key`) in the root of the repo.

## Steps

1. Clone the repo and change into the `nginx-gateway-fabric` directory:

   ```shell
   git clone https://github.com/nginx/nginx-gateway-fabric.git --branch v1.6.0
   cd nginx-gateway-fabric
   ```

1. Build the images:
   - To build both the NGINX Gateway Fabric and NGINX images:

      ```makefile
      make PREFIX=myregistry.example.com/nginx-gateway-fabric build-prod-images
      ```

   - To build both the NGINX Gateway Fabric and NGINX Plus images:

      ```makefile
      make PREFIX=myregistry.example.com/nginx-gateway-fabric build-prod-images-with-plus
      ```

   - To build just the NGINX Gateway Fabric image:

     ```makefile
     make PREFIX=myregistry.example.com/nginx-gateway-fabric build-prod-ngf-image
     ```

   - To build just the NGINX image:

     ```makefile
     make PREFIX=myregistry.example.com/nginx-gateway-fabric build-prod-nginx-image
     ```

   - To build just the NGINX Plus image:

     ```makefile
     make PREFIX=myregistry.example.com/nginx-gateway-fabric/nginx-plus build-prod-nginx-plus-image
     ```

   Set the `PREFIX` variable to the name of the registry you'd like to push the image to. By default, the images will be
   named `nginx-gateway-fabric:1.6.0` and `nginx-gateway-fabric/nginx:1.6.0` or `nginx-gateway-fabric/nginx-plus:1.6.0`.

1. Push the images to your container registry:

   ```shell
   docker push myregistry.example.com/nginx-gateway-fabric:1.6.0
   docker push myregistry.example.com/nginx-gateway-fabric/nginx:1.6.0
   ```

   or

   ```shell
   docker push myregistry.example.com/nginx-gateway-fabric:1.6.0
   docker push myregistry.example.com/nginx-gateway-fabric/nginx-plus:1.6.0
   ```

   Make sure to substitute `myregistry.example.com/nginx-gateway-fabric` with your registry.
