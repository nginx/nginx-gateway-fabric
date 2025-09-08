# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 8da61831385ee3c0a93a3b1e346da6f14581b5fd
- Date: 2025-09-08T17:29:56Z
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
Requests      [total, rate, throughput]         30000, 1000.06, 1000.03
Duration      [total, attack, wait]             29.999s, 29.998s, 819.961µs
Latencies     [min, mean, 50, 90, 95, 99, max]  667.855µs, 879.274µs, 830.804µs, 931.361µs, 973.936µs, 1.181ms, 22.836ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 885.212µs
Latencies     [min, mean, 50, 90, 95, 99, max]  709.202µs, 894.505µs, 876.458µs, 969.646µs, 1.008ms, 1.151ms, 15.125ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 922.026µs
Latencies     [min, mean, 50, 90, 95, 99, max]  714.367µs, 908.312µs, 887.952µs, 980.782µs, 1.019ms, 1.169ms, 13.995ms
Bytes In      [total, mean]                     5010000, 167.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 809.344µs
Latencies     [min, mean, 50, 90, 95, 99, max]  711.052µs, 890.469µs, 865.617µs, 952.378µs, 990.499µs, 1.144ms, 23.111ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 890.845µs
Latencies     [min, mean, 50, 90, 95, 99, max]  715.155µs, 887.946µs, 872.331µs, 963.589µs, 1.001ms, 1.142ms, 14.242ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
