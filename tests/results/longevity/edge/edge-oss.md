# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 11c84b1daad3fc9adc1cd52fa86bfb8d4b0dbefd
- Date: 2026-01-30T17:01:09Z
- Dirty: false

GKE Cluster:

- Node count: 3
- k8s version: v1.33.5-gke.2100000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Traffic

HTTP:

```text
Running 60m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.50ms    4.82ms 238.78ms   96.58%
    Req/Sec    25.14k     3.19k   40.85k    72.56%
  179988361 requests in 60.00m, 61.42GB read
  Socket errors: connect 0, read 7773, write 0, timeout 0
Requests/sec:  49996.22
Transfer/sec:     17.47MB
```

HTTPS:

```text
Running 60m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.85ms    4.93ms 288.95ms   96.20%
    Req/Sec    20.59k     3.18k   35.96k    78.80%
  147359791 requests in 60.00m, 49.32GB read
  Socket errors: connect 1, read 8068, write 0, timeout 0
Requests/sec:  40932.54
Transfer/sec:     14.03MB
```


## Error Logs

### nginx-gateway



### nginx




