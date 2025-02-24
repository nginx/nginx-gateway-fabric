# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 9a7a618dab5ed0eee09063de60d80bf0fb76900a
- Date: 2025-02-14T18:44:35Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1023000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGF Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 0.00
Duration      [total, attack, wait]             5m0s, 5m0s, 858.638µs
Latencies     [min, mean, 50, 90, 95, 99, max]  708.304µs, 1.025ms, 1.011ms, 1.147ms, 1.198ms, 1.334ms, 12.731ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:30000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 735.488µs
Latencies     [min, mean, 50, 90, 95, 99, max]  373.179µs, 788.014µs, 784.382µs, 924.851µs, 977.885µs, 1.327ms, 24.158ms
Bytes In      [total, mean]                     4794240, 159.81
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 0.00
Duration      [total, attack, wait]             8m0s, 8m0s, 857.289µs
Latencies     [min, mean, 50, 90, 95, 99, max]  691.12µs, 995.998µs, 977.537µs, 1.129ms, 1.181ms, 1.33ms, 17.92ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:48000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 682.266µs
Latencies     [min, mean, 50, 90, 95, 99, max]  377.142µs, 783.116µs, 781.581µs, 913.66µs, 962.135µs, 1.219ms, 14.262ms
Bytes In      [total, mean]                     7670275, 159.80
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 1.001ms
Latencies     [min, mean, 50, 90, 95, 99, max]  706.893µs, 1.007ms, 991.049µs, 1.133ms, 1.18ms, 1.346ms, 7.866ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 826.889µs
Latencies     [min, mean, 50, 90, 95, 99, max]  372.585µs, 780.28µs, 775.788µs, 906.698µs, 956.978µs, 1.333ms, 7.653ms
Bytes In      [total, mean]                     1917532, 159.79
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 943.408µs
Latencies     [min, mean, 50, 90, 95, 99, max]  660.146µs, 937.594µs, 912.021µs, 1.064ms, 1.125ms, 1.286ms, 6.44ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 867.777µs
Latencies     [min, mean, 50, 90, 95, 99, max]  400.707µs, 775.364µs, 773.19µs, 911.22µs, 964.379µs, 1.182ms, 5.903ms
Bytes In      [total, mean]                     1917555, 159.80
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
Requests      [total, rate, throughput]         30000, 100.00, 0.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.14ms
Latencies     [min, mean, 50, 90, 95, 99, max]  713.279µs, 1.046ms, 1.026ms, 1.205ms, 1.268ms, 1.402ms, 10.036ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:30000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 556.427µs
Latencies     [min, mean, 50, 90, 95, 99, max]  373.773µs, 794.684µs, 790.572µs, 923.193µs, 976.366µs, 1.367ms, 10.154ms
Bytes In      [total, mean]                     4799980, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 0.00
Duration      [total, attack, wait]             16m0s, 16m0s, 826.644µs
Latencies     [min, mean, 50, 90, 95, 99, max]  678.899µs, 1.015ms, 997.838µs, 1.152ms, 1.209ms, 1.359ms, 16.617ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:96000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 868.037µs
Latencies     [min, mean, 50, 90, 95, 99, max]  345.836µs, 786.222µs, 784.106µs, 917.2µs, 966.648µs, 1.32ms, 20.138ms
Bytes In      [total, mean]                     15359899, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 981.383µs
Latencies     [min, mean, 50, 90, 95, 99, max]  715.436µs, 1.028ms, 1.013ms, 1.156ms, 1.207ms, 1.356ms, 8.943ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 679.58µs
Latencies     [min, mean, 50, 90, 95, 99, max]  368.51µs, 773.987µs, 769.924µs, 907.868µs, 959.948µs, 1.281ms, 6.545ms
Bytes In      [total, mean]                     1920036, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 905.663µs
Latencies     [min, mean, 50, 90, 95, 99, max]  665.467µs, 956.577µs, 928.642µs, 1.113ms, 1.181ms, 1.324ms, 8.218ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 889.546µs
Latencies     [min, mean, 50, 90, 95, 99, max]  405.357µs, 785.516µs, 786.332µs, 915.307µs, 959.813µs, 1.182ms, 11.58ms
Bytes In      [total, mean]                     1919990, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
