# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 56940dadad455136037643364353ad1d6f3a1faa
- Date: 2025-09-11T08:56:22Z
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
- TimeToReadyTotal: 17s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 30ms
- Event Batch Processing distribution:
	- 500.0ms: 9
	- 1000.0ms: 9
	- 5000.0ms: 9
	- 10000.0ms: 9
	- 30000.0ms: 9
	- +Infms: 9

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 20ms
- Event Batch Processing distribution:
	- 500.0ms: 9
	- 1000.0ms: 9
	- 5000.0ms: 9
	- 10000.0ms: 9
	- 30000.0ms: 9
	- +Infms: 9

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 254
- Event Batch Processing Average Time: 35ms
- Event Batch Processing distribution:
	- 500.0ms: 244
	- 1000.0ms: 252
	- 5000.0ms: 254
	- 10000.0ms: 254
	- 30000.0ms: 254
	- +Infms: 254

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 127s

### Event Batch Processing

- Event Batch Total: 1266
- Event Batch Processing Average Time: 29ms
- Event Batch Processing distribution:
	- 500.0ms: 1254
	- 1000.0ms: 1255
	- 5000.0ms: 1265
	- 10000.0ms: 1266
	- 30000.0ms: 1266
	- +Infms: 1266

### NGINX Error Logs
