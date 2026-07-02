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

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 633.758µs
Latencies     [min, mean, 50, 90, 95, 99, max]  536.283µs, 669.451µs, 649.392µs, 704.173µs, 728.766µs, 860.679µs, 25.388ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 640.18µs
Latencies     [min, mean, 50, 90, 95, 99, max]  594.22µs, 723.572µs, 704.392µs, 758.476µs, 783.134µs, 926.843µs, 24.228ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 681.542µs
Latencies     [min, mean, 50, 90, 95, 99, max]  574.182µs, 712.357µs, 691.3µs, 752.546µs, 781.166µs, 918.71µs, 27.582ms
Bytes In      [total, mean]                     5040000, 168.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 717.034µs
Latencies     [min, mean, 50, 90, 95, 99, max]  575.472µs, 709.687µs, 694.878µs, 747.839µs, 774.763µs, 879.372µs, 8.535ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 701.197µs
Latencies     [min, mean, 50, 90, 95, 99, max]  588.421µs, 717.271µs, 696.026µs, 755.474µs, 783.665µs, 915.913µs, 25.301ms
Bytes In      [total, mean]                     4710000, 157.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
