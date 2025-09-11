# Results

## Test environment

NGINX Plus: false

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
- Event Batch Processing Average Time: 3ms
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
- Event Batch Processing Average Time: 8ms
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
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 257
- Event Batch Processing Average Time: 32ms
- Event Batch Processing distribution:
	- 500.0ms: 248
	- 1000.0ms: 257
	- 5000.0ms: 257
	- 10000.0ms: 257
	- 30000.0ms: 257
	- +Infms: 257

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 124s

### Event Batch Processing

- Event Batch Total: 1314
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 1275
	- 1000.0ms: 1314
	- 5000.0ms: 1314
	- 10000.0ms: 1314
	- 30000.0ms: 1314
	- +Infms: 1314

### NGINX Error Logs
