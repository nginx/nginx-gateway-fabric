# Results

## Test environment

NGINX Plus: true

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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.126ms
Latencies     [min, mean, 50, 90, 95, 99, max]  743.034µs, 1.167ms, 1.149ms, 1.306ms, 1.362ms, 1.536ms, 12.554ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:30000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 919.391µs
Latencies     [min, mean, 50, 90, 95, 99, max]  415.673µs, 896.236µs, 880.46µs, 1.05ms, 1.128ms, 1.586ms, 14.815ms
Bytes In      [total, mean]                     4805954, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 0.00
Duration      [total, attack, wait]             8m0s, 8m0s, 963.988µs
Latencies     [min, mean, 50, 90, 95, 99, max]  776.291µs, 1.13ms, 1.111ms, 1.288ms, 1.35ms, 1.52ms, 19.641ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:48000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 865.036µs
Latencies     [min, mean, 50, 90, 95, 99, max]  406.588µs, 874.557µs, 867.338µs, 1.034ms, 1.106ms, 1.424ms, 12.86ms
Bytes In      [total, mean]                     7689623, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 1.183ms
Latencies     [min, mean, 50, 90, 95, 99, max]  788.939µs, 1.187ms, 1.173ms, 1.327ms, 1.378ms, 1.525ms, 8.297ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 963.749µs
Latencies     [min, mean, 50, 90, 95, 99, max]  465.93µs, 901.829µs, 894.352µs, 1.058ms, 1.122ms, 1.419ms, 8.285ms
Bytes In      [total, mean]                     1922423, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 943.87µs
Latencies     [min, mean, 50, 90, 95, 99, max]  796.866µs, 1.128ms, 1.102ms, 1.273ms, 1.344ms, 1.525ms, 9.394ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 670.1µs
Latencies     [min, mean, 50, 90, 95, 99, max]  461.755µs, 893.812µs, 891.241µs, 1.045ms, 1.101ms, 1.3ms, 8.938ms
Bytes In      [total, mean]                     1922478, 160.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

## Multiple NGF Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 0.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.19ms
Latencies     [min, mean, 50, 90, 95, 99, max]  858.7µs, 1.242ms, 1.217ms, 1.412ms, 1.483ms, 1.664ms, 14.623ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:30000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 900.27µs
Latencies     [min, mean, 50, 90, 95, 99, max]  430.177µs, 901.388µs, 888.439µs, 1.073ms, 1.149ms, 1.483ms, 13.7ms
Bytes In      [total, mean]                     4806054, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 0.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.031ms
Latencies     [min, mean, 50, 90, 95, 99, max]  785.977µs, 1.187ms, 1.171ms, 1.342ms, 1.407ms, 1.56ms, 13.904ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:96000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 778.233µs
Latencies     [min, mean, 50, 90, 95, 99, max]  432.092µs, 892.532µs, 883.987µs, 1.054ms, 1.127ms, 1.449ms, 15.863ms
Bytes In      [total, mean]                     15379267, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 1.172ms
Latencies     [min, mean, 50, 90, 95, 99, max]  830.87µs, 1.181ms, 1.167ms, 1.318ms, 1.38ms, 1.564ms, 8.596ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 755.064µs
Latencies     [min, mean, 50, 90, 95, 99, max]  426.988µs, 854.651µs, 850.22µs, 992.666µs, 1.047ms, 1.305ms, 9.343ms
Bytes In      [total, mean]                     1922343, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 0.00
Duration      [total, attack, wait]             2m0s, 2m0s, 863.355µs
Latencies     [min, mean, 50, 90, 95, 99, max]  713.621µs, 1.09ms, 1.057ms, 1.268ms, 1.349ms, 1.521ms, 9.522ms
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:12000  
Error Set:
Get "https://cafe.example.com/tea": remote error: tls: unrecognized name
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 690.251µs
Latencies     [min, mean, 50, 90, 95, 99, max]  413.604µs, 866.429µs, 860.042µs, 1.031ms, 1.093ms, 1.304ms, 9.603ms
Bytes In      [total, mean]                     1922447, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
