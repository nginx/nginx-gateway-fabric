# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: d97dd7debc1ea5d51f4413b6564b27921a1fc982
- Date: 2026-02-27T17:29:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.34.3-gke.1318000
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.64
Duration      [total, attack, wait]             5m0s, 5m0s, 953.148µs
Latencies     [min, mean, 50, 90, 95, 99, max]  535.015µs, 1.014ms, 999.157µs, 1.189ms, 1.254ms, 1.627ms, 17.111ms
Bytes In      [total, mean]                     4818357, 160.61
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.63%
Status Codes  [code:count]                      0:110  200:29890  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.90:80: connect: connection refused
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.64
Duration      [total, attack, wait]             5m0s, 5m0s, 1.025ms
Latencies     [min, mean, 50, 90, 95, 99, max]  484.878µs, 1.053ms, 1.03ms, 1.232ms, 1.306ms, 1.669ms, 12.226ms
Bytes In      [total, mean]                     4635992, 154.53
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.63%
Status Codes  [code:count]                      0:110  200:29890  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.90:443: connect: connection refused
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 830.941µs
Latencies     [min, mean, 50, 90, 95, 99, max]  553.711µs, 1.046ms, 1.03ms, 1.223ms, 1.29ms, 1.524ms, 46.662ms
Bytes In      [total, mean]                     7737647, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.191ms
Latencies     [min, mean, 50, 90, 95, 99, max]  642.939µs, 1.097ms, 1.076ms, 1.279ms, 1.352ms, 1.625ms, 43.853ms
Bytes In      [total, mean]                     7444824, 155.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 817.625µs
Latencies     [min, mean, 50, 90, 95, 99, max]  542.895µs, 1.011ms, 994.951µs, 1.173ms, 1.237ms, 1.418ms, 69.411ms
Bytes In      [total, mean]                     1934398, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.336ms
Latencies     [min, mean, 50, 90, 95, 99, max]  633.232µs, 1.07ms, 1.047ms, 1.209ms, 1.265ms, 1.426ms, 80.317ms
Bytes In      [total, mean]                     1861188, 155.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.463ms
Latencies     [min, mean, 50, 90, 95, 99, max]  588.463µs, 1.035ms, 1.028ms, 1.208ms, 1.266ms, 1.433ms, 28.922ms
Bytes In      [total, mean]                     1934426, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.189ms
Latencies     [min, mean, 50, 90, 95, 99, max]  660.223µs, 1.063ms, 1.049ms, 1.212ms, 1.261ms, 1.398ms, 29.128ms
Bytes In      [total, mean]                     1861201, 155.10
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.08ms
Latencies     [min, mean, 50, 90, 95, 99, max]  605.249µs, 1.088ms, 1.061ms, 1.25ms, 1.321ms, 1.848ms, 51.81ms
Bytes In      [total, mean]                     4652960, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.208ms
Latencies     [min, mean, 50, 90, 95, 99, max]  580.127µs, 1ms, 981.101µs, 1.149ms, 1.211ms, 1.599ms, 29.263ms
Bytes In      [total, mean]                     4836030, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.055ms
Latencies     [min, mean, 50, 90, 95, 99, max]  574.445µs, 1.044ms, 1.01ms, 1.202ms, 1.268ms, 1.597ms, 163.347ms
Bytes In      [total, mean]                     15475203, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.257ms
Latencies     [min, mean, 50, 90, 95, 99, max]  614.967µs, 1.121ms, 1.075ms, 1.277ms, 1.346ms, 1.688ms, 169.971ms
Bytes In      [total, mean]                     14889488, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 956.111µs
Latencies     [min, mean, 50, 90, 95, 99, max]  644.737µs, 1.094ms, 1.045ms, 1.202ms, 1.259ms, 1.631ms, 109.183ms
Bytes In      [total, mean]                     1861219, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.444ms
Latencies     [min, mean, 50, 90, 95, 99, max]  590.342µs, 1.042ms, 1.003ms, 1.174ms, 1.228ms, 1.555ms, 108.563ms
Bytes In      [total, mean]                     1934470, 161.21
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.533ms
Latencies     [min, mean, 50, 90, 95, 99, max]  651.065µs, 1.106ms, 1.088ms, 1.274ms, 1.345ms, 1.595ms, 9.818ms
Bytes In      [total, mean]                     1861226, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.372ms
Latencies     [min, mean, 50, 90, 95, 99, max]  583.986µs, 1.052ms, 1.042ms, 1.225ms, 1.29ms, 1.527ms, 3.805ms
Bytes In      [total, mean]                     1934393, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
