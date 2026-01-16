# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: d4376776aecc98294dc881a49cfbfa491773f74d
- Date: 2026-01-15T17:08:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2019000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 45
- Event Batch Processing Average Time: 0ms
- Event Batch Processing distribution:
	- 500.0ms: 45
	- 1000.0ms: 45
	- 5000.0ms: 45
	- 10000.0ms: 45
	- 30000.0ms: 45
	- +Infms: 45

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 39s

### Event Batch Processing

- Event Batch Total: 67
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 67
	- 1000.0ms: 67
	- 5000.0ms: 67
	- 10000.0ms: 67
	- 30000.0ms: 67
	- +Infms: 67

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 422
- Event Batch Processing Average Time: 13ms
- Event Batch Processing distribution:
	- 500.0ms: 421
	- 1000.0ms: 422
	- 5000.0ms: 422
	- 10000.0ms: 422
	- 30000.0ms: 422
	- +Infms: 422

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 100s

### Event Batch Processing

- Event Batch Total: 1708
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 1708
	- 1000.0ms: 1708
	- 5000.0ms: 1708
	- 10000.0ms: 1708
	- 30000.0ms: 1708
	- +Infms: 1708

### NGINX Error Logs
