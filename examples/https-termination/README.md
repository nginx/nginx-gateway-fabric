# HTTPS Termination Example

In this example, we expand on the simple [cafe-example](../cafe-example) by adding HTTPS termination to our routes and
an HTTPS redirect from port 80 to 443. We will also show how you can use a ReferenceGrant to permit your Gateway to
reference a Secret in a different Namespace.

## Running the Example

## 1. Deploy NGINX Gateway Fabric

1. Follow the [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/installation/) to deploy NGINX Gateway Fabric.

## 2. Deploy the Cafe Application

1. Create the coffee and the tea Deployments and Services:

   ```shell
   kubectl apply -f cafe.yaml
   ```

1. Check that the Pods are running in the `default` namespace:

   ```shell
   kubectl -n default get pods
   ```

   ```text
   NAME                      READY   STATUS    RESTARTS   AGE
   coffee-6f4b79b975-2sb28   1/1     Running   0          12s
   tea-6fb46d899f-fm7zr      1/1     Running   0          12s
   ```

## 3. Configure HTTPS Termination and Routing

1. Create the Namespace `certificate` and a Secret with a TLS certificate and key:

   ```shell
   kubectl apply -f certificate-ns-and-cafe-secret.yaml
   ```

   The TLS certificate and key in this Secret are used to terminate the TLS connections for the cafe application.
   > **Important**: This certificate and key are for demo purposes only.

1. Create the ReferenceGrant:

   ```shell
   kubectl apply -f reference-grant.yaml
   ```

   This ReferenceGrant allows all Gateways in the `default` namespace to reference the `cafe-secret` Secret in
   the `certificate` Namespace.

1. Create the Gateway resource:

   ```shell
   kubectl apply -f gateway.yaml
   ```

   This [Gateway](./gateway.yaml) configures:
    - `http` listener for HTTP traffic
    - `https` listener for HTTPS traffic. It terminates TLS connections using the `cafe-secret` we created in step 1.

    After creating the Gateway resource, NGINX Gateway Fabric will provision an NGINX Pod and Service fronting it to route traffic.

    Save the public IP address and ports of the NGINX Service into shell variables:

     ```text
     GW_IP=XXX.YYY.ZZZ.III
     GW_HTTP_PORT=<http port number>
     GW_HTTPS_PORT=<https port number>
     ```

1. Create the HTTPRoute resources:

   ```shell
   kubectl apply -f cafe-routes.yaml
   ```

   To configure HTTPS termination for our cafe application, we will bind our `coffee` and `tea` HTTPRoutes to
   the `https` listener in [cafe-routes.yaml](./cafe-routes.yaml) using
   the [`parentReference`](https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1.ParentReference)
   field:

   ```yaml
   parentRefs:
   - name: gateway
     sectionName: https
   ```

   To configure an HTTPS redirect from port 80 to 443, we will bind the special `cafe-tls-redirect` HTTPRoute with
   a [`HTTPRequestRedirectFilter`](https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1.HTTPRequestRedirectFilter)
   to the `http` listener:

   ```yaml
   parentRefs:
   - name: gateway
     sectionName: http
   ```

## 4. Test the Application

To access the application, we will use `curl` to send requests to the `coffee` and `tea` Services. First, we will access
the application over HTTP to test that the HTTPS redirect works. Then we will use HTTPS.

### 4.1 Test HTTPS Redirect

To test that NGINX sends an HTTPS redirect, we will send requests to the `coffee` and `tea` Services on HTTP port. We
will use curl's `--include` option to print the response headers (we are interested in the `Location` header).

To get a redirect for coffee:

```shell
curl --resolve cafe.example.com:$GW_HTTP_PORT:$GW_IP http://cafe.example.com:$GW_HTTP_PORT/coffee --include
```

```text
HTTP/1.1 302 Moved Temporarily
...
Location: https://cafe.example.com/coffee
...
```

To get a redirect for tea:

```shell
curl --resolve cafe.example.com:$GW_HTTP_PORT:$GW_IP http://cafe.example.com:$GW_HTTP_PORT/tea --include
```

```text
HTTP/1.1 302 Moved Temporarily
...
Location: https://cafe.example.com/tea
...
```

### 4.2 Access Coffee and Tea

Now we will access the application over HTTPS. Since our certificate is self-signed, we will use curl's `--insecure`
option to turn off certificate verification.

To get coffee:

```shell
curl --resolve cafe.example.com:$GW_HTTPS_PORT:$GW_IP https://cafe.example.com:$GW_HTTPS_PORT/coffee --insecure
```

```text
Server address: 10.12.0.18:80
Server name: coffee-7586895968-r26zn
```

To get tea:

```shell
curl --resolve cafe.example.com:$GW_HTTPS_PORT:$GW_IP https://cafe.example.com:$GW_HTTPS_PORT/tea --insecure
```

```text
Server address: 10.12.0.19:80
Server name: tea-7cd44fcb4d-xfw2x
```

### 4.3 Remove the ReferenceGrant

To restrict access to the `cafe-secret` in the `certificate` Namespace, we can delete the ReferenceGrant we created in
Step 3:

```shell
kubectl delete -f reference-grant.yaml
```

Now, if we try to access the application over HTTPS, we will get a connection refused error:

```shell
curl --resolve cafe.example.com:$GW_HTTPS_PORT:$GW_IP https://cafe.example.com:$GW_HTTPS_PORT/coffee --insecure -vvv
```

```text
...
curl: (7) Failed to connect to cafe.example.com port 443 after 0 ms: Connection refused
```


You can also check the conditions of the Gateway `https` Listener to verify the that the reference is not permitted:

```shell
 kubectl describe gateway gateway
```

```text
 Name:                    https
 Conditions:
   Last Transition Time:  2023-06-26T20:23:56Z
   Message:               Certificate ref to secret certificate/cafe-secret not permitted by any ReferenceGrant
   Observed Generation:   1
   Reason:                RefNotPermitted
   Status:                False
   Type:                  Accepted
   Last Transition Time:  2023-06-26T20:23:56Z
   Message:               Certificate ref to secret certificate/cafe-secret not permitted by any ReferenceGrant
   Observed Generation:   1
   Reason:                RefNotPermitted
   Status:                False
   Type:                  ResolvedRefs
   Last Transition Time:  2023-06-26T20:23:56Z
   Message:               Certificate ref to secret certificate/cafe-secret not permitted by any ReferenceGrant
   Observed Generation:   1
   Reason:                Invalid
   Status:                False
   Type:                  Programmed
```
