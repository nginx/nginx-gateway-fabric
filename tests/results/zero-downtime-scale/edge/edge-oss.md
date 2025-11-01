# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: b41c973c8399458984def3c2a8a268a237c864c8
- Date: 2025-10-30T03:04:40Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1162000
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.199ms
Latencies     [min, mean, 50, 90, 95, 99, max]  656.893µs, 1.225ms, 1.203ms, 1.401ms, 1.476ms, 1.854ms, 19.035ms
Bytes In      [total, mean]                     4595980, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.296ms
Latencies     [min, mean, 50, 90, 95, 99, max]  624.671µs, 1.167ms, 1.155ms, 1.338ms, 1.406ms, 1.763ms, 28.176ms
Bytes In      [total, mean]                     4776047, 159.20
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.29ms
Latencies     [min, mean, 50, 90, 95, 99, max]  662.224µs, 1.191ms, 1.186ms, 1.354ms, 1.414ms, 1.69ms, 44.396ms
Bytes In      [total, mean]                     7641398, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.135ms
Latencies     [min, mean, 50, 90, 95, 99, max]  611.101µs, 1.23ms, 1.217ms, 1.391ms, 1.456ms, 1.756ms, 38.315ms
Bytes In      [total, mean]                     7353577, 153.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.183ms
Latencies     [min, mean, 50, 90, 95, 99, max]  713.793µs, 1.224ms, 1.206ms, 1.377ms, 1.436ms, 1.736ms, 61.965ms
Bytes In      [total, mean]                     1838418, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.218ms
Latencies     [min, mean, 50, 90, 95, 99, max]  677.244µs, 1.168ms, 1.161ms, 1.318ms, 1.37ms, 1.628ms, 60.958ms
Bytes In      [total, mean]                     1910358, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.196ms
Latencies     [min, mean, 50, 90, 95, 99, max]  730.614µs, 1.173ms, 1.17ms, 1.313ms, 1.361ms, 1.48ms, 24.887ms
Bytes In      [total, mean]                     1838461, 153.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 749.033µs
Latencies     [min, mean, 50, 90, 95, 99, max]  678.893µs, 1.134ms, 1.137ms, 1.288ms, 1.335ms, 1.472ms, 24.877ms
Bytes In      [total, mean]                     1910421, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.192ms
Latencies     [min, mean, 50, 90, 95, 99, max]  609.835µs, 1.243ms, 1.152ms, 1.435ms, 1.573ms, 2.68ms, 254.202ms
Bytes In      [total, mean]                     4778944, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 11.951ms
Latencies     [min, mean, 50, 90, 95, 99, max]  665.563µs, 1.312ms, 1.202ms, 1.486ms, 1.615ms, 3.345ms, 254.509ms
Bytes In      [total, mean]                     4602012, 153.40
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.073ms
Latencies     [min, mean, 50, 90, 95, 99, max]  560.828µs, 1.114ms, 1.095ms, 1.305ms, 1.403ms, 1.692ms, 43.4ms
Bytes In      [total, mean]                     15292873, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-oss.png](gradual-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.062ms
Latencies     [min, mean, 50, 90, 95, 99, max]  621.419µs, 1.16ms, 1.131ms, 1.332ms, 1.431ms, 1.751ms, 46.8ms
Bytes In      [total, mean]                     14726471, 153.40
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.062ms
Latencies     [min, mean, 50, 90, 95, 99, max]  605.908µs, 1.124ms, 1.087ms, 1.255ms, 1.313ms, 1.559ms, 126.926ms
Bytes In      [total, mean]                     1911527, 159.29
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.18ms
Latencies     [min, mean, 50, 90, 95, 99, max]  654.11µs, 1.179ms, 1.138ms, 1.295ms, 1.347ms, 1.651ms, 146.52ms
Bytes In      [total, mean]                     1840744, 153.40
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.028ms
Latencies     [min, mean, 50, 90, 95, 99, max]  625.064µs, 1.078ms, 1.076ms, 1.233ms, 1.285ms, 1.437ms, 36.22ms
Bytes In      [total, mean]                     1911578, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.263ms
Latencies     [min, mean, 50, 90, 95, 99, max]  661.105µs, 1.123ms, 1.119ms, 1.265ms, 1.312ms, 1.442ms, 36.719ms
Bytes In      [total, mean]                     1840708, 153.39
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
