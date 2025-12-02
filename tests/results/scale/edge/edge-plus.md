# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: dbebd9791cb7aa5e8d10735800f776fd516b06c3
- Date: 2025-12-02T17:38:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 250
- Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 244
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

- Total: 342
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 337
	- 1000.0ms: 341
	- 5000.0ms: 342
	- 10000.0ms: 342
	- 30000.0ms: 342
	- +Infms: 342

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

- Total: 1328
- Average Time: 132ms
- Event Batch Processing distribution:
	- 500.0ms: 1313
	- 1000.0ms: 1328
	- 5000.0ms: 1328
	- 10000.0ms: 1328
	- 30000.0ms: 1328
	- +Infms: 1328

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

- Total: 89
- Average Time: 196ms
- Event Batch Processing distribution:
	- 500.0ms: 78
	- 1000.0ms: 89
	- 5000.0ms: 89
	- 10000.0ms: 89
	- 30000.0ms: 89
	- +Infms: 89

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
Duration      [total, attack, wait]             30s, 29.999s, 971.575µs
Latencies     [min, mean, 50, 90, 95, 99, max]  766.279µs, 1.032ms, 999.517µs, 1.155ms, 1.223ms, 1.442ms, 20.963ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.98
Duration      [total, attack, wait]             30s, 29.999s, 1.007ms
Latencies     [min, mean, 50, 90, 95, 99, max]  870.05µs, 1.119ms, 1.089ms, 1.255ms, 1.328ms, 1.533ms, 17.698ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
