# Running Gateway API Conformance Tests on OpenShift

This document describes the steps required to run Gateway API conformance tests on an OpenShift cluster.

## Prerequisites

- Access to an OpenShift cluster
- `oc` CLI tool installed and configured
- `kubectl` configured to access your OpenShift cluster
- Docker/Podman for building images
- Access to a container registry (e.g., quay.io)
- NGF should be preinstalled on the cluster before running the tests. You can install using the Operator or Helm.
**Note** :
  - the NGINX service type needs to be set to `ClusterIP`
  - the NGINX image referenced in the `NginxProxy` resource needs to be accessible to the cluster

## Overview

OpenShift has stricter security constraints than standard Kubernetes, requiring additional configuration to run the Gateway API conformance test suite.

## Step 1: Check Gateway API Version

OpenShift ships with Gateway API CRDs pre-installed. To find out which version is installed, run the following command:

    ```bash
    kubectl get crd gateways.gateway.networking.k8s.io -o jsonpath='{.metadata.annotations.gateway\.networking\.k8s\.io/bundle-version}'
    ```

### Updating NGF to Match OpenShift's Gateway API Version

To run conformance tests that match the exact Gateway API version on OpenShift:

1. Update Go modules:

   ```bash
   # Update parent module
   go get sigs.k8s.io/gateway-api@<OCP-version>
   go mod tidy

   # Update tests module
   cd tests
   go get sigs.k8s.io/gateway-api@<OCP-version>
   go mod tidy
   cd ..
   ```

    **Important:** Due to the `replace` directive in `tests/go.mod`, you must update both the parent and tests modules for the version change to take effect.

2. Update test configuration to remove features not available in the OCP-installed Gateway API version.

For **Gateway API v1.2.1**, you must update tests/conformance/conformance_test.go to eliminate references to v1beta1.GatewayStaticAddresses. This field was only introduced in Gateway API v1.3.0, and leaving it in place will cause the test to fail to compile in a v1.2.1 environment.

**Note:** This is separate from `SUPPORTED_EXTENDED_FEATURES_OPENSHIFT` in the Makefile, which controls which features are tested. This change is required because the conformance test code itself references v1.3.0+ features that don't exist in v1.2.1.

## Step 2: Build and Push Conformance Test Image

OpenShift typically runs on amd64 architecture. If you are building images from an arm64 machine, make sure to specify the target platform so the image is built for the correct architecture

1. Build the conformance test runner image for amd64:

   ```bash
   make -C tests build-test-runner-image GOARCH=amd64 CONFORMANCE_PREFIX=<public-repo>/<your-org>/conformance-test-runner CONFORMANCE_TAG=<tag>
   ```

2. Push the image to your registry:

   ```bash
   docker push <public-repo>/<your-org>/conformance-test-runner:<tag>
   ```

## Step 3: Configure Security Context Constraints (SCC)

OpenShift requires explicit permissions for pods to run with elevated privileges. To apply SCC permissions to allow coredns and other infrastructure pods, run:

   ```bash
   oc adm policy add-scc-to-group anyuid system:serviceaccounts:gateway-conformance-infra
   ```

**Note:** These permissions persist even if the namespace is deleted and recreated during test runs.

## Step 4: Run Conformance Tests

### Using the Makefile (Recommended)

Run the OpenShift-specific conformance test target:

```bash
make -C tests run-conformance-tests-openshift \
  CONFORMANCE_PREFIX=quay.io/your-org/conformance-test-runner \
  CONFORMANCE_TAG=<OCP-version> \
```

This target:

- Applies the RBAC configuration
- Runs only the extended features supported on the GatewayAPIs shipped with OpenShift
- Skips `HTTPRouteServiceTypes` test (incompatible with OpenShift)
- Pulls the image from your registry

## Step 5: Known Test Failures on OpenShift

### HTTPRouteServiceTypes

This test fails on OpenShift due to security restrictions on EndpointSlice creation:

```text
endpointslices.discovery.k8s.io "manual-endpointslices-ip4" is forbidden:
endpoint address 10.x.x.x is not allowed
```

**Solution:** Skip this test using `--skip-tests=HTTPRouteServiceTypes`

This is expected behavior - OpenShift validates that endpoint IPs belong to approved ranges, and the conformance test tries to create EndpointSlices with arbitrary IPs.

## Cleanup

```bash
kubectl delete pod conformance
kubectl delete -f tests/conformance/conformance-rbac.yaml
```

## Troubleshooting

### coredns pod fails with "Operation not permitted"

**Cause:** Missing SCC permissions

**Solution:** Apply the anyuid SCC as described in Step 3

### DNS resolution failures for LoadBalancer services

**Cause:** OpenShift cluster DNS cannot resolve external ELB/LoadBalancer hostnames

**Solution:** Use `GW_SERVICE_TYPE=ClusterIP`

### Architecture mismatch errors ("Exec format error")

**Cause:** Image built for wrong architecture (e.g., arm64 instead of amd64)

**Solution:** Rebuild with `GOARCH=amd64` as described in Step 3

## Summary

The key differences when running conformance tests on OpenShift vs. standard Kubernetes:

1. **SCC Permissions:** Required for coredns and infrastructure pods
2. **Service Type:** Must use `ClusterIP` to avoid DNS issues
3. **Architecture:** Explicit amd64 build required when building from arm64 machines
4. **Test Skips:** HTTPRouteServiceTypes must be skipped due to EndpointSlice restrictions
5. **Image Registry:** Images must be pushed to a registry accessible by OpenShift
