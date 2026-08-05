# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 394bdf0e0c8ae008009546b7a21d8f80248d52be
- Date: 2026-07-30T17:54:44Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1250000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 866.057µs
Latencies     [min, mean, 50, 90, 95, 99, max]  616.385µs, 854.969µs, 769.812µs, 1.011ms, 1.114ms, 1.776ms, 40.341ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 925.716µs
Latencies     [min, mean, 50, 90, 95, 99, max]  621.995µs, 1.054ms, 918.871µs, 1.241ms, 1.347ms, 3.004ms, 43.698ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 800.853µs
Latencies     [min, mean, 50, 90, 95, 99, max]  646.994µs, 1.009ms, 883.809µs, 1.203ms, 1.32ms, 3.247ms, 41.932ms
Bytes In      [total, mean]                     5010000, 167.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 998.51
Duration      [total, attack, wait]             30s, 29.999s, 992.202µs
Latencies     [min, mean, 50, 90, 95, 99, max]  144.307µs, 1.118ms, 900.901µs, 1.237ms, 1.363ms, 4.425ms, 209.639ms
Bytes In      [total, mean]                     4672980, 155.77
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.85%
Status Codes  [code:count]                      0:45  200:29955  
Error Set:
Get "http://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.85:80: connect: network is unreachable
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 866.962µs
Latencies     [min, mean, 50, 90, 95, 99, max]  599.312µs, 1.006ms, 894.106µs, 1.208ms, 1.319ms, 2.673ms, 21.446ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
