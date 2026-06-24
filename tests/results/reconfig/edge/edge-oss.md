# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: abb4c6861bf41b5b3786b982af13408da5ec3db5
- Date: 2026-06-15T16:55:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1000000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 34s

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
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 22
- Event Batch Processing Average Time: 5ms
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
- TimeToReadyTotal: 27s

### Event Batch Processing

- Event Batch Total: 401
- Event Batch Processing Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 399
	- 1000.0ms: 401
	- 5000.0ms: 401
	- 10000.0ms: 401
	- 30000.0ms: 401
	- +Infms: 401

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 109s

### Event Batch Processing

- Event Batch Total: 1668
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 1665
	- 1000.0ms: 1668
	- 5000.0ms: 1668
	- 10000.0ms: 1668
	- 30000.0ms: 1668
	- +Infms: 1668

### NGINX Error Logs
