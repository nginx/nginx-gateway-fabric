# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 996.298µs
Latencies     [min, mean, 50, 90, 95, 99, max]  633.278µs, 1.143ms, 1.104ms, 1.35ms, 1.444ms, 1.865ms, 23.375ms
Bytes In      [total, mean]                     4625965, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.488ms
Latencies     [min, mean, 50, 90, 95, 99, max]  585.365µs, 1.092ms, 1.064ms, 1.302ms, 1.396ms, 1.826ms, 18.227ms
Bytes In      [total, mean]                     4805984, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.276ms
Latencies     [min, mean, 50, 90, 95, 99, max]  282.986µs, 1.274ms, 1.193ms, 1.498ms, 1.593ms, 2.064ms, 247.106ms
Bytes In      [total, mean]                     7401179, 154.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      0:2  200:47998  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.36:443: connect: network is unreachable
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 972.874µs
Latencies     [min, mean, 50, 90, 95, 99, max]  260.575µs, 1.184ms, 1.112ms, 1.439ms, 1.538ms, 1.892ms, 246.203ms
Bytes In      [total, mean]                     7689366, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      0:2  200:47998  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.36:80: connect: network is unreachable
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 974.911µs
Latencies     [min, mean, 50, 90, 95, 99, max]  639.245µs, 1.052ms, 1.035ms, 1.2ms, 1.257ms, 1.487ms, 66.745ms
Bytes In      [total, mean]                     1922420, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.095ms
Latencies     [min, mean, 50, 90, 95, 99, max]  669.229µs, 1.108ms, 1.078ms, 1.237ms, 1.292ms, 1.574ms, 69.621ms
Bytes In      [total, mean]                     1850465, 154.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.384ms
Latencies     [min, mean, 50, 90, 95, 99, max]  616.231µs, 1.036ms, 1.026ms, 1.202ms, 1.261ms, 1.422ms, 13.896ms
Bytes In      [total, mean]                     1922423, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.379ms
Latencies     [min, mean, 50, 90, 95, 99, max]  664.315µs, 1.103ms, 1.082ms, 1.259ms, 1.321ms, 1.492ms, 21.264ms
Bytes In      [total, mean]                     1850399, 154.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 2.529ms
Latencies     [min, mean, 50, 90, 95, 99, max]  639.626µs, 1.166ms, 1.11ms, 1.371ms, 1.49ms, 2.104ms, 38.399ms
Bytes In      [total, mean]                     4626000, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 896.326µs
Latencies     [min, mean, 50, 90, 95, 99, max]  598.172µs, 1.107ms, 1.073ms, 1.316ms, 1.428ms, 2.018ms, 37.657ms
Bytes In      [total, mean]                     4806022, 160.20
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
Duration      [total, attack, wait]             16m0s, 16m0s, 950.076µs
Latencies     [min, mean, 50, 90, 95, 99, max]  600.609µs, 1.057ms, 1.027ms, 1.214ms, 1.279ms, 1.666ms, 118.625ms
Bytes In      [total, mean]                     15379054, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.004ms
Latencies     [min, mean, 50, 90, 95, 99, max]  635.023µs, 1.126ms, 1.08ms, 1.274ms, 1.351ms, 1.828ms, 220.128ms
Bytes In      [total, mean]                     14803138, 154.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.291ms
Latencies     [min, mean, 50, 90, 95, 99, max]  676.053µs, 1.115ms, 1.05ms, 1.214ms, 1.275ms, 1.545ms, 123.45ms
Bytes In      [total, mean]                     1850366, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.02ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.831µs, 1.059ms, 1.005ms, 1.171ms, 1.225ms, 1.401ms, 123.661ms
Bytes In      [total, mean]                     1922438, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 992.236µs
Latencies     [min, mean, 50, 90, 95, 99, max]  684.079µs, 1.073ms, 1.045ms, 1.22ms, 1.278ms, 1.471ms, 64.409ms
Bytes In      [total, mean]                     1850451, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.097ms
Latencies     [min, mean, 50, 90, 95, 99, max]  658.552µs, 1.027ms, 1.011ms, 1.186ms, 1.242ms, 1.418ms, 31.685ms
Bytes In      [total, mean]                     1922385, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
