# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: b41c973c8399458984def3c2a8a268a237c864c8
- Date: 2025-10-30T03:04:40Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1162000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 17s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 18ms
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
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 18ms
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
- TimeToReadyTotal: 27s

### Event Batch Processing

- Event Batch Total: 266
- Event Batch Processing Average Time: 32ms
- Event Batch Processing distribution:
	- 500.0ms: 255
	- 1000.0ms: 266
	- 5000.0ms: 266
	- 10000.0ms: 266
	- 30000.0ms: 266
	- +Infms: 266

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 126s

### Event Batch Processing

- Event Batch Total: 1286
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 1260
	- 1000.0ms: 1272
	- 5000.0ms: 1286
	- 10000.0ms: 1286
	- 30000.0ms: 1286
	- +Infms: 1286

### NGINX Error Logs
