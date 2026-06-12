# Results

## Test environment

NGINX Plus: false

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
    Latency     1.95ms    2.43ms 200.05ms   86.19%
    Req/Sec    33.45k     6.02k   46.02k    77.55%
  239411657 requests in 60.00m, 31.21GB read
  Socket errors: connect 0, read 21671, write 0, timeout 0
Requests/sec:  66501.88
Transfer/sec:      8.88MB
```

HTTPS:

```text
Running 60m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.87ms    3.52ms 212.09ms   85.21%
    Req/Sec    18.83k     3.56k   51.43k    77.96%
  134532528 requests in 60.00m, 17.54GB read
  Socket errors: connect 0, read 23698, write 0, timeout 0
Requests/sec:  37369.25
Transfer/sec:      4.99MB
```


## Error Logs

### nginx-gateway

error=ratelimitpolicies.gateway.nginx.org is forbidden: User "system:serviceaccount:nginx-gateway:ngf-longevity-nginx-gateway-fabric" cannot watch resource "ratelimitpolicies" in API group "gateway.nginx.org" at the cluster scope;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func2
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:87
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:88
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-12T22:48:15Z;type=*v1alpha1.RateLimitPolicy

### nginx




