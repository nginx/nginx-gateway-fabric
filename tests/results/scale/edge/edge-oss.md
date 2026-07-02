# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 903211b7f256263c546d17dbbc037f7756f492e1
- Date: 2026-06-30T17:57:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1163012
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 1287
- Average Time: 11ms
- Event Batch Processing distribution:
	- 500.0ms: 1272
	- 1000.0ms: 1287
	- 5000.0ms: 1287
	- 10000.0ms: 1287
	- 30000.0ms: 1287
	- +Infms: 1287

### Errors

- NGF errors: 9
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1350
- Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 1340
	- 1000.0ms: 1350
	- 5000.0ms: 1350
	- 10000.0ms: 1350
	- 30000.0ms: 1350
	- +Infms: 1350

### Errors

- NGF errors: 14
- NGF container restarts: 0
- NGINX errors: 0
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2079
- Average Time: 89ms
- Event Batch Processing distribution:
	- 500.0ms: 1991
	- 1000.0ms: 2079
	- 5000.0ms: 2079
	- 10000.0ms: 2079
	- 30000.0ms: 2079
	- +Infms: 2079

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

- Total: 88
- Average Time: 148ms
- Event Batch Processing distribution:
	- 500.0ms: 77
	- 1000.0ms: 88
	- 5000.0ms: 88
	- 10000.0ms: 88
	- 30000.0ms: 88
	- +Infms: 88

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
Duration      [total, attack, wait]             30s, 29.999s, 711.747µs
Latencies     [min, mean, 50, 90, 95, 99, max]  604.835µs, 823.962µs, 800.223µs, 908.223µs, 952.074µs, 1.091ms, 21.704ms
Bytes In      [total, mean]                     4830000, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         29999, 1000.01, 999.97
Duration      [total, attack, wait]             30s, 29.999s, 1.046ms
Latencies     [min, mean, 50, 90, 95, 99, max]  721.217µs, 969.837µs, 936.354µs, 1.063ms, 1.118ms, 1.297ms, 22.528ms
Bytes In      [total, mean]                     4829839, 161.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:29999  
Error Set:
```
