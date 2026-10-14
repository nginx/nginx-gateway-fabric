# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: ea67c576e39284e758352dda72242024cc98fa7f
- Date: 2026-10-01T15:28:08Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.8-gke.1225000
- vCPUs per node: 16
- RAM per node: 65848288Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 7s

### Event Batch Processing

- Event Batch Total: 17
- Event Batch Processing Average Time: 2ms
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
- TimeToReadyTotal: 33s

### Event Batch Processing

- Event Batch Total: 22
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 22
	- 1000.0ms: 22
	- 5000.0ms: 22
	- 10000.0ms: 22
	- 30000.0ms: 22
	- +Infms: 22

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 410
- Event Batch Processing Average Time: 13ms
- Event Batch Processing distribution:
	- 500.0ms: 410
	- 1000.0ms: 410
	- 5000.0ms: 410
	- 10000.0ms: 410
	- 30000.0ms: 410
	- +Infms: 410

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 106s

### Event Batch Processing

- Event Batch Total: 1690
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1690
	- 1000.0ms: 1690
	- 5000.0ms: 1690
	- 10000.0ms: 1690
	- 30000.0ms: 1690
	- +Infms: 1690

### NGINX Error Logs
