# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.546ms
Latencies     [min, mean, 50, 90, 95, 99, max]  587.726µs, 1.12ms, 1.103ms, 1.334ms, 1.425ms, 1.697ms, 21.048ms
Bytes In      [total, mean]                     4806046, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.788ms
Latencies     [min, mean, 50, 90, 95, 99, max]  615.769µs, 1.17ms, 1.148ms, 1.368ms, 1.468ms, 1.765ms, 18.457ms
Bytes In      [total, mean]                     4626010, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.158ms
Latencies     [min, mean, 50, 90, 95, 99, max]  589.181µs, 1.116ms, 1.103ms, 1.327ms, 1.413ms, 1.676ms, 37.904ms
Bytes In      [total, mean]                     7689567, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.332ms
Latencies     [min, mean, 50, 90, 95, 99, max]  654.366µs, 1.196ms, 1.173ms, 1.425ms, 1.535ms, 1.82ms, 26.789ms
Bytes In      [total, mean]                     7401530, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.45ms
Latencies     [min, mean, 50, 90, 95, 99, max]  653.711µs, 1.186ms, 1.156ms, 1.437ms, 1.556ms, 1.818ms, 10.398ms
Bytes In      [total, mean]                     1850498, 154.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.249ms
Latencies     [min, mean, 50, 90, 95, 99, max]  612.714µs, 1.108ms, 1.097ms, 1.335ms, 1.415ms, 1.624ms, 3.361ms
Bytes In      [total, mean]                     1922475, 160.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.205ms
Latencies     [min, mean, 50, 90, 95, 99, max]  650.443µs, 1.165ms, 1.145ms, 1.34ms, 1.404ms, 1.579ms, 62.355ms
Bytes In      [total, mean]                     1850362, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 931.018µs
Latencies     [min, mean, 50, 90, 95, 99, max]  592.675µs, 1.094ms, 1.082ms, 1.28ms, 1.338ms, 1.489ms, 59.718ms
Bytes In      [total, mean]                     1922434, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.061ms
Latencies     [min, mean, 50, 90, 95, 99, max]  605.291µs, 1.164ms, 1.145ms, 1.39ms, 1.478ms, 1.754ms, 24.541ms
Bytes In      [total, mean]                     4806034, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.388ms
Latencies     [min, mean, 50, 90, 95, 99, max]  688.572µs, 1.222ms, 1.197ms, 1.431ms, 1.52ms, 1.818ms, 20.85ms
Bytes In      [total, mean]                     4626050, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.081ms
Latencies     [min, mean, 50, 90, 95, 99, max]  662.635µs, 1.223ms, 1.199ms, 1.434ms, 1.531ms, 1.813ms, 54.988ms
Bytes In      [total, mean]                     14803215, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.27ms
Latencies     [min, mean, 50, 90, 95, 99, max]  604.814µs, 1.147ms, 1.129ms, 1.362ms, 1.446ms, 1.702ms, 52.075ms
Bytes In      [total, mean]                     15379280, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.24ms
Latencies     [min, mean, 50, 90, 95, 99, max]  630.681µs, 1.134ms, 1.125ms, 1.342ms, 1.414ms, 1.651ms, 4.496ms
Bytes In      [total, mean]                     1922408, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.046ms
Latencies     [min, mean, 50, 90, 95, 99, max]  653.141µs, 1.194ms, 1.171ms, 1.405ms, 1.494ms, 1.753ms, 12.191ms
Bytes In      [total, mean]                     1850384, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.329ms
Latencies     [min, mean, 50, 90, 95, 99, max]  679.895µs, 1.294ms, 1.226ms, 1.439ms, 1.516ms, 1.776ms, 124.639ms
Bytes In      [total, mean]                     1850386, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.044ms
Latencies     [min, mean, 50, 90, 95, 99, max]  625.359µs, 1.194ms, 1.149ms, 1.36ms, 1.432ms, 1.621ms, 41.056ms
Bytes In      [total, mean]                     1922339, 160.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
