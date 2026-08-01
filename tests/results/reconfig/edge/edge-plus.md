# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 394bdf0e0c8ae008009546b7a21d8f80248d52be
- Date: 2026-07-30T17:54:44Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1250000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 14s

### Event Batch Processing

- Event Batch Total: 15
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 15
	- 1000.0ms: 15
	- 5000.0ms: 15
	- 10000.0ms: 15
	- 30000.0ms: 15
	- +Infms: 15

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 41s

### Event Batch Processing

- Event Batch Total: 19
- Event Batch Processing Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 19
	- 1000.0ms: 19
	- 5000.0ms: 19
	- 10000.0ms: 19
	- 30000.0ms: 19
	- +Infms: 19

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 319
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 310
	- 1000.0ms: 319
	- 5000.0ms: 319
	- 10000.0ms: 319
	- 30000.0ms: 319
	- +Infms: 319

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 128s

### Event Batch Processing

- Event Batch Total: 1487
- Event Batch Processing Average Time: 21ms
- Event Batch Processing distribution:
	- 500.0ms: 1456
	- 1000.0ms: 1480
	- 5000.0ms: 1487
	- 10000.0ms: 1487
	- 30000.0ms: 1487
	- +Infms: 1487

### NGINX Error Logs
