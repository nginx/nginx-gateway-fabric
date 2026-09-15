# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: a43969ce15ea40ce0548f0b187b16b9740113f82
- Date: 2026-09-15T14:01:26Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.7-gke.1222000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 0.00
Duration      [total, attack, wait]             1m30s, 59.99s, 30.001s
Latencies     [min, mean, 50, 90, 95, 99, max]  30s, 30.001s, 30.001s, 30.001s, 30.001s, 30.001s, 30.021s
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:6000  
Error Set:
Get "https://cafe.example.com/tea": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```

![https-oss.png](https-oss.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 0.00
Duration      [total, attack, wait]             1m30s, 59.99s, 30.001s
Latencies     [min, mean, 50, 90, 95, 99, max]  30s, 30.001s, 30.001s, 30.001s, 30.001s, 30.001s, 30.021s
Bytes In      [total, mean]                     0, 0.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           0.00%
Status Codes  [code:count]                      0:6000  
Error Set:
Get "http://cafe.example.com/coffee": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```

![http-oss.png](http-oss.png)
