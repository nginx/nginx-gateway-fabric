# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: d4376776aecc98294dc881a49cfbfa491773f74d
- Date: 2026-01-15T17:08:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2019000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 292
- Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 291
	- 1000.0ms: 292
	- 5000.0ms: 292
	- 10000.0ms: 292
	- 30000.0ms: 292
	- +Infms: 292

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

- Total: 342
- Average Time: 8ms
- Event Batch Processing distribution:
	- 500.0ms: 342
	- 1000.0ms: 342
	- 5000.0ms: 342
	- 10000.0ms: 342
	- 30000.0ms: 342
	- +Infms: 342

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

- Total: 1275
- Average Time: 115ms
- Event Batch Processing distribution:
	- 500.0ms: 1233
	- 1000.0ms: 1275
	- 5000.0ms: 1275
	- 10000.0ms: 1275
	- 30000.0ms: 1275
	- +Infms: 1275

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

- Total: 281
- Average Time: 79ms
- Event Batch Processing distribution:
	- 500.0ms: 273
	- 1000.0ms: 281
	- 5000.0ms: 281
	- 10000.0ms: 281
	- 30000.0ms: 281
	- +Infms: 281

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
Requests      [total, rate, throughput]         30000, 1000.04, 999.58
Duration      [total, attack, wait]             30s, 29.999s, 706.432µs
Latencies     [min, mean, 50, 90, 95, 99, max]  421.317µs, 749.32µs, 726.273µs, 816.043µs, 854.407µs, 1.021ms, 23.321ms
Bytes In      [total, mean]                     4797920, 159.93
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.96%
Status Codes  [code:count]                      0:13  200:29987  
Error Set:
Get "http://cafe.example.com/latte": dial tcp 0.0.0.0:0->10.138.0.47:80: connect: connection refused
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 840.211µs
Latencies     [min, mean, 50, 90, 95, 99, max]  703.481µs, 877.718µs, 849.727µs, 943.489µs, 986.841µs, 1.167ms, 21.96ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
