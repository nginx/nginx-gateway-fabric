# Results

## Test environment

NGINX Plus: true

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

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 933.92µs
Latencies     [min, mean, 50, 90, 95, 99, max]  696.807µs, 948.181µs, 918.062µs, 1.073ms, 1.136ms, 1.32ms, 21.007ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 864.164µs
Latencies     [min, mean, 50, 90, 95, 99, max]  741.461µs, 970.664µs, 946.574µs, 1.094ms, 1.16ms, 1.335ms, 13.499ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.07, 1000.04
Duration      [total, attack, wait]             29.999s, 29.998s, 854.97µs
Latencies     [min, mean, 50, 90, 95, 99, max]  715.183µs, 972.37µs, 945.664µs, 1.087ms, 1.148ms, 1.318ms, 14.577ms
Bytes In      [total, mean]                     4980000, 166.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 860.759µs
Latencies     [min, mean, 50, 90, 95, 99, max]  734.96µs, 971.915µs, 944.546µs, 1.101ms, 1.165ms, 1.342ms, 13.991ms
Bytes In      [total, mean]                     4650000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 930.014µs
Latencies     [min, mean, 50, 90, 95, 99, max]  728.848µs, 964.985µs, 934.806µs, 1.082ms, 1.143ms, 1.325ms, 17.795ms
Bytes In      [total, mean]                     4650000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
