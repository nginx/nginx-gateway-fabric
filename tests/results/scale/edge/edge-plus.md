# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: d97dd7debc1ea5d51f4413b6564b27921a1fc982
- Date: 2026-02-27T17:29:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.34.3-gke.1318000
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 264
- Average Time: 155ms
- Event Batch Processing distribution:
	- 500.0ms: 208
	- 1000.0ms: 264
	- 5000.0ms: 264
	- 10000.0ms: 264
	- 30000.0ms: 264
	- +Infms: 264

### Errors

- NGF errors: 4
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 328
- Average Time: 146ms
- Event Batch Processing distribution:
	- 500.0ms: 270
	- 1000.0ms: 328
	- 5000.0ms: 328
	- 10000.0ms: 328
	- 30000.0ms: 328
	- +Infms: 328

### Errors

- NGF errors: 9
- NGF container restarts: 0
- NGINX errors: 39
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1011
- Average Time: 216ms
- Event Batch Processing distribution:
	- 500.0ms: 970
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

- Total: 50
- Average Time: 419ms
- Event Batch Processing distribution:
	- 500.0ms: 28
	- 1000.0ms: 49
	- 5000.0ms: 50
	- 10000.0ms: 50
	- 30000.0ms: 50
	- +Infms: 50

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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 713.594µs
Latencies     [min, mean, 50, 90, 95, 99, max]  638.576µs, 853.129µs, 828.117µs, 950.246µs, 1.006ms, 1.187ms, 19.151ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.194ms
Latencies     [min, mean, 50, 90, 95, 99, max]  786.299µs, 1.044ms, 1.023ms, 1.169ms, 1.245ms, 1.434ms, 22.426ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
