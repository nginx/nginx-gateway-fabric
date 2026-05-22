# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 13s

### Event Batch Processing

- Event Batch Total: 16
- Event Batch Processing Average Time: 13ms
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
- TimeToReadyTotal: 40s

### Event Batch Processing

- Event Batch Total: 17
- Event Batch Processing Average Time: 19ms
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
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 300
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 290
	- 1000.0ms: 300
	- 5000.0ms: 300
	- 10000.0ms: 300
	- 30000.0ms: 300
	- +Infms: 300

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 129s

### Event Batch Processing

- Event Batch Total: 1450
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1418
	- 1000.0ms: 1443
	- 5000.0ms: 1450
	- 10000.0ms: 1450
	- 30000.0ms: 1450
	- +Infms: 1450

### NGINX Error Logs
