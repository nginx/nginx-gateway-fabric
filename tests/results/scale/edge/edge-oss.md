# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: ea67c576e39284e758352dda72242024cc98fa7f
- Date: 2026-10-01T15:28:08Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.8-gke.1225000
- vCPUs per node: 16
- RAM per node: 65848288Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1284
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1271
	- 1000.0ms: 1284
	- 5000.0ms: 1284
	- 10000.0ms: 1284
	- 30000.0ms: 1284
	- +Infms: 1284

### Errors

- NGF errors: 23
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1354
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1347
	- 1000.0ms: 1354
	- 5000.0ms: 1354
	- 10000.0ms: 1354
	- 30000.0ms: 1354
	- +Infms: 1354

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

- Total: 2074
- Average Time: 77ms
- Event Batch Processing distribution:
	- 500.0ms: 2008
	- 1000.0ms: 2074
	- 5000.0ms: 2074
	- 10000.0ms: 2074
	- 30000.0ms: 2074
	- +Infms: 2074

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

- Total: 191
- Average Time: 82ms
- Event Batch Processing distribution:
	- 500.0ms: 184
	- 1000.0ms: 191
	- 5000.0ms: 191
	- 10000.0ms: 191
	- 30000.0ms: 191
	- +Infms: 191

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
Duration      [total, attack, wait]             30s, 29.999s, 812.258µs
Latencies     [min, mean, 50, 90, 95, 99, max]  610.491µs, 826.192µs, 804.964µs, 923.576µs, 969.828µs, 1.126ms, 13.879ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 847.853µs
Latencies     [min, mean, 50, 90, 95, 99, max]  711.344µs, 959.971µs, 935.591µs, 1.079ms, 1.14ms, 1.306ms, 15.853ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
