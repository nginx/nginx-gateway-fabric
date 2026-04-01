# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 2f3153c547e0442fbb26aa9165118f4dc2b20f23
- Date: 2026-04-01T15:39:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848316Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 99.99, 99.75
Duration      [total, attack, wait]             1m0s, 1m0s, 8.112ms
Latencies     [min, mean, 50, 90, 95, 99, max]  645.133µs, 727.133ms, 1.134ms, 3.23s, 6.282s, 8.609s, 9.175s
Bytes In      [total, mean]                     957760, 159.63
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.94:80: connect: connection refused
```

![http-oss.png](http-oss.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.00, 99.74
Duration      [total, attack, wait]             1m0s, 1m0s, 12.426ms
Latencies     [min, mean, 50, 90, 95, 99, max]  714.487µs, 733.06ms, 1.2ms, 3.287s, 6.307s, 8.629s, 9.181s
Bytes In      [total, mean]                     921844, 153.64
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.94:443: connect: connection refused
```

![https-oss.png](https-oss.png)
