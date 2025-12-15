# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 89aee48bf6e660a828ffd32ca35fc7f52e358e00
- Date: 2025-12-12T20:04:38Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.996s, 59.992s, 4.093ms
Latencies     [min, mean, 50, 90, 95, 99, max]  533.611µs, 1.584s, 1.102ms, 7.659s, 10.645s, 12.968s, 13.51s
Bytes In      [total, mean]                     915552, 152.59
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.58:443: connect: connection refused
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.996s, 59.993s, 3.335ms
Latencies     [min, mean, 50, 90, 95, 99, max]  445.697µs, 1.571s, 1.121ms, 7.597s, 10.625s, 12.95s, 13.505s
Bytes In      [total, mean]                     951456, 158.58
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.58:80: connect: connection refused
```

![http-oss.png](http-oss.png)
