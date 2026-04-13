# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 17c42c8bbbb004ba9c0e9b867396c5f8937207cd
- Date: 2026-04-01T18:33:47Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 264
- Average Time: 49ms
- Event Batch Processing distribution:
	- 500.0ms: 254
	- 1000.0ms: 264
	- 5000.0ms: 264
	- 10000.0ms: 264
	- 30000.0ms: 264
	- +Infms: 264

### Errors

- NGF errors: 6
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 330
- Average Time: 40ms
- Event Batch Processing distribution:
	- 500.0ms: 319
	- 1000.0ms: 330
	- 5000.0ms: 330
	- 10000.0ms: 330
	- 30000.0ms: 330
	- +Infms: 330

### Errors

- NGF errors: 12
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1013
- Average Time: 154ms
- Event Batch Processing distribution:
	- 500.0ms: 954
	- 1000.0ms: 1013
	- 5000.0ms: 1013
	- 10000.0ms: 1013
	- 30000.0ms: 1013
	- +Infms: 1013

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

- Total: 91
- Average Time: 167ms
- Event Batch Processing distribution:
	- 500.0ms: 83
	- 1000.0ms: 91
	- 5000.0ms: 91
	- 10000.0ms: 91
	- 30000.0ms: 91
	- +Infms: 91

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 741.932µs
Latencies     [min, mean, 50, 90, 95, 99, max]  612.885µs, 799.729µs, 777.706µs, 887.424µs, 937.249µs, 1.095ms, 12.351ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 905.48µs
Latencies     [min, mean, 50, 90, 95, 99, max]  690.698µs, 909.184µs, 885.855µs, 1.009ms, 1.068ms, 1.251ms, 13.947ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
