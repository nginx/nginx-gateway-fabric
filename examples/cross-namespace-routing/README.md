# Example

In this example, we expand on the simple [cafe-example](../cafe-example) by using a ReferenceGrant to route to backends
in a different namespace from our HTTPRoutes.

## Running the Example

## 1. Deploy NGINX Gateway Fabric

1. Follow the [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/install/) to deploy NGINX Gateway Fabric.

## 2. Deploy the Cafe Application

1. Create the cafe namespace and cafe application:

   ```shell
   kubectl apply -f cafe-ns-and-app.yaml
   ```

1. Check that the Pods are running in the `cafe` Namespace:

   ```shell
   kubectl -n cafe get pods
   ```

   ```text
   NAME                      READY   STATUS    RESTARTS   AGE
   coffee-6f4b79b975-2sb28   1/1     Running   0          12s
   tea-6fb46d899f-fm7zr      1/1     Running   0          12s
   ```

## 3. Configure Routing

1. Create the Gateway:

   ```shell
   kubectl apply -f gateway.yaml
   ```

    After creating the Gateway resource, NGINX Gateway Fabric will provision an NGINX Pod and Service fronting it to route traffic.

    Save the public IP address and port of the NGINX Service into shell variables:

    ```text
    GW_IP=XXX.YYY.ZZZ.III
    GW_PORT=<port number>
    ```

1. Create the HTTPRoute resources:

   ```shell
   kubectl apply -f cafe-routes.yaml
   ```

1. Create the ReferenceGrant:

   ```shell
   kubectl apply -f reference-grant.yaml
   ```

   This ReferenceGrant allows all HTTPRoutes in the `default` Namespace to reference all Services in the `cafe`
   Namespace.

## 4. Test the Application

To access the application, we will use `curl` to send requests to the `coffee` and `tea` Services.

To get coffee:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
```

```text
Server address: 10.12.0.18:80
Server name: coffee-7586895968-r26zn
```

To get tea:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
```

```text
Server address: 10.12.0.19:80
Server name: tea-7cd44fcb4d-xfw2x
```

## 5. Remove the ReferenceGrant

To restrict access to Services in the `cafe` Namespace, we can delete the ReferenceGrant we created in
Step 3:

```shell
kubectl delete -f reference-grant.yaml
```

Now, if we try to access the application over HTTP, we will get an internal server error:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
```

```text
<html>
<head><title>500 Internal Server Error</title></head>
<body>
<center><h1>500 Internal Server Error</h1></center>
<hr><center>nginx/1.25.1</center>
</body>
</html>
```

You can also check the conditions of the HTTPRoutes `coffee` and `tea` to verify that the reference is not permitted:

```shell
kubectl describe httproute coffee
```

```text
Condtions:
      Message:               Backend ref to Service cafe/coffee not permitted by any ReferenceGrant
      Observed Generation:   1
      Reason:                RefNotPermitted
      Status:                False
      Type:                  ResolvedRefs
      Controller Name:       gateway.nginx.org/nginx-gateway-controller
```

```shell
kubectl describe httproute tea
```

```text
Condtions:
      Message:               Backend ref to Service cafe/tea not permitted by any ReferenceGrant
      Observed Generation:   1
      Reason:                RefNotPermitted
      Status:                False
      Type:                  ResolvedRefs
      Controller Name:       gateway.nginx.org/nginx-gateway-controller
```
