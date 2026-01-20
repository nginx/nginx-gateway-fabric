# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: fe52764cce240bb5a3713f56aa113694c2793f93
- Date: 2026-01-20T16:40:22Z
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
- TimeToReadyTotal: 16s

### Event Batch Processing

- Event Batch Total: 49
- Event Batch Processing Average Time: 0ms
- Event Batch Processing distribution:
	- 500.0ms: 49
	- 1000.0ms: 49
	- 5000.0ms: 49
	- 10000.0ms: 49
	- 30000.0ms: 49
	- +Infms: 49

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 20s

### Event Batch Processing

- Event Batch Total: 58
- Event Batch Processing Average Time: 1ms
- Event Batch Processing distribution:
	- 500.0ms: 58
	- 1000.0ms: 58
	- 5000.0ms: 58
	- 10000.0ms: 58
	- 30000.0ms: 58
	- +Infms: 58

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 410
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 408
	- 1000.0ms: 410
	- 5000.0ms: 410
	- 10000.0ms: 410
	- 30000.0ms: 410
	- +Infms: 410

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 105s

### Event Batch Processing

- Event Batch Total: 1724
- Event Batch Processing Average Time: 15ms
- Event Batch Processing distribution:
	- 500.0ms: 1724
	- 1000.0ms: 1724
	- 5000.0ms: 1724
	- 10000.0ms: 1724
	- 30000.0ms: 1724
	- +Infms: 1724

### NGINX Error Logs
