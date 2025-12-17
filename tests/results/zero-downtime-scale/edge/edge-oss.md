# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 89aee48bf6e660a828ffd32ca35fc7f52e358e00
- Date: 2025-12-12T20:04:38Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.057ms
Latencies     [min, mean, 50, 90, 95, 99, max]  611.729µs, 1.189ms, 1.132ms, 1.41ms, 1.506ms, 2.014ms, 20.318ms
Bytes In      [total, mean]                     4565949, 152.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 973.114µs
Latencies     [min, mean, 50, 90, 95, 99, max]  548.676µs, 1.131ms, 1.083ms, 1.372ms, 1.472ms, 1.938ms, 19.747ms
Bytes In      [total, mean]                     4743093, 158.10
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.046ms
Latencies     [min, mean, 50, 90, 95, 99, max]  580.229µs, 1.076ms, 1.071ms, 1.236ms, 1.291ms, 1.527ms, 36.184ms
Bytes In      [total, mean]                     7588797, 158.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.05ms
Latencies     [min, mean, 50, 90, 95, 99, max]  621.91µs, 1.129ms, 1.119ms, 1.286ms, 1.349ms, 1.578ms, 36.271ms
Bytes In      [total, mean]                     7305667, 152.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.286ms
Latencies     [min, mean, 50, 90, 95, 99, max]  661.367µs, 1.158ms, 1.15ms, 1.299ms, 1.35ms, 1.527ms, 56.957ms
Bytes In      [total, mean]                     1826321, 152.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.162ms
Latencies     [min, mean, 50, 90, 95, 99, max]  611.551µs, 1.113ms, 1.104ms, 1.268ms, 1.324ms, 1.521ms, 56.569ms
Bytes In      [total, mean]                     1897186, 158.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.208ms
Latencies     [min, mean, 50, 90, 95, 99, max]  617.907µs, 1.2ms, 1.191ms, 1.369ms, 1.42ms, 1.556ms, 31.902ms
Bytes In      [total, mean]                     1897176, 158.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.427ms
Latencies     [min, mean, 50, 90, 95, 99, max]  677.391µs, 1.227ms, 1.227ms, 1.395ms, 1.445ms, 1.566ms, 34.53ms
Bytes In      [total, mean]                     1826283, 152.19
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.07ms
Latencies     [min, mean, 50, 90, 95, 99, max]  616.117µs, 1.167ms, 1.137ms, 1.343ms, 1.421ms, 1.788ms, 23.31ms
Bytes In      [total, mean]                     4575007, 152.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 924.719µs
Latencies     [min, mean, 50, 90, 95, 99, max]  568.031µs, 1.101ms, 1.084ms, 1.293ms, 1.37ms, 1.686ms, 22.501ms
Bytes In      [total, mean]                     4751882, 158.40
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.273ms
Latencies     [min, mean, 50, 90, 95, 99, max]  625.445µs, 1.116ms, 1.101ms, 1.263ms, 1.321ms, 1.615ms, 46.343ms
Bytes In      [total, mean]                     14640119, 152.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 962.211µs
Latencies     [min, mean, 50, 90, 95, 99, max]  571.262µs, 1.05ms, 1.041ms, 1.211ms, 1.267ms, 1.53ms, 41.688ms
Bytes In      [total, mean]                     15206486, 158.40
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.097ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.706µs, 1.13ms, 1.112ms, 1.291ms, 1.348ms, 1.598ms, 32.067ms
Bytes In      [total, mean]                     1900886, 158.41
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 900.044µs
Latencies     [min, mean, 50, 90, 95, 99, max]  670.974µs, 1.208ms, 1.168ms, 1.347ms, 1.409ms, 1.619ms, 114.368ms
Bytes In      [total, mean]                     1830149, 152.51
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.192ms
Latencies     [min, mean, 50, 90, 95, 99, max]  621.151µs, 1.142ms, 1.131ms, 1.295ms, 1.351ms, 1.526ms, 50.078ms
Bytes In      [total, mean]                     1830042, 152.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.198ms
Latencies     [min, mean, 50, 90, 95, 99, max]  556.938µs, 1.074ms, 1.069ms, 1.245ms, 1.3ms, 1.461ms, 9.95ms
Bytes In      [total, mean]                     1900758, 158.40
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
