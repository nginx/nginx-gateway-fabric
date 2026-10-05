# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: ea67c576e39284e758352dda72242024cc98fa7f
- Date: 2026-10-01T15:28:08Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.8-gke.1225000
- vCPUs per node: 16
- RAM per node: 65848288Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 762.632µs
Latencies     [min, mean, 50, 90, 95, 99, max]  579.427µs, 770.339µs, 738.765µs, 831.688µs, 874.957µs, 1.028ms, 42.856ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 916.474µs
Latencies     [min, mean, 50, 90, 95, 99, max]  630.461µs, 811.645µs, 785.132µs, 882.517µs, 931.693µs, 1.113ms, 27.901ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 700.017µs
Latencies     [min, mean, 50, 90, 95, 99, max]  607.47µs, 811.782µs, 782.378µs, 884.052µs, 930.531µs, 1.1ms, 21.486ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 883.396µs
Latencies     [min, mean, 50, 90, 95, 99, max]  618.337µs, 807.411µs, 783.451µs, 886.789µs, 935.691µs, 1.091ms, 25.741ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 748.688µs
Latencies     [min, mean, 50, 90, 95, 99, max]  618.476µs, 794.655µs, 768.385µs, 861.669µs, 904.662µs, 1.069ms, 25.065ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
