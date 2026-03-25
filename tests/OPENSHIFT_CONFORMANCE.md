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

## Step 1: Setup tests to run or skip

OpenShift ships with Gateway API CRDs pre-installed. To find out which version is installed, run the following command:

```bash
kubectl get crd gateways.gateway.networking.k8s.io -o jsonpath='{.metadata.annotations.gateway\.networking\.k8s\.io/bundle-version}'
```

1. Update the `SKIP_TESTS_OPENSHIFT` list in the Makefile to remove features not available in the OCP-installed Gateway API version.

2. Add any missing extended supported features to `SUPPORTED_EXTENDED_FEATURES_OPENSHIFT` in the Makefile that can be run.

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

Helm install NGF using publicly accessible images. These could be the release candidate (RC) images in Github, or locally built images (amd64) that you've pushed to a public repository. **Don't push NGINX Plus images to a public repository.** Ensure that the NGINX service type is set to NodePort.

Run the OpenShift-specific conformance test target:

```bash
make -C tests run-conformance-tests-openshift \
  CONFORMANCE_PREFIX=quay.io/your-org/conformance-test-runner \
  CONFORMANCE_TAG=<OCP-version> \
```

This target:

- Applies the RBAC configuration
- Runs only the extended features supported on the GatewayAPIs shipped with OpenShift
- Skips `HTTPRouteServiceTypes` test (incompatible with OpenShift) and any other tests you've added to the list
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

**Solution:** Ensure that the NGINX Service type is set to NodePort in the NginxProxy CRD

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
