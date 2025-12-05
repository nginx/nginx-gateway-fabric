# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 296
- Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 295
	- 1000.0ms: 296
	- 5000.0ms: 296
	- 10000.0ms: 296
	- 30000.0ms: 296
	- +Infms: 296

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

- Total: 346
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 346
	- 1000.0ms: 346
	- 5000.0ms: 346
	- 10000.0ms: 346
	- 30000.0ms: 346
	- +Infms: 346

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

- Total: 1253
- Average Time: 137ms
- Event Batch Processing distribution:
	- 500.0ms: 1164
	- 1000.0ms: 1253
	- 5000.0ms: 1253
	- 10000.0ms: 1253
	- 30000.0ms: 1253
	- +Infms: 1253

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

- Total: 110
- Average Time: 115ms
- Event Batch Processing distribution:
	- 500.0ms: 98
	- 1000.0ms: 110
	- 5000.0ms: 110
	- 10000.0ms: 110
	- 30000.0ms: 110
	- +Infms: 110

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 898.497µs
Latencies     [min, mean, 50, 90, 95, 99, max]  777.931µs, 1.019ms, 990.843µs, 1.113ms, 1.164ms, 1.337ms, 16.826ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.061ms
Latencies     [min, mean, 50, 90, 95, 99, max]  866.498µs, 1.085ms, 1.058ms, 1.173ms, 1.228ms, 1.425ms, 25.851ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
