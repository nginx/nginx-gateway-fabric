# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.10
Duration      [total, attack, wait]             30s, 29.999s, 644.252µs
Latencies     [min, mean, 50, 90, 95, 99, max]  395.1µs, 692.071µs, 670.989µs, 783.951µs, 831.561µs, 981.286µs, 11.924ms
Bytes In      [total, mean]                     4800810, 160.03
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.91%
Status Codes  [code:count]                      200:29973  503:27  
Error Set:
503 Service Temporarily Unavailable
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 742.547µs
Latencies     [min, mean, 50, 90, 95, 99, max]  526.073µs, 723.557µs, 705.777µs, 821.771µs, 871.667µs, 1.03ms, 7.776ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 836.301µs
Latencies     [min, mean, 50, 90, 95, 99, max]  547.095µs, 729.646µs, 709.925µs, 824.774µs, 877.901µs, 1.048ms, 11.919ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 893.999µs
Latencies     [min, mean, 50, 90, 95, 99, max]  535.829µs, 739.818µs, 712.106µs, 835.35µs, 886.583µs, 1.042ms, 16.201ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 574.313µs
Latencies     [min, mean, 50, 90, 95, 99, max]  533.952µs, 708.295µs, 690.41µs, 792.688µs, 841.213µs, 982.776µs, 10.31ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
