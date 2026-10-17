# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 0042ca1a05b3e6ee63f08f0cf2ca8090a4f6a1e6
- Date: 2026-10-15T15:55:29Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.36.4-gke.1391002
- vCPUs per node: 16
- RAM per node: 65848244Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 707.712µs
Latencies     [min, mean, 50, 90, 95, 99, max]  565.031µs, 750.708µs, 731.277µs, 834.528µs, 876.829µs, 1.007ms, 18.019ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 770.71µs
Latencies     [min, mean, 50, 90, 95, 99, max]  586.563µs, 801.666µs, 780.206µs, 888.14µs, 927.964µs, 1.061ms, 20.343ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 920.585µs
Latencies     [min, mean, 50, 90, 95, 99, max]  604.671µs, 802.078µs, 774.532µs, 878.151µs, 917.818µs, 1.034ms, 21.896ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 834.524µs
Latencies     [min, mean, 50, 90, 95, 99, max]  594.229µs, 783.783µs, 762.508µs, 867.077µs, 905.869µs, 1.034ms, 17.51ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 867.834µs
Latencies     [min, mean, 50, 90, 95, 99, max]  587.681µs, 780.903µs, 757.498µs, 870.056µs, 910.526µs, 1.038ms, 20.035ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
