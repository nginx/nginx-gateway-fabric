# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 28d0224c5f1617ace603b72889b5bb7aa272ea20
- Date: 2026-06-01T17:32:15Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 869.446µs
Latencies     [min, mean, 50, 90, 95, 99, max]  658.404µs, 885.684µs, 861.192µs, 967.882µs, 1.012ms, 1.203ms, 29.664ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 860.332µs
Latencies     [min, mean, 50, 90, 95, 99, max]  703.562µs, 942.74µs, 912.796µs, 1.028ms, 1.074ms, 1.244ms, 26.946ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 909.395µs
Latencies     [min, mean, 50, 90, 95, 99, max]  722.514µs, 943.279µs, 915.309µs, 1.039ms, 1.093ms, 1.261ms, 27.833ms
Bytes In      [total, mean]                     5100000, 170.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 929.62µs
Latencies     [min, mean, 50, 90, 95, 99, max]  692.606µs, 915.593µs, 892.451µs, 1.001ms, 1.046ms, 1.195ms, 13.413ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 890.053µs
Latencies     [min, mean, 50, 90, 95, 99, max]  708.964µs, 927.564µs, 904.043µs, 1.023ms, 1.068ms, 1.237ms, 23.682ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
