# Results

## Test environment

NGINX Plus: true

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
- TimeToReadyTotal: 17s

### Event Batch Processing

- Event Batch Total: 56
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 56
	- 1000.0ms: 56
	- 5000.0ms: 56
	- 10000.0ms: 56
	- 30000.0ms: 56
	- +Infms: 56

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 54
- Event Batch Processing Average Time: 4ms
- Event Batch Processing distribution:
	- 500.0ms: 54
	- 1000.0ms: 54
	- 5000.0ms: 54
	- 10000.0ms: 54
	- 30000.0ms: 54
	- +Infms: 54

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 333
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 325
	- 1000.0ms: 333
	- 5000.0ms: 333
	- 10000.0ms: 333
	- 30000.0ms: 333
	- +Infms: 333

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 123s

### Event Batch Processing

- Event Batch Total: 1543
- Event Batch Processing Average Time: 20ms
- Event Batch Processing distribution:
	- 500.0ms: 1516
	- 1000.0ms: 1530
	- 5000.0ms: 1543
	- 10000.0ms: 1543
	- 30000.0ms: 1543
	- +Infms: 1543

### NGINX Error Logs
