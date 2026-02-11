# Enable GatewayLink and F5 WAF in NGF

## Prerequisites

- Deploy CIS in your cluster
- Clone the NGF git repo and checkout `poc/gw-link-waf`
- Fetch and then edit the `deploy-plm-and-ngf.sh` script to match your environment:
    NAP_CHART_DIRECTORY_PATH -> Local directory for the NAP Helm chart to be untarred to
    NGF_CHART_PATH -> Local directory of the existing NGF Helm chart, e.g. ~/nginx-gateway-fabric/charts/nginx-gateway-fabric
    NGINX_IMAGE_REPO -> Private registry for the NGF NGINX with WAF image, must be accessible to the cluster
    NGINX_IMAGE_TAG -> Tag for the NGF NGINX with WAF image
    NGF_IMAGE_REPO -> Can change if required
    NGF_IMAGE_TAG -> Can change if required
    BUILD_AND_PUSH_NAP_IMAGES=false # Set to push the NAP images to a different registry if artifactory unavailable to the cluster
    NGF_NAP_REGISTRY= # Required if BUILD_AND_PUSH_NAP_IMAGES=true, must be accessible to the cluster
    NEED_CIS_CRD=false # Set to true if not testing full e2e with CIS
- Download required NGINX Plus licence JWT, cert, and key, and save them into `license.jwt`, `nginx-repo.crt` and `nginx-repo.key` respectively. Reach out if you need instructions on how to get these, general instructions are available in the [docs](https://docs.nginx.com/nginx-gateway-fabric/install/nginx-plus/#download-the-jwt-from-myf5).
- Update the `nginx-proxy-waf.yaml` file:
  - Update the `wafContainers` section with the images specified in the `deploy-plm-and-ngf.sh` script:

  ```yaml
    wafContainers:
    enforcer:
        image:
        repository: WAF_ENFORCER_IMAGE
        tag: WAF_IMAGE_TAG
    configManager:
        image:
        repository: WAF_CONFIG_MGR_IMAGE
        tag: WAF_IMAGE_TAG
  ```

  - Update the `gatewayLink` section to match your environment
  - Update the `service` type if required (e.g. NodePort instead of ClusterIP)
  - Update the `rewriteClientIP.trustedAddresses` if required

## Deploy the components

- Run the `deploy-plm-and-ngf.sh` script.
- The script will:
  - Pull, tag, and repush the NAP images if required
  - Fetch the NAP Helm chart anf untar it to the specified directory
  - Install PLM using the Helm chart to enable the in-cluster NAP Policy compilation
  - Build the NGF specific NGINX image with the dev build of the NAP module, and push it to your configured registry
  - Install NGF using Helm with the flags enabling the Big IP and WAF integrations

## Check all pods are running

```text
> k get pods -n nginx-gateway
NAME                                                  READY   STATUS    RESTARTS   AGE
nginx-gateway-nginx-gateway-fabric-6dc5bf7c47-f8fcb   1/1     Running   0          3h12m

```text
> k get pods -n ngf-nginx-app-protect
NAME                                                    READY   STATUS    RESTARTS        AGE
nginx-app-protect-compiler-service-5c849f65fc-vdszj     1/1     Running   0               6h41m
nginx-app-protect-policy-controller-575644d8bf-r6lz4    1/1     Running   0               6h35m
nginx-app-protect-redis-88596fcc5-8lqvq                 1/1     Running   0               6h41m
nginx-app-protect-seaweed-filer-0                       1/1     Running   2 (6h40m ago)   6h40m
nginx-app-protect-seaweed-master-0                      1/1     Running   2 (6h40m ago)   6h40m
nginx-app-protect-seaweed-volume-0                      1/1     Running   2 (6h40m ago)   6h40m
nginx-app-protect-seaweed-volume-1                      1/1     Running   2 (6h40m ago)   6h40m
nginx-app-protect-seaweed-volume-2                      1/1     Running   2 (6h39m ago)   6h40m
nginx-app-protect-seaweedfs-operator-7c5fb967b7-589b6   1/1     Running   0               6h41m
```

## Deploy the Gateway API, WAF, and NGF resources

Note: All kubectl commands are assumed to be ran in the root of the `nginx-gateway-fabric` repo.

1. Create the APPolicy. This will be picked up by the NAP Policy Controller, compiled, and published to in cluster storage.

    ```text
    k apply -f examples/waf-gwlink/ap-policy.yaml
    ```

2. Create the backend applications and services.

    ```text
    k apply -f examples/waf-gwlink/cafe.yaml
    ```

3. Create the NginxProxy resource. This adds the configuration to enable WAF and GatewayLink.

    ```text
    k apply -f examples/waf-gwlink/nginx-proxy-waf.yaml
    ```

4. Create the Gateway. This will set off the creation of the NGINX Deployment with the WAF sidecars, Service, and IngressLink.

    ```text
    k apply -f examples/waf-gwlink/gateway.yaml
    ```

5. Deploy the HTTPRoutes to configure NGINX to route traffic to our backend applications.

    ```text
    k apply -f examples/waf-gwlink/httproutes.yaml
    ```

6. Create the WAFGatewayBindingPolicy. This links the Gateway to the previously created APPolicy. Once accepted, NGF will fetch the compiled policy from the PLM in-cluster storage and configure the NGINX deployment to use it for WAF enforcement.

    ```text
    k apply -f examples/waf-gwlink/wafgatewaybindingpolicy.yaml
    ```

7. Check the status of the WAFGatewayBindingPolicy to ensure the Policy was accepted and the Dataplane has been configured:

    ```text
    k describe wafgatewaybindingpolicies.gateway.nginx.org gateway-base-protection
    Name:         gateway-base-protection
    Namespace:    default
    Labels:       <none>
    Annotations:  <none>
    API Version:  gateway.nginx.org/v1alpha1
    Kind:         WAFGatewayBindingPolicy
    Metadata:
    Creation Timestamp:  2026-02-10T15:11:20Z
    Generation:          1
    Resource Version:    1770746581548623022
    UID:                 8480ee44-3047-4cff-8bdf-efc1cdfca645
    Spec:
    Ap Policy Source:
        Name:  dataguard-blocking
    Target Refs:
        Group:  gateway.networking.k8s.io
        Kind:   Gateway
        Name:   gateway
    Status:
    Ancestors:
        Ancestor Ref:
        Group:      gateway.networking.k8s.io
        Kind:       Gateway
        Name:       gateway
        Namespace:  default
        Conditions:
        Last Transition Time:  2026-02-10T18:02:44Z
        Message:               The Policy is accepted
        Observed Generation:   1
        Reason:                Accepted
        Status:                True
        Type:                  Accepted
        Last Transition Time:  2026-02-10T18:02:44Z
        Message:               All references are resolved
        Observed Generation:   1
        Reason:                ResolvedRefs
        Status:                True
        Type:                  ResolvedRefs
        Last Transition Time:  2026-02-10T18:02:44Z
        Message:               Policy is programmed in the data plane
        Observed Generation:   1
        Reason:                Programmed
        Status:                True
        Type:                  Programmed
        Controller Name:         gateway.nginx.org/nginx-gateway-controller
    Events:                      <none>
    ```

8. Get the Gateway Address (will match the configuration from the GatewayLink) and save it and your configured port into variables

    ```text
    k get gateway gateway
    NAME      CLASS   ADDRESS      PROGRAMMED   AGE
    gateway   nginx   10.8.3.102   True         4h19m
    ```

    ```text
    GW_IP=10.8.3.102
    GW_PORT=80
    ```

9. Test traffic and WAF enforcement. The `coffee` backend application has been configured to return dummy credit card numbers and social security numbers to demonstrate the WAF enforcement.

    Tea works:

    ```text
    curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
    Server address: 10.12.2.16:8080
    Server name: tea-859766c68c-76qng
    Date: 10/Feb/2026:21:54:27 +0000
    URI: /tea
    Request ID: cb9db43f6aa1b35fe51b9c01cc92e8f4
    ```

    But coffee is blocked:

    ```text
    curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
    <html><head><title>Request Rejected</title></head><body>The requested URL was rejected. Please consult with your administrator.<br><br>Your support ID is: 10210489357486385403<br><br><a href='javascript:history.back();'>[Go Back]</a></body></html>
    ```
