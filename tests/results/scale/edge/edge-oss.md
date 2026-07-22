# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1289
- Average Time: 12ms
- Event Batch Processing distribution:
	- 500.0ms: 1277
	- 1000.0ms: 1289
	- 5000.0ms: 1289
	- 10000.0ms: 1289
	- 30000.0ms: 1289
	- +Infms: 1289

### Errors

- NGF errors: 14
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1355
- Average Time: 8ms
- Event Batch Processing distribution:
	- 500.0ms: 1351
	- 1000.0ms: 1355
	- 5000.0ms: 1355
	- 10000.0ms: 1355
	- 30000.0ms: 1355
	- +Infms: 1355

### Errors

- NGF errors: 18
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2084
- Average Time: 81ms
- Event Batch Processing distribution:
	- 500.0ms: 2011
	- 1000.0ms: 2084
	- 5000.0ms: 2084
	- 10000.0ms: 2084
	- 30000.0ms: 2084
	- +Infms: 2084

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

- Total: 79
- Average Time: 184ms
- Event Batch Processing distribution:
	- 500.0ms: 69
	- 1000.0ms: 78
	- 5000.0ms: 79
	- 10000.0ms: 79
	- 30000.0ms: 79
	- +Infms: 79

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
Duration      [total, attack, wait]             30s, 29.999s, 915.951µs
Latencies     [min, mean, 50, 90, 95, 99, max]  611.275µs, 834.551µs, 794.663µs, 925.702µs, 985.587µs, 1.254ms, 206.078ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 935.004µs
Latencies     [min, mean, 50, 90, 95, 99, max]  728.221µs, 982.39µs, 943.132µs, 1.085ms, 1.151ms, 1.381ms, 209.668ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
