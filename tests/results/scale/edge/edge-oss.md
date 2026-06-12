# Results

## Test environment

NGINX Plus: false

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

- Total: 1289
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1278
	- 1000.0ms: 1289
	- 5000.0ms: 1289
	- 10000.0ms: 1289
	- 30000.0ms: 1289
	- +Infms: 1289

### Errors

- NGF errors: 9
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1349
- Average Time: 7ms
- Event Batch Processing distribution:
	- 500.0ms: 1345
	- 1000.0ms: 1349
	- 5000.0ms: 1349
	- 10000.0ms: 1349
	- 30000.0ms: 1349
	- +Infms: 1349

### Errors

- NGF errors: 8
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2076
- Average Time: 76ms
- Event Batch Processing distribution:
	- 500.0ms: 2022
	- 1000.0ms: 2076
	- 5000.0ms: 2076
	- 10000.0ms: 2076
	- 30000.0ms: 2076
	- +Infms: 2076

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

- Total: 80
- Average Time: 123ms
- Event Batch Processing distribution:
	- 500.0ms: 75
	- 1000.0ms: 80
	- 5000.0ms: 80
	- 10000.0ms: 80
	- 30000.0ms: 80
	- +Infms: 80

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.183ms
Latencies     [min, mean, 50, 90, 95, 99, max]  736.413µs, 1.197ms, 1.154ms, 1.5ms, 1.604ms, 1.884ms, 31.531ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.95
Duration      [total, attack, wait]             30.002s, 30s, 1.821ms
Latencies     [min, mean, 50, 90, 95, 99, max]  835.696µs, 1.374ms, 1.337ms, 1.667ms, 1.801ms, 2.271ms, 22.116ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
