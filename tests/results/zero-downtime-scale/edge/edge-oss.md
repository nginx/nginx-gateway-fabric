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

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.162ms
Latencies     [min, mean, 50, 90, 95, 99, max]  652.925µs, 1.062ms, 1.051ms, 1.205ms, 1.258ms, 1.622ms, 13.719ms
Bytes In      [total, mean]                     4776023, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.183ms
Latencies     [min, mean, 50, 90, 95, 99, max]  643.787µs, 1.099ms, 1.078ms, 1.232ms, 1.288ms, 1.652ms, 14.001ms
Bytes In      [total, mean]                     4595985, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 875.099µs
Latencies     [min, mean, 50, 90, 95, 99, max]  698.132µs, 1.096ms, 1.071ms, 1.218ms, 1.273ms, 1.583ms, 51.926ms
Bytes In      [total, mean]                     7353454, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.03ms
Latencies     [min, mean, 50, 90, 95, 99, max]  678.708µs, 1.058ms, 1.042ms, 1.194ms, 1.253ms, 1.548ms, 49.543ms
Bytes In      [total, mean]                     7641530, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.057ms
Latencies     [min, mean, 50, 90, 95, 99, max]  738.892µs, 1.107ms, 1.09ms, 1.223ms, 1.269ms, 1.391ms, 60.229ms
Bytes In      [total, mean]                     1838414, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.137ms
Latencies     [min, mean, 50, 90, 95, 99, max]  685.719µs, 1.083ms, 1.065ms, 1.221ms, 1.271ms, 1.438ms, 60.158ms
Bytes In      [total, mean]                     1910397, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.154ms
Latencies     [min, mean, 50, 90, 95, 99, max]  628.282µs, 1.056ms, 1.046ms, 1.198ms, 1.248ms, 1.385ms, 38.208ms
Bytes In      [total, mean]                     1910436, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.316ms
Latencies     [min, mean, 50, 90, 95, 99, max]  720.5µs, 1.11ms, 1.094ms, 1.231ms, 1.278ms, 1.43ms, 37.662ms
Bytes In      [total, mean]                     1838385, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.043ms
Latencies     [min, mean, 50, 90, 95, 99, max]  661.885µs, 1.145ms, 1.081ms, 1.242ms, 1.309ms, 2.011ms, 856.94ms
Bytes In      [total, mean]                     4599013, 153.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.007ms
Latencies     [min, mean, 50, 90, 95, 99, max]  628.052µs, 1.078ms, 1.056ms, 1.216ms, 1.279ms, 2.022ms, 25.662ms
Bytes In      [total, mean]                     4779087, 159.30
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.248ms
Latencies     [min, mean, 50, 90, 95, 99, max]  615.948µs, 1.067ms, 1.045ms, 1.188ms, 1.236ms, 1.647ms, 90.055ms
Bytes In      [total, mean]                     15292781, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.35ms
Latencies     [min, mean, 50, 90, 95, 99, max]  661.544µs, 1.12ms, 1.088ms, 1.226ms, 1.275ms, 1.756ms, 98.262ms
Bytes In      [total, mean]                     14716730, 153.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.099ms
Latencies     [min, mean, 50, 90, 95, 99, max]  736.624µs, 1.147ms, 1.08ms, 1.213ms, 1.263ms, 1.603ms, 126.094ms
Bytes In      [total, mean]                     1839560, 153.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.206ms
Latencies     [min, mean, 50, 90, 95, 99, max]  692.34µs, 1.09ms, 1.05ms, 1.186ms, 1.231ms, 1.532ms, 38.811ms
Bytes In      [total, mean]                     1911654, 159.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 2.008ms
Latencies     [min, mean, 50, 90, 95, 99, max]  691.067µs, 1.073ms, 1.06ms, 1.206ms, 1.268ms, 1.544ms, 21.344ms
Bytes In      [total, mean]                     1911570, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.275ms
Latencies     [min, mean, 50, 90, 95, 99, max]  744.338µs, 1.125ms, 1.107ms, 1.255ms, 1.316ms, 1.547ms, 21.699ms
Bytes In      [total, mean]                     1839613, 153.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
