# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 28d0224c5f1617ace603b72889b5bb7aa272ea20
- Date: 2026-06-01T17:32:15Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1291
- Average Time: 33ms
- Event Batch Processing distribution:
	- 500.0ms: 1231
	- 1000.0ms: 1291
	- 5000.0ms: 1291
	- 10000.0ms: 1291
	- 30000.0ms: 1291
	- +Infms: 1291

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 10
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1359
- Average Time: 34ms
- Event Batch Processing distribution:
	- 500.0ms: 1295
	- 1000.0ms: 1359
	- 5000.0ms: 1359
	- 10000.0ms: 1359
	- 30000.0ms: 1359
	- +Infms: 1359

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 60
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2085
- Average Time: 112ms
- Event Batch Processing distribution:
	- 500.0ms: 2005
	- 1000.0ms: 2084
	- 5000.0ms: 2085
	- 10000.0ms: 2085
	- 30000.0ms: 2085
	- +Infms: 2085

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

- Total: 61
- Average Time: 396ms
- Event Batch Processing distribution:
	- 500.0ms: 33
	- 1000.0ms: 61
	- 5000.0ms: 61
	- 10000.0ms: 61
	- 30000.0ms: 61
	- +Infms: 61

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 243
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 875.196µs
Latencies     [min, mean, 50, 90, 95, 99, max]  678.635µs, 869.05µs, 848.65µs, 946.774µs, 986.269µs, 1.171ms, 15.151ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.039ms
Latencies     [min, mean, 50, 90, 95, 99, max]  843.037µs, 1.046ms, 1.023ms, 1.146ms, 1.208ms, 1.385ms, 18.785ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
