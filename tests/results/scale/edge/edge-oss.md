# Results

## Test environment

NGINX Plus: false

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

- Total: 207
- Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 202
	- 1000.0ms: 207
	- 5000.0ms: 207
	- 10000.0ms: 207
	- 30000.0ms: 207
	- +Infms: 207

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

- Total: 269
- Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 263
	- 1000.0ms: 269
	- 5000.0ms: 269
	- 10000.0ms: 269
	- 30000.0ms: 269
	- +Infms: 269

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1009
- Average Time: 600ms
- Event Batch Processing distribution:
	- 500.0ms: 295
	- 1000.0ms: 1009
	- 5000.0ms: 1009
	- 10000.0ms: 1009
	- 30000.0ms: 1009
	- +Infms: 1009

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

- Total: 46
- Average Time: 405ms
- Event Batch Processing distribution:
	- 500.0ms: 29
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

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
Requests      [total, rate, throughput]         29999, 1000.01, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 1.057ms
Latencies     [min, mean, 50, 90, 95, 99, max]  751.608µs, 1.002ms, 965.548µs, 1.092ms, 1.151ms, 1.335ms, 22.262ms
Bytes In      [total, mean]                     4829839, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.059ms
Latencies     [min, mean, 50, 90, 95, 99, max]  823.833µs, 1.06ms, 1.039ms, 1.168ms, 1.227ms, 1.393ms, 16.671ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
