# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 09f31a0defdd4bf13c648139f55567bf908cfaac
- Date: 2026-04-15T14:59:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848324Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 741.773µs
Latencies     [min, mean, 50, 90, 95, 99, max]  456.611µs, 628.168µs, 600.948µs, 694.254µs, 735.553µs, 916.177µs, 27.118ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 30s, 599.108µs
Latencies     [min, mean, 50, 90, 95, 99, max]  499.323µs, 660.544µs, 632.793µs, 723.318µs, 763.36µs, 952.902µs, 22.46ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 599.836µs
Latencies     [min, mean, 50, 90, 95, 99, max]  500.692µs, 664.798µs, 635.179µs, 736.087µs, 781.001µs, 977.923µs, 22.599ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 646.798µs
Latencies     [min, mean, 50, 90, 95, 99, max]  477.19µs, 634.312µs, 603.476µs, 690.045µs, 732.273µs, 924.585µs, 24.203ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 574.116µs
Latencies     [min, mean, 50, 90, 95, 99, max]  475.98µs, 642.388µs, 611.323µs, 701.346µs, 742.312µs, 896.567µs, 22.789ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
