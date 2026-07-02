# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 903211b7f256263c546d17dbbc037f7756f492e1
- Date: 2026-06-30T17:57:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1163012
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## One NGINX Pod runs per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 1.039ms
Latencies     [min, mean, 50, 90, 95, 99, max]  563.124µs, 990.635µs, 937.641µs, 1.189ms, 1.293ms, 1.687ms, 20.204ms
Bytes In      [total, mean]                     4592984, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-affinity-https-plus.png](gradual-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 821.404µs
Latencies     [min, mean, 50, 90, 95, 99, max]  517.974µs, 930.515µs, 882.16µs, 1.125ms, 1.252ms, 1.657ms, 20.377ms
Bytes In      [total, mean]                     4772973, 159.10
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
Duration      [total, attack, wait]             8m0s, 8m0s, 982.612µs
Latencies     [min, mean, 50, 90, 95, 99, max]  558.617µs, 919.347µs, 902.018µs, 1.046ms, 1.094ms, 1.311ms, 60.834ms
Bytes In      [total, mean]                     7636776, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:48000  
Error Set:
```

![gradual-scale-down-affinity-http-plus.png](gradual-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         48000, 100.00, 100.00
Duration      [total, attack, wait]             8m0s, 8m0s, 966.696µs
Latencies     [min, mean, 50, 90, 95, 99, max]  595.927µs, 976.979µs, 954.385µs, 1.091ms, 1.138ms, 1.35ms, 63.254ms
Bytes In      [total, mean]                     7348782, 153.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.037ms
Latencies     [min, mean, 50, 90, 95, 99, max]  583.653µs, 929.724µs, 910.562µs, 1.029ms, 1.071ms, 1.187ms, 68.567ms
Bytes In      [total, mean]                     1837173, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-affinity-https-plus.png](abrupt-scale-up-affinity-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 830.223µs
Latencies     [min, mean, 50, 90, 95, 99, max]  547.824µs, 872.73µs, 855.253µs, 983.922µs, 1.022ms, 1.127ms, 69.145ms
Bytes In      [total, mean]                     1909205, 159.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 892.796µs
Latencies     [min, mean, 50, 90, 95, 99, max]  514.33µs, 869.314µs, 861.462µs, 992.683µs, 1.033ms, 1.151ms, 30.58ms
Bytes In      [total, mean]                     1909187, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-http-plus.png](abrupt-scale-down-affinity-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 911.747µs
Latencies     [min, mean, 50, 90, 95, 99, max]  585.297µs, 900.592µs, 890.379µs, 1.008ms, 1.046ms, 1.146ms, 31.125ms
Bytes In      [total, mean]                     1837184, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-affinity-https-plus.png](abrupt-scale-down-affinity-https-plus.png)

## Multiple NGINX Pods run per node Test Results

### Scale Up Gradually

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 912.826µs
Latencies     [min, mean, 50, 90, 95, 99, max]  592.327µs, 969.848µs, 949.012µs, 1.087ms, 1.141ms, 1.584ms, 27.666ms
Bytes In      [total, mean]                     4592944, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-https-plus.png](gradual-scale-up-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         30000, 100.00, 100.00
Duration      [total, attack, wait]             5m0s, 5m0s, 855.902µs
Latencies     [min, mean, 50, 90, 95, 99, max]  527.235µs, 916.331µs, 898.402µs, 1.052ms, 1.107ms, 1.483ms, 26.77ms
Bytes In      [total, mean]                     4773036, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

![gradual-scale-up-http-plus.png](gradual-scale-up-http-plus.png)

### Scale Down Gradually

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 926.093µs
Latencies     [min, mean, 50, 90, 95, 99, max]  549.161µs, 963.363µs, 925.554µs, 1.065ms, 1.112ms, 1.405ms, 1.035s
Bytes In      [total, mean]                     15273583, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:96000  
Error Set:
```

![gradual-scale-down-http-plus.png](gradual-scale-down-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         96000, 100.00, 100.00
Duration      [total, attack, wait]             16m0s, 16m0s, 1.131ms
Latencies     [min, mean, 50, 90, 95, 99, max]  616.153µs, 1.014ms, 981.047µs, 1.117ms, 1.167ms, 1.501ms, 413.215ms
Bytes In      [total, mean]                     14697791, 153.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 854.71µs
Latencies     [min, mean, 50, 90, 95, 99, max]  582.583µs, 939.434µs, 905.886µs, 1.044ms, 1.085ms, 1.427ms, 124.886ms
Bytes In      [total, mean]                     1909146, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-up-http-plus.png](abrupt-scale-up-http-plus.png)

#### Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 850.82µs
Latencies     [min, mean, 50, 90, 95, 99, max]  608.798µs, 975.488µs, 940.815µs, 1.068ms, 1.113ms, 1.402ms, 30.131ms
Bytes In      [total, mean]                     1837183, 153.10
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
Duration      [total, attack, wait]             2m0s, 2m0s, 1.066ms
Latencies     [min, mean, 50, 90, 95, 99, max]  590.324µs, 926.716µs, 916.855µs, 1.036ms, 1.073ms, 1.182ms, 9.6ms
Bytes In      [total, mean]                     1837207, 153.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-https-plus.png](abrupt-scale-down-https-plus.png)

#### Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         12000, 100.01, 100.01
Duration      [total, attack, wait]             2m0s, 2m0s, 949.321µs
Latencies     [min, mean, 50, 90, 95, 99, max]  571.901µs, 882.962µs, 877.089µs, 1.01ms, 1.052ms, 1.16ms, 8.708ms
Bytes In      [total, mean]                     1909237, 159.10
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:12000  
Error Set:
```

![abrupt-scale-down-http-plus.png](abrupt-scale-down-http-plus.png)
