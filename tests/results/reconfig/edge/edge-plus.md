# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 89aee48bf6e660a828ffd32ca35fc7f52e358e00
- Date: 2025-12-12T20:04:38Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 15s

### Event Batch Processing

- Event Batch Total: 44
- Event Batch Processing Average Time: 6ms
- Event Batch Processing distribution:
	- 500.0ms: 44
	- 1000.0ms: 44
	- 5000.0ms: 44
	- 10000.0ms: 44
	- 30000.0ms: 44
	- +Infms: 44

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 55
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 55
	- 1000.0ms: 55
	- 5000.0ms: 55
	- 10000.0ms: 55
	- 30000.0ms: 55
	- +Infms: 55

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 330
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 319
	- 1000.0ms: 330
	- 5000.0ms: 330
	- 10000.0ms: 330
	- 30000.0ms: 330
	- +Infms: 330

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 124s

### Event Batch Processing

- Event Batch Total: 1513
- Event Batch Processing Average Time: 21ms
- Event Batch Processing distribution:
	- 500.0ms: 1486
	- 1000.0ms: 1499
	- 5000.0ms: 1513
	- 10000.0ms: 1513
	- 30000.0ms: 1513
	- +Infms: 1513

### NGINX Error Logs
