# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 8da61831385ee3c0a93a3b1e346da6f14581b5fd
- Date: 2025-09-08T17:29:56Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 939.656µs
Latencies     [min, mean, 50, 90, 95, 99, max]  723.016µs, 997.844µs, 956.411µs, 1.086ms, 1.14ms, 1.358ms, 18.381ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 906.42µs
Latencies     [min, mean, 50, 90, 95, 99, max]  764.299µs, 1.021ms, 999.014µs, 1.138ms, 1.193ms, 1.352ms, 12.527ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 895.728µs
Latencies     [min, mean, 50, 90, 95, 99, max]  779.713µs, 1.015ms, 991.768µs, 1.124ms, 1.18ms, 1.352ms, 14.725ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 920.161µs
Latencies     [min, mean, 50, 90, 95, 99, max]  738.545µs, 997.263µs, 978.266µs, 1.102ms, 1.156ms, 1.302ms, 14.905ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         29999, 1000.00, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 944.175µs
Latencies     [min, mean, 50, 90, 95, 99, max]  755.263µs, 1.023ms, 996.356µs, 1.135ms, 1.191ms, 1.344ms, 21.057ms
Bytes In      [total, mean]                     4709843, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```
