# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 635b3fcd6e643f4bd24ebbd4c901619a030c4bc0
- Date: 2025-09-15T17:56:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1036000
- vCPUs per node: 16
- RAM per node: 65851528Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 998.895µs
Latencies     [min, mean, 50, 90, 95, 99, max]  671.983µs, 1.293ms, 1.278ms, 1.478ms, 1.554ms, 1.86ms, 16.369ms
Bytes In      [total, mean]                     4656088, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.197ms
Latencies     [min, mean, 50, 90, 95, 99, max]  659.146µs, 1.213ms, 1.207ms, 1.387ms, 1.452ms, 1.75ms, 17.398ms
Bytes In      [total, mean]                     4835973, 161.20
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.077ms
Latencies     [min, mean, 50, 90, 95, 99, max]  649.696µs, 1.228ms, 1.217ms, 1.411ms, 1.48ms, 1.72ms, 38.048ms
Bytes In      [total, mean]                     7737483, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.437ms
Latencies     [min, mean, 50, 90, 95, 99, max]  705.961µs, 1.261ms, 1.247ms, 1.436ms, 1.507ms, 1.78ms, 37.314ms
Bytes In      [total, mean]                     7449488, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.229ms
Latencies     [min, mean, 50, 90, 95, 99, max]  675.074µs, 1.205ms, 1.2ms, 1.378ms, 1.435ms, 1.589ms, 59.446ms
Bytes In      [total, mean]                     1934397, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.118ms
Latencies     [min, mean, 50, 90, 95, 99, max]  703.38µs, 1.241ms, 1.229ms, 1.404ms, 1.467ms, 1.662ms, 57.97ms
Bytes In      [total, mean]                     1862451, 155.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 869.184µs
Latencies     [min, mean, 50, 90, 95, 99, max]  714.269µs, 1.258ms, 1.253ms, 1.422ms, 1.48ms, 1.651ms, 14.173ms
Bytes In      [total, mean]                     1862397, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.116ms
Latencies     [min, mean, 50, 90, 95, 99, max]  689.491µs, 1.189ms, 1.195ms, 1.361ms, 1.413ms, 1.544ms, 5.134ms
Bytes In      [total, mean]                     1934379, 161.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.206ms
Latencies     [min, mean, 50, 90, 95, 99, max]  678.928µs, 1.281ms, 1.246ms, 1.448ms, 1.531ms, 1.943ms, 29.127ms
Bytes In      [total, mean]                     4656049, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.056ms
Latencies     [min, mean, 50, 90, 95, 99, max]  634.533µs, 1.203ms, 1.189ms, 1.388ms, 1.473ms, 1.859ms, 25.911ms
Bytes In      [total, mean]                     4835915, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.373ms
Latencies     [min, mean, 50, 90, 95, 99, max]  675.608µs, 1.264ms, 1.249ms, 1.441ms, 1.512ms, 1.792ms, 50.861ms
Bytes In      [total, mean]                     14899194, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.127ms
Latencies     [min, mean, 50, 90, 95, 99, max]  648.252µs, 1.205ms, 1.199ms, 1.387ms, 1.453ms, 1.718ms, 50.561ms
Bytes In      [total, mean]                     15475157, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 967.985µs
Latencies     [min, mean, 50, 90, 95, 99, max]  741.113µs, 1.297ms, 1.255ms, 1.415ms, 1.468ms, 1.606ms, 116.955ms
Bytes In      [total, mean]                     1862469, 155.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.185ms
Latencies     [min, mean, 50, 90, 95, 99, max]  670.703µs, 1.227ms, 1.204ms, 1.374ms, 1.42ms, 1.553ms, 111.729ms
Bytes In      [total, mean]                     1934414, 161.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.15ms
Latencies     [min, mean, 50, 90, 95, 99, max]  660.653µs, 1.213ms, 1.213ms, 1.382ms, 1.435ms, 1.57ms, 13.809ms
Bytes In      [total, mean]                     1934319, 161.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.352ms
Latencies     [min, mean, 50, 90, 95, 99, max]  713.339µs, 1.254ms, 1.252ms, 1.413ms, 1.463ms, 1.619ms, 13.187ms
Bytes In      [total, mean]                     1862473, 155.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
