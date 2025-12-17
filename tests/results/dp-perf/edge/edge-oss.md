# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: e8ee7c1c4f14e249927a5447a1af2615ddbe0f87
- Date: 2025-12-17T20:04:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         29997, 999.86, 999.83
Duration      [total, attack, wait]             30.002s, 30.001s, 989.477µs
Latencies     [min, mean, 50, 90, 95, 99, max]  696.421µs, 938.076µs, 919.396µs, 1.047ms, 1.095ms, 1.228ms, 12.602ms
Bytes In      [total, mean]                     4829517, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29997  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.20, 999.91
Duration      [total, attack, wait]             30.003s, 29.994s, 8.815ms
Latencies     [min, mean, 50, 90, 95, 99, max]  709.334µs, 988.676µs, 961.7µs, 1.099ms, 1.157ms, 1.365ms, 11.341ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.05, 1000.02
Duration      [total, attack, wait]             30s, 29.999s, 945.612µs
Latencies     [min, mean, 50, 90, 95, 99, max]  700.932µs, 984.229µs, 957.21µs, 1.093ms, 1.152ms, 1.327ms, 27.743ms
Bytes In      [total, mean]                     5100000, 170.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 1.039ms
Latencies     [min, mean, 50, 90, 95, 99, max]  724.983µs, 980.084µs, 957.757µs, 1.093ms, 1.149ms, 1.31ms, 23.78ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 981.355µs
Latencies     [min, mean, 50, 90, 95, 99, max]  712.774µs, 979.923µs, 955.273µs, 1.107ms, 1.161ms, 1.312ms, 19.623ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
