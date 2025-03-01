# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: bfd685d3805042ac77865a9823104404a80b06b9
- Date: 2025-02-28T18:00:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1169000
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

- Total: 387
- Average Time: 136ms
- Event Batch Processing distribution:
	- 500.0ms: 349
	- 1000.0ms: 386
	- 5000.0ms: 387
	- 10000.0ms: 387
	- 30000.0ms: 387
	- +Infms: 387

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 6
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Reloads

- Total: 127
- Total Errors: 0
- Average Time: 160ms
- Reload distribution:
	- 500.0ms: 127
	- 1000.0ms: 127
	- 5000.0ms: 127
	- 10000.0ms: 127
	- 30000.0ms: 127
	- +Infms: 127

### Event Batch Processing

- Total: 448
- Average Time: 132ms
- Event Batch Processing distribution:
	- 500.0ms: 400
	- 1000.0ms: 444
	- 5000.0ms: 448
	- 10000.0ms: 448
	- 30000.0ms: 448
	- +Infms: 448

### Errors

- NGF errors: 1
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
- Average Time: 188ms
- Reload distribution:
	- 500.0ms: 1001
	- 1000.0ms: 1001
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 262ms
- Event Batch Processing distribution:
	- 500.0ms: 1001
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
- Average Time: 151ms
- Reload distribution:
	- 500.0ms: 3
	- 1000.0ms: 3
	- 5000.0ms: 3
	- 10000.0ms: 3
	- 30000.0ms: 3
	- +Infms: 3

### Event Batch Processing

- Total: 46
- Average Time: 429ms
- Event Batch Processing distribution:
	- 500.0ms: 29
	- 1000.0ms: 44
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 817.262µs
Latencies     [min, mean, 50, 90, 95, 99, max]  551.84µs, 826.876µs, 779.526µs, 920.687µs, 974.492µs, 1.191ms, 22.674ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 804.129µs
Latencies     [min, mean, 50, 90, 95, 99, max]  626.783µs, 900.323µs, 878.869µs, 1.028ms, 1.085ms, 1.234ms, 15.666ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
