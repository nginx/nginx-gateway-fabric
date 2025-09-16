# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 635b3fcd6e643f4bd24ebbd4c901619a030c4bc0
- Date: 2025-09-15T17:56:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1036000
- vCPUs per node: 16
- RAM per node: 65851528Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 203
- Average Time: 39ms
- Event Batch Processing distribution:
	- 500.0ms: 199
	- 1000.0ms: 201
	- 5000.0ms: 203
	- 10000.0ms: 203
	- 30000.0ms: 203
	- +Infms: 203

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 266
- Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 261
	- 1000.0ms: 264
	- 5000.0ms: 266
	- 10000.0ms: 266
	- 30000.0ms: 266
	- +Infms: 266

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

- Total: 1010
- Average Time: 696ms
- Event Batch Processing distribution:
	- 500.0ms: 163
	- 1000.0ms: 992
	- 5000.0ms: 1010
	- 10000.0ms: 1010
	- 30000.0ms: 1010
	- +Infms: 1010

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

- Total: 54
- Average Time: 403ms
- Event Batch Processing distribution:
	- 500.0ms: 37
	- 1000.0ms: 53
	- 5000.0ms: 54
	- 10000.0ms: 54
	- 30000.0ms: 54
	- +Infms: 54

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.078ms
Latencies     [min, mean, 50, 90, 95, 99, max]  740.672µs, 968.805µs, 941.919µs, 1.063ms, 1.113ms, 1.293ms, 13.259ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.012ms
Latencies     [min, mean, 50, 90, 95, 99, max]  841.073µs, 1.062ms, 1.042ms, 1.16ms, 1.218ms, 1.386ms, 15.126ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
