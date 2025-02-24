# Results

## Test environment

NGINX Plus: false

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
- NGINX Reload Average Time: 114ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 47ms
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
- Event Batch Processing Average Time: 43ms
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
- NGINX Reloads: 63
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 63
	- 1000.0ms: 63
	- 5000.0ms: 63
	- 10000.0ms: 63
	- 30000.0ms: 63
	- +Infms: 63

### Event Batch Processing

- Event Batch Total: 339
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 339
	- 1000.0ms: 339
	- 5000.0ms: 339
	- 10000.0ms: 339
	- 30000.0ms: 339
	- +Infms: 339

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 44s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 342
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 342
	- 1000.0ms: 342
	- 5000.0ms: 342
	- 10000.0ms: 342
	- 30000.0ms: 342
	- +Infms: 342

### Event Batch Processing

- Event Batch Total: 1693
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 1693
	- 1000.0ms: 1693
	- 5000.0ms: 1693
	- 10000.0ms: 1693
	- 30000.0ms: 1693
	- +Infms: 1693

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 62
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 62
	- 1000.0ms: 62
	- 5000.0ms: 62
	- 10000.0ms: 62
	- 30000.0ms: 62
	- +Infms: 62

### Event Batch Processing

- Event Batch Total: 309
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 309
	- 1000.0ms: 309
	- 5000.0ms: 309
	- 10000.0ms: 309
	- 30000.0ms: 309
	- +Infms: 309

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 343
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 343
	- 1000.0ms: 343
	- 5000.0ms: 343
	- 10000.0ms: 343
	- 30000.0ms: 343
	- +Infms: 343

### Event Batch Processing

- Event Batch Total: 1573
- Event Batch Processing Average Time: 27ms
- Event Batch Processing distribution:
	- 500.0ms: 1573
	- 1000.0ms: 1573
	- 5000.0ms: 1573
	- 10000.0ms: 1573
	- 30000.0ms: 1573
	- +Infms: 1573

### NGINX Error Logs

