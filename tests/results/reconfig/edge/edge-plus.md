# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 76a2cea7c19f4aeb19d6610048db93fe3545dedc
- Date: 2025-12-03T19:53:07Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851512Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 14s

### Event Batch Processing

- Event Batch Total: 44
- Event Batch Processing Average Time: 3ms
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

- Event Batch Total: 41
- Event Batch Processing Average Time: 4ms
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
- TimeToReadyTotal: 26s

### Event Batch Processing

- Event Batch Total: 309
- Event Batch Processing Average Time: 27ms
- Event Batch Processing distribution:
	- 500.0ms: 298
	- 1000.0ms: 309
	- 5000.0ms: 309
	- 10000.0ms: 309
	- 30000.0ms: 309
	- +Infms: 309

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 117s

### Event Batch Processing

- Event Batch Total: 1482
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 1456
	- 1000.0ms: 1470
	- 5000.0ms: 1482
	- 10000.0ms: 1482
	- 30000.0ms: 1482
	- +Infms: 1482

### NGINX Error Logs
