# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 89aee48bf6e660a828ffd32ca35fc7f52e358e00
- Date: 2025-12-12T20:04:38Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 845.184µs
Latencies     [min, mean, 50, 90, 95, 99, max]  693.423µs, 920.807µs, 895.018µs, 1.02ms, 1.074ms, 1.244ms, 31.952ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 868.585µs
Latencies     [min, mean, 50, 90, 95, 99, max]  734.673µs, 969.426µs, 944.002µs, 1.078ms, 1.132ms, 1.32ms, 18.236ms
Bytes In      [total, mean]                     4890000, 163.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 963.831µs
Latencies     [min, mean, 50, 90, 95, 99, max]  714.486µs, 967.797µs, 942.965µs, 1.085ms, 1.14ms, 1.315ms, 18.507ms
Bytes In      [total, mean]                     5130000, 171.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 890.472µs
Latencies     [min, mean, 50, 90, 95, 99, max]  711.296µs, 913.484µs, 890.957µs, 1.007ms, 1.054ms, 1.249ms, 22.525ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 865.442µs
Latencies     [min, mean, 50, 90, 95, 99, max]  708.989µs, 926.09µs, 903.755µs, 1.009ms, 1.052ms, 1.206ms, 17.261ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
