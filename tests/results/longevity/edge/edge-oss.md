# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: de21497a039277bbb4f0e5e57d66c0aa83a1887d
- Date: 2026-01-29T18:23:08Z
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
Running 5m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.73ms    7.31ms 204.46ms   98.07%
    Req/Sec    25.90k     3.66k   44.51k    74.24%
  15444294 requests in 5.00m, 5.28GB read
  Socket errors: connect 0, read 1352, write 0, timeout 0
Requests/sec:  51467.35
Transfer/sec:     18.01MB
```

HTTPS:

```text
Running 5m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     3.06ms    7.45ms 203.58ms   97.95%
    Req/Sec    21.45k     2.74k   33.42k    85.60%
  12783318 requests in 5.00m, 4.28GB read
  Socket errors: connect 0, read 1458, write 0, timeout 0
Requests/sec:  42598.03
Transfer/sec:     14.60MB
```


## Error Logs

