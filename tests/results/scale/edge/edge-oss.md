# Results

## Test environment

NGINX Plus: false

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
- Average Time: 127ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 386
- Average Time: 114ms
- Event Batch Processing distribution:
	- 500.0ms: 359
	- 1000.0ms: 385
	- 5000.0ms: 386
	- 10000.0ms: 386
	- 30000.0ms: 386
	- +Infms: 386

### Errors

- NGF errors: 0
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
- Average Time: 144ms
- Reload distribution:
	- 500.0ms: 128
	- 1000.0ms: 128
	- 5000.0ms: 128
	- 10000.0ms: 128
	- 30000.0ms: 128
	- +Infms: 128

### Event Batch Processing

- Total: 450
- Average Time: 111ms
- Event Batch Processing distribution:
	- 500.0ms: 416
	- 1000.0ms: 449
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
- Average Time: 170ms
- Reload distribution:
	- 500.0ms: 1001
	- 1000.0ms: 1001
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 216ms
- Event Batch Processing distribution:
	- 500.0ms: 1005
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

- Total: 90
- Total Errors: 0
- Average Time: 125ms
- Reload distribution:
	- 500.0ms: 90
	- 1000.0ms: 90
	- 5000.0ms: 90
	- 10000.0ms: 90
	- 30000.0ms: 90
	- +Infms: 90

### Event Batch Processing

- Total: 93
- Average Time: 123ms
- Event Batch Processing distribution:
	- 500.0ms: 93
	- 1000.0ms: 93
	- 5000.0ms: 93
	- 10000.0ms: 93
	- 30000.0ms: 93
	- +Infms: 93

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
Requests      [total, rate, throughput]         30000, 1000.01, 999.99
Duration      [total, attack, wait]             30s, 30s, 617.109µs
Latencies     [min, mean, 50, 90, 95, 99, max]  501.392µs, 672.18µs, 658.398µs, 745.091µs, 774.574µs, 849.941µs, 15.109ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.02, 999.99
Duration      [total, attack, wait]             30s, 29.999s, 746.145µs
Latencies     [min, mean, 50, 90, 95, 99, max]  587.739µs, 766.078µs, 749.849µs, 859.178µs, 907.539µs, 1.012ms, 9.958ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
