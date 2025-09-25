# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 8241478604f782eca497329ae47507b978d117b1
- Date: 2025-09-24T18:19:40Z
- Dirty: false

GKE Cluster:

- Node count: 15
- k8s version: v1.33.4-gke.1134000
- vCPUs per node: 2
- RAM per node: 4015668Ki
- Max pods per node: 110
- Zone: us-east1-b
- Instance Type: e2-medium

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 23s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 12ms
- Event Batch Processing distribution:
	- 500.0ms: 10
	- 1000.0ms: 10
	- 5000.0ms: 10
	- 10000.0ms: 10
	- 30000.0ms: 10
	- +Infms: 10

### NGINX Error Logs

## Test 1: Resources exist before startup - NumResources 150

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 25s

### Event Batch Processing

- Event Batch Total: 9
- Event Batch Processing Average Time: 37ms
- Event Batch Processing distribution:
	- 500.0ms: 9
	- 1000.0ms: 9
	- 5000.0ms: 9
	- 10000.0ms: 9
	- 30000.0ms: 9
	- +Infms: 9

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 24s

### Event Batch Processing

- Event Batch Total: 272
- Event Batch Processing Average Time: 28ms
- Event Batch Processing distribution:
	- 500.0ms: 266
	- 1000.0ms: 272
	- 5000.0ms: 272
	- 10000.0ms: 272
	- 30000.0ms: 272
	- +Infms: 272

### NGINX Error Logs
2025/09/25 03:10:51 [emerg] 8#8: unexpected end of file, expecting "}" in /etc/nginx/conf.d/http.conf:459
2025/09/25 03:10:56 [emerg] 8#8: unexpected end of file, expecting "}" in /etc/nginx/conf.d/http.conf:2433

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 123s

### Event Batch Processing

- Event Batch Total: 1315
- Event Batch Processing Average Time: 23ms
- Event Batch Processing distribution:
	- 500.0ms: 1277
	- 1000.0ms: 1314
	- 5000.0ms: 1315
	- 10000.0ms: 1315
	- 30000.0ms: 1315
	- +Infms: 1315

### NGINX Error Logs
2025/09/25 03:15:10 [emerg] 8#8: unexpected end of file, expecting "}" in /etc/nginx/conf.d/http.conf:1466
2025/09/25 03:15:14 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:3065
2025/09/25 03:15:15 [emerg] 8#8: unexpected end of file, expecting "}" in /etc/nginx/conf.d/http.conf:3560
2025/09/25 03:15:23 [emerg] 8#8: directive "upstream" has no opening "{" in /etc/nginx/conf.d/http.conf:7773
