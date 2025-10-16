# Results

## Test environment

NGINX Plus: false

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

- Total: 249
- Average Time: 13ms
- Event Batch Processing distribution:
	- 500.0ms: 249
	- 1000.0ms: 249
	- 5000.0ms: 249
	- 10000.0ms: 249
	- 30000.0ms: 249
	- +Infms: 249

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 1
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 296
- Average Time: 13ms
- Event Batch Processing distribution:
	- 500.0ms: 295
	- 1000.0ms: 296
	- 5000.0ms: 296
	- 10000.0ms: 296
	- 30000.0ms: 296
	- +Infms: 296

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 1
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1009
- Average Time: 158ms
- Event Batch Processing distribution:
	- 500.0ms: 938
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

- Total: 104
- Average Time: 126ms
- Event Batch Processing distribution:
	- 500.0ms: 98
	- 1000.0ms: 104
	- 5000.0ms: 104
	- 10000.0ms: 104
	- 30000.0ms: 104
	- +Infms: 104

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
Duration      [total, attack, wait]             30s, 29.999s, 790.118µs
Latencies     [min, mean, 50, 90, 95, 99, max]  731.157µs, 932.108µs, 908.756µs, 1.007ms, 1.048ms, 1.214ms, 12.628ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.043ms
Latencies     [min, mean, 50, 90, 95, 99, max]  815.619µs, 1.028ms, 1.005ms, 1.131ms, 1.184ms, 1.342ms, 14.667ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
