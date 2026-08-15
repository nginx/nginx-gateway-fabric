# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: fef4f734728239e3248cda180e74f72e436b06b9
- Date: 2026-08-14T15:07:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1258000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 20s

### Event Batch Processing

- Event Batch Total: 15
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 15
	- 1000.0ms: 15
	- 5000.0ms: 15
	- 10000.0ms: 15
	- 30000.0ms: 15
	- +Infms: 15

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

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
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 339
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 329
	- 1000.0ms: 339
	- 5000.0ms: 339
	- 10000.0ms: 339
	- 30000.0ms: 339
	- +Infms: 339

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 118s

### Event Batch Processing

- Event Batch Total: 1401
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 1368
	- 1000.0ms: 1393
	- 5000.0ms: 1401
	- 10000.0ms: 1401
	- 30000.0ms: 1401
	- +Infms: 1401

### NGINX Error Logs
