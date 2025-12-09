# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851512Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 959.135µs
Latencies     [min, mean, 50, 90, 95, 99, max]  663.558µs, 875.826µs, 845.323µs, 958.333µs, 1.007ms, 1.194ms, 23.064ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 860.551µs
Latencies     [min, mean, 50, 90, 95, 99, max]  712.205µs, 923.729µs, 901.1µs, 1.02ms, 1.069ms, 1.227ms, 21.375ms
Bytes In      [total, mean]                     4829839, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 968.736µs
Latencies     [min, mean, 50, 90, 95, 99, max]  737.91µs, 952.257µs, 928.142µs, 1.05ms, 1.105ms, 1.292ms, 21.593ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 870.48µs
Latencies     [min, mean, 50, 90, 95, 99, max]  699.503µs, 896.1µs, 872.493µs, 987.672µs, 1.041ms, 1.214ms, 23.127ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 943.532µs
Latencies     [min, mean, 50, 90, 95, 99, max]  681.741µs, 906.971µs, 887.005µs, 998.855µs, 1.046ms, 1.198ms, 11.182ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
