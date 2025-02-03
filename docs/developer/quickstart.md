# Development Quickstart

This guide will assist you in setting up your development environment for NGINX Gateway Fabric, covering the steps
to build, install, and execute tasks necessary for submitting pull requests. By following this guide, you'll have a
fully prepared development environment that allows you to contribute to the project effectively.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Setup Your Development Environment](#setup-your-development-environment)
- [Build the Binary and Images](#build-the-binary-and-images)
  - [Setting GOARCH](#setting-goarch)
  - [Build the Binary](#build-the-binary)
  - [Build the Images](#build-the-images)
  - [Build the Images with NGINX Plus](#build-the-images-with-nginx-plus)
- [Deploy on Kind](#deploy-on-kind)
  - [Run Examples](#run-examples)
- [Run the Unit Tests](#run-the-unit-tests)
- [Gateway API Conformance Testing](#gateway-api-conformance-testing)
- [Run the Linter](#run-the-linter)
- [Run the Helm Linter](#run-the-helm-linter)
- [Update all the generated files](#update-all-the-generated-files)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Setup Your Development Environment

Follow these steps to set up your development environment.

1. Install:

   - [Go](https://golang.org/doc/install) v1.21.0+
   - [Docker](https://docs.docker.com/get-docker/) v18.09+
   - [Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
   - [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
   - [Helm](https://helm.sh/docs/intro/quickstart/#install-helm)
   - [git](https://git-scm.com/)
   - [GNU Make](https://www.gnu.org/software/software.html)
   - [yq](https://github.com/mikefarah/yq/#install)
   - [fieldalignment](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/fieldalignment):

   ```shell
   go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
   ```

   - [pre-commit](https://pre-commit.com/#install):

   ```shell
   brew install pre-commit
   ```

2. [Fork the project repository](https://github.com/nginx/nginx-gateway-fabric/fork)
3. Clone your repository with ssh, and install the project dependencies:

   ```shell
   git clone git@github.com:<YOUR-USERNAME>/nginx-gateway-fabric.git
   cd nginx-gateway-fabric
   ```

   and then run

   ```shell
   pre-commit install
   ```

   in the project root directory to install the git hooks.

   ```makefile
   make deps
   ```

4. Finally, add the original project repository as the remote upstream:

   ```shell
   git remote add upstream git@github.com:nginx/nginx-gateway-fabric.git
   ```


## Build the Binary and Images

### Setting GOARCH

The [Makefile](/Makefile) uses the GOARCH variable to build the binary and container images. The default value of GOARCH is `amd64`.

If you are deploying NGINX Gateway Fabric on a kind cluster, and the architecture of your machine is not `amd64`, you will want to set the GOARCH variable to the architecture of your local machine. You can find the value of GOARCH by running `go env`. Export the GOARCH variable in your `~/.zshrc` or `~/.bashrc`.

```shell
echo "export GOARCH=< Your architecture (e.g. arm64 or amd64) >" >> ~/.bashrc
source ~/.bashrc
```

or for zsh:

```shell
echo "export GOARCH=< Your architecture (e.g. arm64 or amd64) >" >> ~/.zshrc
source ~/.zshrc
```

### Build the Binary

To build the binary, run the make build command from the project's root directory:

```makefile
make GOARCH=$GOARCH build
```

This command will build the binary and output it to the `/build/.out` directory.

### Build the Images

To build the NGINX Gateway Fabric and NGINX container images from source run the following make command:

```makefile
make GOARCH=$GOARCH TAG=$(whoami) build-images
```

This will build the docker images `nginx-gateway-fabric:<your-user>` and `nginx-gateway-fabric/nginx:<your-user>`.

### Build the Images with NGINX Plus

> Note: You will need a valid NGINX Plus license certificate and key named `nginx-repo.crt` and `nginx-repo.key` in the
> root of this repo to build the NGINX Plus image.
> You will also need a valid NGINX Plus JSON Web Token (JWT) to deploy NGF with NGINX Plus. That JWT should be stored in the `license.jwt` file in the root of the `nginx-gateway-fabric/` directory. See the [documentation](https://docs.nginx.com/nginx-gateway-fabric/installation/nginx-plus-jwt/) for instructions on how to download and set up the JWT.
> Additionally, you need to set the NGINX Plus usage endpoint in your environment. For development and testing, export `PLUS_USAGE_ENDPOINT=<N1 staging endpoint>`.

To build the NGINX Gateway Fabric and NGINX Plus container images from source run the following make command:

```makefile
make TAG=$(whoami) build-images-with-plus
```

This will build the docker images `nginx-gateway-fabric:<your-user>` and `nginx-gateway-fabric/nginx-plus:<your-user>`.

## Deploy on Kind

1. Create a `kind` cluster:

   To create a `kind` cluster with dual (IPv4 and IPv6) enabled:

   ```makefile
   make create-kind-cluster
   ```

   To create a `kind` cluster with IPv6 or IPv4 only, edit kind cluster config located at `nginx-gateway-fabric/config/cluster/kind-cluster.yaml`:

   ```yaml
   kind: Cluster
   apiVersion: kind.x-k8s.io/v1alpha4
   nodes:
   - role: control-plane
   networking:
     ipFamily: ipv6
     apiServerAddress: 127.0.0.1
   ```

2. Load the previously built images onto your `kind` cluster:

   ```shell
   kind load docker-image nginx-gateway-fabric:$(whoami) nginx-gateway-fabric/nginx:$(whoami)
   ```

   or

   ```shell
   kind load docker-image nginx-gateway-fabric:$(whoami) nginx-gateway-fabric/nginx-plus:$(whoami)
   ```

3. Install Gateway API CRDs:

   ```shell
   kubectl kustomize config/crd/gateway-api/standard | kubectl apply -f -
   ```

   If you're implementing experimental Gateway API features, install Gateway API CRDs from the experimental channel:

   ```shell
   kubectl kustomize config/crd/gateway-api/experimental | kubectl apply -f -
   ```

4. Install NGF using your custom image and expose NGF with a NodePort Service:

   - To install with Helm (where your release name is `my-release`):

     ```shell
     helm install my-release ./charts/nginx-gateway-fabric --create-namespace --wait --set service.type=NodePort --set nginxGateway.image.repository=nginx-gateway-fabric --set nginxGateway.image.tag=$(whoami) --set nginxGateway.image.pullPolicy=Never --set nginx.image.repository=nginx-gateway-fabric/nginx --set nginx.image.tag=$(whoami) --set nginx.image.pullPolicy=Never -n nginx-gateway
     ```

   - To install NGINX Plus with Helm (where your release name is `my-release`):

     ```shell
     helm install my-release ./charts/nginx-gateway-fabric --create-namespace --wait --set service.type=NodePort --set nginxGateway.image.repository=nginx-gateway-fabric --set nginxGateway.image.tag=$(whoami) --set nginxGateway.image.pullPolicy=Never --set nginx.image.repository=nginx-gateway-fabric/nginx-plus --set nginx.image.tag=$(whoami) --set nginx.image.pullPolicy=Never --set nginx.plus=true -n nginx-gateway
     ```

   > For more information on Helm configuration options see the Helm [README](../../charts/nginx-gateway-fabric/README.md).

   - To install with manifests:

     The mainifests files are genarated using Helm from the [examples](/examples/helm) directory. To generate a custom one you can modify the `values.yaml` file in the example you want to use with the desired values and follow the instructions about [manifests generation](/examples/helm/README.md#manifests-generation).

     If the only change is the image repository and tag, you can update the `kustomization.yaml` file in `deploy/` with the desired values and deployment mainifest and run the following commands:

     ```shell
      kubectl apply -f deploy/crds.yaml
      kubectl kustomize deploy | kubectl apply -f -
     ```

  > For more information on how to use the manifests, see the [deployment manifests](/deploy/README.md) documentation.

### Run Examples

To make sure NGF is running properly, try out the [examples](/examples).

## Run the Unit Tests

To run all the unit tests, run the make unit-test command from the project's root directory:

```makefile
make unit-test
```

For more details on testing, see the [testing](/docs/developer/testing.md) documentation.

## Gateway API Conformance Testing

To run Gateway API conformance tests, please follow the instructions on [this](/tests/README.md#conformance-testing) page.

## Run the Linter

To lint the code, run the following make command from the project's root directory:

```makefile
make lint
```

> **Note**
> fieldalignment errors can be fixed by running: `fieldalignment -fix <path-to-package>`

## Run the Helm Linter

Run the following make command from the project's root directory to lint the Helm Chart code:

```shell
make lint-helm
```

## Update all the generated files

To update all the generated files, run the following make command from the project's root directory:

```makefile
make generate-all
```
