# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 991.25µs
Latencies     [min, mean, 50, 90, 95, 99, max]  590.041µs, 1.003ms, 983.857µs, 1.125ms, 1.175ms, 1.649ms, 213.778ms
Bytes In      [total, mean]                     4832964, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-plus.png](gradual-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 911.251µs
Latencies     [min, mean, 50, 90, 95, 99, max]  657.213µs, 1.047ms, 1.023ms, 1.162ms, 1.215ms, 1.665ms, 216.728ms
Bytes In      [total, mean]                     4653104, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.144ms
Latencies     [min, mean, 50, 90, 95, 99, max]  600.824µs, 1.013ms, 995.549µs, 1.139ms, 1.191ms, 1.457ms, 211.395ms
Bytes In      [total, mean]                     7732883, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.137ms
Latencies     [min, mean, 50, 90, 95, 99, max]  636.075µs, 1.064ms, 1.028ms, 1.172ms, 1.229ms, 1.612ms, 217.704ms
Bytes In      [total, mean]                     7444947, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.149ms
Latencies     [min, mean, 50, 90, 95, 99, max]  682.112µs, 1.066ms, 1.042ms, 1.188ms, 1.245ms, 1.466ms, 50.777ms
Bytes In      [total, mean]                     1861178, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 961.707µs
Latencies     [min, mean, 50, 90, 95, 99, max]  617.03µs, 1.008ms, 990.748µs, 1.131ms, 1.18ms, 1.373ms, 51.011ms
Bytes In      [total, mean]                     1933194, 161.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.084ms
Latencies     [min, mean, 50, 90, 95, 99, max]  634.258µs, 1.06ms, 1.014ms, 1.15ms, 1.198ms, 1.387ms, 214.274ms
Bytes In      [total, mean]                     1933218, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.156ms
Latencies     [min, mean, 50, 90, 95, 99, max]  690.001µs, 1.106ms, 1.054ms, 1.184ms, 1.233ms, 1.425ms, 216.613ms
Bytes In      [total, mean]                     1861211, 155.10
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
Duration      [total, attack, wait]             5m0s, 5m0s, 975.29µs
Latencies     [min, mean, 50, 90, 95, 99, max]  614.875µs, 1.031ms, 992.706µs, 1.135ms, 1.191ms, 1.774ms, 214.626ms
Bytes In      [total, mean]                     4832964, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.006ms
Latencies     [min, mean, 50, 90, 95, 99, max]  633.991µs, 1.051ms, 1.029ms, 1.174ms, 1.238ms, 1.779ms, 21.447ms
Bytes In      [total, mean]                     4653060, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.069ms
Latencies     [min, mean, 50, 90, 95, 99, max]  582.594µs, 1.036ms, 1.014ms, 1.171ms, 1.23ms, 1.695ms, 218.584ms
Bytes In      [total, mean]                     15465463, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.013ms
Latencies     [min, mean, 50, 90, 95, 99, max]  667.537µs, 1.111ms, 1.064ms, 1.219ms, 1.283ms, 1.738ms, 218.964ms
Bytes In      [total, mean]                     14889726, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.217ms
Latencies     [min, mean, 50, 90, 95, 99, max]  692.686µs, 1.117ms, 1.059ms, 1.246ms, 1.356ms, 1.793ms, 93.539ms
Bytes In      [total, mean]                     1861202, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.108ms
Latencies     [min, mean, 50, 90, 95, 99, max]  633.957µs, 1.155ms, 1.036ms, 1.218ms, 1.304ms, 1.867ms, 218.597ms
Bytes In      [total, mean]                     1933217, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 963.201µs
Latencies     [min, mean, 50, 90, 95, 99, max]  640.744µs, 1.063ms, 1.055ms, 1.185ms, 1.228ms, 1.394ms, 4.852ms
Bytes In      [total, mean]                     1933222, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.171ms
Latencies     [min, mean, 50, 90, 95, 99, max]  722.416µs, 1.106ms, 1.086ms, 1.22ms, 1.27ms, 1.567ms, 29.442ms
Bytes In      [total, mean]                     1861280, 155.11
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
