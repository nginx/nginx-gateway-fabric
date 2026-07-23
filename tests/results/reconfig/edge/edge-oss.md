# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 3f79877f3b0abebd33ccda280a3a8a906fae5359
- Date: 2026-07-15T15:34:03Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1241004
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 20s

### Event Batch Processing

- Event Batch Total: 22
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 22
	- 1000.0ms: 22
	- 5000.0ms: 22
	- 10000.0ms: 22
	- 30000.0ms: 22
	- +Infms: 22

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 40s

### Event Batch Processing

- Event Batch Total: 21
- Event Batch Processing Average Time: 7ms
- Event Batch Processing distribution:
	- 500.0ms: 21
	- 1000.0ms: 21
	- 5000.0ms: 21
	- 10000.0ms: 21
	- 30000.0ms: 21
	- +Infms: 21

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 399
- Event Batch Processing Average Time: 14ms
- Event Batch Processing distribution:
	- 500.0ms: 399
	- 1000.0ms: 399
	- 5000.0ms: 399
	- 10000.0ms: 399
	- 30000.0ms: 399
	- +Infms: 399

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 129s

### Event Batch Processing

- Event Batch Total: 1736
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1735
	- 1000.0ms: 1736
	- 5000.0ms: 1736
	- 10000.0ms: 1736
	- 30000.0ms: 1736
	- +Infms: 1736

### NGINX Error Logs
