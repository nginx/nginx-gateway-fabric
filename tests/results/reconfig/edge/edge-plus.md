# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: d97dd7debc1ea5d51f4413b6564b27921a1fc982
- Date: 2026-02-27T17:29:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.34.3-gke.1318000
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 18s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 20ms
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
- TimeToReadyTotal: 26s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 26ms
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
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 285
- Event Batch Processing Average Time: 35ms
- Event Batch Processing distribution:
	- 500.0ms: 273
	- 1000.0ms: 283
	- 5000.0ms: 285
	- 10000.0ms: 285
	- 30000.0ms: 285
	- +Infms: 285

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 130s

### Event Batch Processing

- Event Batch Total: 1462
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 1431
	- 1000.0ms: 1454
	- 5000.0ms: 1462
	- 10000.0ms: 1462
	- 30000.0ms: 1462
	- +Infms: 1462

### NGINX Error Logs
