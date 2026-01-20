# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fe52764cce240bb5a3713f56aa113694c2793f93
- Date: 2026-01-20T16:40:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2072000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 731.624µs
Latencies     [min, mean, 50, 90, 95, 99, max]  531.616µs, 737.2µs, 710.562µs, 824.769µs, 870.982µs, 1.027ms, 31.713ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 726.014µs
Latencies     [min, mean, 50, 90, 95, 99, max]  563.089µs, 764.535µs, 741.428µs, 849.233µs, 892.246µs, 1.055ms, 20.25ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 1.015ms
Latencies     [min, mean, 50, 90, 95, 99, max]  577.693µs, 781.693µs, 751.151µs, 874.866µs, 923.617µs, 1.101ms, 26.653ms
Bytes In      [total, mean]                     5100000, 170.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 799.426µs
Latencies     [min, mean, 50, 90, 95, 99, max]  548.53µs, 778.66µs, 753.047µs, 876.101µs, 926.148µs, 1.086ms, 22.245ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 705.237µs
Latencies     [min, mean, 50, 90, 95, 99, max]  567.331µs, 784.11µs, 749.346µs, 864.213µs, 913.142µs, 1.091ms, 24.993ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
