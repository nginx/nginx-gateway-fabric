# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: a43969ce15ea40ce0548f0b187b16b9740113f82
- Date: 2026-09-15T14:01:26Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.7-gke.1222000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.228ms
Latencies     [min, mean, 50, 90, 95, 99, max]  676.324µs, 1.193ms, 1.187ms, 1.349ms, 1.414ms, 1.756ms, 25.607ms
Bytes In      [total, mean]                     4653071, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-oss.png](gradual-scale-up-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.278ms
Latencies     [min, mean, 50, 90, 95, 99, max]  637.298µs, 1.154ms, 1.149ms, 1.321ms, 1.387ms, 1.693ms, 47.865ms
Bytes In      [total, mean]                     4832976, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-http-oss.png](gradual-scale-up-affinity-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.599ms
Latencies     [min, mean, 50, 90, 95, 99, max]  694.744µs, 1.235ms, 1.227ms, 1.395ms, 1.452ms, 1.717ms, 43.971ms
Bytes In      [total, mean]                     7444759, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-https-oss.png](gradual-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 1.246ms
Latencies     [min, mean, 50, 90, 95, 99, max]  626.827µs, 1.185ms, 1.182ms, 1.348ms, 1.404ms, 1.667ms, 44.17ms
Bytes In      [total, mean]                     7732710, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-oss.png](gradual-scale-down-affinity-http-oss.png)

### Scale Up Abruptly

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.467ms
Latencies     [min, mean, 50, 90, 95, 99, max]  655.248µs, 1.214ms, 1.194ms, 1.362ms, 1.418ms, 1.742ms, 82.914ms
Bytes In      [total, mean]                     1933215, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-http-oss.png](abrupt-scale-up-affinity-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.353ms
Latencies     [min, mean, 50, 90, 95, 99, max]  735.104µs, 1.284ms, 1.254ms, 1.419ms, 1.475ms, 1.769ms, 82.89ms
Bytes In      [total, mean]                     1861286, 155.11
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.4ms
Latencies     [min, mean, 50, 90, 95, 99, max]  687.263µs, 1.272ms, 1.265ms, 1.435ms, 1.486ms, 1.64ms, 35.274ms
Bytes In      [total, mean]                     1861206, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-oss.png](abrupt-scale-down-affinity-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.306ms
Latencies     [min, mean, 50, 90, 95, 99, max]  693.432µs, 1.216ms, 1.214ms, 1.384ms, 1.435ms, 1.583ms, 35.688ms
Bytes In      [total, mean]                     1933186, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-oss.png](abrupt-scale-down-affinity-http-oss.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.27ms
Latencies     [min, mean, 50, 90, 95, 99, max]  714.719µs, 1.254ms, 1.243ms, 1.413ms, 1.476ms, 1.903ms, 41.866ms
Bytes In      [total, mean]                     4652956, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-oss.png](gradual-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.332ms
Latencies     [min, mean, 50, 90, 95, 99, max]  616.707µs, 1.208ms, 1.201ms, 1.368ms, 1.429ms, 1.816ms, 32.244ms
Bytes In      [total, mean]                     4832908, 161.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-oss.png](gradual-scale-up-http-oss.png)

### Scale Down Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.163ms
Latencies     [min, mean, 50, 90, 95, 99, max]  674.826µs, 1.266ms, 1.247ms, 1.436ms, 1.514ms, 1.817ms, 77.74ms
Bytes In      [total, mean]                     14889577, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-https-oss.png](gradual-scale-down-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.056ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.251µs, 1.231ms, 1.218ms, 1.416ms, 1.492ms, 1.815ms, 48ms
Bytes In      [total, mean]                     15465551, 161.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.005ms
Latencies     [min, mean, 50, 90, 95, 99, max]  698.966µs, 1.214ms, 1.208ms, 1.376ms, 1.431ms, 1.646ms, 10.782ms
Bytes In      [total, mean]                     1861205, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-https-oss.png](abrupt-scale-up-https-oss.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.02ms
Latencies     [min, mean, 50, 90, 95, 99, max]  658.559µs, 1.187ms, 1.183ms, 1.368ms, 1.432ms, 1.657ms, 4.214ms
Bytes In      [total, mean]                     1933193, 161.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.195ms
Latencies     [min, mean, 50, 90, 95, 99, max]  658.142µs, 1.224ms, 1.173ms, 1.343ms, 1.4ms, 1.644ms, 182.14ms
Bytes In      [total, mean]                     1933139, 161.09
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-oss.png](abrupt-scale-down-http-oss.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 1.312ms
Latencies     [min, mean, 50, 90, 95, 99, max]  680.516µs, 1.285ms, 1.214ms, 1.374ms, 1.43ms, 1.661ms, 156.37ms
Bytes In      [total, mean]                     1861214, 155.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-oss.png](abrupt-scale-down-https-oss.png)
