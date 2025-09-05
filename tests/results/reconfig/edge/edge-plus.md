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

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 13s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 37ms
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

- Event Batch Total: 8
- Event Batch Processing Average Time: 27ms
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
- TimeToReadyTotal: 29s

### Event Batch Processing

- Event Batch Total: 233
- Event Batch Processing Average Time: 59ms
- Event Batch Processing distribution:
	- 500.0ms: 227
	- 1000.0ms: 228
	- 5000.0ms: 233
	- 10000.0ms: 233
	- 30000.0ms: 233
	- +Infms: 233

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 146s

### Event Batch Processing

- Event Batch Total: 1317
- Event Batch Processing Average Time: 31ms
- Event Batch Processing distribution:
	- 500.0ms: 1305
	- 1000.0ms: 1306
	- 5000.0ms: 1316
	- 10000.0ms: 1317
	- 30000.0ms: 1317
	- +Infms: 1317

### NGINX Error Logs
