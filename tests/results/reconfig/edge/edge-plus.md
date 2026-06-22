# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 14s

### Event Batch Processing

- Event Batch Total: 15
- Event Batch Processing Average Time: 17ms
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
- TimeToReadyTotal: 20s

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

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 299
- Event Batch Processing Average Time: 32ms
- Event Batch Processing distribution:
	- 500.0ms: 286
	- 1000.0ms: 299
	- 5000.0ms: 299
	- 10000.0ms: 299
	- 30000.0ms: 299
	- +Infms: 299

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 127s

### Event Batch Processing

- Event Batch Total: 1450
- Event Batch Processing Average Time: 21ms
- Event Batch Processing distribution:
	- 500.0ms: 1415
	- 1000.0ms: 1443
	- 5000.0ms: 1450
	- 10000.0ms: 1450
	- 30000.0ms: 1450
	- +Infms: 1450

### NGINX Error Logs
