# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 09f31a0defdd4bf13c648139f55567bf908cfaac
- Date: 2026-04-15T14:59:42Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848324Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 12s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 8
	- 1000.0ms: 8
	- 5000.0ms: 8
	- 10000.0ms: 8
	- 30000.0ms: 8
	- +Infms: 8

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 19s

### Event Batch Processing

- Event Batch Total: 7
- Event Batch Processing Average Time: 40ms
- Event Batch Processing distribution:
	- 500.0ms: 7
	- 1000.0ms: 7
	- 5000.0ms: 7
	- 10000.0ms: 7
	- 30000.0ms: 7
	- +Infms: 7

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 280
- Event Batch Processing Average Time: 27ms
- Event Batch Processing distribution:
	- 500.0ms: 271
	- 1000.0ms: 280
	- 5000.0ms: 280
	- 10000.0ms: 280
	- 30000.0ms: 280
	- +Infms: 280

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 123s

### Event Batch Processing

- Event Batch Total: 1414
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 1381
	- 1000.0ms: 1405
	- 5000.0ms: 1414
	- 10000.0ms: 1414
	- 30000.0ms: 1414
	- +Infms: 1414

### NGINX Error Logs
