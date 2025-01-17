# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: b5b8783c79a51c8ef46585249921f3642f563642
- Date: 2025-01-15T21:46:31Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.30.6-gke.1596000
- vCPUs per node: 16
- RAM per node: 65853984Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 2
- NGINX Reload Average Time: 113ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 49ms
- Event Batch Processing distribution:
	- 500.0ms: 6
	- 1000.0ms: 6
	- 5000.0ms: 6
	- 10000.0ms: 6
	- 30000.0ms: 6
	- +Infms: 6

### NGINX Error Logs


## Test 1: Resources exist before startup - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 3
- NGINX Reload Average Time: 135ms
- Reload distribution:
	- 500.0ms: 3
	- 1000.0ms: 3
	- 5000.0ms: 3
	- 10000.0ms: 3
	- 30000.0ms: 3
	- +Infms: 3

### Event Batch Processing

- Event Batch Total: 7
- Event Batch Processing Average Time: 67ms
- Event Batch Processing distribution:
	- 500.0ms: 7
	- 1000.0ms: 7
	- 5000.0ms: 7
	- 10000.0ms: 7
	- 30000.0ms: 7
	- +Infms: 7

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: 8s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 53
- NGINX Reload Average Time: 149ms
- Reload distribution:
	- 500.0ms: 53
	- 1000.0ms: 53
	- 5000.0ms: 53
	- 10000.0ms: 53
	- 30000.0ms: 53
	- +Infms: 53

### Event Batch Processing

- Event Batch Total: 328
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 328
	- 1000.0ms: 328
	- 5000.0ms: 328
	- 10000.0ms: 328
	- 30000.0ms: 328
	- +Infms: 328

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 44s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 288
- NGINX Reload Average Time: 150ms
- Reload distribution:
	- 500.0ms: 288
	- 1000.0ms: 288
	- 5000.0ms: 288
	- 10000.0ms: 288
	- 30000.0ms: 288
	- +Infms: 288

### Event Batch Processing

- Event Batch Total: 1642
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 1642
	- 1000.0ms: 1642
	- 5000.0ms: 1642
	- 10000.0ms: 1642
	- 30000.0ms: 1642
	- +Infms: 1642

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 61
- NGINX Reload Average Time: 131ms
- Reload distribution:
	- 500.0ms: 61
	- 1000.0ms: 61
	- 5000.0ms: 61
	- 10000.0ms: 61
	- 30000.0ms: 61
	- +Infms: 61

### Event Batch Processing

- Event Batch Total: 305
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 305
	- 1000.0ms: 305
	- 5000.0ms: 305
	- 10000.0ms: 305
	- 30000.0ms: 305
	- +Infms: 305

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 314
- NGINX Reload Average Time: 138ms
- Reload distribution:
	- 500.0ms: 314
	- 1000.0ms: 314
	- 5000.0ms: 314
	- 10000.0ms: 314
	- 30000.0ms: 314
	- +Infms: 314

### Event Batch Processing

- Event Batch Total: 1508
- Event Batch Processing Average Time: 29ms
- Event Batch Processing distribution:
	- 500.0ms: 1508
	- 1000.0ms: 1508
	- 5000.0ms: 1508
	- 10000.0ms: 1508
	- 30000.0ms: 1508
	- +Infms: 1508

### NGINX Error Logs
