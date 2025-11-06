# Results

## Test environment

NGINX Plus: true

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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.262ms
Latencies     [min, mean, 50, 90, 95, 99, max]  604.971µs, 1.089ms, 1.071ms, 1.237ms, 1.318ms, 1.575ms, 17.179ms
Bytes In      [total, mean]                     4650000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.102ms
Latencies     [min, mean, 50, 90, 95, 99, max]  603.196µs, 1.059ms, 1.042ms, 1.224ms, 1.293ms, 1.543ms, 17.18ms
Bytes In      [total, mean]                     4832883, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.137ms
Latencies     [min, mean, 50, 90, 95, 99, max]  585.117µs, 1.043ms, 1.033ms, 1.197ms, 1.258ms, 1.483ms, 25.897ms
Bytes In      [total, mean]                     7732706, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.037ms
Latencies     [min, mean, 50, 90, 95, 99, max]  635.972µs, 1.077ms, 1.07ms, 1.21ms, 1.271ms, 1.513ms, 33.327ms
Bytes In      [total, mean]                     7440000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.259ms
Latencies     [min, mean, 50, 90, 95, 99, max]  613.422µs, 1.051ms, 1.042ms, 1.224ms, 1.288ms, 1.49ms, 2.917ms
Bytes In      [total, mean]                     1933181, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.18ms
Latencies     [min, mean, 50, 90, 95, 99, max]  624.028µs, 1.089ms, 1.074ms, 1.257ms, 1.334ms, 1.59ms, 11.64ms
Bytes In      [total, mean]                     1860000, 155.00
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.362ms
Latencies     [min, mean, 50, 90, 95, 99, max]  671.744µs, 1.071ms, 1.064ms, 1.186ms, 1.231ms, 1.368ms, 69.111ms
Bytes In      [total, mean]                     1860000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.119ms
Latencies     [min, mean, 50, 90, 95, 99, max]  589.355µs, 1.034ms, 1.024ms, 1.176ms, 1.219ms, 1.336ms, 55.375ms
Bytes In      [total, mean]                     1933207, 161.10
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
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 2.776ms
Latencies     [min, mean, 50, 90, 95, 99, max]  181.378µs, 1.303ms, 1.17ms, 1.523ms, 1.646ms, 3.663ms, 249.866ms
Bytes In      [total, mean]                     4649535, 154.98
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:3  200:29997  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.120:443: connect: network is unreachable
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 1.465ms
Latencies     [min, mean, 50, 90, 95, 99, max]  186.771µs, 1.245ms, 1.125ms, 1.487ms, 1.611ms, 3.606ms, 211.76ms
Bytes In      [total, mean]                     4832124, 161.07
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.98%
Status Codes  [code:count]                      0:5  200:29995  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.120:80: connect: network is unreachable
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.175ms
Latencies     [min, mean, 50, 90, 95, 99, max]  617.196µs, 1.136ms, 1.125ms, 1.31ms, 1.379ms, 1.634ms, 45.469ms
Bytes In      [total, mean]                     15465457, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.154ms
Latencies     [min, mean, 50, 90, 95, 99, max]  658.233µs, 1.167ms, 1.151ms, 1.319ms, 1.386ms, 1.647ms, 46.055ms
Bytes In      [total, mean]                     14880000, 155.00
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
Duration      [total, attack, wait]             2m0s, 2m0s, 968.661µs
Latencies     [min, mean, 50, 90, 95, 99, max]  610.048µs, 1.079ms, 1.071ms, 1.232ms, 1.283ms, 1.495ms, 12.744ms
Bytes In      [total, mean]                     1933166, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 905.036µs
Latencies     [min, mean, 50, 90, 95, 99, max]  657.673µs, 1.121ms, 1.112ms, 1.264ms, 1.325ms, 1.557ms, 12.948ms
Bytes In      [total, mean]                     1860000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 983.56µs
Latencies     [min, mean, 50, 90, 95, 99, max]  695.284µs, 1.197ms, 1.137ms, 1.289ms, 1.352ms, 1.585ms, 117.338ms
Bytes In      [total, mean]                     1860000, 155.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.165ms
Latencies     [min, mean, 50, 90, 95, 99, max]  648.424µs, 1.134ms, 1.095ms, 1.265ms, 1.319ms, 1.478ms, 117.435ms
Bytes In      [total, mean]                     1933194, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
