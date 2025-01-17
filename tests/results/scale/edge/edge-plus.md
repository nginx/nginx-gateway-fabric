# Results

## Test environment

NGINX Plus: true

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
- Average Time: 231ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 387
- Average Time: 160ms
- Event Batch Processing distribution:
	- 500.0ms: 338
	- 1000.0ms: 384
	- 5000.0ms: 387
	- 10000.0ms: 387
	- 30000.0ms: 387
	- +Infms: 387

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 17
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Reloads

- Total: 128
- Total Errors: 0
- Average Time: 258ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 451
- Average Time: 151ms
- Event Batch Processing distribution:
	- 500.0ms: 383
	- 1000.0ms: 446
	- 5000.0ms: 451
	- 10000.0ms: 451
	- 30000.0ms: 451
	- +Infms: 451

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 8
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Reloads

- Total: 1001
- Total Errors: 0
- Average Time: 1442ms
- Reload distribution:
	- 500.0ms: 143
	- 1000.0ms: 340
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 1556ms
- Event Batch Processing distribution:
	- 500.0ms: 126
	- 1000.0ms: 308
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
- Average Time: 151ms
- Reload distribution:
	- 500.0ms: 3
	- 1000.0ms: 3
	- 5000.0ms: 3
	- 10000.0ms: 3
	- 30000.0ms: 3
	- +Infms: 3

### Event Batch Processing

- Total: 115
- Average Time: 229ms
- Event Batch Processing distribution:
	- 500.0ms: 107
	- 1000.0ms: 115
	- 5000.0ms: 115
	- 10000.0ms: 115
	- 30000.0ms: 115
	- +Infms: 115

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
Requests      [total, rate, throughput]         30000, 1000.00, 999.98
Duration      [total, attack, wait]             30.001s, 30s, 647.928µs
Latencies     [min, mean, 50, 90, 95, 99, max]  547.261µs, 685.262µs, 669.565µs, 745.861µs, 776.204µs, 897.386µs, 12.01ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.02
Duration      [total, attack, wait]             30s, 29.999s, 785.981µs
Latencies     [min, mean, 50, 90, 95, 99, max]  600.936µs, 765.586µs, 743.136µs, 847.332µs, 899.272µs, 1.018ms, 10.282ms
Bytes In      [total, mean]                     4860000, 162.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
