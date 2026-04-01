# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 17c42c8bbbb004ba9c0e9b867396c5f8937207cd
- Date: 2026-04-01T18:33:47Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test: Send https /tea traffic

```text
Requests      [total, rate, throughput]         6000, 100.02, 99.79
Duration      [total, attack, wait]             59.995s, 59.99s, 5.108ms
Latencies     [min, mean, 50, 90, 95, 99, max]  689.07µs, 753.451ms, 1.016ms, 3.414s, 6.445s, 8.763s, 9.32s
Bytes In      [total, mean]                     927985, 154.66
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.78%
Status Codes  [code:count]                      0:13  200:5987  
Error Set:
Get "https://cafe.example.com/tea": write tcp 10.138.0.76:39953->10.138.0.23:443: write: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.76:60053->10.138.0.23:443: read: connection reset by peer
Get "https://cafe.example.com/tea": read tcp 10.138.0.76:43989->10.138.0.23:443: read: connection reset by peer
Get "https://cafe.example.com/tea": dial tcp 0.0.0.0:0->10.138.0.23:443: connect: connection refused
```

![https-plus.png](https-plus.png)

## Test: Send http /coffee traffic

```text
Requests      [total, rate, throughput]         6000, 100.01, 99.77
Duration      [total, attack, wait]             59.995s, 59.993s, 1.967ms
Latencies     [min, mean, 50, 90, 95, 99, max]  692.794µs, 753.743ms, 973.241µs, 3.544s, 6.448s, 8.783s, 9.354s
Bytes In      [total, mean]                     963746, 160.62
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.77%
Status Codes  [code:count]                      0:14  200:5986  
Error Set:
Get "http://cafe.example.com/coffee": read tcp 10.138.0.76:51869->10.138.0.23:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.76:49419->10.138.0.23:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.76:37093->10.138.0.23:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": read tcp 10.138.0.76:56579->10.138.0.23:80: read: connection reset by peer
Get "http://cafe.example.com/coffee": dial tcp 0.0.0.0:0->10.138.0.23:80: connect: connection refused
```

![http-plus.png](http-plus.png)
