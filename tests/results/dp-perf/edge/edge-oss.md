# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: d4376776aecc98294dc881a49cfbfa491773f74d
- Date: 2026-01-15T17:08:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2019000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 1.039ms
Latencies     [min, mean, 50, 90, 95, 99, max]  614.794µs, 950.334µs, 928.66µs, 1.101ms, 1.169ms, 1.38ms, 11.218ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 876.859µs
Latencies     [min, mean, 50, 90, 95, 99, max]  652.317µs, 998.055µs, 977.903µs, 1.159ms, 1.224ms, 1.389ms, 24.118ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 834.707µs
Latencies     [min, mean, 50, 90, 95, 99, max]  612.263µs, 990.316µs, 959.231µs, 1.139ms, 1.205ms, 1.397ms, 23.654ms
Bytes In      [total, mean]                     5100000, 170.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 975.302µs
Latencies     [min, mean, 50, 90, 95, 99, max]  625.001µs, 986.676µs, 960.96µs, 1.15ms, 1.221ms, 1.397ms, 24.372ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.056ms
Latencies     [min, mean, 50, 90, 95, 99, max]  604.923µs, 966.07µs, 942.639µs, 1.111ms, 1.173ms, 1.363ms, 25.54ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
