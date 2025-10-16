# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 9010072ecd34a8fa99bfdd3d7580c9d725fb063e
- Date: 2025-10-01T09:39:27Z
- Dirty: false

GKE Cluster:

- Node count: 12
- k8s version: v1.33.4-gke.1172000
- vCPUs per node: 16
- RAM per node: 65851524Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Test 1: Resources exist before startup - NumResources 30

### Time to Ready

Time To Ready Description: From when NGF starts to when the NGINX configuration is fully configured
- TimeToReadyTotal: 11s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 2ms
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
- TimeToReadyTotal: 30s

### Event Batch Processing

- Event Batch Total: 10
- Event Batch Processing Average Time: 6ms
- Event Batch Processing distribution:
	- 500.0ms: 10
	- 1000.0ms: 10
	- 5000.0ms: 10
	- 10000.0ms: 10
	- 30000.0ms: 10
	- +Infms: 10

### NGINX Error Logs

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 30

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 29s

### Event Batch Processing

- Event Batch Total: 347
- Event Batch Processing Average Time: 19ms
- Event Batch Processing distribution:
	- 500.0ms: 343
	- 1000.0ms: 347
	- 5000.0ms: 347
	- 10000.0ms: 347
	- 30000.0ms: 347
	- +Infms: 347

### NGINX Error Logs
2025/10/01 17:53:44 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:1559
2025/10/01 17:53:45 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:1942
2025/10/01 17:53:46 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:2418
2025/10/01 17:53:46 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:2482

## Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, create many resources attached to GW - NumResources 150

### Time to Ready

Time To Ready Description: From when NGINX receives the first configuration created by NGF to when the NGINX configuration is fully configured
- TimeToReadyTotal: 134s

### Event Batch Processing

- Event Batch Total: 1654
- Event Batch Processing Average Time: 19ms
- Event Batch Processing distribution:
	- 500.0ms: 1651
	- 1000.0ms: 1654
	- 5000.0ms: 1654
	- 10000.0ms: 1654
	- 30000.0ms: 1654
	- +Infms: 1654

### NGINX Error Logs
2025/10/01 17:58:09 [emerg] 8#8: unexpected "$" in /etc/nginx/conf.d/http.conf:158
2025/10/01 17:58:20 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:5610
2025/10/01 17:58:21 [emerg] 8#8: pread() returned only 0 bytes instead of 4088 in /etc/nginx/conf.d/http.conf:5103
2025/10/01 17:58:23 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:7480
2025/10/01 17:58:24 [emerg] 8#8: pread() returned only 0 bytes instead of 4092 in /etc/nginx/conf.d/http.conf:2185
2025/10/01 17:58:25 [emerg] 8#8: pread() returned only 0 bytes instead of 4092 in /etc/nginx/conf.d/http.conf:1566
2025/10/01 17:58:27 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:9656
2025/10/01 17:58:28 [emerg] 8#8: pread() returned only 0 bytes instead of 4073 in /etc/nginx/conf.d/http.conf:2399
2025/10/01 17:58:29 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:10272
2025/10/01 17:58:31 [emerg] 8#8: pread() returned only 0 bytes instead of 4080 in /etc/nginx/conf.d/http.conf:9487
2025/10/01 17:58:32 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:12057
2025/10/01 17:58:33 [emerg] 8#8: pread() returned only 0 bytes instead of 4086 in /etc/nginx/conf.d/http.conf:4474
2025/10/01 17:58:36 [emerg] 8#8: pread() returned only 0 bytes instead of 4057 in /etc/nginx/conf.d/http.conf:4673
2025/10/01 17:58:37 [emerg] 8#8: pread() returned only 0 bytes instead of 4095 in /etc/nginx/conf.d/http.conf:11857
2025/10/01 17:58:39 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:16362
2025/10/01 17:58:40 [emerg] 8#8: pread() returned only 0 bytes instead of 4095 in /etc/nginx/conf.d/http.conf:1797
2025/10/01 17:58:41 [emerg] 8#8: pread() returned only 0 bytes instead of 4093 in /etc/nginx/conf.d/http.conf:2472
2025/10/01 17:58:42 [emerg] 8#8: pread() returned only 0 bytes instead of 4087 in /etc/nginx/conf.d/http.conf:15554
2025/10/01 17:58:42 [emerg] 8#8: unexpected end of file, expecting ";" or "}" in /etc/nginx/conf.d/http.conf:17671
