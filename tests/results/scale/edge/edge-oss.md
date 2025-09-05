# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 299
- Average Time: 118ms
- Event Batch Processing distribution:
	- 500.0ms: 261
	- 1000.0ms: 299
	- 5000.0ms: 299
	- 10000.0ms: 299
	- 30000.0ms: 299
	- +Infms: 299

### Errors

- NGF errors: 29
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 371
- Average Time: 159ms
- Event Batch Processing distribution:
	- 500.0ms: 308
	- 1000.0ms: 371
	- 5000.0ms: 371
	- 10000.0ms: 371
	- 30000.0ms: 371
	- +Infms: 371

### Errors

- NGF errors: 21
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1011
- Average Time: 593ms
- Event Batch Processing distribution:
	- 500.0ms: 300
	- 1000.0ms: 1011
	- 5000.0ms: 1011
	- 10000.0ms: 1011
	- 30000.0ms: 1011
	- +Infms: 1011

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 1

### Graphs and Logs

See [output directory](./TestScale_HTTPRoutes) for more details.
The logs are attached only if there are errors.

## Test TestScale_UpstreamServers

### Event Batch Processing

- Total: 36
- Average Time: 420ms
- Event Batch Processing distribution:
	- 500.0ms: 21
	- 1000.0ms: 36
	- 5000.0ms: 36
	- 10000.0ms: 36
	- 30000.0ms: 36
	- +Infms: 36

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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 939.879µs
Latencies     [min, mean, 50, 90, 95, 99, max]  753.271µs, 1.041ms, 1.009ms, 1.143ms, 1.198ms, 1.379ms, 23.531ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 1.063ms
Latencies     [min, mean, 50, 90, 95, 99, max]  859.018µs, 1.095ms, 1.068ms, 1.209ms, 1.271ms, 1.478ms, 20.525ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
