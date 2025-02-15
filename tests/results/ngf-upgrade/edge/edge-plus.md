# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9a7a618dab5ed0eee09063de60d80bf0fb76900a
- Date: 2025-02-14T18:44:35Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1023000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.02
Duration      [total, attack, wait]             59.991s, 59.99s, 830.221µs
Latencies     [min, mean, 50, 90, 95, 99, max]  644.64µs, 947.057µs, 900.934µs, 1.167ms, 1.289ms, 1.59ms, 6.147ms
Bytes In      [total, mean]                     932055, 155.34
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.02
Duration      [total, attack, wait]             59.991s, 59.99s, 722.114µs
Latencies     [min, mean, 50, 90, 95, 99, max]  396.842µs, 781.486µs, 778.698µs, 927.818µs, 1.001ms, 1.234ms, 4.32ms
Bytes In      [total, mean]                     967990, 161.33
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![http-plus.png](http-plus.png)
