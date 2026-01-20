# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fe52764cce240bb5a3713f56aa113694c2793f93
- Date: 2026-01-20T16:40:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2072000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.034ms
Latencies     [min, mean, 50, 90, 95, 99, max]  667.038µs, 1.072ms, 1.034ms, 1.23ms, 1.336ms, 1.723ms, 17.452ms
Bytes In      [total, mean]                     4655824, 155.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 798.274µs
Latencies     [min, mean, 50, 90, 95, 99, max]  552.744µs, 1.007ms, 986.915µs, 1.172ms, 1.242ms, 1.596ms, 17.477ms
Bytes In      [total, mean]                     4835974, 161.20
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.035ms
Latencies     [min, mean, 50, 90, 95, 99, max]  610.147µs, 1.034ms, 1.014ms, 1.197ms, 1.258ms, 1.521ms, 49.751ms
Bytes In      [total, mean]                     7737758, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.147ms
Latencies     [min, mean, 50, 90, 95, 99, max]  649.742µs, 1.076ms, 1.051ms, 1.217ms, 1.282ms, 1.591ms, 50.523ms
Bytes In      [total, mean]                     7449625, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.211ms
Latencies     [min, mean, 50, 90, 95, 99, max]  703.557µs, 1.094ms, 1.065ms, 1.218ms, 1.278ms, 1.477ms, 73.106ms
Bytes In      [total, mean]                     1862400, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 966.898µs
Latencies     [min, mean, 50, 90, 95, 99, max]  614.849µs, 1.034ms, 1.014ms, 1.177ms, 1.232ms, 1.468ms, 61.337ms
Bytes In      [total, mean]                     1934379, 161.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.037ms
Latencies     [min, mean, 50, 90, 95, 99, max]  649.746µs, 1.013ms, 1.002ms, 1.16ms, 1.217ms, 1.479ms, 4.131ms
Bytes In      [total, mean]                     1934432, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.037ms
Latencies     [min, mean, 50, 90, 95, 99, max]  699.651µs, 1.072ms, 1.048ms, 1.22ms, 1.292ms, 1.572ms, 12.187ms
Bytes In      [total, mean]                     1862427, 155.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.054ms
Latencies     [min, mean, 50, 90, 95, 99, max]  638.909µs, 1.182ms, 1.091ms, 1.497ms, 1.726ms, 2.291ms, 31.244ms
Bytes In      [total, mean]                     4656038, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 900.594µs
Latencies     [min, mean, 50, 90, 95, 99, max]  604.439µs, 1.119ms, 1.051ms, 1.432ms, 1.654ms, 2.166ms, 19.526ms
Bytes In      [total, mean]                     4835905, 161.20
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
Duration      [total, attack, wait]             16m0s, 16m0s, 851.885µs
Latencies     [min, mean, 50, 90, 95, 99, max]  589.125µs, 1.035ms, 1.008ms, 1.186ms, 1.268ms, 1.701ms, 65.903ms
Bytes In      [total, mean]                     15475287, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.14ms
Latencies     [min, mean, 50, 90, 95, 99, max]  631.287µs, 1.09ms, 1.052ms, 1.222ms, 1.305ms, 1.743ms, 62.292ms
Bytes In      [total, mean]                     14899150, 155.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.068ms
Latencies     [min, mean, 50, 90, 95, 99, max]  682.654µs, 1.121ms, 1.052ms, 1.221ms, 1.282ms, 1.499ms, 114.996ms
Bytes In      [total, mean]                     1862385, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 866.211µs
Latencies     [min, mean, 50, 90, 95, 99, max]  647.719µs, 1.069ms, 1.012ms, 1.177ms, 1.237ms, 1.497ms, 119.513ms
Bytes In      [total, mean]                     1934406, 161.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 959.174µs
Latencies     [min, mean, 50, 90, 95, 99, max]  736.652µs, 1.101ms, 1.071ms, 1.258ms, 1.335ms, 1.619ms, 29.183ms
Bytes In      [total, mean]                     1862358, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.109ms
Latencies     [min, mean, 50, 90, 95, 99, max]  644.272µs, 1.033ms, 1.02ms, 1.185ms, 1.244ms, 1.455ms, 4.636ms
Bytes In      [total, mean]                     1934427, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
