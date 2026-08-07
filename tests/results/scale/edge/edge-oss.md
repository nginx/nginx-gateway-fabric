# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 394bdf0e0c8ae008009546b7a21d8f80248d52be
- Date: 2026-07-30T17:54:44Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1250000
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1281
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1269
	- 1000.0ms: 1281
	- 5000.0ms: 1281
	- 10000.0ms: 1281
	- 30000.0ms: 1281
	- +Infms: 1281

### Errors

- NGF errors: 14
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1350
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 1340
	- 1000.0ms: 1350
	- 5000.0ms: 1350
	- 10000.0ms: 1350
	- 30000.0ms: 1350
	- +Infms: 1350

### Errors

- NGF errors: 19
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2075
- Average Time: 85ms
- Event Batch Processing distribution:
	- 500.0ms: 1985
	- 1000.0ms: 2075
	- 5000.0ms: 2075
	- 10000.0ms: 2075
	- 30000.0ms: 2075
	- +Infms: 2075

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

- Total: 66
- Average Time: 202ms
- Event Batch Processing distribution:
	- 500.0ms: 54
	- 1000.0ms: 66
	- 5000.0ms: 66
	- 10000.0ms: 66
	- 30000.0ms: 66
	- +Infms: 66

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 897.41µs
Latencies     [min, mean, 50, 90, 95, 99, max]  752.181µs, 950.287µs, 929.833µs, 1.027ms, 1.068ms, 1.225ms, 17.408ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.203ms
Latencies     [min, mean, 50, 90, 95, 99, max]  823.259µs, 1.048ms, 1.029ms, 1.139ms, 1.191ms, 1.327ms, 15.837ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
