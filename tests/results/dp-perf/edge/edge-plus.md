# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 896.156µs
Latencies     [min, mean, 50, 90, 95, 99, max]  578.615µs, 760.414µs, 731.174µs, 832.426µs, 875.55µs, 1.059ms, 31.777ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 789.726µs
Latencies     [min, mean, 50, 90, 95, 99, max]  621.682µs, 833.177µs, 790.437µs, 906.609µs, 961.076µs, 1.181ms, 209.401ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 760.575µs
Latencies     [min, mean, 50, 90, 95, 99, max]  626.749µs, 811.385µs, 787.33µs, 899.085µs, 949.574µs, 1.134ms, 18.601ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 762.83µs
Latencies     [min, mean, 50, 90, 95, 99, max]  604.419µs, 792.759µs, 771.772µs, 876.437µs, 924.553µs, 1.142ms, 17.615ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 741.925µs
Latencies     [min, mean, 50, 90, 95, 99, max]  611.321µs, 789.054µs, 769.921µs, 871.458µs, 918.39µs, 1.114ms, 17.511ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
