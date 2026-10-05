# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: ea67c576e39284e758352dda72242024cc98fa7f
- Date: 2026-10-01T15:28:08Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.8-gke.1225000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1291
- Average Time: 30ms
- Event Batch Processing distribution:
	- 500.0ms: 1239
	- 1000.0ms: 1291
	- 5000.0ms: 1291
	- 10000.0ms: 1291
	- 30000.0ms: 1291
	- +Infms: 1291

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 9
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1354
- Average Time: 33ms
- Event Batch Processing distribution:
	- 500.0ms: 1297
	- 1000.0ms: 1354
	- 5000.0ms: 1354
	- 10000.0ms: 1354
	- 30000.0ms: 1354
	- +Infms: 1354

### Errors

- NGF errors: 2
- NGF container restarts: 0
- NGINX errors: 61
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2094
- Average Time: 93ms
- Event Batch Processing distribution:
	- 500.0ms: 2042
	- 1000.0ms: 2093
	- 5000.0ms: 2094
	- 10000.0ms: 2094
	- 30000.0ms: 2094
	- +Infms: 2094

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

- Total: 127
- Average Time: 224ms
- Event Batch Processing distribution:
	- 500.0ms: 121
	- 1000.0ms: 127
	- 5000.0ms: 127
	- 10000.0ms: 127
	- 30000.0ms: 127
	- +Infms: 127

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
Requests      [total, rate, throughput]         30000, 1000.04, 998.14
Duration      [total, attack, wait]             30s, 29.999s, 794.709µs
Latencies     [min, mean, 50, 90, 95, 99, max]  421.299µs, 769.5µs, 746.773µs, 856.831µs, 909.698µs, 1.085ms, 10.001ms
Bytes In      [total, mean]                     4820984, 160.70
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           99.81%
Status Codes  [code:count]                      0:56  200:29944  
Error Set:
Get "http://cafe.example.com/latte": dial tcp 0.0.0.0:0->10.138.0.54:80: connect: connection refused
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 833.133µs
Latencies     [min, mean, 50, 90, 95, 99, max]  727.962µs, 953.628µs, 921.551µs, 1.057ms, 1.116ms, 1.324ms, 18.129ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
