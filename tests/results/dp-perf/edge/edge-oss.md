# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 903211b7f256263c546d17dbbc037f7756f492e1
- Date: 2026-06-30T17:57:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1163012
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         29999, 999.84, 999.82
Duration      [total, attack, wait]             30.004s, 30.004s, 739.621µs
Latencies     [min, mean, 50, 90, 95, 99, max]  554.407µs, 1.04ms, 850.048µs, 1.202ms, 1.351ms, 3.325ms, 61.541ms
Bytes In      [total, mean]                     4799840, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.27, 1000.25
Duration      [total, attack, wait]             29.993s, 29.992s, 735.188µs
Latencies     [min, mean, 50, 90, 95, 99, max]  611.905µs, 941.885µs, 847.064µs, 1.129ms, 1.277ms, 2.006ms, 32.578ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 770.249µs
Latencies     [min, mean, 50, 90, 95, 99, max]  623.399µs, 817.204µs, 793.064µs, 895.931µs, 943.077µs, 1.116ms, 22.801ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 832.71µs
Latencies     [min, mean, 50, 90, 95, 99, max]  598.37µs, 827.996µs, 806.621µs, 918.391µs, 963.826µs, 1.124ms, 23.207ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 688.945µs
Latencies     [min, mean, 50, 90, 95, 99, max]  613.939µs, 829.616µs, 804.539µs, 918.956µs, 964.595µs, 1.138ms, 21.45ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
