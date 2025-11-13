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

The Operator release process is largely automated. Once NGF has released, follow these steps:

#### Automated Release Process

1. Production Release: During the NGF production release (via the CI workflow), set the `operator_version` input to the new operator version (e.g., `v1.0.1`). This will:
   - Build and push the operator image with the release tag
   - Automatically submit the operator UBI image for RedHat certification via preflight

2. Bundle Generation: After the production release completes, run the [Operator Bundle PR workflow](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/operator-bundle-pr.yml):
   - Set `operator-version` to the operator version without the `v` prefix (e.g., `1.0.1`)
   - Set `submit-to-redhat` to `true` to automatically submit to the RedHat certified-operators repository

   This workflow will:
   - Generate bundle manifests with NGF image versions updated to use image digests
   - Create a draft PR in the NGF repository with the bundle changes
   - If enabled, automatically:
     - Fork and update the [RedHat certified-operators repository](https://github.com/redhat-openshift-ecosystem/certified-operators)
     - Create a branch with the new bundle version
     - Open a PR to the upstream certified-operators repo

3. Review and Merge:
   - Review and merge the internal bundle PR once approved
   - Monitor the RedHat certified-operators PR for review feedback from RedHat
   - Ensure the bundle PR changes are duplicated back to the main branch as well

#### RBAC Synchronization

The Operator requires RBAC rules that include:

- Permissions for anything the NGF Helm chart can deploy (e.g. Pods, ConfigMaps, Gateways, HPAs, etc)
- All permissions that NGF itself has (e.g. all the Gateway API resources)

Automated Verification: A CI check runs automatically on PRs that modify RBAC files to ensure the operator RBAC includes all permissions from the Helm chart. You can also verify locally:

```bash
./operators/scripts/verify-rbac-sync.sh
```

The verification script:

- Renders the Helm chart with all features enabled to extract the maximum permission set
- Compares operator RBAC permissions with the rendered chart
- Handles wildcard permissions (`verbs: ["*"]`) correctly
- Fails if any required permissions are missing

If RBAC permissions in the Helm chart change, update [config/rbac/role.yaml](config/rbac/role.yaml) accordingly. The next time `make bundle` runs, these RBAC changes will be reflected in the bundle manifests.

#### Manual Items to Check

Before releasing, verify these items are up-to-date:

1. Sample manifest: The [example manifest](config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml) may need updates to add new important fields or change existing entries.

2. Operator version: The VERSION in the [Makefile](Makefile) is automatically updated during the release process, but verify it matches the intended release version.

#### Local Testing

To test the operator bundle locally before releasing, follow the [Building and Testing the Operator Locally](#building-and-testing-the-operator-locally) instructions above.

## License

This project is licensed under the Apache License 2.0. See [LICENSE](../LICENSE) for details.

## Support

- Documentation: [NGINX Gateway Fabric Docs](https://docs.nginx.com/nginx-gateway-fabric/)
- Issues: [GitHub Issues](https://github.com/nginx/nginx-gateway-fabric/issues)
- Community: [NGINX Community Forum](https://community.nginx.org/c/nginx-gateway-fabric)

For commercial support, contact [F5 NGINX](https://www.f5.com/products/nginx).
