# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fef4f734728239e3248cda180e74f72e436b06b9
- Date: 2026-08-14T15:07:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.6-gke.1258000
- vCPUs per node: 16
- RAM per node: 65848288Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 44s

### Event Batch Processing

- Event Batch Total: 24
- Event Batch Processing Average Time: 2ms
- Event Batch Processing distribution:
	- 500.0ms: 24
	- 1000.0ms: 24
	- 5000.0ms: 24
	- 10000.0ms: 24
	- 30000.0ms: 24
	- +Infms: 24

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 30s

### Event Batch Processing

- Event Batch Total: 23
- Event Batch Processing Average Time: 4ms
- Event Batch Processing distribution:
	- 500.0ms: 23
	- 1000.0ms: 23
	- 5000.0ms: 23
	- 10000.0ms: 23
	- 30000.0ms: 23
	- +Infms: 23

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 18s

### Event Batch Processing

- Event Batch Total: 379
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 377
	- 1000.0ms: 379
	- 5000.0ms: 379
	- 10000.0ms: 379
	- 30000.0ms: 379
	- +Infms: 379

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 101s

### Event Batch Processing

- Event Batch Total: 1614
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1613
	- 1000.0ms: 1614
	- 5000.0ms: 1614
	- 10000.0ms: 1614
	- 30000.0ms: 1614
	- +Infms: 1614

### NGINX Error Logs
