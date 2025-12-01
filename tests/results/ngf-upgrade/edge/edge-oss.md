# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 86462c0021cad59ad02c4b2f95d6a2dc99def444
- Date: 2025-12-01T21:26:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.994s, 59.992s, 2.234ms
Latencies     [min, mean, 50, 90, 95, 99, max]  593.695µs, 592.557ms, 1.165ms, 2.545s, 5.4s, 7.731s, 8.28s
Bytes In      [total, mean]                     915858, 152.64
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.74:443: connect: connection refused
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.994s, 59.992s, 2.556ms
Latencies     [min, mean, 50, 90, 95, 99, max]  589.598µs, 597.622ms, 1.09ms, 2.398s, 5.428s, 7.733s, 8.282s
Bytes In      [total, mean]                     951774, 158.63
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.74:80: connect: connection refused
```

![http-oss.png](http-oss.png)
