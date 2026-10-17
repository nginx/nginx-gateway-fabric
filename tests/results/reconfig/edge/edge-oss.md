# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 0042ca1a05b3e6ee63f08f0cf2ca8090a4f6a1e6
- Date: 2026-10-15T15:55:29Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.36.4-gke.1391002
- vCPUs per node: 16
- RAM per node: 65848244Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 41s

### Event Batch Processing

- Event Batch Total: 23
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 23
	- 1000.0ms: 23
	- 5000.0ms: 23
	- 10000.0ms: 23
	- 30000.0ms: 23
	- +Infms: 23

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 18
- Event Batch Processing Average Time: 7ms
- Event Batch Processing distribution:
	- 500.0ms: 18
	- 1000.0ms: 18
	- 5000.0ms: 18
	- 10000.0ms: 18
	- 30000.0ms: 18
	- +Infms: 18

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 376
- Event Batch Processing Average Time: 20ms
- Event Batch Processing distribution:
	- 500.0ms: 371
	- 1000.0ms: 376
	- 5000.0ms: 376
	- 10000.0ms: 376
	- 30000.0ms: 376
	- +Infms: 376

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 173s

### Event Batch Processing

- Event Batch Total: 1875
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1871
	- 1000.0ms: 1875
	- 5000.0ms: 1875
	- 10000.0ms: 1875
	- 30000.0ms: 1875
	- +Infms: 1875

### NGINX Error Logs
