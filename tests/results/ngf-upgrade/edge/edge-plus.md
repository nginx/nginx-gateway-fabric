# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.992s, 59.991s, 1.464ms
Latencies     [min, mean, 50, 90, 95, 99, max]  863.132µs, 1.215ms, 1.2ms, 1.404ms, 1.463ms, 1.629ms, 13.007ms
Bytes In      [total, mean]                     949961, 158.33
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![http-plus.png](http-plus.png)

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 100.01
Duration      [total, attack, wait]             59.992s, 59.991s, 1.394ms
Latencies     [min, mean, 50, 90, 95, 99, max]  925.823µs, 1.261ms, 1.233ms, 1.416ms, 1.473ms, 1.63ms, 12.857ms
Bytes In      [total, mean]                     913978, 152.33
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:6000  
Error Set:
```

![https-plus.png](https-plus.png)
