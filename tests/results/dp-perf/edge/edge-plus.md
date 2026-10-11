# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: ea67c576e39284e758352dda72242024cc98fa7f
- Date: 2026-10-01T15:28:08Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.8-gke.1225000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 756.451µs
Latencies     [min, mean, 50, 90, 95, 99, max]  552.488µs, 750.449µs, 703.996µs, 795.335µs, 833.364µs, 981.991µs, 43.977ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 727.958µs
Latencies     [min, mean, 50, 90, 95, 99, max]  589.345µs, 785.294µs, 755.434µs, 856.137µs, 904.178µs, 1.109ms, 28.511ms
Bytes In      [total, mean]                     4829839, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 728.344µs
Latencies     [min, mean, 50, 90, 95, 99, max]  594.517µs, 772.424µs, 744.362µs, 847.315µs, 895.595µs, 1.071ms, 28.937ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 730.283µs
Latencies     [min, mean, 50, 90, 95, 99, max]  577.395µs, 752.595µs, 732.582µs, 827.216µs, 870.874µs, 1.017ms, 9.21ms
Bytes In      [total, mean]                     4739842, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 744.082µs
Latencies     [min, mean, 50, 90, 95, 99, max]  583.378µs, 765.395µs, 741.603µs, 840.602µs, 887.986µs, 1.048ms, 24.03ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
