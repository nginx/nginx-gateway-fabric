# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 28d0224c5f1617ace603b72889b5bb7aa272ea20
- Date: 2026-06-01T17:32:15Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 923.382µs
Latencies     [min, mean, 50, 90, 95, 99, max]  637.442µs, 843.009µs, 822.743µs, 921.755µs, 962.557µs, 1.121ms, 20.151ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 807.906µs
Latencies     [min, mean, 50, 90, 95, 99, max]  668.975µs, 879.909µs, 853.618µs, 959.343µs, 1.006ms, 1.186ms, 20.065ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 954.424µs
Latencies     [min, mean, 50, 90, 95, 99, max]  686.422µs, 870.297µs, 842.123µs, 941.07µs, 986.522µs, 1.172ms, 21.443ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 775.631µs
Latencies     [min, mean, 50, 90, 95, 99, max]  674.231µs, 850.71µs, 821.549µs, 918.266µs, 958.646µs, 1.136ms, 30.615ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 869.59µs
Latencies     [min, mean, 50, 90, 95, 99, max]  661.756µs, 848.937µs, 829.335µs, 919.024µs, 958.181µs, 1.125ms, 18.178ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
