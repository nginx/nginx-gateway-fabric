# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: d97dd7debc1ea5d51f4413b6564b27921a1fc982
- Date: 2026-02-27T17:29:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.34.3-gke.1318000
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.76
Duration      [total, attack, wait]             59.994s, 59.992s, 2.179ms
Latencies     [min, mean, 50, 90, 95, 99, max]  625.778µs, 400.177ms, 1.064ms, 954.833ms, 3.901s, 6.233s, 6.806s
Bytes In      [total, mean]                     969570, 161.59
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.75%
Status Codes  [code:count]                      0:15  200:5985  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.59:55503->10.138.0.67:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.59:47661->10.138.0.67:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.59:33407->10.138.0.67:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.67:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.994s, 59.991s, 2.498ms
Latencies     [min, mean, 50, 90, 95, 99, max]  632.406µs, 412.038ms, 1.118ms, 1.058s, 4.028s, 6.332s, 6.851s
Bytes In      [total, mean]                     933504, 155.58
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": write tcp 10.138.0.59:38407->10.138.0.67:443: write: connection reset by peer
Get "https://cafe.example.com/tea": write tcp 10.138.0.59:47053->10.138.0.67:443: write: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.59:58923->10.138.0.67:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.59:50429->10.138.0.67:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.67:443: connect: connection refused
```

![https-plus.png](https-plus.png)
