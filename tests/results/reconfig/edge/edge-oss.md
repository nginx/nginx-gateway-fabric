# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: d7571435664996802806309dbc413621df9d16fc
- Date: 2025-09-12T10:12:30Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.3-gke.1136000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 16s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 3ms
- Event Batch Processing distribution:
	- 500.0ms: 9
	- 1000.0ms: 9
	- 5000.0ms: 9
	- 10000.0ms: 9
	- 30000.0ms: 9
	- +Infms: 9

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 11
- Event Batch Processing Average Time: 8ms
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
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 256
- Event Batch Processing Average Time: 33ms
- Event Batch Processing distribution:
	- 500.0ms: 245
	- 1000.0ms: 256
	- 5000.0ms: 256
	- 10000.0ms: 256
	- 30000.0ms: 256
	- +Infms: 256

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 131s

### Event Batch Processing

- Event Batch Total: 1286
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 1246
	- 1000.0ms: 1286
	- 5000.0ms: 1286
	- 10000.0ms: 1286
	- 30000.0ms: 1286
	- +Infms: 1286

### NGINX Error Logs
