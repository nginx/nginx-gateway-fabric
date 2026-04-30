# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: d469ad4919212e6ba08635e5f08266ff3453f783
- Date: 2026-04-30T19:09:09Z
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.937ms
Latencies     [min, mean, 50, 90, 95, 99, max]  246.791µs, 1.26ms, 1.178ms, 1.52ms, 1.646ms, 2.025ms, 249.114ms
Bytes In      [total, mean]                     4595847, 153.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:2  200:29998  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.19:443: connect: network is unreachable
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 1.278ms
Latencies     [min, mean, 50, 90, 95, 99, max]  246.556µs, 1.191ms, 1.139ms, 1.445ms, 1.57ms, 1.939ms, 205.206ms
Bytes In      [total, mean]                     4775336, 159.18
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:4  200:29996  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.19:80: connect: network is unreachable
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.175ms
Latencies     [min, mean, 50, 90, 95, 99, max]  683.088µs, 1.227ms, 1.172ms, 1.491ms, 1.605ms, 1.914ms, 82.905ms
Bytes In      [total, mean]                     7353529, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.174ms
Latencies     [min, mean, 50, 90, 95, 99, max]  637.564µs, 1.172ms, 1.132ms, 1.423ms, 1.553ms, 1.869ms, 35.213ms
Bytes In      [total, mean]                     7641745, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.378ms
Latencies     [min, mean, 50, 90, 95, 99, max]  740.103µs, 1.197ms, 1.176ms, 1.359ms, 1.424ms, 1.642ms, 16.137ms
Bytes In      [total, mean]                     1838365, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 998.279µs
Latencies     [min, mean, 50, 90, 95, 99, max]  635.634µs, 1.099ms, 1.084ms, 1.252ms, 1.305ms, 1.437ms, 59.237ms
Bytes In      [total, mean]                     1910414, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.073ms
Latencies     [min, mean, 50, 90, 95, 99, max]  695.432µs, 1.159ms, 1.152ms, 1.332ms, 1.387ms, 1.562ms, 24.267ms
Bytes In      [total, mean]                     1910349, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.192ms
Latencies     [min, mean, 50, 90, 95, 99, max]  688.41µs, 1.223ms, 1.21ms, 1.388ms, 1.449ms, 1.688ms, 24.204ms
Bytes In      [total, mean]                     1838469, 153.21
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.076ms
Latencies     [min, mean, 50, 90, 95, 99, max]  633.491µs, 1.165ms, 1.146ms, 1.335ms, 1.398ms, 1.95ms, 24.555ms
Bytes In      [total, mean]                     4599052, 153.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.336ms
Latencies     [min, mean, 50, 90, 95, 99, max]  561.22µs, 1.103ms, 1.086ms, 1.273ms, 1.335ms, 1.838ms, 24.589ms
Bytes In      [total, mean]                     4779130, 159.30
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.139ms
Latencies     [min, mean, 50, 90, 95, 99, max]  629.064µs, 1.121ms, 1.101ms, 1.29ms, 1.359ms, 1.738ms, 50.267ms
Bytes In      [total, mean]                     15292623, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.126ms
Latencies     [min, mean, 50, 90, 95, 99, max]  666.521µs, 1.184ms, 1.159ms, 1.346ms, 1.416ms, 1.813ms, 52.501ms
Bytes In      [total, mean]                     14716863, 153.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.198ms
Latencies     [min, mean, 50, 90, 95, 99, max]  712.199µs, 1.173ms, 1.113ms, 1.284ms, 1.349ms, 1.726ms, 116.965ms
Bytes In      [total, mean]                     1839620, 153.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 987.243µs
Latencies     [min, mean, 50, 90, 95, 99, max]  649.288µs, 1.103ms, 1.064ms, 1.219ms, 1.278ms, 1.704ms, 141.208ms
Bytes In      [total, mean]                     1911548, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.241ms
Latencies     [min, mean, 50, 90, 95, 99, max]  665.095µs, 1.092ms, 1.091ms, 1.236ms, 1.283ms, 1.419ms, 9.193ms
Bytes In      [total, mean]                     1911464, 159.29
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.151ms
Latencies     [min, mean, 50, 90, 95, 99, max]  701.799µs, 1.139ms, 1.132ms, 1.273ms, 1.323ms, 1.478ms, 12.695ms
Bytes In      [total, mean]                     1839651, 153.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
