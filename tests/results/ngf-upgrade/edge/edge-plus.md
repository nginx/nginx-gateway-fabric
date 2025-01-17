# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.992s, 59.991s, 882.315µs
Latencies     [min, mean, 50, 90, 95, 99, max]  623.681µs, 864.008µs, 845.597µs, 963.317µs, 1.01ms, 1.148ms, 12.788ms
Bytes In      [total, mean]                     924000, 154.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.992s, 59.991s, 892.199µs
Latencies     [min, mean, 50, 90, 95, 99, max]  442.841µs, 803.691µs, 807.231µs, 948.964µs, 998.139µs, 1.128ms, 5.978ms
Bytes In      [total, mean]                     961987, 160.33
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![http-plus.png](http-plus.png)
