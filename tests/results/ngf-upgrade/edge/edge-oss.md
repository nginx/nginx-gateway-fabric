# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.992s, 59.991s, 1.22ms
Latencies     [min, mean, 50, 90, 95, 99, max]  935.494µs, 1.253ms, 1.233ms, 1.412ms, 1.464ms, 1.598ms, 12.878ms
Bytes In      [total, mean]                     966000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![http-oss.png](http-oss.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.992s, 59.991s, 1.377ms
Latencies     [min, mean, 50, 90, 95, 99, max]  958.726µs, 1.311ms, 1.289ms, 1.418ms, 1.468ms, 1.602ms, 13.054ms
Bytes In      [total, mean]                     930000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![https-oss.png](https-oss.png)
