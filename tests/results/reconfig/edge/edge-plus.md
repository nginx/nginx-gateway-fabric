# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: dbebd9791cb7aa5e8d10735800f776fd516b06c3
- Date: 2025-12-02T17:38:16Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1201000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 31s

### Event Batch Processing

- Event Batch Total: 51
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 51
	- 1000.0ms: 51
	- 5000.0ms: 51
	- 10000.0ms: 51
	- 30000.0ms: 51
	- +Infms: 51

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 41
- Event Batch Processing Average Time: 4ms
- Event Batch Processing distribution:
	- 500.0ms: 41
	- 1000.0ms: 41
	- 5000.0ms: 41
	- 10000.0ms: 41
	- 30000.0ms: 41
	- +Infms: 41

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 21s

### Event Batch Processing

- Event Batch Total: 326
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 315
	- 1000.0ms: 325
	- 5000.0ms: 326
	- 10000.0ms: 326
	- 30000.0ms: 326
	- +Infms: 326

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 115s

### Event Batch Processing

- Event Batch Total: 1469
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 1442
	- 1000.0ms: 1458
	- 5000.0ms: 1469
	- 10000.0ms: 1469
	- 30000.0ms: 1469
	- +Infms: 1469

### NGINX Error Logs
