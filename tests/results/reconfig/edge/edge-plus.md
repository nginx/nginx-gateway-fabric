# Results

## Test environment

NGINX Plus: true

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

- TimeToReadyTotal: 2s
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
- Event Batch Processing Average Time: 59ms
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

- TimeToReadyTotal: 2s
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

- TimeToReadyTotal: 7s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 46
- NGINX Reload Average Time: 147ms
- Reload distribution:
	- 500.0ms: 46
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

### Event Batch Processing

- Event Batch Total: 320
- Event Batch Processing Average Time: 25ms
- Event Batch Processing distribution:
	- 500.0ms: 320
	- 1000.0ms: 320
	- 5000.0ms: 320
	- 10000.0ms: 320
	- 30000.0ms: 320
	- +Infms: 320

### NGINX Error Logs


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 25s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 142
- NGINX Reload Average Time: 150ms
- Reload distribution:
	- 500.0ms: 142
	- 1000.0ms: 142
	- 5000.0ms: 142
	- 10000.0ms: 142
	- 30000.0ms: 142
	- +Infms: 142

### Event Batch Processing

- Event Batch Total: 1495
- Event Batch Processing Average Time: 17ms
- Event Batch Processing distribution:
	- 500.0ms: 1495
	- 1000.0ms: 1495
	- 5000.0ms: 1495
	- 10000.0ms: 1495
	- 30000.0ms: 1495
	- +Infms: 1495

### NGINX Error Logs
2025/03/01 18:17:34 [crit] 45#45: pread() read only 0 of 48 from "/var/lib/nginx/state/nginx-mgmt-state"


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 45
- NGINX Reload Average Time: 129ms
- Reload distribution:
	- 500.0ms: 45
	- 1000.0ms: 45
	- 5000.0ms: 45
	- 10000.0ms: 45
	- 30000.0ms: 45
	- +Infms: 45

### Event Batch Processing

- Event Batch Total: 294
- Event Batch Processing Average Time: 28ms
- Event Batch Processing distribution:
	- 500.0ms: 293
	- 1000.0ms: 294
	- 5000.0ms: 294
	- 10000.0ms: 294
	- 30000.0ms: 294
	- +Infms: 294

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: -15s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 184
- NGINX Reload Average Time: 142ms
- Reload distribution:
	- 500.0ms: 184
	- 1000.0ms: 184
	- 5000.0ms: 184
	- 10000.0ms: 184
	- 30000.0ms: 184
	- +Infms: 184

### Event Batch Processing

- Event Batch Total: 1434
- Event Batch Processing Average Time: 24ms
- Event Batch Processing distribution:
	- 500.0ms: 1433
	- 1000.0ms: 1434
	- 5000.0ms: 1434
	- 10000.0ms: 1434
	- 30000.0ms: 1434
	- +Infms: 1434

### NGINX Error Logs
2025/03/01 18:54:10 [emerg] 45#45: invalid instance state file "/var/lib/nginx/state/nginx-mgmt-state"

