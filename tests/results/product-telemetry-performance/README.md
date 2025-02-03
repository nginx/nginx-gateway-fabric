# Product Telemetry Performance

## Goals

- Ensure the product telemetry feature doesn't negatively affect NGINX Gateway Fabric performance: CPU and memory usage.

> Note: this is a one-off test not expected to be run regularly.

## Test Environment

### Base

NGINX Plus: false

GKE Cluster:

- Node count: 3
- k8s version: v1.27.8-gke.1067004
- vCPUs per node: 2
- RAM per node: 4022900Ki
- Max pods per node: 110
- Zone: us-central1-c
- Instance Type: e2-medium

> Note: test environment was created by the longevity test.

The cluster also includes an installed OTel collector:

```text
helm install otel-collector open-telemetry/opentelemetry-collector -f values.yaml -n collector --create-namespace
```

Also, the longevity test setup creates traffic generation of HTTP and HTTPs traffic to NGF.

### Environments

- Clusters with NGF configured with telemetry period:
  - 1 min, version 799ea762cf8942b1022d766d841cbbdaba7345d4
  - 1 hour, version 799ea762cf8942b1022d766d841cbbdaba7345d4
  - 1 day (3 clusters):
    - One, version 799ea762cf8942b1022d766d841cbbdaba7345d4
    - Two, version 799ea762cf8942b1022d766d841cbbdaba7345d4
    - Three, version 92c291021526e60cbed67d5f91e91fa64c1a2d71, with [Enabled Memory Profiling](#enabled-memory-profiling)
  - Collect every 1 day but not report version 799ea762cf8942b1022d766d841cbbdaba7345d4
  - No telemetry enabled, version 92c291021526e60cbed67d5f91e91fa64c1a2d71, with [Enabled Memory Profiling](#enabled-memory-profiling)
  - No telemetry features exist in the code, version 9c1d3e9d0746426741e62bf57d20de6c46540fd4, with [Enabled Memory Profiling](#enabled-memory-profiling)


## Steps

1. Prepare images.
2. Provision clusters.
3. Deploy a OTel collector.
4. Deploy NGF and kick off traffic generation using longevity test setup.
5. Collect the results.

## Results

Notes:

- Description of go stats - see [here](#description-of-go-stats).
- High CPU usage of NGINX corresponds to generated HTTP traffic load.

### Cluster 1 min

Pod name: ngf-longevity-nginx-gateway-fabric-6796966485-dn42w

Graphs:

![cluster-1min-ngf-mem-usage.png](img%2Fcluster-1min-ngf-mem-usage.png)

![cluster-1min-ngf-go-stats.png](img%2Fcluster-1min-ngf-go-stats.png)

![cluster-1min-cpu-usage.png](img%2Fcluster-1min-cpu-usage.png)

Error in logs:

```text
ERROR 2024-03-13T01:54:30.481041447Z [resource.labels.containerName: nginx-gateway] error retrieving resource lock nginx-gateway/ngf-longevity-nginx-gateway-fabric-leader-election: Get "https://10.80.0.1:443/apis/coordination.k8s.io/v1/namespaces/nginx-gateway/leases/ngf-longevity-nginx-gateway-fabric-leader-election?timeout=10s": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
ERROR 2024-03-12 21:54:30.481 failed to start control loop: leader election lost
ERROR 2024-03-13T01:54:43.936492804Z [resource.labels.containerName: nginx-gateway] failed to start control loop: error setting initial control plane configuration: NginxGateway nginx-gateway/ngf-longevity-config not found: failed to get API group resources: unable to retrieve the complete list of server APIs: gateway.nginx.org/v1alpha1: Get "https://10.80.0.1:443/apis/gateway.nginx.org/v1alpha1?timeout=10s": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
```

Note: NGF lost connectivity with the Kubernetes API server (for some GKE-related reason). After that,
NGF container was restarted (expected).

### Cluster 1 hour

Pod name: ngf-longevity-nginx-gateway-fabric-54c698bc-dwcgm

Graphs:

![cluster-1hour-ngf-mem-usage.png](img%2Fcluster-1hour-ngf-mem-usage.png)

![cluster-1hour-ngf-go-stats.png](img%2Fcluster-1hour-ngf-go-stats.png)

![cluster-1hour-cpu-usage.png](img%2Fcluster-1hour-cpu-usage.png)

Error in logs:

```text
ERROR 2024-03-13T11:06:12.010500300Z [resource.labels.containerName: nginx-gateway] error retrieving resource lock nginx-gateway/ngf-longevity-nginx-gateway-fabric-leader-election: Get "https://10.67.208.1:443/apis/coordination.k8s.io/v1/namespaces/nginx-gateway/leases/ngf-longevity-nginx-gateway-fabric-leader-election?timeout=10s": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
ERROR 2024-03-13T11:06:12.153485820Z [resource.labels.containerName: nginx-gateway] failed to start control loop: leader election lost
ERROR 2024-03-13T15:55:38.956069598Z [resource.labels.containerName: nginx-gateway] failed to start control loop: error setting initial control plane configuration: NginxGateway nginx-gateway/ngf-longevity-config not found: failed to get API group resources: unable to retrieve the complete list of server APIs: gateway.nginx.org/v1alpha1: Get "https://10.67.208.1:443/apis/gateway.nginx.org/v1alpha1?timeout=10s": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
```

Note: NGF lost connectivity with the Kubernetes API server (for some GKE-related reason). After that,
NGF container was restarted (expected).

```text
INFO 2024-03-14T21:44:02.993719249Z [resource.labels.containerName: nginx-gateway] Trace[255962658]: "DeltaFIFO Pop Process" ID:gatewayclasses.gateway.networking.k8s.io,Depth:26,Reason:slow event handlers blocking the queue (14-Mar-2024 21:44:02.846) (total time: 143ms):
ERROR 2024-03-14T21:44:02.993747394Z [resource.labels.containerName: nginx-gateway] Trace[255962658]: [143.318633ms] [143.318633ms] END
```

Error comes from https://github.com/kubernetes/client-go/blob/1518fca9f06c6a73fc091535b8966c71704e657b/tools/cache/delta_fifo.go#L600
Not related to product telemetry feature.
Further investigation is needed to see if any fix is needed -- https://github.com/nginx/nginx-gateway-fabric/issues/1726

### Cluster 1 day One

Pod name: ngf-longevity-nginx-gateway-fabric-76754b6649-brjzm

Graphs:

![cluster-1day-one-ngf-mem-usage.png](img%2Fcluster-1day-one-ngf-mem-usage.png)

![cluster-1day-one-ngf-go-stats.png](img%2Fcluster-1day-one-ngf-go-stats.png)

![cluster-1day-one-cpu-usage.png](img%2Fcluster-1day-one-cpu-usage.png)


Error in logs: NONE

### Cluster 1 day Two

Pod name: ngf-longevity-nginx-gateway-fabric-76754b6649-zcmxq

Graphs:

![cluster-1day-two-ngf-mem-usage.png](img%2Fcluster-1day-two-ngf-mem-usage.png)

![cluster-1day-two-ngf-go-stats.png](img%2Fcluster-1day-two-ngf-go-stats.png)

![cluster-1day-two-cpu-usage.png](img%2Fcluster-1day-two-cpu-usage.png)

Error in logs: NONE

### Cluster 1 day Three with Enabled Memory Profiling

Pod name: ngf-longevity-nginx-gateway-fabric-79f664f755-wggvh

Graphs:

![cluster-1day-three-ngf-mem-usage.png](img%2Fcluster-1day-three-ngf-mem-usage.png)

![cluster-1day-three-ngf-go-stats.png](img%2Fcluster-1day-three-ngf-go-stats.png)

![cluster-1day-three-cpu-usage.png](img%2Fcluster-1day-three-cpu-usage.png)

Error in logs: NONE

Memory profile ~1day after start:

![cluster-1day-three-mem-profile.png](img%2Fcluster-1day-three-mem-profile.png)

### Cluster Collect every 1 day but not report

Pod name: ngf-longevity-nginx-gateway-fabric-6f47684cdc-n7h87

Graphs:

![cluster-1day-collect-not-report-ngf-mem-usage.png](img%2Fcluster-1day-collect-not-report-ngf-mem-usage.png)

![cluster-1day-collect-not-report-ngf-go-stats.png](img%2Fcluster-1day-collect-not-report-ngf-go-stats.png)

![cluster-1day-collect-not-report-cpu-usage.png](img%2Fcluster-1day-collect-not-report-cpu-usage.png)

Error in logs:

```text
ERROR 2024-03-13T08:30:09.930929048Z [resource.labels.containerName: nginx-gateway] error retrieving resource lock nginx-gateway/ngf-longevity-nginx-gateway-fabric-leader-election: Get "https://10.48.16.1:443/apis/coordination.k8s.io/v1/namespaces/nginx-gateway/leases/ngf-longevity-nginx-gateway-fabric-leader-election?timeout=10s": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
ERROR 2024-03-13T08:30:09.944459347Z [resource.labels.containerName: nginx-gateway] failed to start control loop: leader election lost
ERROR 2024-03-13T08:30:22.873021882Z [resource.labels.containerName: nginx-gateway] failed to start control loop: error setting initial control plane configuration: NginxGateway nginx-gateway/ngf-longevity-config not found: failed to get API group resources: unable to retrieve the complete list of server APIs: gateway.nginx.org/v1alpha1: Get "https://10.48.16.1:443/apis/gateway.nginx.org/v1alpha1?timeout=10s": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
ERROR 2024-03-13T08:30:46.186351040Z [resource.labels.containerName: nginx-gateway] failed to start control loop: error setting initial control plane configuration: NginxGateway nginx-gateway/ngf-longevity-config not found: failed to get API group resources: unable to retrieve the complete list of server APIs: gateway.nginx.org/v1alpha1: Get "https://10.48.16.1:443/apis/gateway.nginx.org/v1alpha1?timeout=10s": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
```

Note: NGF lost connectivity with the Kubernetes API server (for some GKE-related reason). After that,
NGF container was restarted (expected).

### Cluster No telemetry enabled with Enabled Memory Profiling

Pod name: ngf-longevity-nginx-gateway-fabric-d5b9c8cfc-bnlgs

Graphs:

![cluster-tel-disabled-ngf-mem-usage.png](img%2Fcluster-tel-disabled-ngf-mem-usage.png)

![cluster-tel-disabled-ngf-go-stats.png](img%2Fcluster-tel-disabled-ngf-go-stats.png)

![cluster-tel-disabled-cpu-usage.png](img%2Fcluster-tel-disabled-cpu-usage.png)

Error in logs: NONE

Memory profile ~1day after start:
![cluster-tel-disabled-mem-profile.png](img%2Fcluster-tel-disabled-mem-profile.png)

### Cluster No telemetry features exist in the code

Pod name: ngf-longevity-nginx-gateway-fabric-67c488fdbc-xbnf5

Graphs:

![cluster-no-tel-ngf-mem-usage.png](img%2Fcluster-no-tel-ngf-mem-usage.png)

![cluster-no-tel-ngf-go-stats.png](img%2Fcluster-no-tel-ngf-go-stats.png)

![cluster-no-tel-cpu-usage.png](img%2Fcluster-no-tel-cpu-usage.png)

Error in logs: NONE

Memory profile ~1day after start:

![cluster-no-tel-mem-profile.png](img%2Fcluster-no-tel-mem-profile.png)

## Conclusion

- The product telemetry feature doesn't introduce any memory leaks.
- The product telemetry feature doesn't introduce any visible CPU usage spikes.
- NGF memory usage without telemetry added in the code is slightly lower.
- There is an increase in evictable memory usage in NGF with telemetry enabled in 2 out of 3 similar clusters, but
  non-evictable memory usage stays constant (note: from the docs of `container/memory/used_bytes` memory here
  https://cloud.google.com/monitoring/api/metrics_kubernetes -  Evictable memory is memory that can be easily reclaimed
  by the kernel, while non-evictable memory cannot.)
- Memory profile with telemetry enabled doesn't show any telemetry related allocations.
- Unrelated to product telemetry but captured potential issue -- https://github.com/nginx/nginx-gateway-fabric/issues/1726

## Appendix

### Enabled Memory Profiling

```diff
diff --git a/cmd/gateway/main.go b/cmd/gateway/main.go
index 8761e3f1..2b914395 100644
--- a/cmd/gateway/main.go
+++ b/cmd/gateway/main.go
@@ -2,6 +2,8 @@ package main

 import (
        "fmt"
+       "net/http"
+       _ "net/http/pprof"
        "os"
 )

@@ -20,6 +22,10 @@ var (
 )

 func main() {
+       go func() {
+               fmt.Println(http.ListenAndServe("localhost:6060", nil))
+       }()
+
        rootCmd := createRootCommand()

        rootCmd.AddCommand(
```

Getting memory profile after port-forwarding into the NGF pod:

```text
go tool pprof -png http://localhost:6060/debug/pprof/heap > out-1.png
```

### Description of Go Stats

- heap bytes

  ```text
  # HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.
  # TYPE go_memstats_alloc_bytes gauge
  ```

- sys bytes

  ```text
  # HELP go_memstats_sys_bytes Number of bytes obtained from system.
  # TYPE go_memstats_sys_bytes gauge
  ```

- heap inuse bytes

  ```text
  # HELP go_memstats_heap_inuse_bytes Number of heap bytes that are in use.
  # TYPE go_memstats_heap_inuse_bytes gauge

- heap idle bytes

  ```text
  # HELP go_memstats_heap_idle_bytes Number of heap bytes waiting to be used.
  # TYPE go_memstats_heap_idle_bytes gauge
  ```

- heap released bytes

  ```text
  # HELP go_memstats_heap_released_bytes Number of heap bytes released to OS.
  # TYPE go_memstats_heap_released_bytes gauge
  ```

- stack inuse bytes

  ```text
  # HELP go_memstats_stack_inuse_bytes Number of bytes in use by the stack allocator.
  # TYPE go_memstats_stack_inuse_bytes gauge
  ```

See also https://pkg.go.dev/runtime#MemStats
