# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: ea67c576e39284e758352dda72242024cc98fa7f
- Date: 2026-10-01T15:28:08Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.35.8-gke.1225000
- vCPUs per node: 16
- RAM per node: 65848284Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 19s

### Event Batch Processing

- Event Batch Total: 18
- Event Batch Processing Average Time: 16ms
- Event Batch Processing distribution:
	- 500.0ms: 18
	- 1000.0ms: 18
	- 5000.0ms: 18
	- 10000.0ms: 18
	- 30000.0ms: 18
	- +Infms: 18

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 46s

### Event Batch Processing

- Event Batch Total: 20
- Event Batch Processing Average Time: 8ms
- Event Batch Processing distribution:
	- 500.0ms: 20
	- 1000.0ms: 20
	- 5000.0ms: 20
	- 10000.0ms: 20
	- 30000.0ms: 20
	- +Infms: 20

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 22s

### Event Batch Processing

- Event Batch Total: 303
- Event Batch Processing Average Time: 28ms
- Event Batch Processing distribution:
	- 500.0ms: 292
	- 1000.0ms: 303
	- 5000.0ms: 303
	- 10000.0ms: 303
	- 30000.0ms: 303
	- +Infms: 303

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 125s

### Event Batch Processing

- Event Batch Total: 1445
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1412
	- 1000.0ms: 1438
	- 5000.0ms: 1445
	- 10000.0ms: 1445
	- 30000.0ms: 1445
	- +Infms: 1445

### NGINX Error Logs
