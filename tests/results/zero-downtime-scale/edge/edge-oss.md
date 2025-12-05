# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.081ms
Latencies     [min, mean, 50, 90, 95, 99, max]  634.098µs, 1.091ms, 1.084ms, 1.242ms, 1.296ms, 1.665ms, 29.797ms
Bytes In      [total, mean]                     4803053, 160.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.214ms
Latencies     [min, mean, 50, 90, 95, 99, max]  658.336µs, 1.16ms, 1.147ms, 1.292ms, 1.347ms, 1.725ms, 29.863ms
Bytes In      [total, mean]                     4623030, 154.10
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.42ms
Latencies     [min, mean, 50, 90, 95, 99, max]  677.426µs, 1.272ms, 1.198ms, 1.422ms, 1.797ms, 3.06ms, 113.012ms
Bytes In      [total, mean]                     7396768, 154.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.296ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.978µs, 1.165ms, 1.141ms, 1.319ms, 1.413ms, 2.136ms, 43.502ms
Bytes In      [total, mean]                     7684842, 160.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.044ms
Latencies     [min, mean, 50, 90, 95, 99, max]  624.82µs, 1.184ms, 1.156ms, 1.341ms, 1.424ms, 1.884ms, 75.076ms
Bytes In      [total, mean]                     1921146, 160.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 2.196ms
Latencies     [min, mean, 50, 90, 95, 99, max]  716.296µs, 1.25ms, 1.214ms, 1.394ms, 1.477ms, 1.97ms, 69.759ms
Bytes In      [total, mean]                     1849184, 154.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.367ms
Latencies     [min, mean, 50, 90, 95, 99, max]  716.154µs, 1.263ms, 1.231ms, 1.458ms, 1.589ms, 2.143ms, 14.098ms
Bytes In      [total, mean]                     1849212, 154.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.223ms
Latencies     [min, mean, 50, 90, 95, 99, max]  683.426µs, 1.219ms, 1.194ms, 1.426ms, 1.559ms, 2.133ms, 14.107ms
Bytes In      [total, mean]                     1921201, 160.10
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
Duration      [total, attack, wait]             5m0s, 5m0s, 2.48ms
Latencies     [min, mean, 50, 90, 95, 99, max]  496.429µs, 1.349ms, 1.209ms, 1.571ms, 1.706ms, 4.278ms, 206.314ms
Bytes In      [total, mean]                     4622671, 154.09
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:2  200:29998  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.65:443: connect: network is unreachable
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 1.121ms
Latencies     [min, mean, 50, 90, 95, 99, max]  227.475µs, 1.256ms, 1.146ms, 1.492ms, 1.633ms, 3.51ms, 204.439ms
Bytes In      [total, mean]                     4802368, 160.08
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:4  200:29996  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.65:80: connect: network is unreachable
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.179ms
Latencies     [min, mean, 50, 90, 95, 99, max]  663.448µs, 1.27ms, 1.216ms, 1.494ms, 1.636ms, 2.046ms, 123.056ms
Bytes In      [total, mean]                     14793576, 154.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.209ms
Latencies     [min, mean, 50, 90, 95, 99, max]  630.877µs, 1.225ms, 1.185ms, 1.441ms, 1.562ms, 1.963ms, 118.34ms
Bytes In      [total, mean]                     15369742, 160.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.006ms
Latencies     [min, mean, 50, 90, 95, 99, max]  638.466µs, 1.24ms, 1.175ms, 1.437ms, 1.566ms, 1.932ms, 140.015ms
Bytes In      [total, mean]                     1921064, 160.09
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.144ms
Latencies     [min, mean, 50, 90, 95, 99, max]  705.944µs, 1.293ms, 1.191ms, 1.462ms, 1.611ms, 2.081ms, 140.253ms
Bytes In      [total, mean]                     1849187, 154.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.165ms
Latencies     [min, mean, 50, 90, 95, 99, max]  654.248µs, 1.156ms, 1.139ms, 1.338ms, 1.421ms, 1.836ms, 38.829ms
Bytes In      [total, mean]                     1921251, 160.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.38ms
Latencies     [min, mean, 50, 90, 95, 99, max]  672.181µs, 1.231ms, 1.199ms, 1.427ms, 1.544ms, 1.976ms, 39.123ms
Bytes In      [total, mean]                     1849149, 154.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
