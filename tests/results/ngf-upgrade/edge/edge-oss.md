# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 903211b7f256263c546d17dbbc037f7756f492e1
- Date: 2026-06-30T17:57:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1163012
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.73
Duration      [total, attack, wait]             59.994s, 59.993s, 1.293ms
Latencies     [min, mean, 50, 90, 95, 99, max]  594.506µs, 314.151ms, 1.024ms, 248.297ms, 3.142s, 5.456s, 6.009s
Bytes In      [total, mean]                     929326, 154.89
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.72%
Status Codes  [code:count]                      0:17  200:5983  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.85:36807->10.138.0.102:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.85:38051->10.138.0.102:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.85:42805->10.138.0.102:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.102:443: connect: connection refused
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.73
Duration      [total, attack, wait]             59.994s, 59.991s, 3.028ms
Latencies     [min, mean, 50, 90, 95, 99, max]  687.558µs, 308.008ms, 1.002ms, 240.74ms, 3.072s, 5.419s, 5.986s
Bytes In      [total, mean]                     965240, 160.87
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.72%
Status Codes  [code:count]                      0:17  200:5983  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.85:32823->10.138.0.102:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.85:54711->10.138.0.102:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.85:41287->10.138.0.102:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.102:80: connect: connection refused
```

![http-oss.png](http-oss.png)
