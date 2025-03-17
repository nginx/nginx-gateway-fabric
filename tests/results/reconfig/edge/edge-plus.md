# Results

## Test environment

NGINX Plus: true

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

- TimeToReadyTotal: 4s
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
- Event Batch Processing Average Time: 52ms
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

- TimeToReadyTotal: 4s
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
- Event Batch Processing Average Time: 53ms
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
- NGINX Reloads: 47
- NGINX Reload Average Time: 148ms
- Reload distribution:
	- 500.0ms: 47
	- 1000.0ms: 47
	- 5000.0ms: 47
	- 10000.0ms: 47
	- 30000.0ms: 47
	- +Infms: 47

### Event Batch Processing

- Event Batch Total: 322
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 322
	- 1000.0ms: 322
	- 5000.0ms: 322
	- 10000.0ms: 322
	- 30000.0ms: 322
	- +Infms: 322

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 20s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 117
- NGINX Reload Average Time: 150ms
- Reload distribution:
	- 500.0ms: 117
	- 1000.0ms: 117
	- 5000.0ms: 117
	- 10000.0ms: 117
	- 30000.0ms: 117
	- +Infms: 117

### Event Batch Processing

- Event Batch Total: 1460
- Event Batch Processing Average Time: 14ms
- Event Batch Processing distribution:
	- 500.0ms: 1460
	- 1000.0ms: 1460
	- 5000.0ms: 1460
	- 10000.0ms: 1460
	- 30000.0ms: 1460
	- +Infms: 1460

### NGINX Error Logs
2025/03/15 17:00:26 [emerg] 48#48: invalid instance state file "/var/lib/nginx/state/nginx-mgmt-state"


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 46
- NGINX Reload Average Time: 133ms
- Reload distribution:
	- 500.0ms: 46
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

### Event Batch Processing

- Event Batch Total: 291
- Event Batch Processing Average Time: 28ms
- Event Batch Processing distribution:
	- 500.0ms: 291
	- 1000.0ms: 291
	- 5000.0ms: 291
	- 10000.0ms: 291
	- 30000.0ms: 291
	- +Infms: 291

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 258
- NGINX Reload Average Time: 132ms
- Reload distribution:
	- 500.0ms: 258
	- 1000.0ms: 258
	- 5000.0ms: 258
	- 10000.0ms: 258
	- 30000.0ms: 258
	- +Infms: 258

### Event Batch Processing

- Event Batch Total: 1501
- Event Batch Processing Average Time: 29ms
- Event Batch Processing distribution:
	- 500.0ms: 1501
	- 1000.0ms: 1501
	- 5000.0ms: 1501
	- 10000.0ms: 1501
	- 30000.0ms: 1501
	- +Infms: 1501

### NGINX Error Logs
