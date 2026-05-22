# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: cd422a074b2f5d3ac6db374b6bc9bb4bf1c67e59
- Date: 2026-05-15T14:36:06Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.3-gke.1389000
- vCPUs per node: 16
- RAM per node: 65848296Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 9s

### Event Batch Processing

- Event Batch Total: 16
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 16
	- 1000.0ms: 16
	- 5000.0ms: 16
	- 10000.0ms: 16
	- 30000.0ms: 16
	- +Infms: 16

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 30s

### Event Batch Processing

- Event Batch Total: 21
- Event Batch Processing Average Time: 4ms
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
- TimeToReadyTotal: 28s

### Event Batch Processing

- Event Batch Total: 384
- Event Batch Processing Average Time: 19ms
- Event Batch Processing distribution:
	- 500.0ms: 382
	- 1000.0ms: 384
	- 5000.0ms: 384
	- 10000.0ms: 384
	- 30000.0ms: 384
	- +Infms: 384

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 123s

### Event Batch Processing

- Event Batch Total: 1772
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1772
	- 1000.0ms: 1772
	- 5000.0ms: 1772
	- 10000.0ms: 1772
	- 30000.0ms: 1772
	- +Infms: 1772

### NGINX Error Logs
