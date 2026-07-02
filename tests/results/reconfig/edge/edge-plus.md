# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 903211b7f256263c546d17dbbc037f7756f492e1
- Date: 2026-06-30T17:57:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1163012
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 4s

### Event Batch Processing

- Event Batch Total: 16
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 16
	- 1000.0ms: 16
	- 5000.0ms: 16
	- 10000.0ms: 16
	- 30000.0ms: 16
	- +Infms: 16

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 31s

### Event Batch Processing

- Event Batch Total: 17
- Event Batch Processing Average Time: 17ms
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
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 294
- Event Batch Processing Average Time: 30ms
- Event Batch Processing distribution:
	- 500.0ms: 282
	- 1000.0ms: 294
	- 5000.0ms: 294
	- 10000.0ms: 294
	- 30000.0ms: 294
	- +Infms: 294

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 126s

### Event Batch Processing

- Event Batch Total: 1463
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 1433
	- 1000.0ms: 1455
	- 5000.0ms: 1463
	- 10000.0ms: 1463
	- 30000.0ms: 1463
	- +Infms: 1463

### NGINX Error Logs
