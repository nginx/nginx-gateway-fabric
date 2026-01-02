# Results

## Test environment

NGINX Plus: true

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

- Total: 249
- Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 243
	- 1000.0ms: 249
	- 5000.0ms: 249
	- 10000.0ms: 249
	- 30000.0ms: 249
	- +Infms: 249

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

- Total: 321
- Average Time: 14ms
- Event Batch Processing distribution:
	- 500.0ms: 315
	- 1000.0ms: 320
	- 5000.0ms: 321
	- 10000.0ms: 321
	- 30000.0ms: 321
	- +Infms: 321

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

- Total: 1310
- Average Time: 166ms
- Event Batch Processing distribution:
	- 500.0ms: 1235
	- 1000.0ms: 1310
	- 5000.0ms: 1310
	- 10000.0ms: 1310
	- 30000.0ms: 1310
	- +Infms: 1310

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

- Total: 83
- Average Time: 209ms
- Event Batch Processing distribution:
	- 500.0ms: 69
	- 1000.0ms: 81
	- 5000.0ms: 83
	- 10000.0ms: 83
	- 30000.0ms: 83
	- +Infms: 83

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
Duration      [total, attack, wait]             30s, 29.999s, 951.219µs
Latencies     [min, mean, 50, 90, 95, 99, max]  728.696µs, 964.757µs, 943.409µs, 1.057ms, 1.107ms, 1.273ms, 13.167ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 965.495µs
Latencies     [min, mean, 50, 90, 95, 99, max]  828.389µs, 1.069ms, 1.046ms, 1.169ms, 1.226ms, 1.407ms, 16.348ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
