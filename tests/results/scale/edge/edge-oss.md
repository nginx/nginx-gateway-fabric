# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fe52764cce240bb5a3713f56aa113694c2793f93
- Date: 2026-01-20T16:40:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2072000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 284
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 283
	- 1000.0ms: 284
	- 5000.0ms: 284
	- 10000.0ms: 284
	- 30000.0ms: 284
	- +Infms: 284

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 347
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 347
	- 1000.0ms: 347
	- 5000.0ms: 347
	- 10000.0ms: 347
	- 30000.0ms: 347
	- +Infms: 347

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1257
- Average Time: 127ms
- Event Batch Processing distribution:
	- 500.0ms: 1193
	- 1000.0ms: 1257
	- 5000.0ms: 1257
	- 10000.0ms: 1257
	- 30000.0ms: 1257
	- +Infms: 1257

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPRoutes) for more details.
The logs are attached only if there are errors.

## Test TestScale_UpstreamServers

### Event Batch Processing

- Total: 346
- Average Time: 63ms
- Event Batch Processing distribution:
	- 500.0ms: 342
	- 1000.0ms: 346
	- 5000.0ms: 346
	- 10000.0ms: 346
	- 30000.0ms: 346
	- +Infms: 346

### Errors

- NGF errors: 4
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.51
Duration      [total, attack, wait]             30s, 29.999s, 798.56µs
Latencies     [min, mean, 50, 90, 95, 99, max]  414.952µs, 775.586µs, 755.262µs, 849.262µs, 889.022µs, 1.047ms, 12.495ms
Bytes In      [total, mean]                     4857570, 161.92
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.95%
Status Codes  [code:count]                      0:15  200:29985  
Error Set:
Get "http://cafe.example.com/latte": dial tcp 0.0.0.0:0->10.138.0.63:80: connect: connection refused
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 857.312µs
Latencies     [min, mean, 50, 90, 95, 99, max]  725.791µs, 947.284µs, 925.615µs, 1.041ms, 1.088ms, 1.283ms, 14.547ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
