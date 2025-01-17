# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test1: Running latte path based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 585.141µs
Latencies     [min, mean, 50, 90, 95, 99, max]  468.725µs, 603.724µs, 588.95µs, 660.92µs, 689.011µs, 780.146µs, 12.497ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test2: Running coffee header based routing

```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 681.623µs
Latencies     [min, mean, 50, 90, 95, 99, max]  480.409µs, 628.988µs, 615.282µs, 686.616µs, 716.228µs, 803.501µs, 11.923ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test3: Running coffee query based routing

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.02
Duration      [total, attack, wait]             30s, 29.999s, 584.715µs
Latencies     [min, mean, 50, 90, 95, 99, max]  509.529µs, 630.766µs, 618.592µs, 688.962µs, 717.023µs, 807.297µs, 12.369ms
Bytes In      [total, mean]                     5070000, 169.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test4: Running tea GET method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 583.6µs
Latencies     [min, mean, 50, 90, 95, 99, max]  503.686µs, 635.461µs, 621.485µs, 694.93µs, 726.102µs, 815.553µs, 10.746ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```

## Test5: Running tea POST method based routing

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 545.737µs
Latencies     [min, mean, 50, 90, 95, 99, max]  492.002µs, 619.268µs, 606.509µs, 677.95µs, 706.49µs, 795.593µs, 9.61ms
Bytes In      [total, mean]                     4740000, 158.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
