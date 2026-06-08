# Results

## Test environment

NGINX Plus: true

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

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 99.74
Duration      [total, attack, wait]             59.993s, 59.99s, 3.598ms
Latencies     [min, mean, 50, 90, 95, 99, max]  696.298µs, 528.455ms, 1.191ms, 1.942s, 4.904s, 7.22s, 7.787s
Bytes In      [total, mean]                     965384, 160.90
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.73:80: connect: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.120:35415->10.138.0.73:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.73:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.993s, 59.992s, 1.166ms
Latencies     [min, mean, 50, 90, 95, 99, max]  580.649µs, 527.733ms, 1.202ms, 2.154s, 4.899s, 7.232s, 7.791s
Bytes In      [total, mean]                     929512, 154.92
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.120:39997->10.138.0.73:443: read: connection reset by peer
Get "https://cafe.example.com/tea": write tcp 10.138.0.120:55173->10.138.0.73:443: write: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.120:59611->10.138.0.73:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.73:443: connect: connection refused
```

![https-plus.png](https-plus.png)
