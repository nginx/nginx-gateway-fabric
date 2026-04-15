# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 09f31a0defdd4bf13c648139f55567bf908cfaac
- Date: 2026-04-15T14:59:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848324Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 263
- Average Time: 156ms
- Event Batch Processing distribution:
	- 500.0ms: 208
	- 1000.0ms: 263
	- 5000.0ms: 263
	- 10000.0ms: 263
	- 30000.0ms: 263
	- +Infms: 263

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

- Total: 322
- Average Time: 151ms
- Event Batch Processing distribution:
	- 500.0ms: 258
	- 1000.0ms: 321
	- 5000.0ms: 322
	- 10000.0ms: 322
	- 30000.0ms: 322
	- +Infms: 322

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 44
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1012
- Average Time: 216ms
- Event Batch Processing distribution:
	- 500.0ms: 966
	- 1000.0ms: 1012
	- 5000.0ms: 1012
	- 10000.0ms: 1012
	- 30000.0ms: 1012
	- +Infms: 1012

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

- Total: 44
- Average Time: 338ms
- Event Batch Processing distribution:
	- 500.0ms: 35
	- 1000.0ms: 43
	- 5000.0ms: 44
	- 10000.0ms: 44
	- 30000.0ms: 44
	- +Infms: 44

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
Duration      [total, attack, wait]             30s, 29.999s, 771.796µs
Latencies     [min, mean, 50, 90, 95, 99, max]  632.63µs, 844.968µs, 821.382µs, 934.849µs, 983.014µs, 1.18ms, 18.064ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.97
Duration      [total, attack, wait]             30.001s, 30s, 1.023ms
Latencies     [min, mean, 50, 90, 95, 99, max]  753.945µs, 1.014ms, 984.203µs, 1.125ms, 1.19ms, 1.375ms, 24.232ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
