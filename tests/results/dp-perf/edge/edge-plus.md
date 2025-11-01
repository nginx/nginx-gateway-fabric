# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: b41c973c8399458984def3c2a8a268a237c864c8
- Date: 2025-10-30T03:04:40Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1162000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 833.602µs
Latencies     [min, mean, 50, 90, 95, 99, max]  679.176µs, 908.167µs, 879.785µs, 1.011ms, 1.069ms, 1.306ms, 24.313ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 885.413µs
Latencies     [min, mean, 50, 90, 95, 99, max]  735.321µs, 993.589µs, 965.051µs, 1.109ms, 1.179ms, 1.454ms, 26.207ms
Bytes In      [total, mean]                     4859838, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 1.588ms
Latencies     [min, mean, 50, 90, 95, 99, max]  728.765µs, 995.743µs, 964.788µs, 1.12ms, 1.205ms, 1.515ms, 22.473ms
Bytes In      [total, mean]                     5100000, 170.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 961.065µs
Latencies     [min, mean, 50, 90, 95, 99, max]  717.726µs, 952.076µs, 925.718µs, 1.072ms, 1.146ms, 1.407ms, 20.945ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 949.913µs
Latencies     [min, mean, 50, 90, 95, 99, max]  718.639µs, 953.232µs, 922.53µs, 1.067ms, 1.144ms, 1.41ms, 20.724ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
