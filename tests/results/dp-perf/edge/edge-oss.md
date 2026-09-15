# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: a43969ce15ea40ce0548f0b187b16b9740113f82
- Date: 2026-09-15T14:01:26Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.7-gke.1222000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 858.115µs
Latencies     [min, mean, 50, 90, 95, 99, max]  698.412µs, 1.005ms, 955.565µs, 1.156ms, 1.242ms, 1.519ms, 13.575ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 950.818µs
Latencies     [min, mean, 50, 90, 95, 99, max]  761.835µs, 991.194µs, 967.716µs, 1.08ms, 1.13ms, 1.289ms, 20.004ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 953.56µs
Latencies     [min, mean, 50, 90, 95, 99, max]  757.396µs, 986.734µs, 964.27µs, 1.076ms, 1.125ms, 1.27ms, 16.513ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.06, 1000.03
Duration      [total, attack, wait]             29.999s, 29.998s, 979.072µs
Latencies     [min, mean, 50, 90, 95, 99, max]  736.919µs, 974.96µs, 959.722µs, 1.066ms, 1.109ms, 1.246ms, 9.583ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 924.799µs
Latencies     [min, mean, 50, 90, 95, 99, max]  726.894µs, 997.18µs, 978.317µs, 1.096ms, 1.148ms, 1.305ms, 17.526ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
