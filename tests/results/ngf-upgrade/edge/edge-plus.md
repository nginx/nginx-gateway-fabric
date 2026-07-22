# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.73
Duration      [total, attack, wait]             59.993s, 59.992s, 1.468ms
Latencies     [min, mean, 50, 90, 95, 99, max]  614.423µs, 251.052ms, 1.021ms, 1.611ms, 2.491s, 4.82s, 5.389s
Bytes In      [total, mean]                     965229, 160.87
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.72%
Status Codes  [code:count]                      0:17  200:5983  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.105:34049->10.138.0.65:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.105:45401->10.138.0.65:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.105:43681->10.138.0.65:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.65:80: connect: connection refused
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.73
Duration      [total, attack, wait]             59.993s, 59.992s, 1.292ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.84µs, 251.571ms, 1.036ms, 1.868ms, 2.497s, 4.824s, 5.39s
Bytes In      [total, mean]                     929388, 154.90
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.72%
Status Codes  [code:count]                      0:17  200:5983  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.105:45663->10.138.0.65:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.105:47415->10.138.0.65:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.105:43321->10.138.0.65:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.65:443: connect: connection refused
```

![https-plus.png](https-plus.png)
