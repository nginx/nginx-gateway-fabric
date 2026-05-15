# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.238ms
Latencies     [min, mean, 50, 90, 95, 99, max]  633.208µs, 1.137ms, 1.111ms, 1.331ms, 1.419ms, 1.801ms, 17.737ms
Bytes In      [total, mean]                     4596118, 153.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.514ms
Latencies     [min, mean, 50, 90, 95, 99, max]  592.997µs, 1.059ms, 1.042ms, 1.24ms, 1.32ms, 1.655ms, 20.284ms
Bytes In      [total, mean]                     4776098, 159.20
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
Duration      [total, attack, wait]             8m0s, 8m0s, 1.28ms
Latencies     [min, mean, 50, 90, 95, 99, max]  589.973µs, 1.091ms, 1.073ms, 1.279ms, 1.353ms, 1.643ms, 38.806ms
Bytes In      [total, mean]                     7641676, 159.20
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.181ms
Latencies     [min, mean, 50, 90, 95, 99, max]  629.979µs, 1.145ms, 1.118ms, 1.323ms, 1.401ms, 1.749ms, 39.072ms
Bytes In      [total, mean]                     7353672, 153.20
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.307ms
Latencies     [min, mean, 50, 90, 95, 99, max]  657.765µs, 1.274ms, 1.239ms, 1.473ms, 1.566ms, 1.86ms, 83.599ms
Bytes In      [total, mean]                     1838294, 153.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.052ms
Latencies     [min, mean, 50, 90, 95, 99, max]  567.122µs, 1.172ms, 1.136ms, 1.389ms, 1.505ms, 1.811ms, 76.668ms
Bytes In      [total, mean]                     1910314, 159.19
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

### Scale Down Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 853.682µs
Latencies     [min, mean, 50, 90, 95, 99, max]  646.981µs, 1.188ms, 1.175ms, 1.36ms, 1.426ms, 1.637ms, 27.958ms
Bytes In      [total, mean]                     1838562, 153.21
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.133ms
Latencies     [min, mean, 50, 90, 95, 99, max]  627.201µs, 1.105ms, 1.098ms, 1.281ms, 1.334ms, 1.475ms, 27.875ms
Bytes In      [total, mean]                     1910426, 159.20
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
Duration      [total, attack, wait]             5m0s, 5m0s, 1.292ms
Latencies     [min, mean, 50, 90, 95, 99, max]  197.164µs, 1.362ms, 1.254ms, 1.674ms, 1.877ms, 3.194ms, 206.649ms
Bytes In      [total, mean]                     4610365, 153.68
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.98%
Status Codes  [code:count]                      0:5  200:29995  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.82:443: connect: network is unreachable
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 99.99
Duration      [total, attack, wait]             5m0s, 5m0s, 1.031ms
Latencies     [min, mean, 50, 90, 95, 99, max]  194.02µs, 1.303ms, 1.208ms, 1.569ms, 1.731ms, 2.937ms, 209.3ms
Bytes In      [total, mean]                     4787510, 159.58
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.99%
Status Codes  [code:count]                      0:4  200:29996  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.82:80: connect: network is unreachable
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.261ms
Latencies     [min, mean, 50, 90, 95, 99, max]  609.752µs, 1.167ms, 1.122ms, 1.396ms, 1.529ms, 2.093ms, 50.057ms
Bytes In      [total, mean]                     14755074, 153.70
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.074ms
Latencies     [min, mean, 50, 90, 95, 99, max]  558.58µs, 1.106ms, 1.072ms, 1.338ms, 1.452ms, 1.899ms, 47.355ms
Bytes In      [total, mean]                     15321629, 159.60
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.24ms
Latencies     [min, mean, 50, 90, 95, 99, max]  577.394µs, 1.213ms, 1.104ms, 1.391ms, 1.522ms, 1.96ms, 180.091ms
Bytes In      [total, mean]                     1915348, 159.61
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.359ms
Latencies     [min, mean, 50, 90, 95, 99, max]  623.355µs, 1.284ms, 1.124ms, 1.532ms, 1.732ms, 2.169ms, 179.759ms
Bytes In      [total, mean]                     1844448, 153.70
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
Duration      [total, attack, wait]             2m0s, 2m0s, 726.69µs
Latencies     [min, mean, 50, 90, 95, 99, max]  625.84µs, 1.074ms, 1.068ms, 1.233ms, 1.289ms, 1.448ms, 23.358ms
Bytes In      [total, mean]                     1844328, 153.69
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 908.857µs
Latencies     [min, mean, 50, 90, 95, 99, max]  601.865µs, 1.032ms, 1.026ms, 1.202ms, 1.261ms, 1.4ms, 10.229ms
Bytes In      [total, mean]                     1915076, 159.59
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
