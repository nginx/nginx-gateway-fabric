# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: e8ee7c1c4f14e249927a5447a1af2615ddbe0f87
- Date: 2025-12-17T20:04:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 257
- Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 253
	- 1000.0ms: 257
	- 5000.0ms: 257
	- 10000.0ms: 257
	- 30000.0ms: 257
	- +Infms: 257

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

- Total: 323
- Average Time: 12ms
- Event Batch Processing distribution:
	- 500.0ms: 317
	- 1000.0ms: 323
	- 5000.0ms: 323
	- 10000.0ms: 323
	- 30000.0ms: 323
	- +Infms: 323

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

- Total: 1313
- Average Time: 142ms
- Event Batch Processing distribution:
	- 500.0ms: 1285
	- 1000.0ms: 1313
	- 5000.0ms: 1313
	- 10000.0ms: 1313
	- 30000.0ms: 1313
	- +Infms: 1313

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

- Total: 89
- Average Time: 255ms
- Event Batch Processing distribution:
	- 500.0ms: 73
	- 1000.0ms: 87
	- 5000.0ms: 89
	- 10000.0ms: 89
	- 30000.0ms: 89
	- +Infms: 89

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
Requests      [total, rate, throughput]         30000, 1000.04, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.34ms
Latencies     [min, mean, 50, 90, 95, 99, max]  799.971µs, 1.061ms, 1.034ms, 1.186ms, 1.254ms, 1.457ms, 22.102ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.275ms
Latencies     [min, mean, 50, 90, 95, 99, max]  879.885µs, 1.167ms, 1.139ms, 1.313ms, 1.381ms, 1.604ms, 8.99ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
