# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 9010072ecd34a8fa99bfdd3d7580c9d725fb063e
- Date: 2025-10-01T09:39:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1172000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 916.575µs
Latencies     [min, mean, 50, 90, 95, 99, max]  691.405µs, 906.279µs, 883.274µs, 995.919µs, 1.041ms, 1.217ms, 26.291ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 893.576µs
Latencies     [min, mean, 50, 90, 95, 99, max]  716.153µs, 951.76µs, 928.081µs, 1.044ms, 1.094ms, 1.271ms, 24.711ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 923.767µs
Latencies     [min, mean, 50, 90, 95, 99, max]  734.163µs, 968.693µs, 939.083µs, 1.072ms, 1.134ms, 1.299ms, 31.453ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 928.615µs
Latencies     [min, mean, 50, 90, 95, 99, max]  705.267µs, 953.188µs, 924.739µs, 1.053ms, 1.112ms, 1.313ms, 23.944ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 903.068µs
Latencies     [min, mean, 50, 90, 95, 99, max]  709.536µs, 932.359µs, 902.638µs, 1.019ms, 1.066ms, 1.234ms, 23.801ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
