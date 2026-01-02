# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 89aee48bf6e660a828ffd32ca35fc7f52e358e00
- Date: 2025-12-12T20:04:38Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.5-gke.1308000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 11s

### Event Batch Processing

- Event Batch Total: 46
- Event Batch Processing Average Time: 0ms
- Event Batch Processing distribution:
	- 500.0ms: 46
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 20s

### Event Batch Processing

- Event Batch Total: 55
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 55
	- 1000.0ms: 55
	- 5000.0ms: 55
	- 10000.0ms: 55
	- 30000.0ms: 55
	- +Infms: 55

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 378
- Event Batch Processing Average Time: 18ms
- Event Batch Processing distribution:
	- 500.0ms: 373
	- 1000.0ms: 378
	- 5000.0ms: 378
	- 10000.0ms: 378
	- 30000.0ms: 378
	- +Infms: 378

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 133s

### Event Batch Processing

- Event Batch Total: 1861
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 1858
	- 1000.0ms: 1861
	- 5000.0ms: 1861
	- 10000.0ms: 1861
	- 30000.0ms: 1861
	- +Infms: 1861

### NGINX Error Logs
