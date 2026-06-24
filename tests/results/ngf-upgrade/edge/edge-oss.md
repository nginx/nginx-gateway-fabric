# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.73
Duration      [total, attack, wait]             59.994s, 59.992s, 1.301ms
Latencies     [min, mean, 50, 90, 95, 99, max]  776.147µs, 236.177ms, 1.085ms, 1.537ms, 2.332s, 4.63s, 5.196s
Bytes In      [total, mean]                     957280, 159.55
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.72%
Status Codes  [code:count]                      0:17  200:5983  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.15.196:58231->10.138.15.200:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.15.196:57485->10.138.15.200:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.15.196:54989->10.138.15.200:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.15.196:33007->10.138.15.200:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.15.200:80: connect: connection refused
```

![http-oss.png](http-oss.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.73
Duration      [total, attack, wait]             59.994s, 59.992s, 2.122ms
Latencies     [min, mean, 50, 90, 95, 99, max]  731.708µs, 238.172ms, 1.119ms, 1.919ms, 2.357s, 4.64s, 5.202s
Bytes In      [total, mean]                     921382, 153.56
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.72%
Status Codes  [code:count]                      0:17  200:5983  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.15.196:41437->10.138.15.200:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.15.196:41259->10.138.15.200:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.15.196:50295->10.138.15.200:443: read: connection reset by peer
Get "https://cafe.example.com/tea": write tcp 10.138.15.196:39411->10.138.15.200:443: write: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.15.200:443: connect: connection refused
```

![https-oss.png](https-oss.png)
