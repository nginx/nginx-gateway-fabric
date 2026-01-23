# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: eb3a090367b0c4a450224993fc4eed39e6dd9dc4
- Date: 2026-01-22T21:37:34Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.2072000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 15s

### Event Batch Processing

- Event Batch Total: 45
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 45
	- 1000.0ms: 45
	- 5000.0ms: 45
	- 10000.0ms: 45
	- 30000.0ms: 45
	- +Infms: 45

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 40
- Event Batch Processing Average Time: 4ms
- Event Batch Processing distribution:
	- 500.0ms: 40
	- 1000.0ms: 40
	- 5000.0ms: 40
	- 10000.0ms: 40
	- 30000.0ms: 40
	- +Infms: 40

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 322
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 311
	- 1000.0ms: 321
	- 5000.0ms: 322
	- 10000.0ms: 322
	- 30000.0ms: 322
	- +Infms: 322

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 106s

### Event Batch Processing

- Event Batch Total: 1433
- Event Batch Processing Average Time: 21ms
- Event Batch Processing distribution:
	- 500.0ms: 1409
	- 1000.0ms: 1420
	- 5000.0ms: 1433
	- 10000.0ms: 1433
	- 30000.0ms: 1433
	- +Infms: 1433

### NGINX Error Logs
