# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 9155a2b6a8d3179165797ef3e789e97283f7a695
- Date: 2025-03-15T07:17:11Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.31.6-gke.1020000
- vCPUs per node: 16
- RAM per node: 65851340Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: 3s
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

- Event Batch Total: 5
- Event Batch Processing Average Time: 53ms
- Event Batch Processing distribution:
	- 500.0ms: 5
	- 1000.0ms: 5
	- 5000.0ms: 5
	- 10000.0ms: 5
	- 30000.0ms: 5
	- +Infms: 5

### NGINX Error Logs


## Test 1: Resources exist before startup - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 3s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 2
- NGINX Reload Average Time: 88ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 45ms
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

- Event Batch Total: 337
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 337
	- 1000.0ms: 337
	- 5000.0ms: 337
	- 10000.0ms: 337
	- 30000.0ms: 337
	- +Infms: 337

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 44s
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

- Event Batch Total: 1689
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 1689
	- 1000.0ms: 1689
	- 5000.0ms: 1689
	- 10000.0ms: 1689
	- 30000.0ms: 1689
	- +Infms: 1689

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 64
- NGINX Reload Average Time: 125ms
- Reload distribution:
	- 500.0ms: 64
	- 1000.0ms: 64
	- 5000.0ms: 64
	- 10000.0ms: 64
	- 30000.0ms: 64
	- +Infms: 64

### Event Batch Processing

- Event Batch Total: 321
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 321
	- 1000.0ms: 321
	- 5000.0ms: 321
	- 10000.0ms: 321
	- 30000.0ms: 321
	- +Infms: 321

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
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

- Event Batch Total: 1639
- Event Batch Processing Average Time: 26ms
- Event Batch Processing distribution:
	- 500.0ms: 1639
	- 1000.0ms: 1639
	- 5000.0ms: 1639
	- 10000.0ms: 1639
	- 30000.0ms: 1639
	- +Infms: 1639

### NGINX Error Logs
