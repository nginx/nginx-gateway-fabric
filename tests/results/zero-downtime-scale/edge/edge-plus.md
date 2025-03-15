# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9155a2b6a8d3179165797ef3e789e97283f7a695
- Date: 2025-03-15T07:17:11Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.6-gke.1020000
- vCPUs per node: 16
- RAM per node: 65851340Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGF Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 766.303µs
Latencies     [min, mean, 50, 90, 95, 99, max]  441.594µs, 875.579µs, 868.868µs, 997.175µs, 1.049ms, 1.357ms, 13.238ms
Bytes In      [total, mean]                     4673932, 155.80
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 727.187µs
Latencies     [min, mean, 50, 90, 95, 99, max]  414.641µs, 846.924µs, 846.028µs, 971.491µs, 1.017ms, 1.294ms, 11.941ms
Bytes In      [total, mean]                     4850987, 161.70
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
Duration      [total, attack, wait]             8m0s, 8m0s, 730.887µs
Latencies     [min, mean, 50, 90, 95, 99, max]  433.836µs, 850.845µs, 848.555µs, 968.862µs, 1.013ms, 1.215ms, 8.39ms
Bytes In      [total, mean]                     7478267, 155.80
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-plus.png](gradual-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 715.71µs
Latencies     [min, mean, 50, 90, 95, 99, max]  405.345µs, 820.868µs, 825.255µs, 941.274µs, 982.586µs, 1.188ms, 11.166ms
Bytes In      [total, mean]                     7761660, 161.70
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 731.03µs
Latencies     [min, mean, 50, 90, 95, 99, max]  423.637µs, 822.777µs, 824.747µs, 937.12µs, 981.297µs, 1.14ms, 3.973ms
Bytes In      [total, mean]                     1940496, 161.71
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-plus.png](abrupt-scale-up-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 887.292µs
Latencies     [min, mean, 50, 90, 95, 99, max]  442.281µs, 858.712µs, 854.673µs, 973.029µs, 1.017ms, 1.179ms, 10.485ms
Bytes In      [total, mean]                     1869632, 155.80
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

### Scale Down Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 804.798µs
Latencies     [min, mean, 50, 90, 95, 99, max]  424.072µs, 837.824µs, 838.15µs, 963.636µs, 1.006ms, 1.123ms, 44.463ms
Bytes In      [total, mean]                     1940409, 161.70
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 685.714µs
Latencies     [min, mean, 50, 90, 95, 99, max]  459.498µs, 865.342µs, 861.335µs, 990.99µs, 1.035ms, 1.151ms, 48.501ms
Bytes In      [total, mean]                     1869571, 155.80
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

## Multiple NGF Pods run per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 499.02µs
Latencies     [min, mean, 50, 90, 95, 99, max]  404.188µs, 862.699µs, 858.402µs, 1.003ms, 1.053ms, 1.359ms, 11.022ms
Bytes In      [total, mean]                     4862948, 162.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 917.782µs
Latencies     [min, mean, 50, 90, 95, 99, max]  452.74µs, 884.958µs, 873.544µs, 1.017ms, 1.07ms, 1.42ms, 11.641ms
Bytes In      [total, mean]                     4682982, 156.10
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
Duration      [total, attack, wait]             16m0s, 16m0s, 760.896µs
Latencies     [min, mean, 50, 90, 95, 99, max]  433.285µs, 934.463µs, 905.034µs, 1.107ms, 1.202ms, 1.549ms, 83.045ms
Bytes In      [total, mean]                     14985575, 156.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 797.537µs
Latencies     [min, mean, 50, 90, 95, 99, max]  389.802µs, 906.16µs, 872.26µs, 1.081ms, 1.271ms, 1.729ms, 78.489ms
Bytes In      [total, mean]                     15561579, 162.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

### Scale Up Abruptly

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.038ms
Latencies     [min, mean, 50, 90, 95, 99, max]  438.072µs, 859.877µs, 851.049µs, 991.439µs, 1.042ms, 1.261ms, 9.194ms
Bytes In      [total, mean]                     1873263, 156.11
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 745.836µs
Latencies     [min, mean, 50, 90, 95, 99, max]  397.717µs, 825.498µs, 823.88µs, 955.33µs, 1.002ms, 1.198ms, 9.229ms
Bytes In      [total, mean]                     1945082, 162.09
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
Duration      [total, attack, wait]             2m0s, 2m0s, 853.74µs
Latencies     [min, mean, 50, 90, 95, 99, max]  434.496µs, 857.503µs, 848.746µs, 975.447µs, 1.022ms, 1.187ms, 26.289ms
Bytes In      [total, mean]                     1945253, 162.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 849.564µs
Latencies     [min, mean, 50, 90, 95, 99, max]  453.708µs, 899.405µs, 881.221µs, 1.024ms, 1.074ms, 1.234ms, 8.51ms
Bytes In      [total, mean]                     1873266, 156.11
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
