# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 17s

### Event Batch Processing

- Event Batch Total: 45
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 45
	- 1000.0ms: 45
	- 5000.0ms: 45
	- 10000.0ms: 45
	- 30000.0ms: 45
	- +Infms: 45

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 17s

### Event Batch Processing

- Event Batch Total: 41
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 41
	- 1000.0ms: 41
	- 5000.0ms: 41
	- 10000.0ms: 41
	- 30000.0ms: 41
	- +Infms: 41

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 27s

### Event Batch Processing

- Event Batch Total: 391
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 387
	- 1000.0ms: 391
	- 5000.0ms: 391
	- 10000.0ms: 391
	- 30000.0ms: 391
	- +Infms: 391

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 116s

### Event Batch Processing

- Event Batch Total: 1795
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 1791
	- 1000.0ms: 1795
	- 5000.0ms: 1795
	- 10000.0ms: 1795
	- 30000.0ms: 1795
	- +Infms: 1795

### NGINX Error Logs
