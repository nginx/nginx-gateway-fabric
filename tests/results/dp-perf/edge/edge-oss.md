# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 9a7a618dab5ed0eee09063de60d80bf0fb76900a
- Date: 2025-02-14T18:44:35Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1023000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         29999, 1000.00, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 635.452µs
Latencies     [min, mean, 50, 90, 95, 99, max]  529.578µs, 706.838µs, 696.335µs, 788.202µs, 825.197µs, 923.488µs, 11.514ms
Bytes In      [total, mean]                     4769841, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 828.849µs
Latencies     [min, mean, 50, 90, 95, 99, max]  558.084µs, 738.48µs, 727.381µs, 816.715µs, 849.889µs, 945.865µs, 9.375ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 672.418µs
Latencies     [min, mean, 50, 90, 95, 99, max]  555.408µs, 741.275µs, 731.14µs, 820.731µs, 853.3µs, 954.583µs, 12.046ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 791.291µs
Latencies     [min, mean, 50, 90, 95, 99, max]  543.174µs, 736.406µs, 723.73µs, 815.265µs, 848.456µs, 950.192µs, 13.544ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 877.063µs
Latencies     [min, mean, 50, 90, 95, 99, max]  565.417µs, 732.16µs, 718.793µs, 812.684µs, 845.717µs, 945.736µs, 16.524ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
