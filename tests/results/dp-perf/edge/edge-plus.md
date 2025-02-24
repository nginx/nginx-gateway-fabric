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

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 618.171µs
Latencies     [min, mean, 50, 90, 95, 99, max]  477.878µs, 647.929µs, 630.059µs, 730.862µs, 781.842µs, 926.293µs, 12.11ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 624.611µs
Latencies     [min, mean, 50, 90, 95, 99, max]  500.184µs, 680.143µs, 657.594µs, 767.853µs, 818.249µs, 1.001ms, 12.545ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 654.862µs
Latencies     [min, mean, 50, 90, 95, 99, max]  511.996µs, 683.306µs, 662.733µs, 775.319µs, 832.514µs, 996.46µs, 12.457ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 681.845µs
Latencies     [min, mean, 50, 90, 95, 99, max]  487.83µs, 680.241µs, 652.959µs, 764.456µs, 815.077µs, 996.309µs, 21.867ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 653.708µs
Latencies     [min, mean, 50, 90, 95, 99, max]  501.209µs, 675.702µs, 655.411µs, 762.813µs, 812.689µs, 974.489µs, 12.214ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
