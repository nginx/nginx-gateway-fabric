# Results

## Test environment

NGINX Plus: false

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

- Total: 262
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 262
	- 1000.0ms: 262
	- 5000.0ms: 262
	- 10000.0ms: 262
	- 30000.0ms: 262
	- +Infms: 262

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

- Total: 290
- Average Time: 12ms
- Event Batch Processing distribution:
	- 500.0ms: 289
	- 1000.0ms: 290
	- 5000.0ms: 290
	- 10000.0ms: 290
	- 30000.0ms: 290
	- +Infms: 290

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
- Average Time: 156ms
- Event Batch Processing distribution:
	- 500.0ms: 944
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

- Total: 74
- Average Time: 204ms
- Event Batch Processing distribution:
	- 500.0ms: 61
	- 1000.0ms: 74
	- 5000.0ms: 74
	- 10000.0ms: 74
	- 30000.0ms: 74
	- +Infms: 74

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
Requests      [total, rate, throughput]         30000, 1000.04, 999.44
Duration      [total, attack, wait]             30s, 29.999s, 985.449µs
Latencies     [min, mean, 50, 90, 95, 99, max]  391.286µs, 979.38µs, 939.236µs, 1.055ms, 1.103ms, 1.351ms, 29.2ms
Bytes In      [total, mean]                     4827263, 160.91
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.94%
Status Codes  [code:count]                      0:17  200:29983  
Error Set:
Get "http://cafe.example.com/latte": dial tcp 0.0.0.0:0->10.138.0.65:80: connect: connection refused
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.149ms
Latencies     [min, mean, 50, 90, 95, 99, max]  846.124µs, 1.062ms, 1.042ms, 1.152ms, 1.203ms, 1.365ms, 20.718ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
