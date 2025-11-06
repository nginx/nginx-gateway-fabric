# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: b41c973c8399458984def3c2a8a268a237c864c8
- Date: 2025-10-30T03:04:40Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1162000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 205
- Average Time: 20ms
- Event Batch Processing distribution:
	- 500.0ms: 199
	- 1000.0ms: 205
	- 5000.0ms: 205
	- 10000.0ms: 205
	- 30000.0ms: 205
	- +Infms: 205

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

- Total: 268
- Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 262
	- 1000.0ms: 268
	- 5000.0ms: 268
	- 10000.0ms: 268
	- 30000.0ms: 268
	- +Infms: 268

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1009
- Average Time: 195ms
- Event Batch Processing distribution:
	- 500.0ms: 967
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

- Total: 45
- Average Time: 387ms
- Event Batch Processing distribution:
	- 500.0ms: 34
	- 1000.0ms: 43
	- 5000.0ms: 45
	- 10000.0ms: 45
	- 30000.0ms: 45
	- +Infms: 45

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
Duration      [total, attack, wait]             30s, 29.999s, 902.858µs
Latencies     [min, mean, 50, 90, 95, 99, max]  716.896µs, 911.6µs, 891.329µs, 980.97µs, 1.017ms, 1.159ms, 16.929ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 963.339µs
Latencies     [min, mean, 50, 90, 95, 99, max]  831.179µs, 1.019ms, 998.918µs, 1.128ms, 1.18ms, 1.32ms, 11.719ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
