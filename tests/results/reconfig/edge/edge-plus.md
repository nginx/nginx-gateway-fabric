# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 17c42c8bbbb004ba9c0e9b867396c5f8937207cd
- Date: 2026-04-01T18:33:47Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 14s

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
- TimeToReadyTotal: 18s

### Event Batch Processing

- Event Batch Total: 7
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 7
	- 1000.0ms: 7
	- 5000.0ms: 7
	- 10000.0ms: 7
	- 30000.0ms: 7
	- +Infms: 7

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 295
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 285
	- 1000.0ms: 295
	- 5000.0ms: 295
	- 10000.0ms: 295
	- 30000.0ms: 295
	- +Infms: 295

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 120s

### Event Batch Processing

- Event Batch Total: 1381
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 1349
	- 1000.0ms: 1374
	- 5000.0ms: 1381
	- 10000.0ms: 1381
	- 30000.0ms: 1381
	- +Infms: 1381

### NGINX Error Logs
