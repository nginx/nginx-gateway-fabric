# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: fef4f734728239e3248cda180e74f72e436b06b9
- Date: 2026-08-14T15:07:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1258000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1274
- Average Time: 31ms
- Event Batch Processing distribution:
	- 500.0ms: 1221
	- 1000.0ms: 1274
	- 5000.0ms: 1274
	- 10000.0ms: 1274
	- 30000.0ms: 1274
	- +Infms: 1274

### Errors

- NGF errors: 9
- NGF container restarts: 0
- NGINX errors: 5
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1362
- Average Time: 34ms
- Event Batch Processing distribution:
	- 500.0ms: 1301
	- 1000.0ms: 1361
	- 5000.0ms: 1362
	- 10000.0ms: 1362
	- 30000.0ms: 1362
	- +Infms: 1362

### Errors

- NGF errors: 7
- NGF container restarts: 0
- NGINX errors: 34
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2071
- Average Time: 100ms
- Event Batch Processing distribution:
	- 500.0ms: 2012
	- 1000.0ms: 2071
	- 5000.0ms: 2071
	- 10000.0ms: 2071
	- 30000.0ms: 2071
	- +Infms: 2071

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

- Total: 60
- Average Time: 405ms
- Event Batch Processing distribution:
	- 500.0ms: 37
	- 1000.0ms: 57
	- 5000.0ms: 60
	- 10000.0ms: 60
	- 30000.0ms: 60
	- +Infms: 60

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 88
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.03, 992.40
Duration      [total, attack, wait]             30s, 29.999s, 1.088ms
Latencies     [min, mean, 50, 90, 95, 99, max]  356.807µs, 1.044ms, 1.019ms, 1.156ms, 1.213ms, 1.379ms, 34.705ms
Bytes In      [total, mean]                     4823064, 160.77
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.24%
Status Codes  [code:count]                      0:228  200:29772  
Error Set:
Get "http://cafe.example.com/latte": dial tcp 0.0.0.0:0->10.138.0.6:80: connect: connection refused
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.12ms
Latencies     [min, mean, 50, 90, 95, 99, max]  861.823µs, 1.132ms, 1.107ms, 1.257ms, 1.319ms, 1.484ms, 23.39ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
