# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fef4f734728239e3248cda180e74f72e436b06b9
- Date: 2026-08-14T15:07:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1258000
- vCPUs per node: 16
- RAM per node: 65848288Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1284
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 1272
	- 1000.0ms: 1284
	- 5000.0ms: 1284
	- 10000.0ms: 1284
	- 30000.0ms: 1284
	- +Infms: 1284

### Errors

- NGF errors: 19
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1320
- Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 1311
	- 1000.0ms: 1320
	- 5000.0ms: 1320
	- 10000.0ms: 1320
	- 30000.0ms: 1320
	- +Infms: 1320

### Errors

- NGF errors: 17
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2076
- Average Time: 83ms
- Event Batch Processing distribution:
	- 500.0ms: 1993
	- 1000.0ms: 2076
	- 5000.0ms: 2076
	- 10000.0ms: 2076
	- 30000.0ms: 2076
	- +Infms: 2076

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

- Total: 278
- Average Time: 82ms
- Event Batch Processing distribution:
	- 500.0ms: 274
	- 1000.0ms: 278
	- 5000.0ms: 278
	- 10000.0ms: 278
	- 30000.0ms: 278
	- +Infms: 278

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
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.094ms
Latencies     [min, mean, 50, 90, 95, 99, max]  750.196µs, 1.125ms, 1.048ms, 1.299ms, 1.405ms, 2.405ms, 16.766ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.106ms
Latencies     [min, mean, 50, 90, 95, 99, max]  836.386µs, 1.248ms, 1.112ms, 1.416ms, 1.585ms, 4.221ms, 33.334ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
