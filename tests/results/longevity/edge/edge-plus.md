# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 58dc1627b8ec4e0ac88157ad73b48cf974f71d28
- Date: 2026-05-26T23:11:43Z
- Dirty: false

GKE Cluster:

- Node count: 3
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Traffic

HTTP:

```text
Running 4320m test @ http://cafe.example.com/coffee
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     1.45ms    1.61ms   1.06s    88.46%
    Req/Sec    40.46k     3.69k   66.49k    64.36%
  20862868910 requests in 4320.00m, 2.66TB read
  Socket errors: connect 0, read 3296, write 0, timeout 0
  Non-2xx or 3xx responses: 383
Requests/sec:  80489.44
Transfer/sec:     10.75MB
```

HTTPS:

```text
Running 4320m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.37ms    2.58ms   1.04s    84.71%
    Req/Sec    22.29k     2.68k   52.84k    75.47%
  11492542094 requests in 4320.00m, 1.46TB read
  Socket errors: connect 0, read 3861, write 0, timeout 0
  Non-2xx or 3xx responses: 198
Requests/sec:  44338.50
Transfer/sec:      5.92MB
```


## Error Logs

### nginx-gateway

error=leader election lost;level=error;msg=error received after stop sequence was engaged;stacktrace=sigs.k8s.io/controller-runtime/pkg/manager.(*controllerManager).engageStopProcedure.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/manager/internal.go:533;ts=2026-06-06T04:28:56Z
error=unexpected error when reading response body. Please retry. Original error: context canceled;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
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
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-06T04:28:56Z;type=*v1alpha1.ClientSettingsPolicy
error=context canceled;level=error;logger=controller-runtime.cache;msg=Unexpected error when reading response body;stacktrace=k8s.io/client-go/rest.(*Request).transformResponse
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/rest/request.go:1196
k8s.io/client-go/rest.(*Request).Watch.func2
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/rest/request.go:811
k8s.io/client-go/rest.(*Request).Watch
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/rest/request.go:816
sigs.k8s.io/controller-runtime/pkg/cache/internal.(*Informers).makeListWatcher.func6
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/cache/internal/informers.go:571
sigs.k8s.io/controller-runtime/pkg/cache/internal.(*Informers).addInformerToMap.func2
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/cache/internal/informers.go:392
k8s.io/client-go/tools/cache.(*ListWatch).WatchWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/listwatch.go:309
k8s.io/client-go/tools/cache.(*Reflector).watch
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:605
k8s.io/client-go/tools/cache.(*Reflector).watchWithResync
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:553
k8s.io/client-go/tools/cache.(*Reflector).ListAndWatchWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:508
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:429
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
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-06T04:28:56Z
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/upstreamsettingspolicies?allowWatchBookmarks=true&resourceVersion=1780667029798804999&timeout=10s&timeoutSeconds=527&watch=true": context canceled;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
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
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-06T04:28:56Z;type=*v1alpha1.UpstreamSettingsPolicy
error=Get "https://34.118.224.1:443/apis/apps/v1/deployments?allowWatchBookmarks=true&resourceVersion=1780720136062719007&timeout=10s&timeoutSeconds=586&watch=true": context canceled;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
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
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-06T04:28:56Z;type=*v1.Deployment
error=Get "https://34.118.224.1:443/apis/coordination.k8s.io/v1/namespaces/nginx-gateway/leases/ngf-longevity-nginx-gateway-fabric-leader-election?timeout=5s": context deadline exceeded;level=error;lock=nginx-gateway/ngf-longevity-nginx-gateway-fabric-leader-election;msg=Error retrieving lease lock;stacktrace=k8s.io/client-go/tools/leaderelection.(*LeaderElector).tryAcquireOrRenew
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:461
k8s.io/client-go/tools/leaderelection.(*LeaderElector).acquire.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:261
k8s.io/apimachinery/pkg/util/wait.BackoffUntilWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:255
k8s.io/apimachinery/pkg/util/wait.BackoffUntilWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:256
k8s.io/apimachinery/pkg/util/wait.JitterUntilWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/backoff.go:223
k8s.io/client-go/tools/leaderelection.(*LeaderElector).acquire
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:259
k8s.io/client-go/tools/leaderelection.(*LeaderElector).Run
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/leaderelection/leaderelection.go:215
sigs.k8s.io/controller-runtime/pkg/manager.(*controllerManager).Start.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/manager/internal.go:470;ts=2026-06-05T14:33:02Z

### nginx
2026/06/06 00:59:06 [error] 55378#55378: *102031860 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:57:02 [error] 55374#55374: *101986100 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:52:06 [error] 55371#55371: *101880294 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:45:07 [error] 55368#55368: *101729743 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:45:05 [error] 55378#55378: *101729033 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:41:05 [error] 55378#55378: *101641705 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:41:05 [error] 55368#55368: *101641864 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:41:05 [error] 55367#55367: *101641912 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:40:06 [error] 55376#55376: *101619939 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:36:04 [error] 54089#54089: *101527409 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54082#54082: *101504676 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54096#54096: *101504739 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54085#54085: *101504482 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54089#54089: *101504540 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54088#54088: *101504520 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54097#54097: *101504606 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54090#54090: *101504628 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54087#54087: *101504860 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54093#54093: *101504392 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54092#54092: *101504579 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54095#54095: *101504552 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:34:05 [error] 54096#54096: *101484111 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:31:08 [error] 54083#54083: *101420025 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:30:06 [error] 54095#54095: *101397066 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:30:06 [error] 54096#54096: *101397085 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:29:06 [error] 54091#54091: *101375856 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:26:06 [error] 54087#54087: *101311518 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:26:06 [error] 54085#54085: *101311756 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:24:05 [error] 54083#54083: *101268787 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:23:03 [error] 54083#54083: *101246717 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:19:06 [error] 54093#54093: *101163050 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:19:04 [error] 54096#54096: *101161804 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:18:04 [error] 54083#54083: *101140716 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:16:04 [error] 54093#54093: *101098112 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:16:04 [error] 54083#54083: *101098117 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:08:04 [error] 54091#54091: *100925030 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:08:04 [error] 54085#54085: *100925145 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:03:03 [error] 54088#54088: *100817288 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:01:03 [error] 54087#54087: *100774018 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:00:03 [error] 54090#54090: *100754069 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:00:03 [error] 54097#54097: *100754066 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 21:58:05 [error] 52807#52807: *98180388 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:44:07 [error] 52800#52800: *97880236 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:44:03 [error] 52807#52807: *97879447 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:44:03 [error] 52797#52797: *97879409 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:41:04 [error] 52798#52798: *97815800 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:39:04 [error] 52804#52804: *97772369 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:25:04 [error] 52161#52161: *97468739 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:21:06 [error] 52159#52159: *97385370 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:11:07 [error] 52147#52147: *97172581 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:10:05 [error] 52160#52160: *97150494 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:10:05 [error] 52147#52147: *97150560 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:09:02 [error] 52146#52146: *97128291 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:07:05 [error] 52157#52157: *97087029 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:07:05 [error] 52152#52152: *97086811 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:06:03 [error] 52157#52157: *97064791 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:06:03 [error] 52151#52151: *97065107 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 18:58:03 [error] 49581#49581: *94360542 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:57:02 [error] 49586#49586: *94337060 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:57:02 [error] 49576#49576: *94337155 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:53:06 [error] 49584#49584: *94247465 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:53:03 [error] 49576#49576: *94245795 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:52:02 [error] 49581#49581: *94223759 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:06 [error] 49583#49583: *94115667 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:06 [error] 49577#49577: *94115708 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:06 [error] 49586#49586: *94116125 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:04 [error] 49579#49579: *94114855 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:04 [error] 49580#49580: *94114699 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:04 [error] 49585#49585: *94114559 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:04 [error] 49586#49586: *94114886 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:46:02 [error] 49571#49571: *94090475 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49570#49570: *93982213 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49582#49582: *93982121 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49579#49579: *93982498 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49578#49578: *93982255 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49584#49584: *93982207 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49575#49575: *93982239 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49585#49585: *93982311 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49576#49576: *93982374 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49586#49586: *93982514 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:40:06 [error] 49578#49578: *93961820 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:38:02 [error] 49581#49581: *93916886 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:38:02 [error] 49577#49577: *93916908 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:37:04 [error] 49575#49575: *93897104 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:36:03 [error] 49581#49581: *93875289 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:36:02 [error] 49570#49570: *93875274 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:35:06 [error] 49576#49576: *93855769 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:22:07 [error] 49575#49575: *93577512 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:21:04 [error] 49570#49570: *93555296 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:21:04 [error] 49577#49577: *93555095 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:19:04 [error] 49583#49583: *93512760 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:17:07 [error] 49577#49577: *93470514 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:16:06 [error] 49578#49578: *93448665 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:15:14 [error] 49577#49577: *93430295 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:15:14 [error] 49576#49576: *93430163 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:11:04 [error] 49581#49581: *93343358 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:10:06 [error] 49577#49577: *93322200 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:03:03 [error] 49581#49581: *93174935 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:01:05 [error] 49582#49582: *93121644 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 15:58:06 [error] 48294#48294: *90552728 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:51:06 [error] 48280#48280: *90404437 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:49:04 [error] 48290#48290: *90363024 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:48:07 [error] 48279#48279: *90342413 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:48:05 [error] 48287#48287: *90341776 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:48:05 [error] 48292#48292: *90342150 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:40:06 [error] 45045#45045: *90173192 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:40:04 [error] 45047#45047: *90172474 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:37:02 [error] 45051#45051: *90107242 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:34:02 [error] 45058#45058: *90044430 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:33:02 [error] 45046#45046: *90023635 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:30:05 [error] 45055#45055: *89961041 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:25:08 [error] 45058#45058: *89855683 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:25:08 [error] 45054#45054: *89855474 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45047#45047: *89768944 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45054#45054: *89769026 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45059#45059: *89769049 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45052#45052: *89769070 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:16:04 [error] 45047#45047: *89661228 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:14:04 [error] 45055#45055: *89619073 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:14:04 [error] 45059#45059: *89619161 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:05:06 [error] 45058#45058: *89428938 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:02:04 [error] 45055#45055: *89364731 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:01:04 [error] 45054#45054: *89344050 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 12:53:06 [error] 45059#45059: *86662331 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:52:03 [error] 45058#45058: *86640055 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:46:08 [error] 45053#45053: *86511081 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:46:08 [error] 45045#45045: *86511020 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45052#45052: *86381962 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45045#45045: *86382080 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45051#45051: *86382271 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45054#45054: *86382309 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45047#45047: *86382342 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45057#45057: *86382133 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:33:02 [error] 45045#45045: *86227047 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:31:02 [error] 45059#45059: *86185000 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:27:05 [error] 45051#45051: *86101711 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:24:06 [error] 45058#45058: *86038454 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:22:14 [error] 45060#45060: *85998042 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:21:05 [error] 45054#45054: *85973966 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:20:02 [error] 45052#45052: *85951884 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:15:02 [error] 45054#45054: *85844223 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:13:05 [error] 45060#45060: *85802749 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:13:05 [error] 45061#45061: *85802993 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:13:05 [error] 45054#45054: *85803016 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:10:03 [error] 45052#45052: *85738489 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:09:07 [error] 45045#45045: *85719049 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:09:03 [error] 45051#45051: *85717353 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:01:06 [error] 45049#45049: *85543762 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:01:06 [error] 45058#45058: *85543615 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 09:49:07 [error] 43130#43130: *82679792 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:46:06 [error] 43135#43135: *82611586 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:31:04 [error] 43127#43127: *82275568 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:25:08 [error] 43120#43120: *82142452 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:25:04 [error] 43128#43128: *82141438 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:18:04 [error] 43131#43131: *81986894 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:13:08 [error] 43130#43130: *81876180 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:11:09 [error] 43131#43131: *81833706 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:06:07 [error] 43135#43135: *81725371 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:06:07 [error] 43134#43134: *81725647 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 06:58:04 [error] 41218#41218: *78927665 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:57:04 [error] 41206#41206: *78902280 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:48:03 [error] 41214#41214: *78669646 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:40:02 [error] 41212#41212: *78481614 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:38:06 [error] 41203#41203: *78445045 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:37:02 [error] 41203#41203: *78422709 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:37:02 [error] 41213#41213: *78423030 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:35:05 [error] 41213#41213: *78375512 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:35:05 [error] 41218#41218: *78375674 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:35:05 [error] 41211#41211: *78375463 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:26:05 [error] 41212#41212: *78172684 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:24:07 [error] 41207#41207: *78129831 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:21:03 [error] 41216#41216: *78061628 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:17:06 [error] 41202#41202: *77976484 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:10:09 [error] 41214#41214: *77826224 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:10:06 [error] 41209#41209: *77824742 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:09:07 [error] 41208#41208: *77804713 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:09:03 [error] 41206#41206: *77802706 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:07 [error] 41217#41217: *77716217 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41209#41209: *77715294 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41216#41216: *77715220 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41205#41205: *77715539 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41202#41202: *77715377 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41215#41215: *77715436 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41206#41206: *77715325 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:04:08 [error] 41205#41205: *77695676 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:04:06 [error] 41215#41215: *77694973 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:02:03 [error] 40562#40562: *77648370 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:02:03 [error] 40559#40559: *77648414 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:02:03 [error] 40571#40571: *77648440 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:02:03 [error] 40564#40564: *77648276 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:00:07 [error] 40562#40562: *77607248 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:00:05 [error] 40562#40562: *77606440 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com

2026/06/06 00:59:06 [error] 55378#55378: *102031860 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:59:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:57:02 [error] 55374#55374: *101986100 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:57:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:52:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:52:06 [error] 55371#55371: *101880294 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:45:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:45:07 [error] 55368#55368: *101729743 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:45:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:45:05 [error] 55378#55378: *101729033 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:41:05 [error] 55378#55378: *101641705 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:41:05 [error] 55368#55368: *101641864 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:41:05 [error] 55367#55367: *101641912 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:40:06 [error] 55376#55376: *101619939 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:36:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:36:04 [error] 54089#54089: *101527409 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54082#54082: *101504676 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54096#54096: *101504739 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:35:02 [error] 54085#54085: *101504482 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54089#54089: *101504540 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54088#54088: *101504520 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54097#54097: *101504606 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54090#54090: *101504628 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54087#54087: *101504860 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54093#54093: *101504392 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54092#54092: *101504579 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:35:02 [error] 54095#54095: *101504552 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:35:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:34:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:34:05 [error] 54096#54096: *101484111 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:31:08 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:30:06 [error] 54095#54095: *101397066 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:30:06 [error] 54096#54096: *101397085 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:30:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:30:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:29:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:29:06 [error] 54091#54091: *101375856 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:26:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:26:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:26:06 [error] 54087#54087: *101311518 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:26:06 [error] 54085#54085: *101311756 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:24:05 [error] 54083#54083: *101268787 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:24:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:23:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:23:03 [error] 54083#54083: *101246717 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:19:06 [error] 54093#54093: *101163050 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:19:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:19:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:19:04 [error] 54096#54096: *101161804 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:18:04 [error] 54083#54083: *101140716 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:18:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:16:04 [error] 54093#54093: *101098112 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:16:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:16:04 [error] 54083#54083: *101098117 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:16:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:08:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:08:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:08:04 [error] 54091#54091: *100925030 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:08:04 [error] 54085#54085: *100925145 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:03:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:03:03 [error] 54088#54088: *100817288 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/06 00:01:03 [error] 54087#54087: *100774018 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:01:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:00:03 [error] 54090#54090: *100754069 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [06/Jun/2026:00:00:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [06/Jun/2026:00:00:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/06 00:00:03 [error] 54097#54097: *100754066 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:58:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:58:05 [error] 52807#52807: *98180388 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:44:07 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:44:07 [error] 52800#52800: *97880236 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:44:03 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:44:03 [error] 52807#52807: *97879447 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:44:03 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:44:03 [error] 52797#52797: *97879409 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:41:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:41:04 [error] 52798#52798: *97815800 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:39:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:39:04 [error] 52804#52804: *97772369 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:25:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:25:04 [error] 52161#52161: *97468739 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:21:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:21:06 [error] 52159#52159: *97385370 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:11:07 [error] 52147#52147: *97172581 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:11:07 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:10:05 [error] 52160#52160: *97150494 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:10:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:10:05 [error] 52147#52147: *97150560 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:10:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:21:09:02 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:09:02 [error] 52146#52146: *97128291 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:07:05 [error] 52157#52157: *97087029 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:07:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:21:07:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:07:05 [error] 52152#52152: *97086811 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:21:06:03 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:21:06:03 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 21:06:03 [error] 52157#52157: *97064791 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 21:06:03 [error] 52151#52151: *97065107 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:58:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:58:03 [error] 49581#49581: *94360542 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:57:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:57:02 [error] 49586#49586: *94337060 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:57:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:57:02 [error] 49576#49576: *94337155 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:53:06 [error] 49584#49584: *94247465 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:53:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:53:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:53:03 [error] 49576#49576: *94245795 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:52:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:52:02 [error] 49581#49581: *94223759 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:06 [error] 49583#49583: *94115667 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:06 [error] 49577#49577: *94115708 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:06 [error] 49586#49586: *94116125 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:47:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:47:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:47:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:47:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:47:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:47:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:47:04 [error] 49579#49579: *94114855 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:47:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:47:04 [error] 49580#49580: *94114699 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:04 [error] 49585#49585: *94114559 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:47:04 [error] 49586#49586: *94114886 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:46:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:46:02 [error] 49571#49571: *94090475 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:41:05 [error] 49570#49570: *93982213 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:41:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:41:05 [error] 49582#49582: *93982121 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49579#49579: *93982498 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49578#49578: *93982255 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49584#49584: *93982207 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49575#49575: *93982239 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49585#49585: *93982311 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49576#49576: *93982374 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:41:05 [error] 49586#49586: *93982514 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:40:06 [error] 49578#49578: *93961820 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:38:02 [error] 49581#49581: *93916886 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:38:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:38:02 [error] 49577#49577: *93916908 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:38:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:37:04 [error] 49575#49575: *93897104 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:37:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:36:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:36:03 [error] 49581#49581: *93875289 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:36:02 [error] 49570#49570: *93875274 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:36:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:35:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:35:06 [error] 49576#49576: *93855769 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:22:07 [error] 49575#49575: *93577512 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:22:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:21:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:21:04 [error] 49570#49570: *93555296 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:21:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:21:04 [error] 49577#49577: *93555095 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:19:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:19:04 [error] 49583#49583: *93512760 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:17:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:17:07 [error] 49577#49577: *93470514 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:16:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:16:06 [error] 49578#49578: *93448665 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:15:14 [error] 49577#49577: *93430295 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:15:14 [error] 49576#49576: *93430163 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:15:14 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:15:14 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:11:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:11:04 [error] 49581#49581: *93343358 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:10:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:18:03:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 18:03:03 [error] 49581#49581: *93174935 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 18:01:05 [error] 49582#49582: *93121644 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:18:01:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:58:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:58:06 [error] 48294#48294: *90552728 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:51:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:51:06 [error] 48280#48280: *90404437 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:49:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:49:04 [error] 48290#48290: *90363024 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:48:07 [error] 48279#48279: *90342413 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:48:07 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:48:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:48:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:48:05 [error] 48287#48287: *90341776 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:48:05 [error] 48292#48292: *90342150 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:40:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:40:06 [error] 45045#45045: *90173192 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:40:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:40:04 [error] 45047#45047: *90172474 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:37:02 [error] 45051#45051: *90107242 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:37:02 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:34:02 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:34:02 [error] 45058#45058: *90044430 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:33:02 [error] 45046#45046: *90023635 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:33:02 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:30:05 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:30:05 [error] 45055#45055: *89961041 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:25:08 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:25:08 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:25:08 [error] 45058#45058: *89855683 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:25:08 [error] 45054#45054: *89855474 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:21:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:21:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:21:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:21:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:21:06 [error] 45047#45047: *89768944 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45054#45054: *89769026 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45059#45059: *89769049 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:21:06 [error] 45052#45052: *89769070 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:16:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:16:04 [error] 45047#45047: *89661228 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:14:04 [error] 45055#45055: *89619073 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:14:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:14:04 [error] 45059#45059: *89619161 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:14:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:15:05:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:05:06 [error] 45058#45058: *89428938 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:02:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 15:02:04 [error] 45055#45055: *89364731 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 15:01:04 [error] 45054#45054: *89344050 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:15:01:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:53:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:53:06 [error] 45059#45059: *86662331 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:52:03 [error] 45058#45058: *86640055 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:52:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:46:08 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:46:08 [error] 45053#45053: *86511081 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:46:08 [error] 45045#45045: *86511020 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:46:08 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:40:06 [error] 45052#45052: *86381962 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:40:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:40:06 [error] 45045#45045: *86382080 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45051#45051: *86382271 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45054#45054: *86382309 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45047#45047: *86382342 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:40:06 [error] 45057#45057: *86382133 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:33:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:33:02 [error] 45045#45045: *86227047 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:31:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:31:02 [error] 45059#45059: *86185000 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:27:05 [error] 45051#45051: *86101711 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:27:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:24:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:24:06 [error] 45058#45058: *86038454 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:22:14 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:22:14 [error] 45060#45060: *85998042 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:21:05 [error] 45054#45054: *85973966 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:21:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:20:02 [error] 45052#45052: *85951884 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:20:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:15:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:15:02 [error] 45054#45054: *85844223 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:13:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:13:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:12:13:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:13:05 [error] 45060#45060: *85802749 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:13:05 [error] 45061#45061: *85802993 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 12:13:05 [error] 45054#45054: *85803016 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:10:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:10:03 [error] 45052#45052: *85738489 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:09:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:09:07 [error] 45045#45045: *85719049 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:09:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:09:03 [error] 45051#45051: *85717353 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:01:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:01:06 [error] 45049#45049: *85543762 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:12:01:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 12:01:06 [error] 45058#45058: *85543615 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 09:49:07 [error] 43130#43130: *82679792 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:49:07 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:46:06 [error] 43135#43135: *82611586 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:46:06 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:31:04 [error] 43127#43127: *82275568 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:31:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:25:08 [error] 43120#43120: *82142452 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:25:08 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:09:25:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:25:04 [error] 43128#43128: *82141438 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:18:04 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:18:04 [error] 43131#43131: *81986894 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:13:08 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:13:08 [error] 43130#43130: *81876180 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:11:09 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 09:11:09 [error] 43131#43131: *81833706 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:06:07 [error] 43135#43135: *81725371 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
2026/06/05 09:06:07 [error] 43134#43134: *81725647 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: http://longevity_tea_80/tea, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:09:06:07 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:09:06:07 +0000] "GET /tea HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:58:04 [error] 41218#41218: *78927665 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:58:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:57:04 [error] 41206#41206: *78902280 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:57:04 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:48:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:48:03 [error] 41214#41214: *78669646 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:40:02 [error] 41212#41212: *78481614 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:40:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:38:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:38:06 [error] 41203#41203: *78445045 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:37:02 [error] 41203#41203: *78422709 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:37:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:37:02 [error] 41213#41213: *78423030 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:37:02 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:35:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:35:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:35:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:35:05 [error] 41213#41213: *78375512 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:35:05 [error] 41218#41218: *78375674 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:35:05 [error] 41211#41211: *78375463 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:26:05 [error] 41212#41212: *78172684 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:26:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:24:07 [error] 41207#41207: *78129831 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:24:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:21:03 [error] 41216#41216: *78061628 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:21:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:17:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:17:06 [error] 41202#41202: *77976484 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:10:09 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:10:09 [error] 41214#41214: *77826224 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:10:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:10:06 [error] 41209#41209: *77824742 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:09:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:09:07 [error] 41208#41208: *77804713 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:09:03 [error] 41206#41206: *77802706 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:09:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:05:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:05:07 [error] 41217#41217: *77716217 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:05:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:05:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:05:05 [error] 41209#41209: *77715294 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41216#41216: *77715220 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41205#41205: *77715539 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41202#41202: *77715377 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:05:05 [error] 41215#41215: *77715436 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:05:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:05:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:05:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:05:05 [error] 41206#41206: *77715325 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:05:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:04:08 [error] 41205#41205: *77695676 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:04:08 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:04:06 [error] 41215#41215: *77694973 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:04:06 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:02:03 [error] 40562#40562: *77648370 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:02:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:02:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:02:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
10.138.0.52 - - [05/Jun/2026:06:02:03 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:02:03 [error] 40559#40559: *77648414 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:02:03 [error] 40571#40571: *77648440 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
2026/06/05 06:02:03 [error] 40564#40564: *77648276 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:00:07 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:00:07 [error] 40562#40562: *77607248 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com
10.138.0.52 - - [05/Jun/2026:06:00:05 +0000] "GET /coffee HTTP/1.1" 502 150 "-" "-"
2026/06/05 06:00:05 [error] 40562#40562: *77606440 no live upstreams while connecting to upstream, client: 10.138.0.52, server: cafe.example.com, request: "GET /coffee HTTP/1.1", upstream: http://longevity_coffee_80/coffee, host: cafe.example.com

