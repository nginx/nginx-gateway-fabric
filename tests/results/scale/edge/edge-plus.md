# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851512Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 252
- Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 246
	- 1000.0ms: 252
	- 5000.0ms: 252
	- 10000.0ms: 252
	- 30000.0ms: 252
	- +Infms: 252

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 313
- Average Time: 13ms
- Event Batch Processing distribution:
	- 500.0ms: 307
	- 1000.0ms: 313
	- 5000.0ms: 313
	- 10000.0ms: 313
	- 30000.0ms: 313
	- +Infms: 313

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

- Total: 1317
- Average Time: 166ms
- Event Batch Processing distribution:
	- 500.0ms: 1237
	- 1000.0ms: 1317
	- 5000.0ms: 1317
	- 10000.0ms: 1317
	- 30000.0ms: 1317
	- +Infms: 1317

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

- Total: 92
- Average Time: 224ms
- Event Batch Processing distribution:
	- 500.0ms: 77
	- 1000.0ms: 91
	- 5000.0ms: 92
	- 10000.0ms: 92
	- 30000.0ms: 92
	- +Infms: 92

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.034ms
Latencies     [min, mean, 50, 90, 95, 99, max]  736.353µs, 950.963µs, 924.056µs, 1.035ms, 1.08ms, 1.246ms, 28.809ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 934.45µs
Latencies     [min, mean, 50, 90, 95, 99, max]  842.51µs, 1.048ms, 1.024ms, 1.151ms, 1.208ms, 1.357ms, 20.64ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
