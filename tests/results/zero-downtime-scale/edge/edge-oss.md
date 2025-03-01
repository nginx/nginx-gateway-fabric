# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: bfd685d3805042ac77865a9823104404a80b06b9
- Date: 2025-02-28T18:00:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1169000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGF Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 717.075µs
Latencies     [min, mean, 50, 90, 95, 99, max]  424.568µs, 861.916µs, 851.403µs, 1.003ms, 1.06ms, 1.341ms, 38.335ms
Bytes In      [total, mean]                     4806010, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 0.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.048ms
Latencies     [min, mean, 50, 90, 95, 99, max]  789.46µs, 1.186ms, 1.169ms, 1.339ms, 1.397ms, 1.548ms, 12.783ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:30000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 0.00
Duration      [total, attack, wait]             8m0s, 8m0s, 990.414µs
Latencies     [min, mean, 50, 90, 95, 99, max]  720.643µs, 1.08ms, 1.061ms, 1.233ms, 1.294ms, 1.437ms, 12.402ms
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
Duration      [total, attack, wait]             8m0s, 8m0s, 877.674µs
Latencies     [min, mean, 50, 90, 95, 99, max]  385.598µs, 815.698µs, 817.489µs, 958.677µs, 1.005ms, 1.207ms, 18.31ms
Bytes In      [total, mean]                     7689658, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 989.421µs
Latencies     [min, mean, 50, 90, 95, 99, max]  720.816µs, 1.105ms, 1.093ms, 1.261ms, 1.312ms, 1.44ms, 9.362ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 457.929µs
Latencies     [min, mean, 50, 90, 95, 99, max]  406.494µs, 822.306µs, 823.099µs, 962.408µs, 1.011ms, 1.197ms, 12.046ms
Bytes In      [total, mean]                     1922457, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 966.535µs
Latencies     [min, mean, 50, 90, 95, 99, max]  715.486µs, 1.037ms, 1.01ms, 1.187ms, 1.267ms, 1.434ms, 8.042ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 905.33µs
Latencies     [min, mean, 50, 90, 95, 99, max]  396.845µs, 823.166µs, 827.937µs, 962.609µs, 1.003ms, 1.11ms, 11.844ms
Bytes In      [total, mean]                     1922378, 160.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1ms
Latencies     [min, mean, 50, 90, 95, 99, max]  765.05µs, 1.156ms, 1.13ms, 1.336ms, 1.411ms, 1.573ms, 12.854ms
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
Duration      [total, attack, wait]             5m0s, 5m0s, 893.59µs
Latencies     [min, mean, 50, 90, 95, 99, max]  399.188µs, 825.149µs, 821.595µs, 965.205µs, 1.02ms, 1.331ms, 12.552ms
Bytes In      [total, mean]                     4808855, 160.30
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
Duration      [total, attack, wait]             16m0s, 16m0s, 961.846µs
Latencies     [min, mean, 50, 90, 95, 99, max]  724.442µs, 1.112ms, 1.095ms, 1.263ms, 1.321ms, 1.457ms, 48.175ms
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
Duration      [total, attack, wait]             16m0s, 16m0s, 883.874µs
Latencies     [min, mean, 50, 90, 95, 99, max]  374.619µs, 827.731µs, 823.943µs, 964.524µs, 1.015ms, 1.313ms, 42.213ms
Bytes In      [total, mean]                     15388657, 160.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.024ms
Latencies     [min, mean, 50, 90, 95, 99, max]  771.219µs, 1.129ms, 1.122ms, 1.288ms, 1.343ms, 1.462ms, 7.033ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 694.891µs
Latencies     [min, mean, 50, 90, 95, 99, max]  390.633µs, 824.207µs, 827.569µs, 964.336µs, 1.012ms, 1.21ms, 6.663ms
Bytes In      [total, mean]                     1923639, 160.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 876.171µs
Latencies     [min, mean, 50, 90, 95, 99, max]  694.98µs, 1.056ms, 1.031ms, 1.222ms, 1.292ms, 1.456ms, 6.709ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 775.476µs
Latencies     [min, mean, 50, 90, 95, 99, max]  412.964µs, 823.831µs, 827.987µs, 961.436µs, 1.003ms, 1.157ms, 7.581ms
Bytes In      [total, mean]                     1923523, 160.29
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)
