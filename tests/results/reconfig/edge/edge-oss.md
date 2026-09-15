# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: a43969ce15ea40ce0548f0b187b16b9740113f82
- Date: 2026-09-15T14:01:26Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.7-gke.1222000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 21
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 21
	- 1000.0ms: 21
	- 5000.0ms: 21
	- 10000.0ms: 21
	- 30000.0ms: 21
	- +Infms: 21

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 38s

### Event Batch Processing

- Event Batch Total: 26
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 26
	- 1000.0ms: 26
	- 5000.0ms: 26
	- 10000.0ms: 26
	- 30000.0ms: 26
	- +Infms: 26

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 372
- Event Batch Processing Average Time: 20ms
- Event Batch Processing distribution:
	- 500.0ms: 366
	- 1000.0ms: 372
	- 5000.0ms: 372
	- 10000.0ms: 372
	- 30000.0ms: 372
	- +Infms: 372

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 139s

### Event Batch Processing

- Event Batch Total: 1780
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1777
	- 1000.0ms: 1780
	- 5000.0ms: 1780
	- 10000.0ms: 1780
	- 30000.0ms: 1780
	- +Infms: 1780

### NGINX Error Logs
