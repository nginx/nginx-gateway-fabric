# Results

## Test environment

NGINX Plus: false

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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 862.731µs
Latencies     [min, mean, 50, 90, 95, 99, max]  718.96µs, 1.008ms, 945.774µs, 1.074ms, 1.128ms, 1.799ms, 14.817ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 903.039µs
Latencies     [min, mean, 50, 90, 95, 99, max]  768.916µs, 1.006ms, 978.237µs, 1.107ms, 1.163ms, 1.325ms, 17.745ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 961.769µs
Latencies     [min, mean, 50, 90, 95, 99, max]  764.992µs, 1.004ms, 976.515µs, 1.099ms, 1.146ms, 1.308ms, 20.72ms
Bytes In      [total, mean]                     5010000, 167.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.106ms
Latencies     [min, mean, 50, 90, 95, 99, max]  741.022µs, 1.004ms, 979.882µs, 1.104ms, 1.15ms, 1.295ms, 18.213ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 889.133µs
Latencies     [min, mean, 50, 90, 95, 99, max]  757.712µs, 1.003ms, 983.285µs, 1.107ms, 1.155ms, 1.288ms, 17.607ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
