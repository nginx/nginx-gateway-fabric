# NGINX Gateway Fabric Operator

A Helm-based Kubernetes operator for deploying and managing [NGINX Gateway Fabric](https://github.com/nginx/nginx-gateway-fabric), an implementation of the Gateway API using NGINX as the data plane.

## Overview

The NGINX Gateway Fabric Operator simplifies the deployment and lifecycle management of NGINX Gateway Fabric in Kubernetes and OpenShift environments. It leverages the official NGINX Gateway Fabric Helm charts to provide a declarative way to install, configure, and manage Gateway API implementations.

## Features

- **Declarative Configuration**: Manage NGINX Gateway Fabric through Kubernetes custom resources
- **Helm Chart Integration**: Uses official NGINX Gateway Fabric Helm charts for reliable deployments
- **OpenShift Compatible**: Certified for Red Hat OpenShift with proper SecurityContextConstraints
- **Full Feature Support**: Supports all NGINX Gateway Fabric configuration options including:
  - NGINX Plus integration
  - Experimental Gateway API features
  - Multiple deployment modes (Deployment/DaemonSet)

## Prerequisites

- Kubernetes 1.25+ or OpenShift 4.19+
- Operator Lifecycle Manager (OLM) installed
- Gateway API CRDs installed

## Installation

### OpenShift OperatorHub

1. Navigate to OperatorHub in your OpenShift console
2. Search for "NGINX Gateway Fabric Operator"
3. Install the operator

## Usage

### Basic Installation

Create a `NginxGatewayFabric` custom resource to deploy NGINX Gateway Fabric:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: NginxGatewayFabric
metadata:
  name: nginx-gateway-fabric
spec:
  nginxGateway:
    replicas: 2
    gatewayClassName: nginx
  nginx:
    service:
      type: LoadBalancer
```

See [the example here](config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml).

## Configuration Reference

The `NginxGatewayFabric` custom resource accepts the same configuration options as the NGINX Gateway Fabric Helm chart.

For complete configuration options, see the [Helm Chart Documentation](https://github.com/nginx/nginx-gateway-fabric/tree/main/charts/nginx-gateway-fabric/README.md#configuration).

## Development

### Building and Testing the Operator Locally

```bash
# Build the operator image. If building for deploying on a cluster with different architecture from your local machine, append ARCH=<targetarch> e.g. `ARCH=amd64` to the below command
make docker-build IMG=<your-registry>/nginx-gateway-fabric/operator:<tag>

# Push the image
make docker-push IMG=<your-registry>/nginx-gateway-fabric/operator:<tag>

# Optionally load the image if running on kind
make docker-load IMG=<your-registry>/nginx-gateway-fabric/operator:<tag>

# Generate and push bundle (must be publicly accessible remote registry, e.g. quay.io)
make bundle-build bundle-push IMG=<your-registry>/nginx-gateway-fabric/operator:<tag> BUNDLE_IMG=<your-registry>/nginx-gateway-fabric/operator-bundle:<tag>

# Install olm on local cluster if required (e.g. if running on kind)
operator-sdk olm install

# Run your bundle image
operator-sdk run bundle <your-registry>/nginx-gateway-fabric/operator-bundle:<tag>

# Deploy NGF operand (modify the manifest if required)
kubectl apply -f config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml

# Deploy test application
kubectl apply -f ../examples/cafe-example/

# Run operator-sdk scorecard - optional
make bundle
operator-sdk scorecard bundle/
```

### Releases

Once NGF has released, we can prepare the Operator release using the published NGF images.

The Operator image is built using the Helm Chart from the root directory, so those changes are kept in sync by running `make docker-build` from the release branch (to be automated). The Operator image can be published and certified at the same time as the UBI based NGF control plane and OSS data plane images (instructions to follow, to be automated).

Once the images are certified and published, we can create the bundle for certification. This is mostly a scripted (note: to be automated) process.
However, there are a few items that need to be kept in sync manually:

1. RBAC:
    The Operator requires RBAC rules to include permissions for anything the NGF Helm chart
    can deploy (e.g. Pods, ConfigMaps, Gateways, HPAs, etc), and all permissions that NGF
    itself has permissions for (e.g. all the Gateway APIs etc).

    If the RBAC permissions either for or of the underlying Helm Chart changes, these need to be updated in [RBAC manifest](config/rbac/role.yaml).

    The next time `make bundle` is ran, these RBAC changes will be reflected in the resulting bundle manifests.

2. Sample manifest:
   The [example manifest](config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml) may need to be updated either to add new important fields, or to change existing entries.

3. Operator version:
    Update the VERSION in the Makefile to reflect the version of the Operator being released.

When you are ready to release the bundle, run `make release-bundle`. This will update the NGF image version tags, and create the bundle manifests.

To test the bundle locally, follow the `Building and Testing the Operator Locally` above.

To submit the bundle for certification, follow TBD.

## License

This project is licensed under the Apache License 2.0. See [LICENSE](../LICENSE) for details.

## Support

- Documentation: [NGINX Gateway Fabric Docs](https://docs.nginx.com/nginx-gateway-fabric/)
- Issues: [GitHub Issues](https://github.com/nginx/nginx-gateway-fabric/issues)
- Community: [NGINX Community Forum](https://community.nginx.org/c/nginx-gateway-fabric)

For commercial support, contact [F5 NGINX](https://www.f5.com/products/nginx).
