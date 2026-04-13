# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 17c42c8bbbb004ba9c0e9b867396c5f8937207cd
- Date: 2026-04-01T18:33:47Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 743.702µs
Latencies     [min, mean, 50, 90, 95, 99, max]  555.863µs, 751.967µs, 726.646µs, 829.115µs, 872.833µs, 1.02ms, 15.845ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 30s, 754.353µs
Latencies     [min, mean, 50, 90, 95, 99, max]  610.735µs, 810.374µs, 787.442µs, 900.173µs, 945.802µs, 1.115ms, 13.581ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 779.118µs
Latencies     [min, mean, 50, 90, 95, 99, max]  613.449µs, 828.949µs, 792.013µs, 908.476µs, 956.712µs, 1.138ms, 18.407ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 803.992µs
Latencies     [min, mean, 50, 90, 95, 99, max]  596.305µs, 806.063µs, 780.953µs, 893.283µs, 939.78µs, 1.113ms, 17.21ms
Bytes In      [total, mean]                     4709843, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 642.122µs
Latencies     [min, mean, 50, 90, 95, 99, max]  593.681µs, 785.013µs, 763.313µs, 870.177µs, 916.03µs, 1.065ms, 15.34ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
