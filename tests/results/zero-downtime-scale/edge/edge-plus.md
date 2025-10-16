# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9010072ecd34a8fa99bfdd3d7580c9d725fb063e
- Date: 2025-10-01T09:39:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1172000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.305ms
Latencies     [min, mean, 50, 90, 95, 99, max]  684.458µs, 1.219ms, 1.206ms, 1.372ms, 1.433ms, 1.721ms, 22.737ms
Bytes In      [total, mean]                     4655986, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.314ms
Latencies     [min, mean, 50, 90, 95, 99, max]  632.222µs, 1.194ms, 1.149ms, 1.318ms, 1.376ms, 1.658ms, 1.022s
Bytes In      [total, mean]                     4835960, 161.20
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.528ms
Latencies     [min, mean, 50, 90, 95, 99, max]  698.076µs, 1.235ms, 1.217ms, 1.429ms, 1.506ms, 1.754ms, 38.715ms
Bytes In      [total, mean]                     7737557, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.711ms
Latencies     [min, mean, 50, 90, 95, 99, max]  723.444µs, 1.301ms, 1.276ms, 1.496ms, 1.584ms, 1.859ms, 38.379ms
Bytes In      [total, mean]                     7449436, 155.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.454ms
Latencies     [min, mean, 50, 90, 95, 99, max]  763.137µs, 1.303ms, 1.281ms, 1.506ms, 1.586ms, 1.752ms, 61.076ms
Bytes In      [total, mean]                     1862406, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.248ms
Latencies     [min, mean, 50, 90, 95, 99, max]  646.444µs, 1.225ms, 1.208ms, 1.441ms, 1.52ms, 1.69ms, 59.673ms
Bytes In      [total, mean]                     1934445, 161.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.65ms
Latencies     [min, mean, 50, 90, 95, 99, max]  703.83µs, 1.256ms, 1.244ms, 1.424ms, 1.491ms, 1.647ms, 33.584ms
Bytes In      [total, mean]                     1862380, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.506ms
Latencies     [min, mean, 50, 90, 95, 99, max]  701.59µs, 1.186ms, 1.177ms, 1.364ms, 1.427ms, 1.602ms, 28.674ms
Bytes In      [total, mean]                     1934402, 161.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.497ms
Latencies     [min, mean, 50, 90, 95, 99, max]  679.64µs, 1.221ms, 1.208ms, 1.404ms, 1.482ms, 1.792ms, 24.983ms
Bytes In      [total, mean]                     4656017, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.017ms
Latencies     [min, mean, 50, 90, 95, 99, max]  618.59µs, 1.17ms, 1.16ms, 1.346ms, 1.414ms, 1.709ms, 24.198ms
Bytes In      [total, mean]                     4836014, 161.20
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.149ms
Latencies     [min, mean, 50, 90, 95, 99, max]  627.623µs, 1.234ms, 1.221ms, 1.451ms, 1.533ms, 1.782ms, 43.412ms
Bytes In      [total, mean]                     15475055, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.197ms
Latencies     [min, mean, 50, 90, 95, 99, max]  668.009µs, 1.295ms, 1.274ms, 1.511ms, 1.593ms, 1.872ms, 43.302ms
Bytes In      [total, mean]                     14899223, 155.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.179ms
Latencies     [min, mean, 50, 90, 95, 99, max]  778.457µs, 1.431ms, 1.371ms, 1.649ms, 1.734ms, 1.933ms, 121.283ms
Bytes In      [total, mean]                     1862387, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.071ms
Latencies     [min, mean, 50, 90, 95, 99, max]  625.684µs, 1.362ms, 1.304ms, 1.582ms, 1.674ms, 1.86ms, 120.921ms
Bytes In      [total, mean]                     1934450, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 91.67
Duration      [total, attack, wait]             2m0s, 2m0s, 1.383ms
Latencies     [min, mean, 50, 90, 95, 99, max]  462.407µs, 1.178ms, 1.202ms, 1.431ms, 1.506ms, 1.662ms, 10.122ms
Bytes In      [total, mean]                     1923193, 160.27
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           91.67%
Status Codes  [code:count]                      200:11000  502:1000  
Error Set:
502 Bad Gateway
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.161ms
Latencies     [min, mean, 50, 90, 95, 99, max]  690.376µs, 1.275ms, 1.261ms, 1.487ms, 1.567ms, 1.751ms, 37.453ms
Bytes In      [total, mean]                     1862482, 155.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
