# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851512Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.994s, 59.992s, 2.101ms
Latencies     [min, mean, 50, 90, 95, 99, max]  507.107µs, 414.573ms, 1.114ms, 1.103s, 4.036s, 6.367s, 6.934s
Bytes In      [total, mean]                     961744, 160.29
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.103:53013->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.103:46203->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.103:47717->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.103:53217->10.138.0.114:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.114:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.78
Duration      [total, attack, wait]             59.994s, 59.993s, 1.947ms
Latencies     [min, mean, 50, 90, 95, 99, max]  600.657µs, 421.024ms, 1.175ms, 1.162s, 4.089s, 6.405s, 6.961s
Bytes In      [total, mean]                     923930, 153.99
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.103:57081->10.138.0.114:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.103:35237->10.138.0.114:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.103:40395->10.138.0.114:443: read: connection reset by peer
Get "https://cafe.example.com/tea": write tcp 10.138.0.103:50087->10.138.0.114:443: write: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.114:443: connect: connection refused
```

![https-plus.png](https-plus.png)
