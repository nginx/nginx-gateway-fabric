# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 28d0224c5f1617ace603b72889b5bb7aa272ea20
- Date: 2026-06-01T17:32:15Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.76
Duration      [total, attack, wait]             59.995s, 59.993s, 2.382ms
Latencies     [min, mean, 50, 90, 95, 99, max]  688.669µs, 660.873ms, 1.188ms, 2.946s, 5.848s, 8.159s, 8.732s
Bytes In      [total, mean]                     921690, 153.62
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.75%
Status Codes  [code:count]                      0:15  200:5985  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.124:443: connect: connection refused
Get "https://cafe.example.com/tea": read tcp 10.138.0.119:44677->10.138.0.124:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.119:34255->10.138.0.124:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.119:45981->10.138.0.124:443: read: connection reset by peer
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.76
Duration      [total, attack, wait]             59.995s, 59.993s, 1.814ms
Latencies     [min, mean, 50, 90, 95, 99, max]  741.911µs, 647.154ms, 1.148ms, 2.855s, 5.777s, 8.072s, 8.675s
Bytes In      [total, mean]                     957600, 159.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.75%
Status Codes  [code:count]                      0:15  200:5985  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.124:80: connect: connection refused
Get "http://cafe.example.com/coffee": read tcp 10.138.0.119:55563->10.138.0.124:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.119:45413->10.138.0.124:80: read: connection reset by peer
```

![http-oss.png](http-oss.png)
