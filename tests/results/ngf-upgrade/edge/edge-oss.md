# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.993s, 59.991s, 1.744ms
Latencies     [min, mean, 50, 90, 95, 99, max]  729.146µs, 65.593ms, 1.307ms, 1.796ms, 15.978ms, 2.15s, 2.712s
Bytes In      [total, mean]                     915552, 152.59
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.113:443: connect: connection refused
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.75
Duration      [total, attack, wait]             59.993s, 59.991s, 1.454ms
Latencies     [min, mean, 50, 90, 95, 99, max]  652.061µs, 66.049ms, 1.149ms, 1.606ms, 12.026ms, 2.153s, 2.709s
Bytes In      [total, mean]                     953462, 158.91
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.113:80: connect: connection refused
```

![http-oss.png](http-oss.png)
