# Results

## Test environment

NGINX Plus: false

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
    Latency     1.42ms    1.73ms   1.06s    89.88%
    Req/Sec    40.41k     3.84k   68.90k    65.35%
  20833634935 requests in 4320.00m, 2.65TB read
  Socket errors: connect 0, read 910913, write 0, timeout 0
Requests/sec:  80376.66
Transfer/sec:     10.73MB
```

HTTPS:

```text
Running 4320m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.34ms    2.69ms   1.07s    85.94%
    Req/Sec    22.78k     2.88k   54.85k    79.52%
  11737589765 requests in 4320.00m, 1.50TB read
  Socket errors: connect 0, read 1054902, write 0, timeout 152
  Non-2xx or 3xx responses: 34531040
Requests/sec:  45283.90
Transfer/sec:      6.07MB
```


## Error Logs

### nginx-gateway

error=Get "https://34.118.224.1:443/apis/gateway.networking.k8s.io/v1/gatewayclasses?allowWatchBookmarks=true&resourceVersion=1780449827730719005&timeout=10s&timeoutSeconds=414&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.GatewayClass
error=Get "https://34.118.224.1:443/apis/gateway.networking.k8s.io/v1/tlsroutes?allowWatchBookmarks=true&resourceVersion=1780449827541627000&timeout=10s&timeoutSeconds=482&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.TLSRoute
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/clientsettingspolicies?allowWatchBookmarks=true&resourceVersion=1780449827985361000&timeout=10s&timeoutSeconds=332&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.ClientSettingsPolicy
error=Get "https://34.118.224.1:443/apis/gateway.networking.k8s.io/v1/referencegrants?allowWatchBookmarks=true&resourceVersion=1780449827546392000&timeout=10s&timeoutSeconds=493&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.ReferenceGrant
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/nginxgateways?allowWatchBookmarks=true&resourceVersion=1780449828064751012&timeout=10s&timeoutSeconds=414&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.NginxGateway
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/ratelimitpolicies?allowWatchBookmarks=true&resourceVersion=1780449827938473000&timeout=10s&timeoutSeconds=545&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.RateLimitPolicy
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha2/observabilitypolicies?allowWatchBookmarks=true&resourceVersion=1780449828009503000&timeout=10s&timeoutSeconds=494&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha2.ObservabilityPolicy
error=Get "https://34.118.224.1:443/apis/autoscaling/v2/horizontalpodautoscalers?allowWatchBookmarks=true&resourceVersion=1780667916858655018&timeout=10s&timeoutSeconds=401&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v2.HorizontalPodAutoscaler
error=Get "https://34.118.224.1:443/apis/discovery.k8s.io/v1/endpointslices?allowWatchBookmarks=true&resourceVersion=1780664361013087022&timeout=10s&timeoutSeconds=315&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.EndpointSlice
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/wafpolicies?allowWatchBookmarks=true&resourceVersion=1780449828073603000&timeout=10s&timeoutSeconds=463&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.WAFPolicy
error=Get "https://34.118.224.1:443/apis/apps/v1/deployments?allowWatchBookmarks=true&resourceVersion=1780664361055023001&timeout=10s&timeoutSeconds=588&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.Deployment
error=Get "https://34.118.224.1:443/apis/apiextensions.k8s.io/v1/customresourcedefinitions?allowWatchBookmarks=true&resourceVersion=1780449827576418000&timeout=9m50s&timeoutSeconds=590&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.PartialObjectMetadata
error=Get "https://34.118.224.1:443/apis/gateway.networking.k8s.io/v1alpha2/udproutes?allowWatchBookmarks=true&resourceVersion=1780449827560633000&timeout=10s&timeoutSeconds=557&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha2.UDPRoute
error=Get "https://34.118.224.1:443/apis/gateway.networking.k8s.io/v1/gateways?allowWatchBookmarks=true&resourceVersion=1780650722932847012&timeout=10s&timeoutSeconds=478&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.Gateway
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/snippetspolicies?allowWatchBookmarks=true&resourceVersion=1780449828031569000&timeout=10s&timeoutSeconds=413&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.SnippetsPolicy
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/authenticationfilters?allowWatchBookmarks=true&resourceVersion=1780449827630194000&timeout=10s&timeoutSeconds=448&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.AuthenticationFilter
error=Get "https://34.118.224.1:443/apis/gateway.nginx.org/v1alpha1/upstreamsettingspolicies?allowWatchBookmarks=true&resourceVersion=1780449828109586000&timeout=10s&timeoutSeconds=375&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1alpha1.UpstreamSettingsPolicy
error=Get "https://34.118.224.1:443/apis/gateway.networking.k8s.io/v1/grpcroutes?allowWatchBookmarks=true&resourceVersion=1780449827537988000&timeout=10s&timeoutSeconds=458&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.GRPCRoute
error=Get "https://34.118.224.1:443/api/v1/namespaces?allowWatchBookmarks=true&resourceVersion=1780449840883551007&timeout=10s&timeoutSeconds=391&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.Namespace
error=Get "https://34.118.224.1:443/api/v1/configmaps?allowWatchBookmarks=true&resourceVersion=1780667914524079016&timeout=10s&timeoutSeconds=301&watch=true": http2: client connection force closed via ClientConn.Close;level=error;logger=controller-runtime.cache.UnhandledError;msg=Failed to watch;reflector=/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:343;stacktrace=k8s.io/apimachinery/pkg/util/runtime.logError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:252
k8s.io/apimachinery/pkg/util/runtime.handleError
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:243
k8s.io/apimachinery/pkg/util/runtime.HandleErrorWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/runtime/runtime.go:229
k8s.io/client-go/tools/cache.DefaultWatchErrorHandler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:227
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:430
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:53
k8s.io/apimachinery/pkg/util/wait.loopConditionUntilContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/loop.go:54
k8s.io/apimachinery/pkg/util/wait.DelayFunc.Until
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/delay.go:39
k8s.io/client-go/tools/cache.(*Reflector).RunWithContext
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/client-go@v0.36.1/tools/cache/reflector.go:428
k8s.io/client-go/tools/cache.(*controller).RunWithContext.(*Group).StartWithContext.func3
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:63
k8s.io/apimachinery/pkg/util/wait.(*Group).Start.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/k8s.io/apimachinery@v0.36.1/pkg/util/wait/wait.go:72;ts=2026-06-05T13:58:37Z;type=*v1.ConfigMap
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"BqgyHZRh4yrBecHnOsfjYgt5BIWZvnfNPjI0lfxgLoU="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:10:58Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"lweQNRshY8brWcCrzWxw7lyTIVgkhCJhWWJk6oOEE2A="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:10:24Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"nMag9lohBNr2W8HBPhdrwlDwY+zaIeAsNGC2n7fGvn0="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:09:25Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"2HCTlEigvXX/e9vXtKrA1Z8oOBG9lOYAmbgWmE2Dzcg="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:08:40Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"EaTgSUee2dCEtY1dJ6dTvzNrgkg6h2G+EhAdtm5fMzE="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:07:51Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"DTPOnqeB0xw65pCHYiga/B/jz4B4oLyfR6x10z02cYU="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:07:16Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"0gH95BL/8XVwfeM4LGS6Duk2q0xpqLNXQ82XwpoD4z8="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:06:19Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"DqhTcOlliuEU2u8gL+JoHrbwnAk2TnhseeJFnp2lXMw="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:05:22Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"EHJu90MuHm9OH+58KwbSRmejAYlvrncS6GxVFoVuh3s="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:04:30Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"6q1+UkLFlIu6GYLDz/9AgCDjT/tm8OXwbmbMx92ZC6Q="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:03:37Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"VEx84iiOH05It8UV/UnzDztB6L5YmF1xfNxI6cArCXA="  permissions:"0644"  size:6257: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:03:02Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"BopGKRZOpCbF+WcRJiWwkZHj3GaXMqt35A+zU9RVr1E="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:02:02Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"BopGKRZOpCbF+WcRJiWwkZHj3GaXMqt35A+zU9RVr1E="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:02:01Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"yLK2jmERDErOLphonYXV4uYUQe5YPR0spxfP1IkP+jY="  permissions:"0644"  size:6229: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:01:25Z
error=msg: Config apply failed, rolling back config; error: error getting file data for name:"/etc/nginx/conf.d/http.conf"  hash:"0qT+jO+pKQrjKwyHJKgoxGFLWM/px10oYnSwbAx7a3Y="  permissions:"0644"  size:6227: rpc error: code = NotFound desc = connection not found;level=error;logger=eventHandler;msg=Failed to update NGINX configuration;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller.(*eventHandlerImpl).waitForStatusUpdates
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/handler.go:514;ts=2026-06-05T09:00:41Z

### nginx
2026/06/05 09:00:19 [warn] 229432#229432: *91933257 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229432#229432: *91933257 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229431#229431: *91932631 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229431#229431: *91932631 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229442#229442: *91933311 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229442#229442: *91933311 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229441#229441: *91932439 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229441#229441: *91932439 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229444#229444: *91932433 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229444#229444: *91932433 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229444#229444: *91932807 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229444#229444: *91932807 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229432#229432: *91933111 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229432#229432: *91933111 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229442#229442: *91933239 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229432#229432: *91933363 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229432#229432: *91933363 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229440#229440: *91932782 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229440#229440: *91932782 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229442#229442: *91933239 upstream prematurely closed connection while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [warn] 229440#229440: *91931908 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [error] 229440#229440: *91931908 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [warn] 229434#229434: *91931811 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [error] 229434#229434: *91931811 upstream prematurely closed connection while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"

2026/06/05 09:00:19 [warn] 229432#229432: *91933257 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229432#229432: *91933257 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229431#229431: *91932631 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229431#229431: *91932631 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229442#229442: *91933311 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229442#229442: *91933311 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229441#229441: *91932439 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229441#229441: *91932439 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229444#229444: *91932433 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229444#229444: *91932433 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229444#229444: *91932807 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229444#229444: *91932807 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229432#229432: *91933111 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229432#229432: *91933111 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229442#229442: *91933239 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229432#229432: *91933363 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229432#229432: *91933363 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [warn] 229440#229440: *91932782 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229440#229440: *91932782 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:19 [error] 229442#229442: *91933239 upstream prematurely closed connection while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.1.31:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [warn] 229440#229440: *91931908 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [error] 229440#229440: *91931908 recv() failed (104: Connection reset by peer) while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [warn] 229434#229434: *91931811 upstream server temporarily disabled while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"
2026/06/05 09:00:18 [error] 229434#229434: *91931811 upstream prematurely closed connection while reading response header from upstream, client: 10.138.0.49, server: cafe.example.com, request: "GET /tea HTTP/1.1", upstream: "http://10.8.0.56:8080/tea", host: "cafe.example.com"

