# Results

## Test environment

NGINX Plus: false

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
- TimeToReadyTotal: 15s

### Event Batch Processing

- Event Batch Total: 11
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 11
	- 1000.0ms: 11
	- 5000.0ms: 11
	- 10000.0ms: 11
	- 30000.0ms: 11
	- +Infms: 11

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 11
- Event Batch Processing Average Time: 8ms
- Event Batch Processing distribution:
	- 500.0ms: 11
	- 1000.0ms: 11
	- 5000.0ms: 11
	- 10000.0ms: 11
	- 30000.0ms: 11
	- +Infms: 11

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 27s

### Event Batch Processing

- Event Batch Total: 270
- Event Batch Processing Average Time: 33ms
- Event Batch Processing distribution:
	- 500.0ms: 265
	- 1000.0ms: 270
	- 5000.0ms: 270
	- 10000.0ms: 270
	- 30000.0ms: 270
	- +Infms: 270

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 122s

### Event Batch Processing

- Event Batch Total: 1309
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1278
	- 1000.0ms: 1309
	- 5000.0ms: 1309
	- 10000.0ms: 1309
	- 30000.0ms: 1309
	- +Infms: 1309

### NGINX Error Logs
