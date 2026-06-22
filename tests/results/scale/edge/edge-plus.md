# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1289
- Average Time: 30ms
- Event Batch Processing distribution:
	- 500.0ms: 1237
	- 1000.0ms: 1289
	- 5000.0ms: 1289
	- 10000.0ms: 1289
	- 30000.0ms: 1289
	- +Infms: 1289

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

- Total: 1354
- Average Time: 34ms
- Event Batch Processing distribution:
	- 500.0ms: 1298
	- 1000.0ms: 1354
	- 5000.0ms: 1354
	- 10000.0ms: 1354
	- 30000.0ms: 1354
	- +Infms: 1354

### Errors

- NGF errors: 4
- NGF container restarts: 0
- NGINX errors: 4
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2090
- Average Time: 99ms
- Event Batch Processing distribution:
	- 500.0ms: 2050
	- 1000.0ms: 2090
	- 5000.0ms: 2090
	- 10000.0ms: 2090
	- 30000.0ms: 2090
	- +Infms: 2090

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

- Total: 62
- Average Time: 351ms
- Event Batch Processing distribution:
	- 500.0ms: 45
	- 1000.0ms: 59
	- 5000.0ms: 62
	- 10000.0ms: 62
	- 30000.0ms: 62
	- +Infms: 62

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 32
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 759.297µs
Latencies     [min, mean, 50, 90, 95, 99, max]  606.455µs, 813.667µs, 788.569µs, 910.196µs, 957.717µs, 1.124ms, 22.801ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.019ms
Latencies     [min, mean, 50, 90, 95, 99, max]  763.815µs, 1.016ms, 982.644µs, 1.125ms, 1.19ms, 1.371ms, 20.793ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
