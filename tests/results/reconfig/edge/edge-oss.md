# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 17c42c8bbbb004ba9c0e9b867396c5f8937207cd
- Date: 2026-04-01T18:33:47Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.1-gke.1396002
- vCPUs per node: 16
- RAM per node: 64305Mi
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 14s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 4ms
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
- TimeToReadyTotal: 20s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 10ms
- Event Batch Processing distribution:
	- 500.0ms: 9
	- 1000.0ms: 9
	- 5000.0ms: 9
	- 10000.0ms: 9
	- 30000.0ms: 9
	- +Infms: 9

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 386
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 385
	- 1000.0ms: 386
	- 5000.0ms: 386
	- 10000.0ms: 386
	- 30000.0ms: 386
	- +Infms: 386

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 125s

### Event Batch Processing

- Event Batch Total: 1707
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1704
	- 1000.0ms: 1707
	- 5000.0ms: 1707
	- 10000.0ms: 1707
	- 30000.0ms: 1707
	- +Infms: 1707

### NGINX Error Logs
