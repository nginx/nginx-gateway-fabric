# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.993s, 59.991s, 1.524ms
Latencies     [min, mean, 50, 90, 95, 99, max]  659.928µs, 252.204ms, 1.035ms, 1.619ms, 2.504s, 4.83s, 5.398s
Bytes In      [total, mean]                     969732, 161.62
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.104:49203->10.138.0.119:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.119:80: connect: connection refused
```

![http-oss.png](http-oss.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.993s, 59.992s, 1.077ms
Latencies     [min, mean, 50, 90, 95, 99, max]  719.431µs, 252.957ms, 1.113ms, 1.632ms, 2.511s, 4.834s, 5.401s
Bytes In      [total, mean]                     935888, 155.98
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.104:48697->10.138.0.119:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.119:443: connect: connection refused
Get "https://cafe.example.com/tea": read tcp 10.138.0.104:46427->10.138.0.119:443: read: connection reset by peer
```

![https-oss.png](https-oss.png)
