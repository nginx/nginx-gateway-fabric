# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 802463fe5e69899eeab05e3bc9324020cd00b01c
- Date: 2025-09-10T10:26:37Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 918.639µs
Latencies     [min, mean, 50, 90, 95, 99, max]  673.206µs, 925.895µs, 895.334µs, 1.008ms, 1.055ms, 1.256ms, 13.34ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 813.28µs
Latencies     [min, mean, 50, 90, 95, 99, max]  737.933µs, 960.914µs, 940.491µs, 1.056ms, 1.1ms, 1.267ms, 16.083ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 862.908µs
Latencies     [min, mean, 50, 90, 95, 99, max]  734.073µs, 956.723µs, 935.737µs, 1.059ms, 1.108ms, 1.269ms, 14.258ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 992.838µs
Latencies     [min, mean, 50, 90, 95, 99, max]  705.441µs, 941.726µs, 922.163µs, 1.035ms, 1.079ms, 1.242ms, 15.043ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 932.373µs
Latencies     [min, mean, 50, 90, 95, 99, max]  725.605µs, 940.878µs, 924.263µs, 1.036ms, 1.08ms, 1.236ms, 13.064ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
