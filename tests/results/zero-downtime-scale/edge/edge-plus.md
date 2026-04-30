# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 0a0355272512a41825999765aa954a73cda2b7c0
- Date: 2026-04-29T21:32:59Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848324Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.203ms
Latencies     [min, mean, 50, 90, 95, 99, max]  607.842µs, 1.033ms, 1.018ms, 1.164ms, 1.22ms, 1.47ms, 14.045ms
Bytes In      [total, mean]                     4626078, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 994.763µs
Latencies     [min, mean, 50, 90, 95, 99, max]  581.064µs, 989.638µs, 977.684µs, 1.129ms, 1.186ms, 1.612ms, 12.694ms
Bytes In      [total, mean]                     4806035, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.071ms
Latencies     [min, mean, 50, 90, 95, 99, max]  571.689µs, 1.001ms, 991.89µs, 1.148ms, 1.201ms, 1.42ms, 35.091ms
Bytes In      [total, mean]                     7689455, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.13ms
Latencies     [min, mean, 50, 90, 95, 99, max]  617.365µs, 1.061ms, 1.044ms, 1.207ms, 1.267ms, 1.52ms, 34.605ms
Bytes In      [total, mean]                     7401665, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.093ms
Latencies     [min, mean, 50, 90, 95, 99, max]  590.779µs, 1.024ms, 1.004ms, 1.173ms, 1.227ms, 1.359ms, 59.473ms
Bytes In      [total, mean]                     1922386, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.187ms
Latencies     [min, mean, 50, 90, 95, 99, max]  687.798µs, 1.101ms, 1.078ms, 1.237ms, 1.292ms, 1.44ms, 59.553ms
Bytes In      [total, mean]                     1850501, 154.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.199ms
Latencies     [min, mean, 50, 90, 95, 99, max]  613.1µs, 1.049ms, 1.041ms, 1.208ms, 1.264ms, 1.423ms, 5.244ms
Bytes In      [total, mean]                     1922387, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.138ms
Latencies     [min, mean, 50, 90, 95, 99, max]  687.586µs, 1.122ms, 1.108ms, 1.275ms, 1.329ms, 1.49ms, 22.844ms
Bytes In      [total, mean]                     1850416, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.298ms
Latencies     [min, mean, 50, 90, 95, 99, max]  627.582µs, 1.036ms, 1.012ms, 1.186ms, 1.261ms, 1.67ms, 16.896ms
Bytes In      [total, mean]                     4629076, 154.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.13ms
Latencies     [min, mean, 50, 90, 95, 99, max]  561.048µs, 979.946µs, 961.912µs, 1.134ms, 1.2ms, 1.639ms, 16.652ms
Bytes In      [total, mean]                     4808923, 160.30
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.095ms
Latencies     [min, mean, 50, 90, 95, 99, max]  613.396µs, 1.104ms, 1.077ms, 1.251ms, 1.32ms, 1.699ms, 74.97ms
Bytes In      [total, mean]                     14812920, 154.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 912.956µs
Latencies     [min, mean, 50, 90, 95, 99, max]  588.94µs, 1.057ms, 1.035ms, 1.211ms, 1.276ms, 1.633ms, 56.595ms
Bytes In      [total, mean]                     15388944, 160.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.311ms
Latencies     [min, mean, 50, 90, 95, 99, max]  630.542µs, 1.135ms, 1.079ms, 1.248ms, 1.308ms, 1.591ms, 114.871ms
Bytes In      [total, mean]                     1851515, 154.29
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 806.646µs
Latencies     [min, mean, 50, 90, 95, 99, max]  649.012µs, 1.072ms, 1.033ms, 1.19ms, 1.25ms, 1.543ms, 115.126ms
Bytes In      [total, mean]                     1923615, 160.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 840.435µs
Latencies     [min, mean, 50, 90, 95, 99, max]  677.614µs, 1.136ms, 1.114ms, 1.301ms, 1.379ms, 1.568ms, 18.134ms
Bytes In      [total, mean]                     1851676, 154.31
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 985.729µs
Latencies     [min, mean, 50, 90, 95, 99, max]  670.853µs, 1.083ms, 1.067ms, 1.252ms, 1.333ms, 1.556ms, 17.785ms
Bytes In      [total, mean]                     1923628, 160.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
