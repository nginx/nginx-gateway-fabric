# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: eb3a090367b0c4a450224993fc4eed39e6dd9dc4
- Date: 2026-01-22T21:37:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2072000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 250
- Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 245
	- 1000.0ms: 250
	- 5000.0ms: 250
	- 10000.0ms: 250
	- 30000.0ms: 250
	- +Infms: 250

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

- Total: 314
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 309
	- 1000.0ms: 314
	- 5000.0ms: 314
	- 10000.0ms: 314
	- 30000.0ms: 314
	- +Infms: 314

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

- Total: 1360
- Average Time: 157ms
- Event Batch Processing distribution:
	- 500.0ms: 1355
	- 1000.0ms: 1360
	- 5000.0ms: 1360
	- 10000.0ms: 1360
	- 30000.0ms: 1360
	- +Infms: 1360

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

- Total: 145
- Average Time: 155ms
- Event Batch Processing distribution:
	- 500.0ms: 144
	- 1000.0ms: 145
	- 5000.0ms: 145
	- 10000.0ms: 145
	- 30000.0ms: 145
	- +Infms: 145

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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 665.716µs
Latencies     [min, mean, 50, 90, 95, 99, max]  571.778µs, 743.901µs, 715.677µs, 807.185µs, 844.855µs, 1.009ms, 32.795ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 797.277µs
Latencies     [min, mean, 50, 90, 95, 99, max]  686.032µs, 889.101µs, 865.087µs, 962.117µs, 1.003ms, 1.19ms, 24.292ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
