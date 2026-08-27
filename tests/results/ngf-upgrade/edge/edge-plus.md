# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: e00bbb16bab5ce1d2fb17b811b04d0217abb08a6
- Date: 2026-08-27T19:21:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1710000
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.71
Duration      [total, attack, wait]             59.994s, 59.993s, 1.364ms
Latencies     [min, mean, 50, 90, 95, 99, max]  572.166µs, 701.058ms, 1.035ms, 3.236s, 6.129s, 8.432s, 8.991s
Bytes In      [total, mean]                     915246, 152.54
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.70%
Status Codes  [code:count]                      0:18  200:5982  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.124:48439->10.138.15.192:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.124:34761->10.138.15.192:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.124:55909->10.138.15.192:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.15.192:443: connect: connection refused
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 99.71
Duration      [total, attack, wait]             59.994s, 59.99s, 4.31ms
Latencies     [min, mean, 50, 90, 95, 99, max]  554.757µs, 690.844ms, 967.1µs, 2.901s, 6.071s, 8.399s, 8.955s
Bytes In      [total, mean]                     951138, 158.52
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.70%
Status Codes  [code:count]                      0:18  200:5982  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.124:57935->10.138.15.192:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.124:59919->10.138.15.192:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.124:38121->10.138.15.192:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.15.192:80: connect: connection refused
```

![http-plus.png](http-plus.png)
