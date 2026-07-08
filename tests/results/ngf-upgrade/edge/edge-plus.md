# Results

## Test environment

NGINX Plus: true

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
Requests      [total, rate, throughput]         6000, 100.01, 99.76
Duration      [total, attack, wait]             59.994s, 59.992s, 2.256ms
Latencies     [min, mean, 50, 90, 95, 99, max]  440.061µs, 521.123ms, 907.419µs, 1.967s, 4.874s, 7.19s, 7.76s
Bytes In      [total, mean]                     915705, 152.62
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.75%
Status Codes  [code:count]                      0:15  200:5985  
Error Set:
Get "https://cafe.example.com/tea": read tcp 10.138.0.86:58039->10.138.0.9:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.86:55943->10.138.0.9:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.86:49107->10.138.0.9:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.9:443: connect: connection refused
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.76
Duration      [total, attack, wait]             59.994s, 59.992s, 2.191ms
Latencies     [min, mean, 50, 90, 95, 99, max]  527.796µs, 520.551ms, 883.011µs, 2.067s, 4.872s, 7.193s, 7.766s
Bytes In      [total, mean]                     951615, 158.60
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.75%
Status Codes  [code:count]                      0:15  200:5985  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.86:40811->10.138.0.9:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.86:43515->10.138.0.9:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.86:35343->10.138.0.9:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.9:80: connect: connection refused
```

![http-plus.png](http-plus.png)
