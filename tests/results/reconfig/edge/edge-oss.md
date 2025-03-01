# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: bfd685d3805042ac77865a9823104404a80b06b9
- Date: 2025-02-28T18:00:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.5-gke.1169000
- vCPUs per node: 16
- RAM per node: 65851368Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

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


## Test 1: Resources exist before startup - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 2
- NGINX Reload Average Time: 100ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 44ms
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

- TimeToReadyTotal: 7s
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
- NGINX Reloads: 341
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 341
	- 1000.0ms: 341
	- 5000.0ms: 341
	- 10000.0ms: 341
	- 30000.0ms: 341
	- +Infms: 341

### Event Batch Processing

- Event Batch Total: 1690
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 1690
	- 1000.0ms: 1690
	- 5000.0ms: 1690
	- 10000.0ms: 1690
	- 30000.0ms: 1690
	- +Infms: 1690

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
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

- Event Batch Total: 312
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 312
	- 1000.0ms: 312
	- 5000.0ms: 312
	- 10000.0ms: 312
	- 30000.0ms: 312
	- +Infms: 312

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 339
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 339
	- 1000.0ms: 339
	- 5000.0ms: 339
	- 10000.0ms: 339
	- 30000.0ms: 339
	- +Infms: 339

### Event Batch Processing

- Event Batch Total: 1584
- Event Batch Processing Average Time: 27ms
- Event Batch Processing distribution:
	- 500.0ms: 1584
	- 1000.0ms: 1584
	- 5000.0ms: 1584
	- 10000.0ms: 1584
	- 30000.0ms: 1584
	- +Infms: 1584

### NGINX Error Logs

