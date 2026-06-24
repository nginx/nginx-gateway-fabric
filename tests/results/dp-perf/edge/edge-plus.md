# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.02
Duration      [total, attack, wait]             30s, 29.999s, 725.621µs
Latencies     [min, mean, 50, 90, 95, 99, max]  563.392µs, 743.072µs, 715.677µs, 820.009µs, 865.856µs, 1.004ms, 25.655ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 832.671µs
Latencies     [min, mean, 50, 90, 95, 99, max]  614.758µs, 793.488µs, 762.096µs, 859.609µs, 904.036µs, 1.076ms, 23.951ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 892.445µs
Latencies     [min, mean, 50, 90, 95, 99, max]  594.395µs, 794.415µs, 764.246µs, 866.486µs, 911.555µs, 1.073ms, 26.194ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 764.378µs
Latencies     [min, mean, 50, 90, 95, 99, max]  602.205µs, 789.449µs, 767.807µs, 871.845µs, 916.967µs, 1.068ms, 10.521ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 843.408µs
Latencies     [min, mean, 50, 90, 95, 99, max]  606.264µs, 789.713µs, 763.631µs, 869.027µs, 916.945µs, 1.087ms, 25.151ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
