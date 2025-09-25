# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 8241478604f782eca497329ae47507b978d117b1
- Date: 2025-09-24T18:19:40Z
- Dirty: false

GKE Cluster:

- Node count: 15
- k8s version: v1.33.4-gke.1134000
- vCPUs per node: 2
- RAM per node: 4015668Ki
- Max pods per node: 110
- Zone: us-east1-b
- Instance Type: e2-medium

## Test TestScale_Listeners

### Event Batch Processing

- Total: 206
- Average Time: 21ms
- Event Batch Processing distribution:
	- 500.0ms: 201
	- 1000.0ms: 206
	- 5000.0ms: 206
	- 10000.0ms: 206
	- 30000.0ms: 206
	- +Infms: 206

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

- Total: 270
- Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 265
	- 1000.0ms: 270
	- 5000.0ms: 270
	- 10000.0ms: 270
	- 30000.0ms: 270
	- +Infms: 270

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

- Total: 1011
- Average Time: 642ms
- Event Batch Processing distribution:
	- 500.0ms: 275
	- 1000.0ms: 952
	- 5000.0ms: 1011
	- 10000.0ms: 1011
	- 30000.0ms: 1011
	- +Infms: 1011

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

- Total: 56
- Average Time: 485ms
- Event Batch Processing distribution:
	- 500.0ms: 25
	- 1000.0ms: 55
	- 5000.0ms: 56
	- 10000.0ms: 56
	- 30000.0ms: 56
	- +Infms: 56

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
Requests      [total, rate, throughput]         30000, 1000.01, 999.93
Duration      [total, attack, wait]             30.002s, 30s, 2.345ms
Latencies     [min, mean, 50, 90, 95, 99, max]  881.212µs, 1.578ms, 1.241ms, 1.674ms, 2.31ms, 8.31ms, 77.09ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.47
Duration      [total, attack, wait]             30.016s, 29.999s, 16.883ms
Latencies     [min, mean, 50, 90, 95, 99, max]  1.013ms, 2.205ms, 1.589ms, 2.295ms, 4.089ms, 18.773ms, 65.367ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
