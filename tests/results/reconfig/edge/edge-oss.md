# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: e03b181692b24863f029d591a9cac59e1d44a8b7
- Date: 2025-09-09T14:26:43Z
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
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 10
	- 1000.0ms: 10
	- 5000.0ms: 10
	- 10000.0ms: 10
	- 30000.0ms: 10
	- +Infms: 10

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 8ms
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
- TimeToReadyTotal: 26s

### Event Batch Processing

- Event Batch Total: 263
- Event Batch Processing Average Time: 31ms
- Event Batch Processing distribution:
	- 500.0ms: 256
	- 1000.0ms: 263
	- 5000.0ms: 263
	- 10000.0ms: 263
	- 30000.0ms: 263
	- +Infms: 263

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 102s

### Event Batch Processing

- Event Batch Total: 1240
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1209
	- 1000.0ms: 1240
	- 5000.0ms: 1240
	- 10000.0ms: 1240
	- 30000.0ms: 1240
	- +Infms: 1240

### NGINX Error Logs
