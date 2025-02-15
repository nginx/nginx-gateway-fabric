# Results

## Test environment

NGINX Plus: true

NGINX Gateway Fabric:

- Commit: 9a7a618dab5ed0eee09063de60d80bf0fb76900a
- Date: 2025-02-14T18:44:35Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1023000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: 2s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 2
- NGINX Reload Average Time: 89ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 51ms
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
- NGINX Reloads: 2
- NGINX Reload Average Time: 101ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 54ms
- Event Batch Processing distribution:
	- 500.0ms: 6
	- 1000.0ms: 6
	- 5000.0ms: 6
	- 10000.0ms: 6
	- 30000.0ms: 6
	- +Infms: 6

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: 8s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 48
- NGINX Reload Average Time: 148ms
- Reload distribution:
	- 500.0ms: 48
	- 1000.0ms: 48
	- 5000.0ms: 48
	- 10000.0ms: 48
	- 30000.0ms: 48
	- +Infms: 48

### Event Batch Processing

- Event Batch Total: 323
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 323
	- 1000.0ms: 323
	- 5000.0ms: 323
	- 10000.0ms: 323
	- 30000.0ms: 323
	- +Infms: 323

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 44s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 249
- NGINX Reload Average Time: 150ms
- Reload distribution:
	- 500.0ms: 249
	- 1000.0ms: 249
	- 5000.0ms: 249
	- 10000.0ms: 249
	- 30000.0ms: 249
	- +Infms: 249

### Event Batch Processing

- Event Batch Total: 1599
- Event Batch Processing Average Time: 27ms
- Event Batch Processing distribution:
	- 500.0ms: 1599
	- 1000.0ms: 1599
	- 5000.0ms: 1599
	- 10000.0ms: 1599
	- 30000.0ms: 1599
	- +Infms: 1599

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 46
- NGINX Reload Average Time: 129ms
- Reload distribution:
	- 500.0ms: 46
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

### Event Batch Processing

- Event Batch Total: 288
- Event Batch Processing Average Time: 28ms
- Event Batch Processing distribution:
	- 500.0ms: 288
	- 1000.0ms: 288
	- 5000.0ms: 288
	- 10000.0ms: 288
	- 30000.0ms: 288
	- +Infms: 288

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 246
- NGINX Reload Average Time: 135ms
- Reload distribution:
	- 500.0ms: 246
	- 1000.0ms: 246
	- 5000.0ms: 246
	- 10000.0ms: 246
	- 30000.0ms: 246
	- +Infms: 246

### Event Batch Processing

- Event Batch Total: 1486
- Event Batch Processing Average Time: 29ms
- Event Batch Processing distribution:
	- 500.0ms: 1485
	- 1000.0ms: 1486
	- 5000.0ms: 1486
	- 10000.0ms: 1486
	- 30000.0ms: 1486
	- +Infms: 1486

### NGINX Error Logs

