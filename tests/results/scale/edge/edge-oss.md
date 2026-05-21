# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1282
- Average Time: 7ms
- Event Batch Processing distribution:
	- 500.0ms: 1274
	- 1000.0ms: 1282
	- 5000.0ms: 1282
	- 10000.0ms: 1282
	- 30000.0ms: 1282
	- +Infms: 1282

### Errors

- NGF errors: 8
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1352
- Average Time: 7ms
- Event Batch Processing distribution:
	- 500.0ms: 1349
	- 1000.0ms: 1352
	- 5000.0ms: 1352
	- 10000.0ms: 1352
	- 30000.0ms: 1352
	- +Infms: 1352

### Errors

- NGF errors: 9
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2084
- Average Time: 72ms
- Event Batch Processing distribution:
	- 500.0ms: 2055
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

- Total: 130
- Average Time: 129ms
- Event Batch Processing distribution:
	- 500.0ms: 118
	- 1000.0ms: 130
	- 5000.0ms: 130
	- 10000.0ms: 130
	- 30000.0ms: 130
	- +Infms: 130

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
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 923.069µs
Latencies     [min, mean, 50, 90, 95, 99, max]  668.248µs, 1.015ms, 967.935µs, 1.218ms, 1.303ms, 1.545ms, 35.592ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.039ms
Latencies     [min, mean, 50, 90, 95, 99, max]  772.512µs, 1.129ms, 1.073ms, 1.348ms, 1.444ms, 1.703ms, 27.878ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
