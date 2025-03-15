# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 9155a2b6a8d3179165797ef3e789e97283f7a695
- Date: 2025-03-15T07:17:11Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.6-gke.1020000
- vCPUs per node: 16
- RAM per node: 65851340Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Reloads

- Total: 127
- Total Errors: 0
- Average Time: 127ms
- Reload distribution:
	- 500.0ms: 127
	- 1000.0ms: 127
	- 5000.0ms: 127
	- 10000.0ms: 127
	- 30000.0ms: 127
	- +Infms: 127

### Event Batch Processing

- Total: 385
- Average Time: 136ms
- Event Batch Processing distribution:
	- 500.0ms: 347
	- 1000.0ms: 382
	- 5000.0ms: 385
	- 10000.0ms: 385
	- 30000.0ms: 385
	- +Infms: 385

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

- Total: 128
- Total Errors: 0
- Average Time: 146ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 450
- Average Time: 166ms
- Event Batch Processing distribution:
	- 500.0ms: 392
	- 1000.0ms: 432
	- 5000.0ms: 450
	- 10000.0ms: 450
	- 30000.0ms: 450
	- +Infms: 450

### Errors

- NGF errors: 0
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
- Average Time: 174ms
- Reload distribution:
	- 500.0ms: 1001
	- 1000.0ms: 1001
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 229ms
- Event Batch Processing distribution:
	- 500.0ms: 1002
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

- Total: 97
- Total Errors: 0
- Average Time: 126ms
- Reload distribution:
	- 500.0ms: 97
	- 1000.0ms: 97
	- 5000.0ms: 97
	- 10000.0ms: 97
	- 30000.0ms: 97
	- +Infms: 97

### Event Batch Processing

- Total: 99
- Average Time: 125ms
- Event Batch Processing distribution:
	- 500.0ms: 99
	- 1000.0ms: 99
	- 5000.0ms: 99
	- 10000.0ms: 99
	- 30000.0ms: 99
	- +Infms: 99

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
Duration      [total, attack, wait]             30s, 29.999s, 663.238µs
Latencies     [min, mean, 50, 90, 95, 99, max]  499.976µs, 677.946µs, 660.823µs, 759.984µs, 799.116µs, 904.939µs, 12.162ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 750.337µs
Latencies     [min, mean, 50, 90, 95, 99, max]  590.522µs, 762.674µs, 740.085µs, 869.449µs, 930.564µs, 1.057ms, 8.287ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
