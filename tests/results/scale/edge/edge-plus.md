# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 8d05ff790967ecafacfd91116c3bf19af64a767d
- Date: 2026-08-27T15:51:41Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1710000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1295
- Average Time: 32ms
- Event Batch Processing distribution:
	- 500.0ms: 1236
	- 1000.0ms: 1295
	- 5000.0ms: 1295
	- 10000.0ms: 1295
	- 30000.0ms: 1295
	- +Infms: 1295

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 4
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1361
- Average Time: 35ms
- Event Batch Processing distribution:
	- 500.0ms: 1302
	- 1000.0ms: 1361
	- 5000.0ms: 1361
	- 10000.0ms: 1361
	- 30000.0ms: 1361
	- +Infms: 1361

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 52
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2085
- Average Time: 98ms
- Event Batch Processing distribution:
	- 500.0ms: 2042
	- 1000.0ms: 2085
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

- Total: 69
- Average Time: 290ms
- Event Batch Processing distribution:
	- 500.0ms: 54
	- 1000.0ms: 68
	- 5000.0ms: 69
	- 10000.0ms: 69
	- 30000.0ms: 69
	- +Infms: 69

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 10
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.064ms
Latencies     [min, mean, 50, 90, 95, 99, max]  756.175µs, 1.02ms, 977.505µs, 1.174ms, 1.264ms, 1.525ms, 21.843ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.16ms
Latencies     [min, mean, 50, 90, 95, 99, max]  845.071µs, 1.095ms, 1.053ms, 1.249ms, 1.355ms, 1.698ms, 14.698ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
