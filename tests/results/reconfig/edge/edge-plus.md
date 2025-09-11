# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: fa83170c0d84a19087bf139dce5427dbea963534
- Date: 2025-09-11T14:58:06Z
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

- Event Batch Total: 8
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 8
	- 1000.0ms: 8
	- 5000.0ms: 8
	- 10000.0ms: 8
	- 30000.0ms: 8
	- +Infms: 8

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 31s

### Event Batch Processing

- Event Batch Total: 7
- Event Batch Processing Average Time: 110ms
- Event Batch Processing distribution:
	- 500.0ms: 7
	- 1000.0ms: 7
	- 5000.0ms: 7
	- 10000.0ms: 7
	- 30000.0ms: 7
	- +Infms: 7

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 249
- Event Batch Processing Average Time: 37ms
- Event Batch Processing distribution:
	- 500.0ms: 238
	- 1000.0ms: 247
	- 5000.0ms: 249
	- 10000.0ms: 249
	- 30000.0ms: 249
	- +Infms: 249

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 120s

### Event Batch Processing

- Event Batch Total: 1259
- Event Batch Processing Average Time: 31ms
- Event Batch Processing distribution:
	- 500.0ms: 1247
	- 1000.0ms: 1248
	- 5000.0ms: 1257
	- 10000.0ms: 1259
	- 30000.0ms: 1259
	- +Infms: 1259

### NGINX Error Logs
