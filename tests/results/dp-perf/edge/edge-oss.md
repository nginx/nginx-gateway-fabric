# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 2f3153c547e0442fbb26aa9165118f4dc2b20f23
- Date: 2026-04-01T15:39:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848316Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 684.779µs
Latencies     [min, mean, 50, 90, 95, 99, max]  566.771µs, 783.513µs, 760.089µs, 882.079µs, 931.894µs, 1.097ms, 18.521ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 30s, 831.372µs
Latencies     [min, mean, 50, 90, 95, 99, max]  599.818µs, 827.024µs, 801.664µs, 926.609µs, 978.174µs, 1.161ms, 16.279ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 816.885µs
Latencies     [min, mean, 50, 90, 95, 99, max]  611.719µs, 840.526µs, 813.325µs, 952.08µs, 1.007ms, 1.17ms, 16.663ms
Bytes In      [total, mean]                     5010000, 167.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30s, 30s, 716.206µs
Latencies     [min, mean, 50, 90, 95, 99, max]  588.027µs, 828.322µs, 803.958µs, 932.971µs, 987.788µs, 1.159ms, 16.651ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 789.584µs
Latencies     [min, mean, 50, 90, 95, 99, max]  596.611µs, 830.635µs, 807.301µs, 936.061µs, 989.605µs, 1.142ms, 15.821ms
Bytes In      [total, mean]                     4680000, 156.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
