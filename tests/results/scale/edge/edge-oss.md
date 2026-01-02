# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 89aee48bf6e660a828ffd32ca35fc7f52e358e00
- Date: 2025-12-12T20:04:38Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 301
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 300
	- 1000.0ms: 301
	- 5000.0ms: 301
	- 10000.0ms: 301
	- 30000.0ms: 301
	- +Infms: 301

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

- Total: 338
- Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 338
	- 1000.0ms: 338
	- 5000.0ms: 338
	- 10000.0ms: 338
	- 30000.0ms: 338
	- +Infms: 338

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1255
- Average Time: 136ms
- Event Batch Processing distribution:
	- 500.0ms: 1176
	- 1000.0ms: 1255
	- 5000.0ms: 1255
	- 10000.0ms: 1255
	- 30000.0ms: 1255
	- +Infms: 1255

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

- Total: 138
- Average Time: 140ms
- Event Batch Processing distribution:
	- 500.0ms: 122
	- 1000.0ms: 137
	- 5000.0ms: 138
	- 10000.0ms: 138
	- 30000.0ms: 138
	- +Infms: 138

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
Requests      [total, rate, throughput]         29999, 1000.00, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 1.04ms
Latencies     [min, mean, 50, 90, 95, 99, max]  733.443µs, 996µs, 970.279µs, 1.097ms, 1.145ms, 1.295ms, 28.664ms
Bytes In      [total, mean]                     4769841, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.97
Duration      [total, attack, wait]             30.001s, 29.999s, 1.383ms
Latencies     [min, mean, 50, 90, 95, 99, max]  833.3µs, 1.062ms, 1.042ms, 1.16ms, 1.213ms, 1.372ms, 18.505ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
