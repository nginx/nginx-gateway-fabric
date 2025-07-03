1. Create a GKE cluster (NAP can't run on arm, we can't run locally using emulated images -> we need a remote cluster)
2. Build, tag, and push your images to a location accessible for your cluster
3. Deploy NGF using something like the following:
```
helm install nginx-gateway-fabric ./charts/nginx-gateway-fabric \
  --create-namespace \
  --namespace nginx-gateway \
  --set nginx.plus=true \
  --set nginx.usage.endpoint=<internal endpoint> \
  --set nginxGateway.image.repository=<gcr registry>/dev/nginx-gateway-fabric \
  --set nginxGateway.image.tag=test \
  --set nginx.image.repository=<gcr registry>/dev/nginx-gateway-fabric/nginx-plus-waf \
  --set nginx.image.tag=test \
  --set nginx.config.waf=enabled \
  --set nginx.wafContainers.enforcer.image.repository=<gcr registry>/dev/nginx-gateway-fabric/nap/waf-enforcer
  \--set nginx.wafContainers.configManager.image.repository=<gcr registry>/dev/nginx-gateway-fabric/nap/waf-config-mgr
  ```
4. Apply the file server yaml and upload the policies:

```
# Get pod name (for easier copying)
POD_NAME=$(kubectl get pod -l app=waf-policy-server -n nginx-gateway -o jsonpath='{.items[0].metadata.name}')

# Copy your policy files
cd policy_bundles
kubectl cp compiled_policy.tgz nginx-gateway/$POD_NAME:/usr/share/nginx/html/policies/policy-v1.tgz
kubectl cp compiled_policy.tgz.sha256 nginx-gateway/$POD_NAME:/usr/share/nginx/html/policies/policy-v1.tgz.sha256
kubectl cp strict_policy.tgz nginx-gateway/$POD_NAME:/usr/share/nginx/html/policies/strict-policy-v1.tgz
kubectl cp strict_policy.tgz.sha256 nginx-gateway/$POD_NAME:/usr/share/nginx/html/policies/strict-policy-v1.tgz.sha256

# Test it works
kubectl exec -n nginx-gateway $POD_NAME -- ls -la /usr/share/nginx/html/policies/
```

5. Apply the cafe.yaml file
6. Apply the wafpolicy.yaml file
7. Apply the gateway.yaml file
8. Apply the cafe-routes.yaml file
9. Check the status of the resources e.g.:

```
k describe httproutes.gateway.networking.k8s.io admin
Name:         admin
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  gateway.networking.k8s.io/v1
Kind:         HTTPRoute
Metadata:
  Creation Timestamp:  2025-06-25T16:12:29Z
  Generation:          1
  Resource Version:    1751550909980559024
  UID:                 b9c3f1a2-242c-42fb-a50a-cecac2741f10
Spec:
  Hostnames:
    cafe.example.com
  Parent Refs:
    Group:         gateway.networking.k8s.io
    Kind:          Gateway
    Name:          secure-gateway
    Section Name:  http
  Rules:
    Backend Refs:
      Group:   
      Kind:    Service
      Name:    admin
      Port:    80
      Weight:  1
    Matches:
      Path:
        Type:   PathPrefix
        Value:  /admin
Status:
  Parents:
    Conditions:
      Last Transition Time:  2025-07-03T13:55:09Z
      Message:               The route is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
      Last Transition Time:  2025-07-03T13:55:09Z
      Message:               All references are resolved
      Observed Generation:   1
      Reason:                ResolvedRefs
      Status:                True
      Type:                  ResolvedRefs
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
    Parent Ref:
      Group:         gateway.networking.k8s.io
      Kind:          Gateway
      Name:          secure-gateway
      Namespace:     default
      Section Name:  http
Events:              <none>
```
```
k describe wafpolicies.gateway.nginx.org httproute-protection-policy
Name:         httproute-protection-policy
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  gateway.nginx.org/v1alpha1
Kind:         WAFPolicy
Metadata:
  Creation Timestamp:  2025-06-25T16:13:12Z
  Generation:          1
  Resource Version:    1750867996697487016
  UID:                 4f73be53-04f9-4fec-9477-3f76c483283c
Spec:
  Policy Source:
    File Location:  http://waf-policy-server.nginx-gateway/policies/strict-policy-v1.tgz
    Retry:
      Attempts:   5
      Backoff:    exponential
      Max Delay:  10m
    Timeout:      60s
    Validation:
      Methods:
        checksum
  Security Logs:
    Destination:
      Type:       stderr
    Log Profile:  log_all
    Name:         stderr-all-logging
  Target Ref:
    Group:  gateway.networking.k8s.io
    Kind:   HTTPRoute
    Name:   admin
Status:
  Ancestors:
    Ancestor Ref:
      Group:      gateway.networking.k8s.io
      Kind:       HTTPRoute
      Name:       admin
      Namespace:  default
    Conditions:
      Last Transition Time:  2025-06-25T16:13:16Z
      Message:               Policy is accepted
      Observed Generation:   1
      Reason:                Accepted
      Status:                True
      Type:                  Accepted
    Controller Name:         gateway.nginx.org/nginx-gateway-controller
Events:                      <none>
```
10. Check the curl commands work as expected

```
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/admin
Server address: 10.32.0.9:8080
Server name: admin-cc4c96df4-sdq8c
Date: 03/Jul/2025:14:18:15 +0000
URI: /admin
Request ID: eadf7d5be634870b8af7a6605c651c2b

curl -i -X POST \
  --resolve cafe.example.com:$GW_PORT:$GW_IP \
  http://cafe.example.com:$GW_PORT/tea \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?><root><unclosedTag></root>'
HTTP/1.1 200 OK
Content-Type: text/html; charset=utf-8
Connection: close
Cache-Control: no-cache
Pragma: no-cache
Content-Length: 247

<html><head><title>Request Rejected</title></head><body>The requested URL was rejected. Please consult with your administrator.<br><br>Your support ID is: 14972426219485154405<br><br><a href='javascript:history.back();'>[Go Back]</a></body></html>%
```
