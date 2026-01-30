# Results

## Test environment

NGINX Plus: true

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
    Latency     2.43ms    3.57ms 437.75ms   94.58%
    Req/Sec    24.56k     2.98k   41.21k    72.08%
  175906540 requests in 60.00m, 59.86GB read
  Non-2xx or 3xx responses: 18
Requests/sec:  48861.91
Transfer/sec:     17.03MB
```

HTTPS:

```text
Running 60m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.76ms    3.70ms 431.69ms   94.04%
    Req/Sec    20.54k     3.15k   29.50k    77.96%
  147074111 requests in 60.00m, 49.08GB read
Requests/sec:  40853.34
Transfer/sec:     13.96MB
```


## Error Logs

### nginx-gateway



### nginx
2026/01/30 18:16:02 [error] 68#68: *902214 no live upstreams while connecting to upstream, client: 10.138.0.61, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: "http://longevity_coffee_80/coffee", host: "cafe.example.com"

10.138.0.61 - - [30/Jan/2026:18:16:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/01/30 18:16:02 [error] 68#68: *902214 no live upstreams while connecting to upstream, client: 10.138.0.61, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: "http://longevity_coffee_80/coffee", host: "cafe.example.com"

