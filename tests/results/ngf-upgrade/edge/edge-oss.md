# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: d4376776aecc98294dc881a49cfbfa491773f74d
- Date: 2026-01-15T17:08:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2019000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.998s, 59.993s, 5.169ms
Latencies     [min, mean, 50, 90, 95, 99, max]  569.825µs, 2.238s, 1.256ms, 10.311s, 13.237s, 15.582s, 16.154s
Bytes In      [total, mean]                     959546, 159.92
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.6:80: connect: connection refused
```

![http-oss.png](http-oss.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.74
Duration      [total, attack, wait]             59.998s, 59.993s, 5.372ms
Latencies     [min, mean, 50, 90, 95, 99, max]  662.96µs, 2.266s, 1.305ms, 10.413s, 13.244s, 15.559s, 16.162s
Bytes In      [total, mean]                     923630, 153.94
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.73%
Status Codes  [code:count]                      0:16  200:5984  
Error Set:
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.6:443: connect: connection refused
```

![https-oss.png](https-oss.png)
