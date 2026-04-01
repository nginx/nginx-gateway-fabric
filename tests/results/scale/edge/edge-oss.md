# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 2f3153c547e0442fbb26aa9165118f4dc2b20f23
- Date: 2026-04-01T15:39:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848316Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 264
- Average Time: 53ms
- Event Batch Processing distribution:
	- 500.0ms: 253
	- 1000.0ms: 264
	- 5000.0ms: 264
	- 10000.0ms: 264
	- 30000.0ms: 264
	- +Infms: 264

### Errors

- NGF errors: 10
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 331
- Average Time: 35ms
- Event Batch Processing distribution:
	- 500.0ms: 325
	- 1000.0ms: 331
	- 5000.0ms: 331
	- 10000.0ms: 331
	- 30000.0ms: 331
	- +Infms: 331

### Errors

- NGF errors: 14
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1011
- Average Time: 182ms
- Event Batch Processing distribution:
	- 500.0ms: 915
	- 1000.0ms: 1011
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

- Total: 77
- Average Time: 216ms
- Event Batch Processing distribution:
	- 500.0ms: 59
	- 1000.0ms: 77
	- 5000.0ms: 77
	- 10000.0ms: 77
	- 30000.0ms: 77
	- +Infms: 77

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
Requests      [total, rate, throughput]         30000, 1000.04, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.406ms
Latencies     [min, mean, 50, 90, 95, 99, max]  656.195µs, 1.057ms, 1.014ms, 1.291ms, 1.373ms, 1.593ms, 26.605ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.121ms
Latencies     [min, mean, 50, 90, 95, 99, max]  779.038µs, 1.241ms, 1.201ms, 1.527ms, 1.627ms, 1.941ms, 17.944ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
