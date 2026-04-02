# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 17c42c8bbbb004ba9c0e9b867396c5f8937207cd
- Date: 2026-04-01T18:33:47Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.066ms
Latencies     [min, mean, 50, 90, 95, 99, max]  711.595µs, 1.165ms, 1.138ms, 1.309ms, 1.375ms, 1.98ms, 23.602ms
Bytes In      [total, mean]                     4653050, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.039ms
Latencies     [min, mean, 50, 90, 95, 99, max]  671.156µs, 1.121ms, 1.104ms, 1.271ms, 1.333ms, 1.93ms, 24.954ms
Bytes In      [total, mean]                     4832985, 161.10
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
Duration      [total, attack, wait]             8m0s, 8m0s, 927.766µs
Latencies     [min, mean, 50, 90, 95, 99, max]  626.354µs, 1.097ms, 1.084ms, 1.245ms, 1.301ms, 1.644ms, 29.898ms
Bytes In      [total, mean]                     7732702, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.181ms
Latencies     [min, mean, 50, 90, 95, 99, max]  703.152µs, 1.162ms, 1.138ms, 1.305ms, 1.363ms, 1.718ms, 40.796ms
Bytes In      [total, mean]                     7444957, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 99.96
Duration      [total, attack, wait]             2m0s, 2m0s, 1.651ms
Latencies     [min, mean, 50, 90, 95, 99, max]  399.984µs, 1.349ms, 1.238ms, 1.577ms, 1.66ms, 2.049ms, 208.828ms
Bytes In      [total, mean]                     1860244, 155.02
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.95%
Status Codes  [code:count]                      0:6  200:11994  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.34:443: connect: network is unreachable
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 99.97
Duration      [total, attack, wait]             2m0s, 2m0s, 1.346ms
Latencies     [min, mean, 50, 90, 95, 99, max]  313.917µs, 1.328ms, 1.175ms, 1.548ms, 1.631ms, 2.014ms, 250.541ms
Bytes In      [total, mean]                     1932573, 161.05
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.97%
Status Codes  [code:count]                      0:4  200:11996  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.34:80: connect: network is unreachable
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.157ms
Latencies     [min, mean, 50, 90, 95, 99, max]  690.667µs, 1.362ms, 1.323ms, 1.638ms, 1.712ms, 2.019ms, 28.007ms
Bytes In      [total, mean]                     1861224, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.74ms
Latencies     [min, mean, 50, 90, 95, 99, max]  703.851µs, 1.319ms, 1.272ms, 1.627ms, 1.714ms, 2.027ms, 27.9ms
Bytes In      [total, mean]                     1933183, 161.10
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.114ms
Latencies     [min, mean, 50, 90, 95, 99, max]  679.52µs, 1.155ms, 1.114ms, 1.277ms, 1.347ms, 2.178ms, 32.854ms
Bytes In      [total, mean]                     4652936, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.247ms
Latencies     [min, mean, 50, 90, 95, 99, max]  611.983µs, 1.1ms, 1.073ms, 1.245ms, 1.313ms, 2.076ms, 38.803ms
Bytes In      [total, mean]                     4832956, 161.10
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.293ms
Latencies     [min, mean, 50, 90, 95, 99, max]  604.189µs, 1.124ms, 1.106ms, 1.288ms, 1.35ms, 1.845ms, 46.631ms
Bytes In      [total, mean]                     15465559, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.076ms
Latencies     [min, mean, 50, 90, 95, 99, max]  647.814µs, 1.19ms, 1.166ms, 1.345ms, 1.408ms, 1.972ms, 46.562ms
Bytes In      [total, mean]                     14889715, 155.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.251ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.231µs, 1.193ms, 1.136ms, 1.294ms, 1.345ms, 1.563ms, 122.664ms
Bytes In      [total, mean]                     1861221, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.187ms
Latencies     [min, mean, 50, 90, 95, 99, max]  653.536µs, 1.11ms, 1.073ms, 1.234ms, 1.287ms, 1.445ms, 122.447ms
Bytes In      [total, mean]                     1933233, 161.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 950.079µs
Latencies     [min, mean, 50, 90, 95, 99, max]  677.489µs, 1.105ms, 1.086ms, 1.225ms, 1.271ms, 1.457ms, 63.724ms
Bytes In      [total, mean]                     1861223, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 807.017µs
Latencies     [min, mean, 50, 90, 95, 99, max]  637.79µs, 1.076ms, 1.058ms, 1.207ms, 1.254ms, 1.506ms, 64.055ms
Bytes In      [total, mean]                     1933152, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
