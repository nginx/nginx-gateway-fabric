# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 35e53177e0234a92ce7b97deca269d747ab60c61
- Date: 2025-09-03T20:40:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test TestScale_Listeners

### Event Batch Processing

- Total: 325
- Average Time: 195ms
- Event Batch Processing distribution:
	- 500.0ms: 263
	- 1000.0ms: 298
	- 5000.0ms: 325
	- 10000.0ms: 325
	- 30000.0ms: 325
	- +Infms: 325

### Errors

- NGF errors: 7
- NGF container restarts: 0
- NGINX errors: 391
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_Listeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPSListeners

### Event Batch Processing

- Total: 392
- Average Time: 364ms
- Event Batch Processing distribution:
	- 500.0ms: 278
	- 1000.0ms: 344
	- 5000.0ms: 392
	- 10000.0ms: 392
	- 30000.0ms: 392
	- +Infms: 392

### Errors

- NGF errors: 7
- NGF container restarts: 0
- NGINX errors: 552
- NGINX container restarts: 0

### Graphs and Logs

See [output directory](./TestScale_HTTPSListeners) for more details.
The logs are attached only if there are errors.

## Test TestScale_HTTPRoutes

### Event Batch Processing

- Total: 1009
- Average Time: 759ms
- Event Batch Processing distribution:
	- 500.0ms: 153
	- 1000.0ms: 945
	- 5000.0ms: 1009
	- 10000.0ms: 1009
	- 30000.0ms: 1009
	- +Infms: 1009

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

- Total: 27
- Average Time: 359ms
- Event Batch Processing distribution:
	- 500.0ms: 19
	- 1000.0ms: 25
	- 5000.0ms: 27
	- 10000.0ms: 27
	- 30000.0ms: 27
	- +Infms: 27

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
Requests      [total, rate, throughput]         30000, 1000.04, 1000.01
Duration      [total, attack, wait]             30s, 29.999s, 801.484µs
Latencies     [min, mean, 50, 90, 95, 99, max]  695.039µs, 882.16µs, 857.064µs, 967.876µs, 1.021ms, 1.218ms, 14.158ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
```text
Requests      [total, rate, throughput]         30000, 1000.04, 1000.00
Duration      [total, attack, wait]             30s, 29.999s, 1.028ms
Latencies     [min, mean, 50, 90, 95, 99, max]  789.103µs, 1.037ms, 1.01ms, 1.152ms, 1.212ms, 1.391ms, 17.399ms
Bytes In      [total, mean]                     4770000, 159.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:30000  
Error Set:
```
