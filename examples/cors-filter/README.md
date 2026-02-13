# CORS Filter Example

This example demonstrates how to use the HTTPCORSFilter in NGINX Gateway Fabric to handle Cross-Origin Resource Sharing (CORS) for your applications.
CORS is a mechanism that allows restricted resources on a web page to be requested from another domain outside the domain from which the first resource was served. The HTTPCORSFilter in Gateway API provides a standard way to configure CORS policies.

## Running the Example

## 1. Deploy NGINX Gateway Fabric

1. Follow the [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/install/) to deploy NGINX Gateway Fabric.

## 2. Deploy the Cafe Application

1. Deploy the cafe application:

   ```shell
   kubectl apply -f cafe.yaml
   ```

2. Check that the Pods are running in the `default` Namespace:

   ```shell
   kubectl -n default get pods
   ```

   ```text
    NAME                      READY   STATUS    RESTARTS   AGE
    coffee-654ddf664b-fzzrf   1/1     Running   0          5s
   ```

## 3. Configure Routing

1. Create the gateway:

   ```shell
   kubectl apply -f gateway.yaml
   ```

    After creating the Gateway resource, NGINX Gateway Fabric will provision an NGINX Pod and Service fronting it to route traffic.

    Save the public IP address and port of the NGINX Service into shell variables:

    ```text
    GW_IP=XXX.YYY.ZZZ.III
    GW_PORT=<port number>
    ```

2. Create the HTTPRoute with CORS filter:

   ```shell
   kubectl apply -f cors-route.yaml
   ```

## 4. Test the Application

To access the application, we will use `curl` to send requests to the `coffee` and `tea` Services.

To get coffee:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee -v
```

```text
> GET /coffee HTTP/1.1
> Host: cafe.example.com:8080
> User-Agent: curl/8.7.1
> Accept: */*
>
* Request completely sent off
< HTTP/1.1 200 OK
< Server: nginx
< Date: Tue, 10 Feb 2026 14:20:10 GMT
< Content-Type: text/plain
< Content-Length: 162
< Connection: keep-alive
< Expires: Tue, 10 Feb 2026 14:20:09 GMT
< Cache-Control: no-cache
< Access-Control-Allow-Origin: *
< Access-Control-Allow-Methods: GET, POST
< Access-Control-Allow-Headers: Content-Type
< Access-Control-Allow-Credentials: true
< Access-Control-Max-Age: 3600
```

## Clean up

```shell
kubectl delete -f cors-route.yaml
kubectl delete -f cafe.yaml
kubectl delete -f gateway.yaml
```
