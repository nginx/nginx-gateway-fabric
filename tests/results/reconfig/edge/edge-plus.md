# Results

## Test environment

NGINX Plus: true

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
- NGINX Reload Average Time: 126ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 58ms
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
- NGINX Reload Average Time: 138ms
- Reload distribution:
	- 500.0ms: 2
	- 1000.0ms: 2
	- 5000.0ms: 2
	- 10000.0ms: 2
	- 30000.0ms: 2
	- +Infms: 2

### Event Batch Processing

- Event Batch Total: 6
- Event Batch Processing Average Time: 62ms
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
- NGINX Reload Average Time: 152ms
- Reload distribution:
	- 500.0ms: 46
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

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


## Test 2: Start NGF, deploy Gateway, create many resources attached to GW - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: 44s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 242
- NGINX Reload Average Time: 155ms
- Reload distribution:
	- 500.0ms: 242
	- 1000.0ms: 242
	- 5000.0ms: 242
	- 10000.0ms: 242
	- 30000.0ms: 242
	- +Infms: 242

### Event Batch Processing

- Event Batch Total: 1592
- Event Batch Processing Average Time: 27ms
- Event Batch Processing distribution:
	- 500.0ms: 1592
	- 1000.0ms: 1592
	- 5000.0ms: 1592
	- 10000.0ms: 1592
	- 30000.0ms: 1592
	- +Infms: 1592

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 30

### Reloads and Time to Ready

- TimeToReadyTotal: < 1s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 46
- NGINX Reload Average Time: 149ms
- Reload distribution:
	- 500.0ms: 46
	- 1000.0ms: 46
	- 5000.0ms: 46
	- 10000.0ms: 46
	- 30000.0ms: 46
	- +Infms: 46

### Event Batch Processing

- Event Batch Total: 286
- Event Batch Processing Average Time: 29ms
- Event Batch Processing distribution:
	- 500.0ms: 286
	- 1000.0ms: 286
	- 5000.0ms: 286
	- 10000.0ms: 286
	- 30000.0ms: 286
	- +Infms: 286

### NGINX Error Logs


## Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway - NumResources 150

### Reloads and Time to Ready

- TimeToReadyTotal: -20s
- TimeToReadyAvgSingle: < 1s
- NGINX Reloads: 165
- NGINX Reload Average Time: 151ms
- Reload distribution:
	- 500.0ms: 165
	- 1000.0ms: 165
	- 5000.0ms: 165
	- 10000.0ms: 165
	- 30000.0ms: 165
	- +Infms: 165

### Event Batch Processing

- Event Batch Total: 1375
- Event Batch Processing Average Time: 22ms
- Event Batch Processing distribution:
	- 500.0ms: 1375
	- 1000.0ms: 1375
	- 5000.0ms: 1375
	- 10000.0ms: 1375
	- 30000.0ms: 1375
	- +Infms: 1375

### NGINX Error Logs
2025/01/16 10:02:48 [emerg] 44#44: invalid instance state file "/var/lib/nginx/state/nginx-mgmt-state"
