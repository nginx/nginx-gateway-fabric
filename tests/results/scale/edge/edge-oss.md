# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: a43969ce15ea40ce0548f0b187b16b9740113f82
- Date: 2026-09-15T14:01:26Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.7-gke.1222000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1264
- Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 1251
	- 1000.0ms: 1264
	- 5000.0ms: 1264
	- 10000.0ms: 1264
	- 30000.0ms: 1264
	- +Infms: 1264

### Errors

- NGF errors: 15
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1352
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 1339
	- 1000.0ms: 1352
	- 5000.0ms: 1352
	- 10000.0ms: 1352
	- 30000.0ms: 1352
	- +Infms: 1352

### Errors

- NGF errors: 23
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2077
- Average Time: 78ms
- Event Batch Processing distribution:
	- 500.0ms: 2010
	- 1000.0ms: 2077
	- 5000.0ms: 2077
	- 10000.0ms: 2077
	- 30000.0ms: 2077
	- +Infms: 2077

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

- Total: 74
- Average Time: 149ms
- Event Batch Processing distribution:
	- 500.0ms: 65
	- 1000.0ms: 73
	- 5000.0ms: 74
	- 10000.0ms: 74
	- 30000.0ms: 74
	- +Infms: 74

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 996.678µs
Latencies     [min, mean, 50, 90, 95, 99, max]  773.068µs, 997.658µs, 975.744µs, 1.112ms, 1.168ms, 1.318ms, 14.151ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 1.046ms
Latencies     [min, mean, 50, 90, 95, 99, max]  822.34µs, 1.077ms, 1.05ms, 1.185ms, 1.246ms, 1.456ms, 18.249ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
