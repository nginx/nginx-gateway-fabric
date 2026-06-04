# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 28d0224c5f1617ace603b72889b5bb7aa272ea20
- Date: 2026-06-01T17:32:15Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 34s

### Event Batch Processing

- Event Batch Total: 19
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 19
	- 1000.0ms: 19
	- 5000.0ms: 19
	- 10000.0ms: 19
	- 30000.0ms: 19
	- +Infms: 19

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 29s

### Event Batch Processing

- Event Batch Total: 17
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 17
	- 1000.0ms: 17
	- 5000.0ms: 17
	- 10000.0ms: 17
	- 30000.0ms: 17
	- +Infms: 17

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 29s

### Event Batch Processing

- Event Batch Total: 310
- Event Batch Processing Average Time: 35ms
- Event Batch Processing distribution:
	- 500.0ms: 299
	- 1000.0ms: 305
	- 5000.0ms: 310
	- 10000.0ms: 310
	- 30000.0ms: 310
	- +Infms: 310

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 129s

### Event Batch Processing

- Event Batch Total: 1451
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1417
	- 1000.0ms: 1444
	- 5000.0ms: 1451
	- 10000.0ms: 1451
	- 30000.0ms: 1451
	- +Infms: 1451

### NGINX Error Logs
