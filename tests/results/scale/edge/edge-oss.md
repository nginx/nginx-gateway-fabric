# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Reloads

- Total: 128
- Total Errors: 0
- Average Time: 222ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 387
- Average Time: 144ms
- Event Batch Processing distribution:
	- 500.0ms: 344
	- 1000.0ms: 385
	- 5000.0ms: 387
	- 10000.0ms: 387
	- 30000.0ms: 387
	- +Infms: 387

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Reloads

- Total: 127
- Total Errors: 0
- Average Time: 242ms
- Reload distribution:
	- 500.0ms: 127
	- 1000.0ms: 127
	- 5000.0ms: 127
	- 10000.0ms: 127
	- 30000.0ms: 127
	- +Infms: 127

### Event Batch Processing

- Total: 449
- Average Time: 134ms
- Event Batch Processing distribution:
	- 500.0ms: 399
	- 1000.0ms: 446
	- 5000.0ms: 449
	- 10000.0ms: 449
	- 30000.0ms: 449
	- +Infms: 449

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Reloads

- Total: 1001
- Total Errors: 0
- Average Time: 1492ms
- Reload distribution:
	- 500.0ms: 138
	- 1000.0ms: 326
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 1572ms
- Event Batch Processing distribution:
	- 500.0ms: 133
	- 1000.0ms: 307
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

- Total: 179
- Total Errors: 0
- Average Time: 150ms
- Reload distribution:
	- 500.0ms: 179
	- 1000.0ms: 179
	- 5000.0ms: 179
	- 10000.0ms: 179
	- 30000.0ms: 179
	- +Infms: 179

### Event Batch Processing

- Total: 182
- Average Time: 150ms
- Event Batch Processing distribution:
	- 500.0ms: 182
	- 1000.0ms: 182
	- 5000.0ms: 182
	- 10000.0ms: 182
	- 30000.0ms: 182
	- +Infms: 182

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
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 745.559µs
Latencies     [min, mean, 50, 90, 95, 99, max]  520.036µs, 716.84µs, 696.434µs, 795.416µs, 835.246µs, 954.923µs, 22.907ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.01, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 896.481µs
Latencies     [min, mean, 50, 90, 95, 99, max]  595.949µs, 771.615µs, 754.14µs, 858.793µs, 899.916µs, 1.024ms, 11.034ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
