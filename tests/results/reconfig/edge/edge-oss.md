# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 635b3fcd6e643f4bd24ebbd4c901619a030c4bc0
- Date: 2025-09-15T17:56:13Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1036000
- vCPUs per node: 16
- RAM per node: 65851528Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 3ms
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
- TimeToReadyTotal: 27s

### Event Batch Processing

- Event Batch Total: 11
- Event Batch Processing Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 11
	- 1000.0ms: 11
	- 5000.0ms: 11
	- 10000.0ms: 11
	- 30000.0ms: 11
	- +Infms: 11

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 247
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 239
	- 1000.0ms: 247
	- 5000.0ms: 247
	- 10000.0ms: 247
	- 30000.0ms: 247
	- +Infms: 247

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 112s

### Event Batch Processing

- Event Batch Total: 1265
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1229
	- 1000.0ms: 1265
	- 5000.0ms: 1265
	- 10000.0ms: 1265
	- 30000.0ms: 1265
	- +Infms: 1265

### NGINX Error Logs
