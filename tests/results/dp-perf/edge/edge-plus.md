# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 635b3fcd6e643f4bd24ebbd4c901619a030c4bc0
- Date: 2025-09-15T17:56:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1036000
- vCPUs per node: 16
- RAM per node: 65851528Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 900.89µs
Latencies     [min, mean, 50, 90, 95, 99, max]  714.789µs, 966.238µs, 944.115µs, 1.062ms, 1.112ms, 1.285ms, 37.418ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 860.973µs
Latencies     [min, mean, 50, 90, 95, 99, max]  753.171µs, 970.828µs, 948.946µs, 1.067ms, 1.118ms, 1.295ms, 20.518ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 967.396µs
Latencies     [min, mean, 50, 90, 95, 99, max]  770.147µs, 988.786µs, 968.93µs, 1.085ms, 1.137ms, 1.289ms, 22.817ms
Bytes In      [total, mean]                     5010000, 167.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.021ms
Latencies     [min, mean, 50, 90, 95, 99, max]  725.58µs, 975.886µs, 954.237µs, 1.07ms, 1.121ms, 1.291ms, 21.906ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 881.157µs
Latencies     [min, mean, 50, 90, 95, 99, max]  740.614µs, 958.919µs, 938.262µs, 1.054ms, 1.105ms, 1.28ms, 19.591ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
