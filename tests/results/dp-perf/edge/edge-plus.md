# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: e8ee7c1c4f14e249927a5447a1af2615ddbe0f87
- Date: 2025-12-17T20:04:03Z
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
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 799.991µs
Latencies     [min, mean, 50, 90, 95, 99, max]  744.28µs, 960.587µs, 941.628µs, 1.059ms, 1.108ms, 1.261ms, 12.133ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 973.367µs
Latencies     [min, mean, 50, 90, 95, 99, max]  735.636µs, 1.015ms, 987.343µs, 1.119ms, 1.171ms, 1.35ms, 23.905ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 1.008ms
Latencies     [min, mean, 50, 90, 95, 99, max]  790.037µs, 1.021ms, 1.006ms, 1.134ms, 1.188ms, 1.327ms, 8.71ms
Bytes In      [total, mean]                     5069831, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.002ms
Latencies     [min, mean, 50, 90, 95, 99, max]  763.343µs, 984.064µs, 963.558µs, 1.084ms, 1.136ms, 1.289ms, 15.862ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 858.956µs
Latencies     [min, mean, 50, 90, 95, 99, max]  755.767µs, 990.293µs, 964.537µs, 1.105ms, 1.159ms, 1.336ms, 16.043ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
