# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 8241478604f782eca497329ae47507b978d117b1
- Date: 2025-09-25T01:19:35Z
- Dirty: false

GKE Cluster:

- Node count: 15
- k8s version: v1.33.4-gke.1134000
- vCPUs per node: 2
- RAM per node: 4015672Ki
- Max pods per node: 110
- Zone: us-south1-a
- Instance Type: e2-medium

## Test TestScale_Listeners

### Event Batch Processing

- Total: 325
- Average Time: 199ms
- Event Batch Processing distribution:
	- 500.0ms: 265
	- 1000.0ms: 297
	- 5000.0ms: 325
	- 10000.0ms: 325
	- 30000.0ms: 325
	- +Infms: 325

### Errors

- NGF errors: 5
- NGF container restarts: 0
- NGINX errors: 401
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 389
- Average Time: 141ms
- Event Batch Processing distribution:
	- 500.0ms: 335
	- 1000.0ms: 389
	- 5000.0ms: 389
	- 10000.0ms: 389
	- 30000.0ms: 389
	- +Infms: 389

### Errors

- NGF errors: 4
- NGF container restarts: 0
- NGINX errors: 174
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1011
- Average Time: 523ms
- Event Batch Processing distribution:
	- 500.0ms: 466
	- 1000.0ms: 948
	- 5000.0ms: 1011
	- 10000.0ms: 1011
	- 30000.0ms: 1011
	- +Infms: 1011

### Errors

- NGF errors: 0
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 1

### Graphs and Logs

See [output directory](./TestScale_HTTPRoutes) for more details.
The logs are attached only if there are errors.

## Test TestScale_UpstreamServers

### Event Batch Processing

- Total: 59
- Average Time: 463ms
- Event Batch Processing distribution:
	- 500.0ms: 33
	- 1000.0ms: 56
	- 5000.0ms: 59
	- 10000.0ms: 59
	- 30000.0ms: 59
	- +Infms: 59

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.03, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.078ms
Latencies     [min, mean, 50, 90, 95, 99, max]  790.398µs, 1.136ms, 1.038ms, 1.248ms, 1.446ms, 3.534ms, 19.472ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.107ms
Latencies     [min, mean, 50, 90, 95, 99, max]  908.588µs, 1.735ms, 1.237ms, 1.606ms, 2.518ms, 13.79ms, 132.196ms
Bytes In      [total, mean]                     4800000, 160.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
