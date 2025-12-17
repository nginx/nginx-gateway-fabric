# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: e8ee7c1c4f14e249927a5447a1af2615ddbe0f87
- Date: 2025-12-17T20:04:03Z
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

- Total: 299
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 298
	- 1000.0ms: 299
	- 5000.0ms: 299
	- 10000.0ms: 299
	- 30000.0ms: 299
	- +Infms: 299

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

- Total: 336
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 336
	- 1000.0ms: 336
	- 5000.0ms: 336
	- 10000.0ms: 336
	- 30000.0ms: 336
	- +Infms: 336

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1259
- Average Time: 126ms
- Event Batch Processing distribution:
	- 500.0ms: 1192
	- 1000.0ms: 1259
	- 5000.0ms: 1259
	- 10000.0ms: 1259
	- 30000.0ms: 1259
	- +Infms: 1259

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

- Total: 96
- Average Time: 113ms
- Event Batch Processing distribution:
	- 500.0ms: 86
	- 1000.0ms: 96
	- 5000.0ms: 96
	- 10000.0ms: 96
	- 30000.0ms: 96
	- +Infms: 96

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
Duration      [total, attack, wait]             30s, 29.999s, 965.322µs
Latencies     [min, mean, 50, 90, 95, 99, max]  751.912µs, 970.822µs, 940.82µs, 1.083ms, 1.15ms, 1.341ms, 26.998ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 1.086ms
Latencies     [min, mean, 50, 90, 95, 99, max]  816.972µs, 1.064ms, 1.044ms, 1.17ms, 1.231ms, 1.407ms, 21.352ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
