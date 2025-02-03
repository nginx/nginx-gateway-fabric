# Results

## Test environment

NGINX Plus: true

GKE Cluster:

- Node count: 3
- k8s version: v1.28.9-gke.1000000
- vCPUs per node: 2
- RAM per node: 4019172Ki
- Max pods per node: 110
- Zone: us-central1-c
- Instance Type: e2-medium
- NGF pod name -- ngf-longevity-nginx-gateway-fabric-c4db4c565-8rhdn

## Traffic

HTTP:

```text
Running 5760m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   192.36ms  124.47ms   1.99s    66.49%
    Req/Sec   279.51    187.21     1.88k    65.53%
  188805938 requests in 5760.00m, 65.81GB read
  Socket errors: connect 0, read 22, write 4, timeout 42
  Non-2xx or 3xx responses: 3
Requests/sec:    546.31
Transfer/sec:    199.68KB
```

HTTPS:

```text
Running 5760m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   192.58ms  124.49ms   2.00s    66.48%
    Req/Sec   278.96    186.56     1.76k    65.59%
  188455380 requests in 5760.00m, 64.69GB read
  Socket errors: connect 10, read 24, write 0, timeout 60
  Non-2xx or 3xx responses: 5
Requests/sec:    545.30
Transfer/sec:    196.26KB
```

Note: Non-2xx or 3xx responses correspond to the error in NGINX log, see below.

### Logs

nginx-gateway:

a lot of expected "usage reporting not enabled" errors.

And:
```text
failed to start control loop: leader election lost

4 x
failed to start control loop: error setting initial control plane configuration: NginxGateway nginx-gateway/ngf-longevity-config not found: failed to get API group resources: unable to retrieve the complete list of server APIs: gateway.nginx.org/v1alpha1: Get "https://10.61.192.1:443/apis/gateway.nginx.org/v1alpha1?timeout=10s": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
```

Those errors correspond to losing connectivity with the Kubernetes API and NGF container restarts (5). Such logs
are expected in that case.

nginx:

```text
2024/06/01 21:34:09 [error] 104#104: *115862644 no live upstreams while connecting to upstream, client: 10.128.0.112, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://longevity_tea_80/tea", host: "cafe.example.com"
2024/06/03 12:01:07 [error] 105#105: *267137988 no live upstreams while connecting to upstream, client: 10.128.0.112, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: "http://longevity_coffee_80/coffee", host: "cafe.example.com"
```

8 errors like that occurred at different times. They occurred when backend pods were updated. Not clear why that happens.
Because number of errors is small compared with total handled requests, no need to further
investigate unless we see it in the future again at larger volume.

### Key Metrics

#### Containers memory

![plus-memory.png](plus-memory.png)

Drop in NGF memory usage in the beginning corresponds to the nginx-gateway container restarts.

Drop in NGINX memory usage corresponds to the end of traffic generation.

#### NGF Container Memory

![plus-ngf-memory.png](plus-ngf-memory.png)

### Containers CPU

![plus-cpu.png](plus-cpu.png)

Drop in NGINX CPU usage corresponds to the end of traffic generation.

### NGINX Plus metrics

![plus-status.png](plus-status.png)

Drop in request corresponds to the end of traffic generation.

### Reloads

Rate of reloads - successful and errors:

![plus-reloads.png](plus-reloads.png)

Note: compared to NGINX, we don't have as many reloads here, because NGF uses NGINX Plus API to reconfigure NGINX
for endpoints changes.

No reloads finished with an error.

Reload time distribution - counts:

![plus-reload-time.png](plus-reload-time.png)

Reload related metrics at the end:

![plus-final-reloads.png](plus-final-reloads.png)

All successful reloads took less than 1 seconds, with most under 0.5 second.

## Comparison with previous runs

Graphs look similar to 1.2.0 results.
As https://github.com/nginx/nginx-gateway-fabric/issues/1112 was fixed, we no longer see the corresponding
reload spikes.

Memory usage is flat, but ~1 Mb higher than in 1.2.0.
