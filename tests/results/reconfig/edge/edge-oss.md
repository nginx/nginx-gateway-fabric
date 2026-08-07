# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 394bdf0e0c8ae008009546b7a21d8f80248d52be
- Date: 2026-07-30T17:54:44Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1250000
- vCPUs per node: 16
- RAM per node: 65848292Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 26s

### Event Batch Processing

- Event Batch Total: 20
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 20
	- 1000.0ms: 20
	- 5000.0ms: 20
	- 10000.0ms: 20
	- 30000.0ms: 20
	- +Infms: 20

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 49s

### Event Batch Processing

- Event Batch Total: 25
- Event Batch Processing Average Time: 5ms
- Event Batch Processing distribution:
	- 500.0ms: 25
	- 1000.0ms: 25
	- 5000.0ms: 25
	- 10000.0ms: 25
	- 30000.0ms: 25
	- +Infms: 25

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 368
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 368
	- 1000.0ms: 368
	- 5000.0ms: 368
	- 10000.0ms: 368
	- 30000.0ms: 368
	- +Infms: 368

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 132s

### Event Batch Processing

- Event Batch Total: 1719
- Event Batch Processing Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 1713
	- 1000.0ms: 1719
	- 5000.0ms: 1719
	- 10000.0ms: 1719
	- 30000.0ms: 1719
	- +Infms: 1719

### NGINX Error Logs
