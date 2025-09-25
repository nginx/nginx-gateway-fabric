# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9d84d9e238809236f2a81e15e77b7928b2b2c48c
- Date: 2025-09-23T17:23:26Z
- Dirty: false

GKE Cluster:

- Node count: 15
- k8s version: v1.33.4-gke.1134000
- vCPUs per node: 2
- RAM per node: 4015672Ki
- Max pods per node: 110
- Zone: us-west1-a
- Instance Type: e2-medium

## Traffic

HTTP:

```text
Running 2040m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   128.14ms   88.53ms   1.26s    73.61%
    Req/Sec   413.29    265.43     2.36k    64.80%
  99143782 requests in 2040.00m, 33.91GB read
Requests/sec:    810.00
Transfer/sec:    290.51KB
```

HTTPS:

```text
Running 2040m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   128.12ms   88.53ms   1.26s    73.62%
    Req/Sec   413.12    264.98     2.26k    64.88%
  99093705 requests in 2040.00m, 33.36GB read
  Non-2xx or 3xx responses: 1
Requests/sec:    809.59
Transfer/sec:    285.78KB
```


## Error Logs

### nginx-gateway



### nginx
2025/09/25 03:48:05 [error] 37#37: *180477844 no live upstreams while connecting to upstream, client: 10.138.0.58, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://longevity_tea_80/tea", host: "cafe.example.com"
