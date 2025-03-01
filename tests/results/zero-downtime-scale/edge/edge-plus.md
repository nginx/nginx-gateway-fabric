# Results

## Test environment

NGINX Plus: true

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

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 0.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.094ms
Latencies     [min, mean, 50, 90, 95, 99, max]  808.948µs, 1.204ms, 1.185ms, 1.368ms, 1.433ms, 1.584ms, 12.427ms
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
Duration      [total, attack, wait]             5m0s, 5m0s, 610.753µs
Latencies     [min, mean, 50, 90, 95, 99, max]  401.638µs, 879.764µs, 866.774µs, 1.061ms, 1.129ms, 1.367ms, 13.59ms
Bytes In      [total, mean]                     4805933, 160.20
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.089ms
Latencies     [min, mean, 50, 90, 95, 99, max]  750.184µs, 1.186ms, 1.168ms, 1.365ms, 1.431ms, 1.593ms, 14.782ms
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
Duration      [total, attack, wait]             8m0s, 8m0s, 877.989µs
Latencies     [min, mean, 50, 90, 95, 99, max]  403.647µs, 911.685µs, 899.759µs, 1.116ms, 1.195ms, 1.444ms, 13.395ms
Bytes In      [total, mean]                     7689596, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.171ms
Latencies     [min, mean, 50, 90, 95, 99, max]  816.218µs, 1.17ms, 1.151ms, 1.338ms, 1.406ms, 1.565ms, 10.031ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.071ms
Latencies     [min, mean, 50, 90, 95, 99, max]  432.772µs, 867.435µs, 860.512µs, 1.051ms, 1.114ms, 1.313ms, 6.262ms
Bytes In      [total, mean]                     1922385, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 975.274µs
Latencies     [min, mean, 50, 90, 95, 99, max]  748.734µs, 1.087ms, 1.061ms, 1.238ms, 1.317ms, 1.516ms, 7.34ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 776.597µs
Latencies     [min, mean, 50, 90, 95, 99, max]  394.222µs, 897.953µs, 899.811µs, 1.1ms, 1.16ms, 1.351ms, 4.176ms
Bytes In      [total, mean]                     1922460, 160.21
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.208ms
Latencies     [min, mean, 50, 90, 95, 99, max]  803.162µs, 1.259ms, 1.229ms, 1.462ms, 1.545ms, 1.736ms, 12.393ms
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
Duration      [total, attack, wait]             5m0s, 5m0s, 784.125µs
Latencies     [min, mean, 50, 90, 95, 99, max]  421.539µs, 925.356µs, 909.853µs, 1.134ms, 1.216ms, 1.498ms, 12.555ms
Bytes In      [total, mean]                     4806016, 160.20
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.343ms
Latencies     [min, mean, 50, 90, 95, 99, max]  786.787µs, 1.224ms, 1.205ms, 1.407ms, 1.48ms, 1.643ms, 26.499ms
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.318ms
Latencies     [min, mean, 50, 90, 95, 99, max]  414.864µs, 941.759µs, 922.044µs, 1.18ms, 1.271ms, 1.513ms, 11.695ms
Bytes In      [total, mean]                     15379180, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.29ms
Latencies     [min, mean, 50, 90, 95, 99, max]  887.779µs, 1.219ms, 1.202ms, 1.378ms, 1.443ms, 1.602ms, 9.674ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.025ms
Latencies     [min, mean, 50, 90, 95, 99, max]  430.709µs, 924.922µs, 907.463µs, 1.134ms, 1.213ms, 1.472ms, 11.55ms
Bytes In      [total, mean]                     1922422, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.187ms
Latencies     [min, mean, 50, 90, 95, 99, max]  826.068µs, 1.236ms, 1.222ms, 1.388ms, 1.443ms, 1.574ms, 10.468ms
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
Duration      [total, attack, wait]             2m0s, 2m0s, 986.571µs
Latencies     [min, mean, 50, 90, 95, 99, max]  441.865µs, 1.056ms, 1.059ms, 1.308ms, 1.381ms, 1.546ms, 10.485ms
Bytes In      [total, mean]                     1922409, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
