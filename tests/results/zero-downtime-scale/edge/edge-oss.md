# Results

## Test environment

NGINX Plus: false

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

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.12ms
Latencies     [min, mean, 50, 90, 95, 99, max]  769.698µs, 1.21ms, 1.179ms, 1.37ms, 1.453ms, 1.877ms, 22.588ms
Bytes In      [total, mean]                     4625909, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 979.298µs
Latencies     [min, mean, 50, 90, 95, 99, max]  688.603µs, 1.13ms, 1.11ms, 1.286ms, 1.356ms, 1.846ms, 23.185ms
Bytes In      [total, mean]                     4806031, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.154ms
Latencies     [min, mean, 50, 90, 95, 99, max]  683.933µs, 1.164ms, 1.136ms, 1.335ms, 1.42ms, 1.858ms, 209.162ms
Bytes In      [total, mean]                     7689602, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.156ms
Latencies     [min, mean, 50, 90, 95, 99, max]  767.771µs, 1.223ms, 1.191ms, 1.394ms, 1.487ms, 1.936ms, 46.225ms
Bytes In      [total, mean]                     7401745, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.403ms
Latencies     [min, mean, 50, 90, 95, 99, max]  718.962µs, 1.177ms, 1.143ms, 1.411ms, 1.514ms, 1.826ms, 12.061ms
Bytes In      [total, mean]                     1922402, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.616ms
Latencies     [min, mean, 50, 90, 95, 99, max]  770.736µs, 1.221ms, 1.179ms, 1.455ms, 1.568ms, 1.882ms, 11.857ms
Bytes In      [total, mean]                     1850416, 154.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.085ms
Latencies     [min, mean, 50, 90, 95, 99, max]  773.551µs, 1.23ms, 1.181ms, 1.408ms, 1.516ms, 1.877ms, 69.073ms
Bytes In      [total, mean]                     1850405, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.13ms
Latencies     [min, mean, 50, 90, 95, 99, max]  719.529µs, 1.146ms, 1.123ms, 1.311ms, 1.398ms, 1.7ms, 27.193ms
Bytes In      [total, mean]                     1922416, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.189ms
Latencies     [min, mean, 50, 90, 95, 99, max]  740.305µs, 1.518ms, 1.165ms, 1.397ms, 1.519ms, 2.277ms, 391.406ms
Bytes In      [total, mean]                     4625872, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 899.064µs
Latencies     [min, mean, 50, 90, 95, 99, max]  647.193µs, 1.428ms, 1.096ms, 1.304ms, 1.391ms, 2.083ms, 383.899ms
Bytes In      [total, mean]                     4805834, 160.19
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.112ms
Latencies     [min, mean, 50, 90, 95, 99, max]  675.663µs, 1.174ms, 1.143ms, 1.36ms, 1.462ms, 1.892ms, 51.795ms
Bytes In      [total, mean]                     15379268, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.095ms
Latencies     [min, mean, 50, 90, 95, 99, max]  734.241µs, 1.22ms, 1.182ms, 1.38ms, 1.473ms, 1.967ms, 51.438ms
Bytes In      [total, mean]                     14803351, 154.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.316ms
Latencies     [min, mean, 50, 90, 95, 99, max]  775.759µs, 1.274ms, 1.199ms, 1.413ms, 1.496ms, 1.923ms, 152.5ms
Bytes In      [total, mean]                     1850371, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.005ms
Latencies     [min, mean, 50, 90, 95, 99, max]  739.542µs, 1.184ms, 1.134ms, 1.31ms, 1.373ms, 1.814ms, 151.992ms
Bytes In      [total, mean]                     1922419, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.311ms
Latencies     [min, mean, 50, 90, 95, 99, max]  797.434µs, 1.248ms, 1.222ms, 1.39ms, 1.456ms, 1.714ms, 32.785ms
Bytes In      [total, mean]                     1850340, 154.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.165ms
Latencies     [min, mean, 50, 90, 95, 99, max]  780.632µs, 1.206ms, 1.191ms, 1.363ms, 1.428ms, 1.639ms, 32.778ms
Bytes In      [total, mean]                     1922444, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
