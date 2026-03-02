# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: d97dd7debc1ea5d51f4413b6564b27921a1fc982
- Date: 2026-02-27T17:29:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.34.3-gke.1318000
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 997.90
Duration      [total, attack, wait]             30s, 29.999s, 1.037ms
Latencies     [min, mean, 50, 90, 95, 99, max]  144.325µs, 1.289ms, 925.133µs, 1.277ms, 1.421ms, 4.933ms, 219.662ms
Bytes In      [total, mean]                     4789920, 159.66
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.79%
Status Codes  [code:count]                      0:63  200:29937  
Error Set:
Get "http://cafe.example.com/latte": dial tcp 0.0.0.0:0->10.138.0.92:80: connect: network is unreachable
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.14, 1000.10
Duration      [total, attack, wait]             29.997s, 29.996s, 1.006ms
Latencies     [min, mean, 50, 90, 95, 99, max]  649.719µs, 1.118ms, 1.005ms, 1.323ms, 1.436ms, 3.71ms, 26.001ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 1.132ms
Latencies     [min, mean, 50, 90, 95, 99, max]  653.265µs, 1.137ms, 999.639µs, 1.322ms, 1.446ms, 4.393ms, 30.229ms
Bytes In      [total, mean]                     5069831, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 894.865µs
Latencies     [min, mean, 50, 90, 95, 99, max]  637.576µs, 1.082ms, 969.773µs, 1.302ms, 1.411ms, 3.264ms, 20.949ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 840.869µs
Latencies     [min, mean, 50, 90, 95, 99, max]  618.834µs, 1.166ms, 935.757µs, 1.288ms, 1.416ms, 6.682ms, 68.342ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
