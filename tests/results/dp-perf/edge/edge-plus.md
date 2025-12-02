# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: dbebd9791cb7aa5e8d10735800f776fd516b06c3
- Date: 2025-12-02T17:38:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 1.074ms
Latencies     [min, mean, 50, 90, 95, 99, max]  711.88µs, 1.023ms, 995.258µs, 1.162ms, 1.228ms, 1.421ms, 28.835ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.051ms
Latencies     [min, mean, 50, 90, 95, 99, max]  766.426µs, 1.059ms, 1.025ms, 1.214ms, 1.292ms, 1.547ms, 24.365ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 960.633µs
Latencies     [min, mean, 50, 90, 95, 99, max]  773.459µs, 1.066ms, 1.031ms, 1.235ms, 1.317ms, 1.583ms, 19.627ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.328ms
Latencies     [min, mean, 50, 90, 95, 99, max]  757.909µs, 1.037ms, 1.009ms, 1.179ms, 1.248ms, 1.498ms, 21.292ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 932.151µs
Latencies     [min, mean, 50, 90, 95, 99, max]  766.748µs, 1.046ms, 1.021ms, 1.197ms, 1.262ms, 1.483ms, 11.743ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
