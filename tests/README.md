# NGINX Gateway Fabric Testing

## Overview

This directory contains the tests for NGINX Gateway Fabric. The tests are divided into two categories:

1. Conformance Testing. This is to ensure that the NGINX Gateway Fabric conforms to the Gateway API specification.
2. System Testing. This is to ensure that the NGINX Gateway Fabric works as expected in a real system.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Prerequisites](#prerequisites)
- [Common steps for all tests](#common-steps-for-all-tests)
  - [Step 1 - Create a Kubernetes cluster](#step-1---create-a-kubernetes-cluster)
  - [Step 2 - Build and Load Images](#step-2---build-and-load-images)
- [Conformance Testing](#conformance-testing)
  - [Step 1 - Install NGINX Gateway Fabric to configured kind cluster](#step-1---install-nginx-gateway-fabric-to-configured-kind-cluster)
    - [Option 1 - Build and install NGINX Gateway Fabric from local to configured kind cluster](#option-1---build-and-install-nginx-gateway-fabric-from-local-to-configured-kind-cluster)
    - [Option 2 - Install NGINX Gateway Fabric from local already built image to configured kind cluster](#option-2---install-nginx-gateway-fabric-from-local-already-built-image-to-configured-kind-cluster)
    - [Option 3 - Install NGINX Gateway Fabric from edge to configured kind cluster](#option-3---install-nginx-gateway-fabric-from-edge-to-configured-kind-cluster)
  - [Step 2 - Build conformance test runner image](#step-2---build-conformance-test-runner-image)
  - [Step 3 - Run Gateway conformance tests](#step-3---run-gateway-conformance-tests)
  - [Step 4 - Cleanup the conformance test fixtures and uninstall NGINX Gateway Fabric](#step-4---cleanup-the-conformance-test-fixtures-and-uninstall-nginx-gateway-fabric)
  - [Step 5 - Revert changes to Go modules](#step-5---revert-changes-to-go-modules)
  - [Step 6 - Delete kind cluster](#step-6---delete-kind-cluster)
- [System Testing](#system-testing)
  - [Logging in tests](#logging-in-tests)
  - [Step 1 - Run the tests](#step-1---run-the-tests)
    - [Run the functional tests locally](#run-the-functional-tests-locally)
    - [Run the NFR tests on a GKE cluster from a GCP VM](#run-the-nfr-tests-on-a-gke-cluster-from-a-gcp-vm)
      - [Longevity testing](#longevity-testing)
  - [Common test amendments](#common-test-amendments)
  - [Step 2 - Cleanup](#step-2---cleanup)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites

- Kubernetes cluster.
- [kind](https://kind.sigs.k8s.io/).
- Docker.
- Golang.
- [yq](https://github.com/mikefarah/yq/#install)
- Make.

If running NFR tests:

- The [gcloud CLI](https://cloud.google.com/sdk/docs/install)
- A GKE cluster (if `master-authorized-networks` is enabled, please set `ADD_VM_IP_AUTH_NETWORKS=true` in your vars.env file)
- Access to GCP Service Account with Kubernetes admin permissions

All the commands below are executed from the `tests` directory. You can see all the available commands by running `make help`.

## Common steps for all tests

### Step 1 - Create a Kubernetes cluster

**Important**: Functional/conformance tests can only be run on a `kind` cluster. NFR tests can only be run on a GKE cluster.

To create a local `kind` cluster:

```makefile
make create-kind-cluster
```

> Note: The default kind cluster deployed is the latest available version. You can specify a different version by
> defining the kind image to use through the KIND_IMAGE variable, e.g.

```makefile
make create-kind-cluster KIND_IMAGE=kindest/node:v1.27.3
```

To create a GKE cluster:

Before running the below `make` command, copy the `scripts/vars.env-example` file to `scripts/vars.env` and populate the
required env vars. `GKE_SVC_ACCOUNT` needs to be the name of a service account that has Kubernetes admin permissions,
and `GKE_NODES_SERVICE_ACCOUNT` needs to be the name of a service account that has `Artifact Registry Reader`,
`Kubernetes Engine Node Service Account` and `Monitoring Viewer` permissions.

```makefile
make create-gke-cluster
```

> Note: The GKE cluster is created with `master-authorized-networks`, meaning only IPs from explicitly allowed CIDR ranges
> will be able to access the cluster. The script will automatically add your current IP to the authorized list, but if
> your IP changes, you can add your new local IP to the `master-authorized-networks` of the cluster by running the
> following:

```makefile
make add-local-ip-to-cluster
```

### Step 2 - Build and Load Images

Loading the images only applies to a `kind` cluster. If using a cloud provider, you will need to tag and push
your images to a registry that is accessible from that cloud provider.

```makefile
make build-images load-images TAG=$(whoami)
```

Or, to build NGF with NGINX Plus enabled (NGINX Plus cert and key must exist in the root of the repo):

```makefile
make build-images-with-plus load-images-with-plus TAG=$(whoami)
```

For the telemetry test, which requires a OTel collector, build an image with the following variables set:

```makefile
TELEMETRY_ENDPOINT=otel-collector-opentelemetry-collector.collector.svc.cluster.local:4317 TELEMETRY_ENDPOINT_INSECURE=true
```

## Conformance Testing

### Step 1 - Install NGINX Gateway Fabric to configured kind cluster

> Note: If you want to run the latest conformance tests from the Gateway API `main` branch, set the following
> environment variable before deploying NGF:

```bash
 export GW_API_VERSION=main
```

> Otherwise, the latest stable version will be used by default.
> Additionally, if you want to run conformance tests with experimental features enabled, set the following
> environment variable before deploying NGF:

```bash
 export ENABLE_EXPERIMENTAL=true
```

#### Option 1 - Build and install NGINX Gateway Fabric from local to configured kind cluster

```makefile
make install-ngf-local-build
```

Or, to install NGF with NGINX Plus enabled (NGINX Plus cert and key must exist in the root of the repo):

```makefile
make install-ngf-local-build-with-plus
```

#### Option 2 - Install NGINX Gateway Fabric from local already built image to configured kind cluster

You can optionally skip the actual _build_ step.

```makefile
make install-ngf-local-no-build
```

Or, to install NGF with NGINX Plus enabled:

```makefile
make install-ngf-local-no-build-with-plus
```

#### Option 3 - Install NGINX Gateway Fabric from edge to configured kind cluster

You can also skip the build NGF image step and prepare the environment to instead use the `edge` image. Note that this
option does not currently support installing with NGINX Plus enabled.

```makefile
make install-ngf-edge
```

### Step 2 - Build conformance test runner image

> Note: If you want to run the latest conformance tests from the Gateway API `main` branch, run the following
> make command to update the Go modules to `main`:

```makefile
make update-go-modules
```

> You can also point to a specific fork/branch by running:

```bash
go mod edit -replace=sigs.k8s.io/gateway-api=<your-fork>@<your-branch>
go mod download
go mod verify
go mod tidy
```

> Otherwise, the latest stable version will be used by default.

```makefile
make build-test-runner-image
```

### Step 3 - Run Gateway conformance tests

```makefile
make run-conformance-tests
```

### Step 4 - Cleanup the conformance test fixtures and uninstall NGINX Gateway Fabric

```makefile
make cleanup-conformance-tests
```

```makefile
make uninstall-ngf
```

### Step 5 - Revert changes to Go modules

**Optional** Not required if you aren't running the `main` Gateway API tests.

```makefile
make reset-go-modules
```

### Step 6 - Delete kind cluster

```makefile
make delete-kind-cluster
```

## System Testing

The system tests are meant to be run on a live Kubernetes environment to verify a real system. These
are similar to the existing conformance tests, but will verify things such as:

- NGF-specific functionality
- Non-Functional requirements (NFR) testing (such as performance, scale, etc.)

When running locally, the tests create a port-forward from your NGF Pod to localhost using a port chosen by the
test framework. Traffic is sent over this port. If running on a GCP VM targeting a GKE cluster, the tests will create an
internal LoadBalancer service which will receive the test traffic.

**Important**: Functional tests can only be run on a `kind` cluster. NFR tests can only be run on a GKE cluster.

Directory structure is as follows:

- `framework`: contains utility functions for running the tests
- `results`: contains the results files for the NFR tests
- `scripts`: contain scripts used to set up the environment and run the tests
- `suite`: contains the test files

### Logging in tests

To log in the tests, use the `GinkgoWriter` interface described here: https://onsi.github.io/ginkgo/#logging-output.

### Step 1 - Run the tests

#### Run the functional tests locally

```makefile
make test TAG=$(whoami)
```

Or, to run the tests with NGINX Plus enabled:

```makefile
make test TAG=$(whoami) PLUS_ENABLED=true
```

> The command above doesn't run the telemetry functional test, which requires a dedicated invocation because it uses a
> specially built image (see above) and it needs to deploy NGF differently from the rest of functional tests.

To run the telemetry test:

```makefile
make test TAG=$(whoami) GINKGO_LABEL=telemetry
```

#### Run the NFR tests on a GKE cluster from a GCP VM

Before running the below `make` commands, copy the `scripts/vars.env-example` file to `scripts/vars.env` and populate the
required env vars. `GKE_SVC_ACCOUNT` needs to be the name of a service account that has Kubernetes admin permissions.

In order to run the tests in GCP, you need a few things:

- GKE router to allow egress traffic (used by upgrade tests for pulling images from Github, and scale/reconfig tests for installing prometheus)
  - this assumes that your GKE cluster is using private nodes. If using public nodes, you don't need this.
- GCP VM and firewall rule to send ingress traffic to GKE

To just set up the VM with no router (this will not run the tests):

```makefile
make create-and-setup-vm
```

To set up just the router:

```makefile
make create-gke-router
```

Otherwise, you can set up the VM, router, and run the tests with a single command. See the options below.

By default, the tests run using the version of NGF that was `git cloned` during the setup. If you want to make
incremental changes and copy your local changes to the VM to test, you can run

```makefile
make sync-files-to-vm
```

To set up the GCP environment with the router and VM and then run the tests, run the following command:

```makefile
make setup-gcp-and-run-nfr-tests
```

To use an existing VM to run the tests, run the following

```makefile
make nfr-test
```

##### Longevity testing

This test is run on its own (and also not in a pipeline) due to its long-running nature. It will run for 4 days before
the tester must collect the results and complete the test.

To start the longevity test, set up your VM (`create-and-setup-vm`), router (`create-gke-router`), and run

```makefile
make start-longevity-test
```

<!--  -->

> Note: If you want to change the time period for which the test runs, update the `wrk` commands in `suite/scripts/longevity-wrk.sh` to the time period you want, and run `make sync-files-to-vm`.

<!--  -->

> Note: If you want to re-run the longevity test, you need to clear out the `cafe.example.com` entry from the `/etc/hosts` file on your VM.

You can verify the test is working by checking nginx logs to see traffic flow, and check that the cronjob is running and redeploying apps.

After 4 days (96h), you can complete the longevity tests and collect results. To ensure that the traffic has stopped flowing, you can ssh to the VM using `gcloud compute ssh` and run `ps aux | grep wrk` to verify the `wrk` commands are no longer running. Then, visit the [GCP Monitoring Dashboards](https://console.cloud.google.com/monitoring/dashboards) page and select the `NGF Longevity Test` dashboard. Take PNG screenshots of each chart for the time period in which your test ran, and save those to be added to the results file.

Finally, run

```makefile
make stop-longevity-test
```

This will tear down the test and collect results into a file, where you can add the PNGs of the dashboard. The results collection creates multiple files that you will need to manually combine as needed (logs file, traffic output file).

### Common test amendments

To run all tests with the label "my-label", use the GINKGO_LABEL variable:

```makefile
make test TAG=$(whoami) GINKGO_LABEL=my-label
```

or to pass a specific flag, e.g. run a specific test, use the GINKGO_FLAGS variable:

```makefile
make test TAG=$(whoami) GINKGO_FLAGS='-ginkgo.focus "writes the system info to a results file"'
```

> Note: if filtering on NFR tests, set the filter in the appropriate field in your `vars.env` file.

If you are running the tests in GCP, add your required label/ flags to `scripts/var.env`.

You can also modify the tests code for a similar outcome. To run a specific test, you can "focus" it by adding the `F`
prefix to the name. For example:

```go
It("runs some test", func(){
    ...
})
```

becomes:

```go
FIt("runs some test", func(){
    ...
})
```

This can also be done at higher levels like `Context`.

To disable a specific test, add the `X` prefix to it, similar to the previous example:

```go
It("runs some test", func(){
    ...
})
```

becomes:

```go
XIt("runs some test", func(){
    ...
})
```

For more information of filtering specs, see [the docs here](https://onsi.github.io/ginkgo/#filtering-specs).

### Step 2 - Cleanup

1. Delete kind cluster, if required

   ```makefile
   make delete-kind-cluster
   ```

2. Delete the GCP components (GKE cluster, GKE router, VM, and firewall rule), if required

   ```makefile
   make cleanup-gcp
   ```

   or

   ```makefile
   make cleanup-router
   ```

   ```makefile
   make cleanup-vm
   ```

   ```makefile
   make delete-gke-cluster
   ```
