# Results

## Test environment

NGINX Plus: true

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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.327ms
Latencies     [min, mean, 50, 90, 95, 99, max]  675.096µs, 1.232ms, 1.209ms, 1.429ms, 1.543ms, 1.768ms, 27.473ms
Bytes In      [total, mean]                     4596075, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.046ms
Latencies     [min, mean, 50, 90, 95, 99, max]  663.466µs, 1.172ms, 1.152ms, 1.361ms, 1.48ms, 1.74ms, 17.181ms
Bytes In      [total, mean]                     4775927, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 99.99
Duration      [total, attack, wait]             8m0s, 8m0s, 1.163ms
Latencies     [min, mean, 50, 90, 95, 99, max]  305.029µs, 1.277ms, 1.217ms, 1.523ms, 1.634ms, 1.847ms, 219.704ms
Bytes In      [total, mean]                     7352590, 153.18
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:6  200:47994  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.47:443: connect: network is unreachable
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 99.99
Duration      [total, attack, wait]             8m0s, 8m0s, 1.045ms
Latencies     [min, mean, 50, 90, 95, 99, max]  243.115µs, 1.215ms, 1.169ms, 1.465ms, 1.598ms, 1.81ms, 214.724ms
Bytes In      [total, mean]                     7640356, 159.17
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:7  200:47993  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.47:80: connect: network is unreachable
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.402ms
Latencies     [min, mean, 50, 90, 95, 99, max]  656.659µs, 1.129ms, 1.133ms, 1.278ms, 1.322ms, 1.507ms, 3.641ms
Bytes In      [total, mean]                     1910438, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.284ms
Latencies     [min, mean, 50, 90, 95, 99, max]  710.396µs, 1.192ms, 1.195ms, 1.323ms, 1.366ms, 1.579ms, 9.731ms
Bytes In      [total, mean]                     1838355, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.314ms
Latencies     [min, mean, 50, 90, 95, 99, max]  730.016µs, 1.229ms, 1.213ms, 1.343ms, 1.388ms, 1.521ms, 64.443ms
Bytes In      [total, mean]                     1838380, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.177ms
Latencies     [min, mean, 50, 90, 95, 99, max]  678.11µs, 1.171ms, 1.161ms, 1.306ms, 1.348ms, 1.474ms, 67.354ms
Bytes In      [total, mean]                     1910385, 159.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.232ms
Latencies     [min, mean, 50, 90, 95, 99, max]  677.29µs, 1.222ms, 1.214ms, 1.361ms, 1.417ms, 1.778ms, 29.484ms
Bytes In      [total, mean]                     4595877, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.116ms
Latencies     [min, mean, 50, 90, 95, 99, max]  652.028µs, 1.156ms, 1.151ms, 1.31ms, 1.364ms, 1.702ms, 29.516ms
Bytes In      [total, mean]                     4775988, 159.20
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.136ms
Latencies     [min, mean, 50, 90, 95, 99, max]  577.2µs, 1.169ms, 1.161ms, 1.316ms, 1.366ms, 1.628ms, 72.479ms
Bytes In      [total, mean]                     15283137, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.188ms
Latencies     [min, mean, 50, 90, 95, 99, max]  687.721µs, 1.229ms, 1.216ms, 1.364ms, 1.419ms, 1.697ms, 68.011ms
Bytes In      [total, mean]                     14707422, 153.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.264ms
Latencies     [min, mean, 50, 90, 95, 99, max]  718.712µs, 1.247ms, 1.217ms, 1.353ms, 1.401ms, 1.716ms, 37.253ms
Bytes In      [total, mean]                     1838307, 153.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.329ms
Latencies     [min, mean, 50, 90, 95, 99, max]  670.205µs, 1.191ms, 1.169ms, 1.31ms, 1.357ms, 1.582ms, 113.243ms
Bytes In      [total, mean]                     1910371, 159.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.262ms
Latencies     [min, mean, 50, 90, 95, 99, max]  488.744µs, 1.133ms, 1.175ms, 1.329ms, 1.374ms, 1.478ms, 3.391ms
Bytes In      [total, mean]                     1901179, 158.43
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.186ms
Latencies     [min, mean, 50, 90, 95, 99, max]  746.375µs, 1.23ms, 1.233ms, 1.364ms, 1.407ms, 1.537ms, 20.761ms
Bytes In      [total, mean]                     1838411, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
