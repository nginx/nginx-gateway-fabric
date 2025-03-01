# Results

## Test environment

NGINX Plus: false

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
- Average Time: 122ms
- Event Batch Processing distribution:
	- 500.0ms: 357
	- 1000.0ms: 384
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

- Total: 448
- Average Time: 111ms
- Event Batch Processing distribution:
	- 500.0ms: 410
	- 1000.0ms: 448
	- 5000.0ms: 448
	- 10000.0ms: 448
	- 30000.0ms: 448
	- +Infms: 448

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
- Average Time: 171ms
- Reload distribution:
	- 500.0ms: 1001
	- 1000.0ms: 1001
	- 5000.0ms: 1001
	- 10000.0ms: 1001
	- 30000.0ms: 1001
	- +Infms: 1001

### Event Batch Processing

- Total: 1008
- Average Time: 217ms
- Event Batch Processing distribution:
	- 500.0ms: 1008
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

- Total: 85
- Total Errors: 0
- Average Time: 126ms
- Reload distribution:
	- 500.0ms: 85
	- 1000.0ms: 85
	- 5000.0ms: 85
	- 10000.0ms: 85
	- 30000.0ms: 85
	- +Infms: 85

### Event Batch Processing

- Total: 88
- Average Time: 123ms
- Event Batch Processing distribution:
	- 500.0ms: 88
	- 1000.0ms: 88
	- 5000.0ms: 88
	- 10000.0ms: 88
	- 30000.0ms: 88
	- +Infms: 88

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
Requests      [total, rate, throughput]         30000, 1000.03, 1000.02
Duration      [total, attack, wait]             30s, 29.999s, 431.266µs
Latencies     [min, mean, 50, 90, 95, 99, max]  352.955µs, 481.036µs, 462.743µs, 555.735µs, 606.472µs, 732.05µs, 7.685ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.05, 999.93
Duration      [total, attack, wait]             30.002s, 29.999s, 3.664ms
Latencies     [min, mean, 50, 90, 95, 99, max]  452.076µs, 2.37ms, 2.604ms, 3.506ms, 4.283ms, 6.706ms, 19.768ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
