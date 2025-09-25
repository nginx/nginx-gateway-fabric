# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 8241478604f782eca497329ae47507b978d117b1
- Date: 2025-09-25T01:19:35Z
- Dirty: false

GKE Cluster:

- Node count: 15
- k8s version: v1.33.4-gke.1134000
- vCPUs per node: 2
- RAM per node: 4015672Ki
- Max pods per node: 110
- Zone: us-south1-a
- Instance Type: e2-medium

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 899.747µs
Latencies     [min, mean, 50, 90, 95, 99, max]  755.39µs, 1.083ms, 972.127µs, 1.139ms, 1.257ms, 3.409ms, 34.066ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 960.429µs
Latencies     [min, mean, 50, 90, 95, 99, max]  790.502µs, 1.086ms, 1.017ms, 1.192ms, 1.293ms, 2.776ms, 13.993ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 956.779µs
Latencies     [min, mean, 50, 90, 95, 99, max]  807.149µs, 1.174ms, 1.02ms, 1.204ms, 1.338ms, 4.79ms, 37.843ms
Bytes In      [total, mean]                     5100000, 170.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.032ms
Latencies     [min, mean, 50, 90, 95, 99, max]  773.587µs, 1.065ms, 1.006ms, 1.176ms, 1.264ms, 2.48ms, 20.731ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 937.91µs
Latencies     [min, mean, 50, 90, 95, 99, max]  805.184µs, 1.194ms, 1.011ms, 1.191ms, 1.312ms, 4.171ms, 58.715ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
