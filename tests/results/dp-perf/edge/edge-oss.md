# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fef4f734728239e3248cda180e74f72e436b06b9
- Date: 2026-08-14T15:07:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1258000
- vCPUs per node: 16
- RAM per node: 65848288Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.085ms
Latencies     [min, mean, 50, 90, 95, 99, max]  755.862µs, 1.005ms, 981.329µs, 1.125ms, 1.19ms, 1.348ms, 18.777ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 920.72µs
Latencies     [min, mean, 50, 90, 95, 99, max]  777.996µs, 1.032ms, 1.009ms, 1.145ms, 1.203ms, 1.372ms, 16.793ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.143ms
Latencies     [min, mean, 50, 90, 95, 99, max]  782.458µs, 1.037ms, 1.015ms, 1.153ms, 1.214ms, 1.382ms, 15.758ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 899.548µs
Latencies     [min, mean, 50, 90, 95, 99, max]  760.533µs, 1.034ms, 1.012ms, 1.162ms, 1.225ms, 1.404ms, 16.855ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 1.315ms
Latencies     [min, mean, 50, 90, 95, 99, max]  783.241µs, 1.031ms, 1.006ms, 1.144ms, 1.202ms, 1.384ms, 18.586ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
