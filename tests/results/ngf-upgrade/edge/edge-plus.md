# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: c0b0abf8c14fae4e620b71693a3d5409f63ee80e
- Date: 2026-09-18T16:55:17Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.7-gke.1222000
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.993s, 59.992s, 1.431ms
Latencies     [min, mean, 50, 90, 95, 99, max]  678.643µs, 244.09ms, 1.278ms, 7.806ms, 2.404s, 4.737s, 5.295s
Bytes In      [total, mean]                     953802, 158.97
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.127:48631->10.138.15.193:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.127:37747->10.138.15.193:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.127:43463->10.138.15.193:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.15.193:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.993s, 59.992s, 1.596ms
Latencies     [min, mean, 50, 90, 95, 99, max]  553.52µs, 247.383ms, 1.346ms, 10.148ms, 2.456s, 4.752s, 5.313s
Bytes In      [total, mean]                     917810, 152.97
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.127:38547->10.138.15.193:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.127:60979->10.138.15.193:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.127:36759->10.138.15.193:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.15.193:443: connect: connection refused
```

![https-plus.png](https-plus.png)
