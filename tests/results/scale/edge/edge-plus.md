# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1293
- Average Time: 32ms
- Event Batch Processing distribution:
	- 500.0ms: 1236
	- 1000.0ms: 1293
	- 5000.0ms: 1293
	- 10000.0ms: 1293
	- 30000.0ms: 1293
	- +Infms: 1293

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

- Total: 1358
- Average Time: 35ms
- Event Batch Processing distribution:
	- 500.0ms: 1294
	- 1000.0ms: 1357
	- 5000.0ms: 1358
	- 10000.0ms: 1358
	- 30000.0ms: 1358
	- +Infms: 1358

### Errors

- NGF errors: 5
- NGF container restarts: 0
- NGINX errors: 38
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2086
- Average Time: 108ms
- Event Batch Processing distribution:
	- 500.0ms: 1996
	- 1000.0ms: 2086
	- 5000.0ms: 2086
	- 10000.0ms: 2086
	- 30000.0ms: 2086
	- +Infms: 2086

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
- Average Time: 318ms
- Event Batch Processing distribution:
	- 500.0ms: 35
	- 1000.0ms: 51
	- 5000.0ms: 53
	- 10000.0ms: 53
	- 30000.0ms: 53
	- +Infms: 53

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 182
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 737.173µs
Latencies     [min, mean, 50, 90, 95, 99, max]  609.352µs, 778.502µs, 752.144µs, 849.709µs, 892.739µs, 1.085ms, 211.568ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 892.609µs
Latencies     [min, mean, 50, 90, 95, 99, max]  717.023µs, 902.473µs, 876.791µs, 990.872µs, 1.041ms, 1.293ms, 12.641ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
