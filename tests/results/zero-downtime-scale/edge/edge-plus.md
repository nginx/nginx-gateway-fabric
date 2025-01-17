# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGF Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 733.336µs
Latencies     [min, mean, 50, 90, 95, 99, max]  423.147µs, 852.064µs, 822.379µs, 1.005ms, 1.076ms, 1.368ms, 22.564ms
Bytes In      [total, mean]                     4593076, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 712.018µs
Latencies     [min, mean, 50, 90, 95, 99, max]  405.38µs, 811.749µs, 790.554µs, 951.791µs, 1.017ms, 1.343ms, 22.455ms
Bytes In      [total, mean]                     4772952, 159.10
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
Duration      [total, attack, wait]             8m0s, 8m0s, 902.364µs
Latencies     [min, mean, 50, 90, 95, 99, max]  376.108µs, 808.123µs, 798.541µs, 956.386µs, 1.016ms, 1.282ms, 12.476ms
Bytes In      [total, mean]                     7636780, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 789.729µs
Latencies     [min, mean, 50, 90, 95, 99, max]  402.835µs, 834.036µs, 820.453µs, 980.9µs, 1.043ms, 1.297ms, 14.647ms
Bytes In      [total, mean]                     7348885, 153.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 774.584µs
Latencies     [min, mean, 50, 90, 95, 99, max]  442.333µs, 852.903µs, 838.838µs, 1.007ms, 1.07ms, 1.277ms, 12.657ms
Bytes In      [total, mean]                     1837235, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 810.786µs
Latencies     [min, mean, 50, 90, 95, 99, max]  414.574µs, 807.385µs, 800.152µs, 943.002µs, 993.994µs, 1.207ms, 12.126ms
Bytes In      [total, mean]                     1909145, 159.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 764.33µs
Latencies     [min, mean, 50, 90, 95, 99, max]  455.707µs, 856.722µs, 842.535µs, 990.87µs, 1.049ms, 1.202ms, 37.702ms
Bytes In      [total, mean]                     1837263, 153.11
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 872.619µs
Latencies     [min, mean, 50, 90, 95, 99, max]  423.472µs, 817.533µs, 812.751µs, 951.811µs, 1.004ms, 1.192ms, 51.785ms
Bytes In      [total, mean]                     1909219, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

## Multiple NGF Pods run per node Test Results

### Scale Up Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 513.348µs
Latencies     [min, mean, 50, 90, 95, 99, max]  394.843µs, 842.052µs, 826.969µs, 987.993µs, 1.055ms, 1.398ms, 11.385ms
Bytes In      [total, mean]                     4778986, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 750.427µs
Latencies     [min, mean, 50, 90, 95, 99, max]  432.953µs, 863.141µs, 840.391µs, 1.008ms, 1.08ms, 1.454ms, 16.966ms
Bytes In      [total, mean]                     4602020, 153.40
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
Duration      [total, attack, wait]             16m0s, 16m0s, 815.462µs
Latencies     [min, mean, 50, 90, 95, 99, max]  426.849µs, 867.772µs, 842.932µs, 1.036ms, 1.115ms, 1.383ms, 52.956ms
Bytes In      [total, mean]                     14726306, 153.40
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-plus.png](gradual-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 526.629µs
Latencies     [min, mean, 50, 90, 95, 99, max]  396.822µs, 829.639µs, 814.625µs, 990.479µs, 1.061ms, 1.342ms, 15.867ms
Bytes In      [total, mean]                     15292918, 159.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 821.462µs
Latencies     [min, mean, 50, 90, 95, 99, max]  432.657µs, 872.62µs, 850.187µs, 1.046ms, 1.121ms, 1.382ms, 10.257ms
Bytes In      [total, mean]                     1840847, 153.40
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-plus.png](abrupt-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 963.019µs
Latencies     [min, mean, 50, 90, 95, 99, max]  408.253µs, 823.044µs, 808.224µs, 978.197µs, 1.041ms, 1.321ms, 9.38ms
Bytes In      [total, mean]                     1911654, 159.30
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
Duration      [total, attack, wait]             2m0s, 2m0s, 862.102µs
Latencies     [min, mean, 50, 90, 95, 99, max]  411.676µs, 828.533µs, 819.66µs, 977.583µs, 1.039ms, 1.226ms, 33.65ms
Bytes In      [total, mean]                     1911634, 159.30
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.104ms
Latencies     [min, mean, 50, 90, 95, 99, max]  468.263µs, 875.117µs, 857.449µs, 1.031ms, 1.099ms, 1.322ms, 24.012ms
Bytes In      [total, mean]                     1840784, 153.40
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)
