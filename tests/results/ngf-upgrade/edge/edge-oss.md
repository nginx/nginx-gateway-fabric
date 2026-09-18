# Results

## Test environment

NGINX Plus: false

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

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.69
Duration      [total, attack, wait]             59.995s, 59.992s, 2.716ms
Latencies     [min, mean, 50, 90, 95, 99, max]  667.253µs, 714.283ms, 1.192ms, 2.986s, 6.207s, 8.515s, 9.081s
Bytes In      [total, mean]                     915093, 152.52
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.68%
Status Codes  [code:count]                      0:19  200:5981  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.109:43779->10.138.15.192:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.109:57883->10.138.15.192:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.109:45745->10.138.15.192:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.15.192:443: connect: connection refused
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.69
Duration      [total, attack, wait]             59.995s, 59.993s, 2.02ms
Latencies     [min, mean, 50, 90, 95, 99, max]  700.647µs, 711.623ms, 1.161ms, 3.102s, 6.192s, 8.512s, 9.074s
Bytes In      [total, mean]                     950979, 158.50
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.68%
Status Codes  [code:count]                      0:19  200:5981  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.109:45229->10.138.15.192:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.109:38357->10.138.15.192:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.109:43485->10.138.15.192:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.15.192:80: connect: connection refused
```

![http-oss.png](http-oss.png)
