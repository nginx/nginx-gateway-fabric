# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 0042ca1a05b3e6ee63f08f0cf2ca8090a4f6a1e6
- Date: 2026-10-15T15:55:29Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.36.4-gke.1391002
- vCPUs per node: 16
- RAM per node: 65848244Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 99.78
Duration      [total, attack, wait]             59.992s, 59.991s, 1.365ms
Latencies     [min, mean, 50, 90, 95, 99, max]  654.892µs, 41.524ms, 959.314µs, 1.106ms, 2.035ms, 1.595s, 2.144s
Bytes In      [total, mean]                     951774, 158.63
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.123:49481->10.138.0.125:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.123:37275->10.138.0.125:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.125:80: connect: connection refused
```

![http-oss.png](http-oss.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 99.78
Duration      [total, attack, wait]             59.992s, 59.991s, 1.628ms
Latencies     [min, mean, 50, 90, 95, 99, max]  543.105µs, 40.665ms, 935.734µs, 1.093ms, 1.754ms, 1.572s, 2.14s
Bytes In      [total, mean]                     917859, 152.98
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.123:38761->10.138.0.125:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.123:34067->10.138.0.125:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.125:443: connect: connection refused
```

![https-oss.png](https-oss.png)
