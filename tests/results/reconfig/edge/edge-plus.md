# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: e03b181692b24863f029d591a9cac59e1d44a8b7
- Date: 2025-09-09T14:26:43Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 16s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 18ms
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
- TimeToReadyTotal: 22s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 21ms
- Event Batch Processing distribution:
	- 500.0ms: 8
	- 1000.0ms: 8
	- 5000.0ms: 8
	- 10000.0ms: 8
	- 30000.0ms: 8
	- +Infms: 8

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 226
- Event Batch Processing Average Time: 57ms
- Event Batch Processing distribution:
	- 500.0ms: 220
	- 1000.0ms: 221
	- 5000.0ms: 226
	- 10000.0ms: 226
	- 30000.0ms: 226
	- +Infms: 226

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 128s

### Event Batch Processing

- Event Batch Total: 1285
- Event Batch Processing Average Time: 30ms
- Event Batch Processing distribution:
	- 500.0ms: 1260
	- 1000.0ms: 1266
	- 5000.0ms: 1285
	- 10000.0ms: 1285
	- 30000.0ms: 1285
	- +Infms: 1285

### NGINX Error Logs
