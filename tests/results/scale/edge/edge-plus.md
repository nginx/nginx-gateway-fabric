# Results

## Test environment

NGINX Plus: true

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

- Total: 1296
- Average Time: 33ms
- Event Batch Processing distribution:
	- 500.0ms: 1236
	- 1000.0ms: 1296
	- 5000.0ms: 1296
	- 10000.0ms: 1296
	- 30000.0ms: 1296
	- +Infms: 1296

### Errors

- NGF errors: 3
- NGF container restarts: 0
- NGINX errors: 4
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 1363
- Average Time: 36ms
- Event Batch Processing distribution:
	- 500.0ms: 1299
	- 1000.0ms: 1363
	- 5000.0ms: 1363
	- 10000.0ms: 1363
	- 30000.0ms: 1363
	- +Infms: 1363

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 53
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 2094
- Average Time: 110ms
- Event Batch Processing distribution:
	- 500.0ms: 2006
	- 1000.0ms: 2094
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

- Total: 60
- Average Time: 346ms
- Event Batch Processing distribution:
	- 500.0ms: 35
	- 1000.0ms: 58
	- 5000.0ms: 60
	- 10000.0ms: 60
	- 30000.0ms: 60
	- +Infms: 60

### Errors

- NGF errors: 1
- NGF container restarts: 0
- NGINX errors: 150
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_UpstreamServers) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPMatches

```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 735.195µs
Latencies     [min, mean, 50, 90, 95, 99, max]  583.324µs, 739.973µs, 723.776µs, 797.512µs, 829.116µs, 966.126µs, 13.693ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 915.575µs
Latencies     [min, mean, 50, 90, 95, 99, max]  698.651µs, 884.771µs, 856.003µs, 946.992µs, 985.568µs, 1.164ms, 23.428ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
