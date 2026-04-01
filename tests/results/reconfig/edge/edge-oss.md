# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 2f3153c547e0442fbb26aa9165118f4dc2b20f23
- Date: 2026-04-01T15:39:22Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 65848316Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 19s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 10
	- 1000.0ms: 10
	- 5000.0ms: 10
	- 10000.0ms: 10
	- 30000.0ms: 10
	- +Infms: 10

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 26s

### Event Batch Processing

- Event Batch Total: 8
- Event Batch Processing Average Time: 9ms
- Event Batch Processing distribution:
	- 500.0ms: 8
	- 1000.0ms: 8
	- 5000.0ms: 8
	- 10000.0ms: 8
	- 30000.0ms: 8
	- +Infms: 8

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 367
- Event Batch Processing Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 366
	- 1000.0ms: 367
	- 5000.0ms: 367
	- 10000.0ms: 367
	- 30000.0ms: 367
	- +Infms: 367

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 110s

### Event Batch Processing

- Event Batch Total: 1617
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 1616
	- 1000.0ms: 1617
	- 5000.0ms: 1617
	- 10000.0ms: 1617
	- 30000.0ms: 1617
	- +Infms: 1617

### NGINX Error Logs
