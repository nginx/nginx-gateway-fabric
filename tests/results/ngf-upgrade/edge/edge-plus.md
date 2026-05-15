# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.75
Duration      [total, attack, wait]             59.993s, 59.992s, 1.275ms
Latencies     [min, mean, 50, 90, 95, 99, max]  539.729µs, 110.385ms, 1.01ms, 1.336ms, 660.271ms, 2.979s, 3.521s
Bytes In      [total, mean]                     915552, 152.59
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.112:43609->10.138.0.114:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.112:43939->10.138.0.114:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.112:37151->10.138.0.114:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.114:443: connect: connection refused
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.75
Duration      [total, attack, wait]             59.993s, 59.992s, 1.179ms
Latencies     [min, mean, 50, 90, 95, 99, max]  557.177µs, 109.297ms, 984.813µs, 1.302ms, 642.011ms, 2.964s, 3.517s
Bytes In      [total, mean]                     951456, 158.58
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.112:43753->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.112:39039->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.112:35533->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.114:80: connect: connection refused
```

![http-plus.png](http-plus.png)
