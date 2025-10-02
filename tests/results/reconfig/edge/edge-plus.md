# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9010072ecd34a8fa99bfdd3d7580c9d725fb063e
- Date: 2025-10-01T09:39:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1172000
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

- Event Batch Total: 8
- Event Batch Processing Average Time: 19ms
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
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 19ms
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

- Event Batch Total: 260
- Event Batch Processing Average Time: 32ms
- Event Batch Processing distribution:
	- 500.0ms: 249
	- 1000.0ms: 260
	- 5000.0ms: 260
	- 10000.0ms: 260
	- 30000.0ms: 260
	- +Infms: 260

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 134s

### Event Batch Processing

- Event Batch Total: 1313
- Event Batch Processing Average Time: 29ms
- Event Batch Processing distribution:
	- 500.0ms: 1282
	- 1000.0ms: 1299
	- 5000.0ms: 1313
	- 10000.0ms: 1313
	- 30000.0ms: 1313
	- +Infms: 1313

### NGINX Error Logs
