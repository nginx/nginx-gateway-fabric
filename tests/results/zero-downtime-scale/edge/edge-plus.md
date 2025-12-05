# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851512Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.152ms
Latencies     [min, mean, 50, 90, 95, 99, max]  642.7µs, 1.101ms, 1.091ms, 1.239ms, 1.295ms, 1.594ms, 12.565ms
Bytes In      [total, mean]                     4596027, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.171ms
Latencies     [min, mean, 50, 90, 95, 99, max]  571.996µs, 1.043ms, 1.038ms, 1.191ms, 1.245ms, 1.547ms, 12.576ms
Bytes In      [total, mean]                     4775982, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.294ms
Latencies     [min, mean, 50, 90, 95, 99, max]  640.818µs, 1.14ms, 1.13ms, 1.279ms, 1.332ms, 1.579ms, 56.666ms
Bytes In      [total, mean]                     7353684, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 818.341µs
Latencies     [min, mean, 50, 90, 95, 99, max]  602.268µs, 1.081ms, 1.075ms, 1.237ms, 1.289ms, 1.485ms, 53.82ms
Bytes In      [total, mean]                     7641687, 159.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.422ms
Latencies     [min, mean, 50, 90, 95, 99, max]  657.686µs, 1.147ms, 1.134ms, 1.28ms, 1.331ms, 1.524ms, 59.669ms
Bytes In      [total, mean]                     1838403, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.128ms
Latencies     [min, mean, 50, 90, 95, 99, max]  603.038µs, 1.087ms, 1.072ms, 1.233ms, 1.283ms, 1.435ms, 60.13ms
Bytes In      [total, mean]                     1910407, 159.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.13ms
Latencies     [min, mean, 50, 90, 95, 99, max]  611.454µs, 1.055ms, 1.056ms, 1.217ms, 1.265ms, 1.403ms, 27.734ms
Bytes In      [total, mean]                     1910389, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.105ms
Latencies     [min, mean, 50, 90, 95, 99, max]  632.489µs, 1.108ms, 1.106ms, 1.253ms, 1.3ms, 1.449ms, 28.253ms
Bytes In      [total, mean]                     1838423, 153.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.131ms
Latencies     [min, mean, 50, 90, 95, 99, max]  623.64µs, 1.118ms, 1.11ms, 1.258ms, 1.316ms, 1.672ms, 24.213ms
Bytes In      [total, mean]                     4605111, 153.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.162ms
Latencies     [min, mean, 50, 90, 95, 99, max]  586.508µs, 1.064ms, 1.058ms, 1.21ms, 1.261ms, 1.548ms, 24.519ms
Bytes In      [total, mean]                     4778909, 159.30
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.344ms
Latencies     [min, mean, 50, 90, 95, 99, max]  581.749µs, 1.096ms, 1.085ms, 1.254ms, 1.312ms, 1.574ms, 54.419ms
Bytes In      [total, mean]                     15292880, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.14ms
Latencies     [min, mean, 50, 90, 95, 99, max]  644.459µs, 1.145ms, 1.131ms, 1.288ms, 1.347ms, 1.612ms, 75.739ms
Bytes In      [total, mean]                     14736105, 153.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.153ms
Latencies     [min, mean, 50, 90, 95, 99, max]  635.858µs, 1.151ms, 1.12ms, 1.286ms, 1.333ms, 1.466ms, 114.708ms
Bytes In      [total, mean]                     1911604, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.029ms
Latencies     [min, mean, 50, 90, 95, 99, max]  692.292µs, 1.221ms, 1.175ms, 1.332ms, 1.384ms, 1.553ms, 117.459ms
Bytes In      [total, mean]                     1842053, 153.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.378ms
Latencies     [min, mean, 50, 90, 95, 99, max]  590.15µs, 1.076ms, 1.083ms, 1.247ms, 1.296ms, 1.412ms, 3.021ms
Bytes In      [total, mean]                     1911649, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 840.028µs
Latencies     [min, mean, 50, 90, 95, 99, max]  647.175µs, 1.151ms, 1.155ms, 1.311ms, 1.361ms, 1.521ms, 9.693ms
Bytes In      [total, mean]                     1841905, 153.49
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
