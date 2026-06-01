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

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.221ms
Latencies     [min, mean, 50, 90, 95, 99, max]  632.951µs, 1.106ms, 1.087ms, 1.277ms, 1.351ms, 1.736ms, 20.35ms
Bytes In      [total, mean]                     4806155, 160.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.064ms
Latencies     [min, mean, 50, 90, 95, 99, max]  686.327µs, 1.166ms, 1.144ms, 1.325ms, 1.393ms, 1.771ms, 23.142ms
Bytes In      [total, mean]                     4625909, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.073ms
Latencies     [min, mean, 50, 90, 95, 99, max]  744.769µs, 1.211ms, 1.182ms, 1.373ms, 1.441ms, 1.739ms, 55.81ms
Bytes In      [total, mean]                     7401484, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.021ms
Latencies     [min, mean, 50, 90, 95, 99, max]  676.519µs, 1.142ms, 1.122ms, 1.317ms, 1.387ms, 1.673ms, 46.894ms
Bytes In      [total, mean]                     7689736, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.302ms
Latencies     [min, mean, 50, 90, 95, 99, max]  752.051µs, 1.178ms, 1.149ms, 1.356ms, 1.428ms, 1.712ms, 71.491ms
Bytes In      [total, mean]                     1922387, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.282ms
Latencies     [min, mean, 50, 90, 95, 99, max]  730.872µs, 1.228ms, 1.192ms, 1.4ms, 1.475ms, 1.808ms, 71.53ms
Bytes In      [total, mean]                     1850416, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.409ms
Latencies     [min, mean, 50, 90, 95, 99, max]  790.794µs, 1.267ms, 1.251ms, 1.446ms, 1.503ms, 1.68ms, 23.848ms
Bytes In      [total, mean]                     1850381, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.263ms
Latencies     [min, mean, 50, 90, 95, 99, max]  708.429µs, 1.219ms, 1.208ms, 1.409ms, 1.474ms, 1.664ms, 5.814ms
Bytes In      [total, mean]                     1922447, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.212ms
Latencies     [min, mean, 50, 90, 95, 99, max]  716.815µs, 1.181ms, 1.143ms, 1.388ms, 1.487ms, 1.9ms, 28.96ms
Bytes In      [total, mean]                     4625937, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.402ms
Latencies     [min, mean, 50, 90, 95, 99, max]  671.585µs, 1.114ms, 1.084ms, 1.298ms, 1.379ms, 1.735ms, 56.877ms
Bytes In      [total, mean]                     4806007, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.251ms
Latencies     [min, mean, 50, 90, 95, 99, max]  748.525µs, 1.239ms, 1.198ms, 1.431ms, 1.542ms, 2.001ms, 87.126ms
Bytes In      [total, mean]                     14803124, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.109ms
Latencies     [min, mean, 50, 90, 95, 99, max]  629.663µs, 1.145ms, 1.119ms, 1.325ms, 1.398ms, 1.762ms, 87.973ms
Bytes In      [total, mean]                     15379345, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.312ms
Latencies     [min, mean, 50, 90, 95, 99, max]  788.972µs, 1.245ms, 1.226ms, 1.412ms, 1.469ms, 1.764ms, 11.665ms
Bytes In      [total, mean]                     1850334, 154.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.462ms
Latencies     [min, mean, 50, 90, 95, 99, max]  720.989µs, 1.174ms, 1.153ms, 1.383ms, 1.467ms, 1.764ms, 5.758ms
Bytes In      [total, mean]                     1922376, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.112ms
Latencies     [min, mean, 50, 90, 95, 99, max]  687.687µs, 1.158ms, 1.106ms, 1.269ms, 1.321ms, 1.456ms, 150.75ms
Bytes In      [total, mean]                     1922410, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.1ms
Latencies     [min, mean, 50, 90, 95, 99, max]  734.247µs, 1.212ms, 1.155ms, 1.304ms, 1.362ms, 1.533ms, 155.613ms
Bytes In      [total, mean]                     1850383, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
