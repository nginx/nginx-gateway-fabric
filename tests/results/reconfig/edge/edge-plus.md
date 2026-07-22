# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 17s

### Event Batch Processing

- Event Batch Total: 17
- Event Batch Processing Average Time: 8ms
- Event Batch Processing distribution:
	- 500.0ms: 17
	- 1000.0ms: 17
	- 5000.0ms: 17
	- 10000.0ms: 17
	- 30000.0ms: 17
	- +Infms: 17

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 47s

### Event Batch Processing

- Event Batch Total: 20
- Event Batch Processing Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 20
	- 1000.0ms: 20
	- 5000.0ms: 20
	- 10000.0ms: 20
	- 30000.0ms: 20
	- +Infms: 20

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 289
- Event Batch Processing Average Time: 33ms
- Event Batch Processing distribution:
	- 500.0ms: 278
	- 1000.0ms: 288
	- 5000.0ms: 289
	- 10000.0ms: 289
	- 30000.0ms: 289
	- +Infms: 289

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 123s

### Event Batch Processing

- Event Batch Total: 1400
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 1368
	- 1000.0ms: 1393
	- 5000.0ms: 1400
	- 10000.0ms: 1400
	- 30000.0ms: 1400
	- +Infms: 1400

### NGINX Error Logs
