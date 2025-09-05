# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.574ms
Latencies     [min, mean, 50, 90, 95, 99, max]  641.287µs, 1.275ms, 1.27ms, 1.472ms, 1.545ms, 1.84ms, 18.199ms
Bytes In      [total, mean]                     4806021, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.671ms
Latencies     [min, mean, 50, 90, 95, 99, max]  745.878µs, 1.352ms, 1.336ms, 1.54ms, 1.62ms, 1.958ms, 17.865ms
Bytes In      [total, mean]                     4626082, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.323ms
Latencies     [min, mean, 50, 90, 95, 99, max]  724.782µs, 1.483ms, 1.399ms, 1.711ms, 1.826ms, 2.104ms, 251.482ms
Bytes In      [total, mean]                     7401528, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.213ms
Latencies     [min, mean, 50, 90, 95, 99, max]  287.087µs, 1.402ms, 1.336ms, 1.651ms, 1.77ms, 2.023ms, 249.966ms
Bytes In      [total, mean]                     7689352, 160.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      0:1  200:47999  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.62:80: connect: network is unreachable
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.265ms
Latencies     [min, mean, 50, 90, 95, 99, max]  715.571µs, 1.288ms, 1.266ms, 1.463ms, 1.55ms, 1.82ms, 58.394ms
Bytes In      [total, mean]                     1850408, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-oss.png](abrupt-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.334ms
Latencies     [min, mean, 50, 90, 95, 99, max]  698.716µs, 1.276ms, 1.264ms, 1.48ms, 1.569ms, 1.841ms, 16.393ms
Bytes In      [total, mean]                     1922424, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.042ms
Latencies     [min, mean, 50, 90, 95, 99, max]  747.757µs, 1.27ms, 1.268ms, 1.43ms, 1.486ms, 1.674ms, 12.014ms
Bytes In      [total, mean]                     1850370, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 869.446µs
Latencies     [min, mean, 50, 90, 95, 99, max]  668.389µs, 1.218ms, 1.227ms, 1.395ms, 1.445ms, 1.601ms, 4.255ms
Bytes In      [total, mean]                     1922406, 160.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.365ms
Latencies     [min, mean, 50, 90, 95, 99, max]  636.196µs, 1.283ms, 1.268ms, 1.481ms, 1.566ms, 1.959ms, 34.44ms
Bytes In      [total, mean]                     4806050, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.266ms
Latencies     [min, mean, 50, 90, 95, 99, max]  734.747µs, 1.342ms, 1.315ms, 1.521ms, 1.605ms, 2.048ms, 36.241ms
Bytes In      [total, mean]                     4626078, 154.20
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
Duration      [total, attack, wait]             16m0s, 16m0s, 1.357ms
Latencies     [min, mean, 50, 90, 95, 99, max]  730.286µs, 1.38ms, 1.344ms, 1.563ms, 1.65ms, 1.978ms, 168.501ms
Bytes In      [total, mean]                     14803159, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.225ms
Latencies     [min, mean, 50, 90, 95, 99, max]  678.894µs, 1.329ms, 1.3ms, 1.519ms, 1.608ms, 1.981ms, 166.039ms
Bytes In      [total, mean]                     15379161, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 946.643µs
Latencies     [min, mean, 50, 90, 95, 99, max]  815.06µs, 1.419ms, 1.359ms, 1.55ms, 1.619ms, 1.869ms, 126.26ms
Bytes In      [total, mean]                     1850385, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.061ms
Latencies     [min, mean, 50, 90, 95, 99, max]  726.134µs, 1.334ms, 1.299ms, 1.494ms, 1.559ms, 1.815ms, 42.55ms
Bytes In      [total, mean]                     1922421, 160.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.369ms
Latencies     [min, mean, 50, 90, 95, 99, max]  701.167µs, 1.314ms, 1.31ms, 1.499ms, 1.568ms, 1.743ms, 28.236ms
Bytes In      [total, mean]                     1922400, 160.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.467ms
Latencies     [min, mean, 50, 90, 95, 99, max]  812.416µs, 1.376ms, 1.365ms, 1.553ms, 1.625ms, 1.857ms, 11.707ms
Bytes In      [total, mean]                     1850430, 154.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
