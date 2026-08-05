# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 394bdf0e0c8ae008009546b7a21d8f80248d52be
- Date: 2026-07-30T17:54:44Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1250000
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 860.098µs
Latencies     [min, mean, 50, 90, 95, 99, max]  710.402µs, 901.802µs, 883.664µs, 969.922µs, 1.006ms, 1.146ms, 15.326ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.065ms
Latencies     [min, mean, 50, 90, 95, 99, max]  777.55µs, 955.139µs, 934.172µs, 1.025ms, 1.062ms, 1.212ms, 15.313ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 977.912µs
Latencies     [min, mean, 50, 90, 95, 99, max]  766.208µs, 957.407µs, 938.259µs, 1.033ms, 1.074ms, 1.214ms, 15.763ms
Bytes In      [total, mean]                     5010000, 167.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.023ms
Latencies     [min, mean, 50, 90, 95, 99, max]  760.09µs, 937.546µs, 918.089µs, 1.009ms, 1.045ms, 1.186ms, 16.048ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 976.189µs
Latencies     [min, mean, 50, 90, 95, 99, max]  755.582µs, 941.502µs, 921.298µs, 1.016ms, 1.054ms, 1.216ms, 14.881ms
Bytes In      [total, mean]                     4679844, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```
