# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.042ms
Latencies     [min, mean, 50, 90, 95, 99, max]  727.59µs, 1.019ms, 989.307µs, 1.159ms, 1.229ms, 1.421ms, 17.801ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 1.062ms
Latencies     [min, mean, 50, 90, 95, 99, max]  773.849µs, 1.051ms, 1.023ms, 1.198ms, 1.268ms, 1.464ms, 13.51ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 995.265µs
Latencies     [min, mean, 50, 90, 95, 99, max]  748.429µs, 1.059ms, 1.024ms, 1.208ms, 1.284ms, 1.484ms, 23.32ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.044ms
Latencies     [min, mean, 50, 90, 95, 99, max]  786.944µs, 1.049ms, 1.02ms, 1.2ms, 1.27ms, 1.476ms, 14.461ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.016ms
Latencies     [min, mean, 50, 90, 95, 99, max]  789.745µs, 1.063ms, 1.027ms, 1.191ms, 1.263ms, 1.481ms, 21.502ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
