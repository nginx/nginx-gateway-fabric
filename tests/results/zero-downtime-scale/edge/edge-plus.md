# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 09f31a0defdd4bf13c648139f55567bf908cfaac
- Date: 2026-04-15T14:59:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848324Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.164ms
Latencies     [min, mean, 50, 90, 95, 99, max]  583.604µs, 1.085ms, 1.062ms, 1.273ms, 1.351ms, 1.731ms, 12.644ms
Bytes In      [total, mean]                     4595931, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 984.995µs
Latencies     [min, mean, 50, 90, 95, 99, max]  592.045µs, 1.025ms, 1.01ms, 1.203ms, 1.269ms, 1.602ms, 12.6ms
Bytes In      [total, mean]                     4775843, 159.19
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.267ms
Latencies     [min, mean, 50, 90, 95, 99, max]  595.634µs, 1.092ms, 1.061ms, 1.279ms, 1.355ms, 1.669ms, 61.232ms
Bytes In      [total, mean]                     7353414, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 974.606µs
Latencies     [min, mean, 50, 90, 95, 99, max]  550.491µs, 1.019ms, 997.009µs, 1.211ms, 1.285ms, 1.551ms, 49.543ms
Bytes In      [total, mean]                     7641690, 159.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.32ms
Latencies     [min, mean, 50, 90, 95, 99, max]  623.1µs, 1.124ms, 1.097ms, 1.312ms, 1.386ms, 1.607ms, 73.792ms
Bytes In      [total, mean]                     1838437, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.241ms
Latencies     [min, mean, 50, 90, 95, 99, max]  622.969µs, 1.056ms, 1.036ms, 1.22ms, 1.283ms, 1.508ms, 59.195ms
Bytes In      [total, mean]                     1910465, 159.21
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.169ms
Latencies     [min, mean, 50, 90, 95, 99, max]  625.06µs, 1.07ms, 1.061ms, 1.247ms, 1.306ms, 1.461ms, 29.369ms
Bytes In      [total, mean]                     1910453, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.243ms
Latencies     [min, mean, 50, 90, 95, 99, max]  651.383µs, 1.157ms, 1.14ms, 1.328ms, 1.393ms, 1.563ms, 31.931ms
Bytes In      [total, mean]                     1838384, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.151ms
Latencies     [min, mean, 50, 90, 95, 99, max]  564.042µs, 1.038ms, 1.019ms, 1.209ms, 1.276ms, 1.725ms, 18.257ms
Bytes In      [total, mean]                     4788053, 159.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 966.126µs
Latencies     [min, mean, 50, 90, 95, 99, max]  637.891µs, 1.108ms, 1.085ms, 1.272ms, 1.343ms, 1.799ms, 22.552ms
Bytes In      [total, mean]                     4607922, 153.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.114ms
Latencies     [min, mean, 50, 90, 95, 99, max]  367.638µs, 1.181ms, 1.092ms, 1.396ms, 1.521ms, 1.874ms, 209.11ms
Bytes In      [total, mean]                     14745494, 153.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      0:1  200:95999  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.6:443: connect: network is unreachable
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.111ms
Latencies     [min, mean, 50, 90, 95, 99, max]  553.393µs, 1.129ms, 1.044ms, 1.34ms, 1.475ms, 1.819ms, 251.515ms
Bytes In      [total, mean]                     15321533, 159.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.028ms
Latencies     [min, mean, 50, 90, 95, 99, max]  595.008µs, 1.057ms, 1.001ms, 1.182ms, 1.253ms, 1.684ms, 149.654ms
Bytes In      [total, mean]                     1915213, 159.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.193ms
Latencies     [min, mean, 50, 90, 95, 99, max]  602.617µs, 1.104ms, 1.029ms, 1.246ms, 1.334ms, 1.782ms, 151.276ms
Bytes In      [total, mean]                     1843334, 153.61
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
Duration      [total, attack, wait]             2m0s, 2m0s, 896.749µs
Latencies     [min, mean, 50, 90, 95, 99, max]  580.695µs, 965.232µs, 956.967µs, 1.106ms, 1.152ms, 1.27ms, 2.996ms
Bytes In      [total, mean]                     1915185, 159.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 662.083µs
Latencies     [min, mean, 50, 90, 95, 99, max]  619.918µs, 1.032ms, 1.015ms, 1.207ms, 1.292ms, 1.495ms, 21.632ms
Bytes In      [total, mean]                     1843116, 153.59
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
