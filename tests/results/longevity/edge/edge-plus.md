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
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Traffic

HTTP:

```text
Running 60m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     1.64ms    1.99ms 222.00ms   89.32%
    Req/Sec    34.79k     6.00k   45.48k    81.07%
  249013401 requests in 60.00m, 32.47GB read
  Socket errors: connect 0, read 71, write 0, timeout 0
Requests/sec:  69169.32
Transfer/sec:      9.23MB
```

HTTPS:

```text
Running 60m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.74ms    3.51ms 423.07ms   85.84%
    Req/Sec    20.51k     3.36k   47.68k    77.72%
  146725271 requests in 60.00m, 19.13GB read
  Socket errors: connect 0, read 82, write 0, timeout 0
  Non-2xx or 3xx responses: 5
Requests/sec:  40756.45
Transfer/sec:      5.44MB
```


## Error Logs

### nginx-gateway

error=context canceled;level=error;lock=nginx-gateway/ngf-longevity-nginx-gateway-fabric-leader-election;msg=Error retrieving lease lock;stacktrace=k8s.io/client-go/tools/leaderelection.(*LeaderElector).tryAcquireOrRenew
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:461
k8s.io/client-go/tools/leaderelection.(*LeaderElector).renew.func1.1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:292
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.PollUntilContextTimeout
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/poll.go:48
k8s.io/client-go/tools/leaderelection.(*LeaderElector).renew.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:285
k8s.io/apimachinery/pkg/util/wait.BackoffUntilWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:255
k8s.io/apimachinery/pkg/util/wait.BackoffUntilWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:256
k8s.io/apimachinery/pkg/util/wait.JitterUntilWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:223
k8s.io/apimachinery/pkg/util/wait.UntilWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:172
k8s.io/client-go/tools/leaderelection.(*LeaderElector).renew
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:284
k8s.io/client-go/tools/leaderelection.(*LeaderElector).Run
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:221
sigs.k8s.io/controller-runtime/pkg/manager.(*controllerManager).Start.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/manager/internal.go:470;ts=2026-06-12T22:47:56Z

### nginx
2026/06/12 21:59:03 [error] 57#57: *664535 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/12 21:57:02 [error] 60#60: *621723 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/12 21:55:06 [error] 61#61: *583483 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/12 21:42:04 [error] 62#62: *299156 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/12 21:35:03 [error] 54#54: *141139 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com

10.138.0.23 - - [12/Jun/2026:21:59:03 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/12 21:59:03 [error] 57#57: *664535 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/12 21:57:02 [error] 60#60: *621723 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.23 - - [12/Jun/2026:21:57:02 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.23 - - [12/Jun/2026:21:55:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/12 21:55:06 [error] 61#61: *583483 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.23 - - [12/Jun/2026:21:42:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/12 21:42:04 [error] 62#62: *299156 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.23 - - [12/Jun/2026:21:35:03 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/12 21:35:03 [error] 54#54: *141139 no live upstreams while connecting to upstream, client: 10.138.0.23, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com

