# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 394bdf0e0c8ae008009546b7a21d8f80248d52be
- Date: 2026-07-30T17:54:44Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1250000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1293
- Average Time: 34ms
- Event Batch Processing distribution:
	- 500.0ms: 1232
	- 1000.0ms: 1292
	- 5000.0ms: 1293
	- 10000.0ms: 1293
	- 30000.0ms: 1293
	- +Infms: 1293

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 4
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1355
- Average Time: 36ms
- Event Batch Processing distribution:
	- 500.0ms: 1291
	- 1000.0ms: 1354
	- 5000.0ms: 1355
	- 10000.0ms: 1355
	- 30000.0ms: 1355
	- +Infms: 1355

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 54
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2093
- Average Time: 93ms
- Event Batch Processing distribution:
	- 500.0ms: 2045
	- 1000.0ms: 2092
	- 5000.0ms: 2093
	- 10000.0ms: 2093
	- 30000.0ms: 2093
	- +Infms: 2093

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

- Total: 53
- Average Time: 403ms
- Event Batch Processing distribution:
	- 500.0ms: 31
	- 1000.0ms: 47
	- 5000.0ms: 53
	- 10000.0ms: 53
	- 30000.0ms: 53
	- +Infms: 53

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 28
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 896.883µs
Latencies     [min, mean, 50, 90, 95, 99, max]  731.843µs, 938.651µs, 917.363µs, 1.01ms, 1.046ms, 1.207ms, 20.039ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.177ms
Latencies     [min, mean, 50, 90, 95, 99, max]  855.019µs, 1.052ms, 1.029ms, 1.144ms, 1.2ms, 1.358ms, 25.392ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
