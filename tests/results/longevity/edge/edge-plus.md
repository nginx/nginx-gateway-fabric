# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: fbb8fffb5108b2b09b0fdc38b87be76092033330
- Date: 2026-06-12T19:13:59Z
- Dirty: false

GKE Cluster:

- Node count: 3
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Traffic

HTTP:

```text
Running 60m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     3.24ms    3.27ms 407.76ms   88.83%
    Req/Sec    17.49k     3.42k   41.94k    76.44%
  125215640 requests in 60.00m, 16.33GB read
Requests/sec:  34781.40
Transfer/sec:      4.64MB
```

HTTPS:

```text
Running 60m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     3.89ms    3.65ms 223.49ms   88.24%
    Req/Sec    13.47k     3.54k   41.90k    77.73%
  96381572 requests in 60.00m, 12.57GB read
Requests/sec:  26772.02
Transfer/sec:      3.57MB
```


## WAF Traffic

WAF HTTP (coffee):

```text
Running 60m test @ http://waf.example.com/coffee
  2 threads and 80 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     8.53ms    6.19ms 424.13ms   83.33%
    Req/Sec     5.03k     0.92k   12.60k    75.56%
  36017498 requests in 60.00m, 4.19GB read
Requests/sec:  10004.70
Transfer/sec:      1.19MB
```

WAF HTTP (tea):

```text
Running 60m test @ http://waf.example.com/tea
  2 threads and 80 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     8.51ms    5.98ms 451.82ms   81.67%
    Req/Sec     5.04k     0.93k   12.02k    75.63%
  36083650 requests in 60.00m, 4.20GB read
Requests/sec:  10023.19
Transfer/sec:      1.19MB
```


## WAF Attack Results

WAF Attack Log (blocked: 1948, unexpected: 0):

All attack probes were blocked successfully.


## Error Logs

### nginx-gateway



### nginx




