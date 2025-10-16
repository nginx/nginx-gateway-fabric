# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9010072ecd34a8fa99bfdd3d7580c9d725fb063e
- Date: 2025-10-01T09:39:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1172000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 204
- Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 198
	- 1000.0ms: 204
	- 5000.0ms: 204
	- 10000.0ms: 204
	- 30000.0ms: 204
	- +Infms: 204

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

- Total: 268
- Average Time: 19ms
- Event Batch Processing distribution:
	- 500.0ms: 261
	- 1000.0ms: 268
	- 5000.0ms: 268
	- 10000.0ms: 268
	- 30000.0ms: 268
	- +Infms: 268

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

- Total: 1009
- Average Time: 219ms
- Event Batch Processing distribution:
	- 500.0ms: 925
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

- Total: 48
- Average Time: 392ms
- Event Batch Processing distribution:
	- 500.0ms: 34
	- 1000.0ms: 47
	- 5000.0ms: 48
	- 10000.0ms: 48
	- 30000.0ms: 48
	- +Infms: 48

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
Duration      [total, attack, wait]             30s, 29.999s, 867.362µs
Latencies     [min, mean, 50, 90, 95, 99, max]  699.209µs, 952.288µs, 915.354µs, 1.046ms, 1.101ms, 1.287ms, 22.891ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.044ms
Latencies     [min, mean, 50, 90, 95, 99, max]  839.937µs, 1.057ms, 1.034ms, 1.158ms, 1.218ms, 1.39ms, 15.677ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
