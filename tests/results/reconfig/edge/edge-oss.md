# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 28d0224c5f1617ace603b72889b5bb7aa272ea20
- Date: 2026-06-01T17:32:15Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389002
- vCPUs per node: 16
- RAM per node: 65848300Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 8s

### Event Batch Processing

- Event Batch Total: 16
- Event Batch Processing Average Time: 2ms
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
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 20
- Event Batch Processing Average Time: 5ms
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
- TimeToReadyTotal: 27s

### Event Batch Processing

- Event Batch Total: 360
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 355
	- 1000.0ms: 360
	- 5000.0ms: 360
	- 10000.0ms: 360
	- 30000.0ms: 360
	- +Infms: 360

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 126s

### Event Batch Processing

- Event Batch Total: 1730
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1728
	- 1000.0ms: 1730
	- 5000.0ms: 1730
	- 10000.0ms: 1730
	- 30000.0ms: 1730
	- +Infms: 1730

### NGINX Error Logs
