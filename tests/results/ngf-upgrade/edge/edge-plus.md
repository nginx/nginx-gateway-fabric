# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.00, 99.73
Duration      [total, attack, wait]             1m0s, 59.999s, 2.3ms
Latencies     [min, mean, 50, 90, 95, 99, max]  672.797µs, 1.275s, 1.156ms, 6.379s, 9.269s, 11.607s, 12.175s
Bytes In      [total, mean]                     959435, 159.91
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.15.195:51163->10.138.15.201:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.15.195:49189->10.138.15.201:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.15.195:54139->10.138.15.201:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.15.195:50537->10.138.15.201:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.15.201:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.00, 99.73
Duration      [total, attack, wait]             1m0s, 59.997s, 6.817ms
Latencies     [min, mean, 50, 90, 95, 99, max]  552.326µs, 1.289s, 1.223ms, 6.429s, 9.331s, 11.639s, 12.191s
Bytes In      [total, mean]                     923543, 153.92
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.15.195:40699->10.138.15.201:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.15.195:41433->10.138.15.201:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.15.195:34231->10.138.15.201:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.15.195:43349->10.138.15.201:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.15.201:443: connect: connection refused
```

![https-plus.png](https-plus.png)
