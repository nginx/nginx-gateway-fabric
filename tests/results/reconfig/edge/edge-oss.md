# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 903211b7f256263c546d17dbbc037f7756f492e1
- Date: 2026-06-30T17:57:05Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.5-gke.1163012
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 44s

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
- TimeToReadyTotal: 42s

### Event Batch Processing

- Event Batch Total: 37
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 37
	- 1000.0ms: 37
	- 5000.0ms: 37
	- 10000.0ms: 37
	- 30000.0ms: 37
	- +Infms: 37

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 381
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 378
	- 1000.0ms: 381
	- 5000.0ms: 381
	- 10000.0ms: 381
	- 30000.0ms: 381
	- +Infms: 381

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 124s

### Event Batch Processing

- Event Batch Total: 1742
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1740
	- 1000.0ms: 1742
	- 5000.0ms: 1742
	- 10000.0ms: 1742
	- 30000.0ms: 1742
	- +Infms: 1742

### NGINX Error Logs
