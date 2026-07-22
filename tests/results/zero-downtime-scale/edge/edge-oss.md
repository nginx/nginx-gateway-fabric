# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 1.437ms
Latencies     [min, mean, 50, 90, 95, 99, max]  164.713µs, 1.242ms, 1.163ms, 1.466ms, 1.561ms, 2.266ms, 249.944ms
Bytes In      [total, mean]                     4835334, 161.18
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:4  200:29996  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.110:80: connect: network is unreachable
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 1.409ms
Latencies     [min, mean, 50, 90, 95, 99, max]  245.788µs, 1.297ms, 1.201ms, 1.511ms, 1.615ms, 2.296ms, 213.657ms
Bytes In      [total, mean]                     4655226, 155.17
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.98%
Status Codes  [code:count]                      0:5  200:29995  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.110:443: connect: network is unreachable
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.071ms
Latencies     [min, mean, 50, 90, 95, 99, max]  616.957µs, 1.089ms, 1.068ms, 1.282ms, 1.358ms, 1.716ms, 35.229ms
Bytes In      [total, mean]                     7737547, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.064ms
Latencies     [min, mean, 50, 90, 95, 99, max]  673.291µs, 1.163ms, 1.117ms, 1.347ms, 1.429ms, 1.847ms, 217.607ms
Bytes In      [total, mean]                     7449675, 155.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 972.218µs
Latencies     [min, mean, 50, 90, 95, 99, max]  688.487µs, 1.142ms, 1.12ms, 1.315ms, 1.383ms, 1.745ms, 52.048ms
Bytes In      [total, mean]                     1934407, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.082ms
Latencies     [min, mean, 50, 90, 95, 99, max]  689.8µs, 1.169ms, 1.144ms, 1.352ms, 1.421ms, 1.786ms, 13.642ms
Bytes In      [total, mean]                     1862359, 155.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 836.965µs
Latencies     [min, mean, 50, 90, 95, 99, max]  705.002µs, 1.145ms, 1.105ms, 1.266ms, 1.324ms, 1.617ms, 211.33ms
Bytes In      [total, mean]                     1862342, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.066ms
Latencies     [min, mean, 50, 90, 95, 99, max]  634.984µs, 1.103ms, 1.07ms, 1.242ms, 1.298ms, 1.515ms, 219.29ms
Bytes In      [total, mean]                     1934419, 161.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 980.303µs
Latencies     [min, mean, 50, 90, 95, 99, max]  620.49µs, 1.107ms, 1.061ms, 1.247ms, 1.321ms, 2.021ms, 215.402ms
Bytes In      [total, mean]                     4835963, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 727.852µs
Latencies     [min, mean, 50, 90, 95, 99, max]  668.649µs, 1.157ms, 1.099ms, 1.291ms, 1.371ms, 2.109ms, 215.894ms
Bytes In      [total, mean]                     4655915, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.147ms
Latencies     [min, mean, 50, 90, 95, 99, max]  684.43µs, 1.177ms, 1.135ms, 1.326ms, 1.411ms, 1.947ms, 218.617ms
Bytes In      [total, mean]                     14899024, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.03ms
Latencies     [min, mean, 50, 90, 95, 99, max]  623.653µs, 1.14ms, 1.108ms, 1.299ms, 1.377ms, 1.907ms, 219.069ms
Bytes In      [total, mean]                     15475377, 161.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.261ms
Latencies     [min, mean, 50, 90, 95, 99, max]  650.583µs, 1.165ms, 1.122ms, 1.279ms, 1.345ms, 1.796ms, 105.75ms
Bytes In      [total, mean]                     1934326, 161.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-oss.png](abrupt-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.172ms
Latencies     [min, mean, 50, 90, 95, 99, max]  714.063µs, 1.219ms, 1.162ms, 1.33ms, 1.395ms, 1.929ms, 107.599ms
Bytes In      [total, mean]                     1862523, 155.21
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
Duration      [total, attack, wait]             2m0s, 2m0s, 829.559µs
Latencies     [min, mean, 50, 90, 95, 99, max]  719.77µs, 1.109ms, 1.099ms, 1.248ms, 1.301ms, 1.603ms, 21.258ms
Bytes In      [total, mean]                     1934356, 161.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 933.774µs
Latencies     [min, mean, 50, 90, 95, 99, max]  703.386µs, 1.174ms, 1.135ms, 1.283ms, 1.338ms, 1.669ms, 216.588ms
Bytes In      [total, mean]                     1862370, 155.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
