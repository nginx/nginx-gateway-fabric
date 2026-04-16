# Results

## Test environment

NGINX Plus: true

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

- Total: 265
- Average Time: 165ms
- Event Batch Processing distribution:
	- 500.0ms: 203
	- 1000.0ms: 265
	- 5000.0ms: 265
	- 10000.0ms: 265
	- 30000.0ms: 265
	- +Infms: 265

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

- Total: 326
- Average Time: 147ms
- Event Batch Processing distribution:
	- 500.0ms: 262
	- 1000.0ms: 326
	- 5000.0ms: 326
	- 10000.0ms: 326
	- 30000.0ms: 326
	- +Infms: 326

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 31
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1011
- Average Time: 200ms
- Event Batch Processing distribution:
	- 500.0ms: 975
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

- Total: 61
- Average Time: 377ms
- Event Batch Processing distribution:
	- 500.0ms: 46
	- 1000.0ms: 60
	- 5000.0ms: 61
	- 10000.0ms: 61
	- 30000.0ms: 61
	- +Infms: 61

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.127ms
Latencies     [min, mean, 50, 90, 95, 99, max]  601.669µs, 768.792µs, 743.745µs, 824.01µs, 862.463µs, 1.035ms, 24.836ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         29999, 1000.00, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 1.023ms
Latencies     [min, mean, 50, 90, 95, 99, max]  720.568µs, 896.253µs, 870.016µs, 963.312µs, 1.007ms, 1.202ms, 26.191ms
Bytes In      [total, mean]                     4829839, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```
