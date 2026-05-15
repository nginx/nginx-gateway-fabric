# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1291
- Average Time: 30ms
- Event Batch Processing distribution:
	- 500.0ms: 1244
	- 1000.0ms: 1291
	- 5000.0ms: 1291
	- 10000.0ms: 1291
	- 30000.0ms: 1291
	- +Infms: 1291

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

- Total: 1352
- Average Time: 34ms
- Event Batch Processing distribution:
	- 500.0ms: 1297
	- 1000.0ms: 1352
	- 5000.0ms: 1352
	- 10000.0ms: 1352
	- 30000.0ms: 1352
	- +Infms: 1352

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 30
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2090
- Average Time: 106ms
- Event Batch Processing distribution:
	- 500.0ms: 2044
	- 1000.0ms: 2090
	- 5000.0ms: 2090
	- 10000.0ms: 2090
	- 30000.0ms: 2090
	- +Infms: 2090

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

- Total: 54
- Average Time: 322ms
- Event Batch Processing distribution:
	- 500.0ms: 42
	- 1000.0ms: 52
	- 5000.0ms: 54
	- 10000.0ms: 54
	- 30000.0ms: 54
	- +Infms: 54

### Errors

- NGF errors: 5
- NGF container restarts: 0
- NGINX errors: 152
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 704.342µs
Latencies     [min, mean, 50, 90, 95, 99, max]  597.997µs, 791.896µs, 767.268µs, 873.669µs, 920.555µs, 1.073ms, 21.397ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 864.07µs
Latencies     [min, mean, 50, 90, 95, 99, max]  732.549µs, 938.821µs, 909.933µs, 1.035ms, 1.105ms, 1.282ms, 22.585ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
