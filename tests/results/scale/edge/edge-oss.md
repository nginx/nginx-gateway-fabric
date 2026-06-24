# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1284
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1270
	- 1000.0ms: 1284
	- 5000.0ms: 1284
	- 10000.0ms: 1284
	- 30000.0ms: 1284
	- +Infms: 1284

### Errors

- NGF errors: 15
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1349
- Average Time: 8ms
- Event Batch Processing distribution:
	- 500.0ms: 1343
	- 1000.0ms: 1349
	- 5000.0ms: 1349
	- 10000.0ms: 1349
	- 30000.0ms: 1349
	- +Infms: 1349

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

- Total: 2078
- Average Time: 85ms
- Event Batch Processing distribution:
	- 500.0ms: 1993
	- 1000.0ms: 2078
	- 5000.0ms: 2078
	- 10000.0ms: 2078
	- 30000.0ms: 2078
	- +Infms: 2078

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

- Total: 153
- Average Time: 128ms
- Event Batch Processing distribution:
	- 500.0ms: 142
	- 1000.0ms: 153
	- 5000.0ms: 153
	- 10000.0ms: 153
	- 30000.0ms: 153
	- +Infms: 153

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
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 703.771µs
Latencies     [min, mean, 50, 90, 95, 99, max]  583.976µs, 743.346µs, 724.626µs, 818.04µs, 864.947µs, 1.067ms, 12.057ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 772.082µs
Latencies     [min, mean, 50, 90, 95, 99, max]  650.414µs, 859.156µs, 833.124µs, 938.325µs, 991.092µs, 1.221ms, 15.713ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
