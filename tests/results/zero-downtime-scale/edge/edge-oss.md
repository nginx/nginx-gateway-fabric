# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGF Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.075ms
Latencies     [min, mean, 50, 90, 95, 99, max]  458.603µs, 919.819µs, 900.857µs, 1.059ms, 1.122ms, 1.434ms, 16.602ms
Bytes In      [total, mean]                     4595960, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 735.586µs
Latencies     [min, mean, 50, 90, 95, 99, max]  430.549µs, 889.346µs, 876.966µs, 1.03ms, 1.092ms, 1.385ms, 12.401ms
Bytes In      [total, mean]                     4775983, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 957.432µs
Latencies     [min, mean, 50, 90, 95, 99, max]  452.453µs, 918.266µs, 907.115µs, 1.059ms, 1.118ms, 1.384ms, 16.269ms
Bytes In      [total, mean]                     7353559, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 829.133µs
Latencies     [min, mean, 50, 90, 95, 99, max]  450.963µs, 889.508µs, 886.373µs, 1.031ms, 1.083ms, 1.352ms, 12.009ms
Bytes In      [total, mean]                     7641702, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 709.347µs
Latencies     [min, mean, 50, 90, 95, 99, max]  438.654µs, 875.603µs, 879.177µs, 1.001ms, 1.042ms, 1.199ms, 11.304ms
Bytes In      [total, mean]                     1910399, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 973.85µs
Latencies     [min, mean, 50, 90, 95, 99, max]  498.406µs, 929.312µs, 921.674µs, 1.062ms, 1.115ms, 1.325ms, 14.014ms
Bytes In      [total, mean]                     1838335, 153.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 949.176µs
Latencies     [min, mean, 50, 90, 95, 99, max]  474.29µs, 920.202µs, 920.503µs, 1.061ms, 1.108ms, 1.259ms, 6.701ms
Bytes In      [total, mean]                     1838425, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 971.028µs
Latencies     [min, mean, 50, 90, 95, 99, max]  442.662µs, 882.699µs, 886.841µs, 1.024ms, 1.068ms, 1.197ms, 11.974ms
Bytes In      [total, mean]                     1910400, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

## Multiple NGF Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 610.147µs
Latencies     [min, mean, 50, 90, 95, 99, max]  452.039µs, 898.58µs, 882.165µs, 1.037ms, 1.1ms, 1.45ms, 12.341ms
Bytes In      [total, mean]                     4617080, 153.90
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 782.953µs
Latencies     [min, mean, 50, 90, 95, 99, max]  434.457µs, 867.498µs, 856.092µs, 1.002ms, 1.059ms, 1.366ms, 13.872ms
Bytes In      [total, mean]                     4778851, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 807.366µs
Latencies     [min, mean, 50, 90, 95, 99, max]  402.775µs, 859.534µs, 854.993µs, 995.71µs, 1.051ms, 1.328ms, 12.297ms
Bytes In      [total, mean]                     15292926, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 982.611µs
Latencies     [min, mean, 50, 90, 95, 99, max]  437.996µs, 890.67µs, 878.228µs, 1.032ms, 1.095ms, 1.369ms, 15.588ms
Bytes In      [total, mean]                     14774458, 153.90
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 847.891µs
Latencies     [min, mean, 50, 90, 95, 99, max]  465.533µs, 893.781µs, 882.321µs, 1.024ms, 1.081ms, 1.321ms, 13.454ms
Bytes In      [total, mean]                     1846795, 153.90
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.156ms
Latencies     [min, mean, 50, 90, 95, 99, max]  446.636µs, 860.82µs, 857.47µs, 990.948µs, 1.039ms, 1.28ms, 11.666ms
Bytes In      [total, mean]                     1911593, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 843.596µs
Latencies     [min, mean, 50, 90, 95, 99, max]  468.558µs, 899.097µs, 897.551µs, 1.032ms, 1.077ms, 1.228ms, 7.476ms
Bytes In      [total, mean]                     1911589, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 841.763µs
Latencies     [min, mean, 50, 90, 95, 99, max]  530.242µs, 933.635µs, 928.814µs, 1.065ms, 1.116ms, 1.274ms, 8.782ms
Bytes In      [total, mean]                     1846810, 153.90
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
