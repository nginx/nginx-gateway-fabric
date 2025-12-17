# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: e8ee7c1c4f14e249927a5447a1af2615ddbe0f87
- Date: 2025-12-17T20:04:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.77
Duration      [total, attack, wait]             59.998s, 59.993s, 4.912ms
Latencies     [min, mean, 50, 90, 95, 99, max]  704.228µs, 1.154s, 1.282ms, 5.715s, 8.66s, 10.963s, 11.519s
Bytes In      [total, mean]                     971830, 161.97
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.114:35953->10.138.0.121:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.114:38957->10.138.0.121:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.114:38795->10.138.0.121:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.114:51121->10.138.0.121:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.121:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.77
Duration      [total, attack, wait]             59.998s, 59.994s, 4.411ms
Latencies     [min, mean, 50, 90, 95, 99, max]  764.331µs, 1.146s, 1.35ms, 5.514s, 8.634s, 10.96s, 11.522s
Bytes In      [total, mean]                     933816, 155.64
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.114:35137->10.138.0.121:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.114:42763->10.138.0.121:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.114:56617->10.138.0.121:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.114:56205->10.138.0.121:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.121:443: connect: connection refused
```

![https-plus.png](https-plus.png)
