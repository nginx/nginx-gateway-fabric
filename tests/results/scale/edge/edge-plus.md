# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9a7a618dab5ed0eee09063de60d80bf0fb76900a
- Date: 2025-02-14T18:44:35Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1023000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Reloads

- Total: 128
- Total Errors: 0
- Average Time: 151ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 383
- Average Time: 179ms
- Event Batch Processing distribution:
	- 500.0ms: 333
	- 1000.0ms: 376
	- 5000.0ms: 383
	- 10000.0ms: 383
	- 30000.0ms: 383
	- +Infms: 383

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 10
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Reloads

- Total: 128
- Total Errors: 0
- Average Time: 160ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 449
- Average Time: 120ms
- Event Batch Processing distribution:
	- 500.0ms: 407
	- 1000.0ms: 449
	- 5000.0ms: 449
	- 10000.0ms: 449
	- 30000.0ms: 449
	- +Infms: 449

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 7
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Reloads

- Total: 1001
- Total Errors: 0
- Average Time: 195ms
- Reload distribution:
	- 500.0ms: 1001
	- 1000.0ms: 1001
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 266ms
- Event Batch Processing distribution:
	- 500.0ms: 1004
	- 1000.0ms: 1008
	- 5000.0ms: 1008
	- 10000.0ms: 1008
	- 30000.0ms: 1008
	- +Infms: 1008

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPRoutes) for more details.
The logs are attached only if there are errors.

## Test TestScale_UpstreamServers

### Reloads

- Total: 3
- Total Errors: 0
- Average Time: 142ms
- Reload distribution:
	- 500.0ms: 3
	- 1000.0ms: 3
	- 5000.0ms: 3
	- 10000.0ms: 3
	- 30000.0ms: 3
	- +Infms: 3

### Event Batch Processing

- Total: 62
- Average Time: 366ms
- Event Batch Processing distribution:
	- 500.0ms: 53
	- 1000.0ms: 61
	- 5000.0ms: 62
	- 10000.0ms: 62
	- 30000.0ms: 62
	- +Infms: 62

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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 765.827µs
Latencies     [min, mean, 50, 90, 95, 99, max]  521.423µs, 723.345µs, 696.083µs, 820.511µs, 874.817µs, 1.064ms, 12.21ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.00, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 787.326µs
Latencies     [min, mean, 50, 90, 95, 99, max]  589.54µs, 830.84µs, 803.995µs, 966.869µs, 1.036ms, 1.215ms, 11.986ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
