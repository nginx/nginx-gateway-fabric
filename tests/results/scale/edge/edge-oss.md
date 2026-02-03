# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 731b1dca2e90c6f393ff52d9eeadf8a18b276540
- Date: 2026-02-03T16:28:10Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2118001
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 252
- Average Time: 12ms
- Event Batch Processing distribution:
	- 500.0ms: 251
	- 1000.0ms: 252
	- 5000.0ms: 252
	- 10000.0ms: 252
	- 30000.0ms: 252
	- +Infms: 252

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

- Total: 287
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 287
	- 1000.0ms: 287
	- 5000.0ms: 287
	- 10000.0ms: 287
	- 30000.0ms: 287
	- +Infms: 287

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

- Total: 1009
- Average Time: 149ms
- Event Batch Processing distribution:
	- 500.0ms: 976
	- 1000.0ms: 1009
	- 5000.0ms: 1009
	- 10000.0ms: 1009
	- 30000.0ms: 1009
	- +Infms: 1009

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

- Total: 157
- Average Time: 86ms
- Event Batch Processing distribution:
	- 500.0ms: 153
	- 1000.0ms: 157
	- 5000.0ms: 157
	- 10000.0ms: 157
	- 30000.0ms: 157
	- +Infms: 157

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
Duration      [total, attack, wait]             30s, 29.999s, 793.814µs
Latencies     [min, mean, 50, 90, 95, 99, max]  587.09µs, 885.647µs, 861.876µs, 996.866µs, 1.051ms, 1.248ms, 14.132ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 902.654µs
Latencies     [min, mean, 50, 90, 95, 99, max]  751.729µs, 1.009ms, 977.849µs, 1.126ms, 1.19ms, 1.375ms, 18.753ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
