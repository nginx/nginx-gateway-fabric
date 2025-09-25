# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 8241478604f782eca497329ae47507b978d117b1
- Date: 2025-09-24T18:19:40Z
- Dirty: false

GKE Cluster:

- Node count: 15
- k8s version: v1.33.4-gke.1134000
- vCPUs per node: 2
- RAM per node: 4015668Ki
- Max pods per node: 110
- Zone: us-east1-b
- Instance Type: e2-medium

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.97
Duration      [total, attack, wait]             30.001s, 29.999s, 1.799ms
Latencies     [min, mean, 50, 90, 95, 99, max]  909.791µs, 1.868ms, 1.507ms, 2.142ms, 2.756ms, 8.611ms, 60.614ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.95
Duration      [total, attack, wait]             30.002s, 30s, 1.712ms
Latencies     [min, mean, 50, 90, 95, 99, max]  991.792µs, 2.102ms, 1.659ms, 2.466ms, 3.321ms, 11.629ms, 73.799ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 999.98
Duration      [total, attack, wait]             30.001s, 29.999s, 1.599ms
Latencies     [min, mean, 50, 90, 95, 99, max]  964.301µs, 2.088ms, 1.589ms, 2.194ms, 2.891ms, 12.188ms, 77.709ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.96
Duration      [total, attack, wait]             30.001s, 30s, 1.373ms
Latencies     [min, mean, 50, 90, 95, 99, max]  935.247µs, 1.88ms, 1.503ms, 2.006ms, 2.429ms, 11.029ms, 73.867ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 1.329ms
Latencies     [min, mean, 50, 90, 95, 99, max]  981.259µs, 1.759ms, 1.535ms, 1.991ms, 2.43ms, 7.522ms, 29.702ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
