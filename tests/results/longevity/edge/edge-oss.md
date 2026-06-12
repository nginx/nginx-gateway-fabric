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
    Latency     1.74ms    2.60ms 415.03ms   90.17%
    Req/Sec    33.96k     6.05k   48.23k    79.05%
  243011984 requests in 60.00m, 31.68GB read
  Socket errors: connect 1, read 0, write 0, timeout 0
Requests/sec:  67502.45
Transfer/sec:      9.01MB
```

HTTPS:

```text
Running 60m test @ https://cafe.example.com/tea
  2 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     2.79ms    3.26ms 222.99ms   84.49%
    Req/Sec    19.64k     3.54k   48.38k    74.71%
  140461364 requests in 60.00m, 18.31GB read
Requests/sec:  39016.09
Transfer/sec:      5.21MB
```


## Error Logs

### nginx-gateway

error=leader election lost;level=error;msg=error received after stop sequence was engaged;stacktrace=sigs.k8s.io/controller-runtime/pkg/manager.(*controllerManager).engageStopProcedure.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/manager/internal.go:533;ts=2026-06-12T20:57:29Z
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
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/sigs.k8s.io/controller-runtime@v0.24.1/pkg/manager/internal.go:470;ts=2026-06-12T20:57:29Z
error=rpc error: code = Internal desc = error creating TokenReview: Post "https://34.118.224.1:443/apis/authentication.k8s.io/v1/tokenreviews?timeout=10s": context canceled;level=error;logger=agentGRPCServer;msg=error validating connection;stacktrace=github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/interceptor.(*ContextSetter).Unary.ContextSetter.Unary.func1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/internal/controller/nginx/agent/grpc/interceptor/interceptor.go:77
github.com/nginx/agent/v3/api/grpc/mpi/v1._CommandService_UpdateDataPlaneStatus_Handler
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/github.com/nginx/agent/v3@v3.10.4/api/grpc/mpi/v1/command_grpc.pb.go:211
google.golang.org/grpc.(*Server).processUnaryRPC
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/google.golang.org/grpc@v1.81.1/server.go:1430
google.golang.org/grpc.(*Server).handleStream
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/google.golang.org/grpc@v1.81.1/server.go:1856
google.golang.org/grpc.(*Server).serveStreams.func2.1
	/opt/actions-runner/_work/nginx-gateway-fabric/nginx-gateway-fabric/.gocache/google.golang.org/grpc@v1.81.1/server.go:1065;ts=2026-06-12T20:56:38Z

### nginx




