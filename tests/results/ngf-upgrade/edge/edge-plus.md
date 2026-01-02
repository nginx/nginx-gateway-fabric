# Results

## Test environment

NGINX Plus: true

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
Requests      [total, rate, throughput]         6000, 100.01, 99.77
Duration      [total, attack, wait]             59.998s, 59.993s, 4.464ms
Latencies     [min, mean, 50, 90, 95, 99, max]  590.027µs, 1.383s, 1.185ms, 6.759s, 9.775s, 12.105s, 12.67s
Bytes In      [total, mean]                     935761, 155.96
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.107:40661->10.138.0.64:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.107:42645->10.138.0.64:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.107:50887->10.138.0.64:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.64:443: connect: connection refused
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.77
Duration      [total, attack, wait]             59.998s, 59.993s, 4.591ms
Latencies     [min, mean, 50, 90, 95, 99, max]  586.766µs, 1.397s, 1.124ms, 6.722s, 9.832s, 12.106s, 12.667s
Bytes In      [total, mean]                     971808, 161.97
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.107:37659->10.138.0.64:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.107:40625->10.138.0.64:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.107:51165->10.138.0.64:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.64:80: connect: connection refused
```

![http-plus.png](http-plus.png)
