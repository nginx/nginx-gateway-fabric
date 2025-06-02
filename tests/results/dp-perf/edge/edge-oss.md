# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 82c22ddf1ede4e6e4d607a582a394be979ead4e0
- Date: 2025-06-02T15:32:21Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.32.4-gke.1106006
- vCPUs per node: 16
- RAM per node: 65851340Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 629.379µs
Latencies     [min, mean, 50, 90, 95, 99, max]  512.191µs, 675.783µs, 653.553µs, 747.846µs, 787.281µs, 910.85µs, 13.642ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.05, 1000.03
Duration      [total, attack, wait]             29.999s, 29.998s, 668.744µs
Latencies     [min, mean, 50, 90, 95, 99, max]  557.236µs, 715.159µs, 698.009µs, 795.413µs, 832.841µs, 952.422µs, 12.05ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 703.205µs
Latencies     [min, mean, 50, 90, 95, 99, max]  554.101µs, 722.577µs, 708.531µs, 807.967µs, 847.118µs, 966.919µs, 9.607ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 711.002µs
Latencies     [min, mean, 50, 90, 95, 99, max]  535.09µs, 709.849µs, 691.196µs, 787.75µs, 823.886µs, 942.194µs, 16.624ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 627.566µs
Latencies     [min, mean, 50, 90, 95, 99, max]  509.265µs, 703.513µs, 686.198µs, 780.509µs, 818.049µs, 928.346µs, 17.29ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
