# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: e8ee7c1c4f14e249927a5447a1af2615ddbe0f87
- Date: 2025-12-17T20:04:03Z
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
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 42
- Event Batch Processing Average Time: 0ms
- Event Batch Processing distribution:
	- 500.0ms: 42
	- 1000.0ms: 42
	- 5000.0ms: 42
	- 10000.0ms: 42
	- 30000.0ms: 42
	- +Infms: 42

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 22s

### Event Batch Processing

- Event Batch Total: 44
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 44
	- 1000.0ms: 44
	- 5000.0ms: 44
	- 10000.0ms: 44
	- 30000.0ms: 44
	- +Infms: 44

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 418
- Event Batch Processing Average Time: 14ms
- Event Batch Processing distribution:
	- 500.0ms: 417
	- 1000.0ms: 418
	- 5000.0ms: 418
	- 10000.0ms: 418
	- 30000.0ms: 418
	- +Infms: 418

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 123s

### Event Batch Processing

- Event Batch Total: 1853
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 1853
	- 1000.0ms: 1853
	- 5000.0ms: 1853
	- 10000.0ms: 1853
	- 30000.0ms: 1853
	- +Infms: 1853

### NGINX Error Logs
