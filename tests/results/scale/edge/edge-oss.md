# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 0042ca1a05b3e6ee63f08f0cf2ca8090a4f6a1e6
- Date: 2026-10-15T15:55:29Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.36.4-gke.1391002
- vCPUs per node: 16
- RAM per node: 65848244Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1292
- Average Time: 12ms
- Event Batch Processing distribution:
	- 500.0ms: 1276
	- 1000.0ms: 1292
	- 5000.0ms: 1292
	- 10000.0ms: 1292
	- 30000.0ms: 1292
	- +Infms: 1292

### Errors

- NGF errors: 11
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1358
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1351
	- 1000.0ms: 1358
	- 5000.0ms: 1358
	- 10000.0ms: 1358
	- 30000.0ms: 1358
	- +Infms: 1358

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

- Total: 2084
- Average Time: 77ms
- Event Batch Processing distribution:
	- 500.0ms: 2013
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

- Total: 99
- Average Time: 217ms
- Event Batch Processing distribution:
	- 500.0ms: 78
	- 1000.0ms: 99
	- 5000.0ms: 99
	- 10000.0ms: 99
	- 30000.0ms: 99
	- +Infms: 99

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 841.949µs
Latencies     [min, mean, 50, 90, 95, 99, max]  594.911µs, 758.361µs, 736.432µs, 820.519µs, 855.535µs, 998.088µs, 16.222ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 802.556µs
Latencies     [min, mean, 50, 90, 95, 99, max]  667.654µs, 885.359µs, 861.919µs, 963.222µs, 1.007ms, 1.178ms, 19.427ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
