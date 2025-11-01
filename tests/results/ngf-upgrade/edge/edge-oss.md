# Results

## Test environment

NGINX Plus: false

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

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.991s, 59.99s, 1.098ms
Latencies     [min, mean, 50, 90, 95, 99, max]  852.123µs, 1.151ms, 1.127ms, 1.302ms, 1.363ms, 1.583ms, 11.026ms
Bytes In      [total, mean]                     925971, 154.33
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.991s, 59.99s, 1.195ms
Latencies     [min, mean, 50, 90, 95, 99, max]  616.849µs, 976.017µs, 987.768µs, 1.167ms, 1.223ms, 1.342ms, 12.457ms
Bytes In      [total, mean]                     961988, 160.33
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![http-oss.png](http-oss.png)
