# gRPC Routing Example

In this example, we deploy NGINX Gateway Fabric, a simple gRPC web application, and then configure NGINX Gateway Fabric
to route traffic to that application using GRPCRoute resources.

## Running the Example

## 1. Deploy NGINX Gateway Fabric

1. Follow the [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/installation/) to deploy NGINX Gateway Fabric.

1. Save the public IP address of NGINX Gateway Fabric into a shell variable:

    ```text
    GW_IP=XXX.YYY.ZZZ.III
    ```

1. Save the port of NGINX Gateway Fabric:

    ```text
    GW_PORT=<port number>
    ```

## 2. Deploy the Helloworld Application

1. Create the two helloworld Deployments and Services:

    ```shell
    kubectl apply -f helloworld.yaml
    ```

1. Check that the Pods are running in the `default` Namespace:

    ```shell
    kubectl -n default get pods
    ```

    ```text
    NAME                      READY   STATUS    RESTARTS   AGE
    grpc-infra-backend-v1-766c7d6788-rg92p   1/1     Running   0          12s
    grpc-infra-backend-v2-546f7c8d48-mjkkx   1/1     Running   0          12s
    ```

   Save these pod names into variables:

    ```text
    POD_V1=<grpc-infra-backend-v1-xxxxxxxxxx-xxxxx>
    POD_V2=<grpc-infra-backend-v2-xxxxxxxxxx-xxxxx>
    ```

## 3. Configure Routing

There are 3 options to configure gRPC routing. To access the application and test the routing rules, we will use [grpcurl](https://github.com/fullstorydev/grpcurl?tab=readme-ov-file#installation).

### 3a. Configure exact method matching based routing

1. Create the Gateway and GRPCRoute resources:

    ```shell
    kubectl apply -f exact-method.yaml
    ```

2. Test the Application:

    ```shell
    grpcurl -plaintext -proto grpc.proto -authority bar.com -d '{"name": "exact"}' ${GW_IP}:${GW_PORT} helloworld.Greeter/SayHello
    ```

    ```text
    {
        "message": "Hello exact"
    }
    ```

3. Clean up the Gateway and GRPCRoute resources:

    ```shell
    kubectl delete -f exact-method.yaml
    ```

### 3b. Configure hostname based routing

1. Create the Gateway and GRPCRoute resources:

    ```shell
    kubectl apply -f hostname.yaml
    ```

2. Test the Application:

    ```shell
    grpcurl -plaintext -proto grpc.proto -authority bar.com -d '{"name": "bar server"}' ${GW_IP}:${GW_PORT} helloworld.Greeter/SayHello
    ```

    ```text
    {
        "message": "Hello bar server"
    }
    ```

   To make sure this came from the correct server, we can check the application server logs:

    ```shell
    kubectl logs ${POD_V1}
    ```

    ```text
    2024/04/29 09:26:54 server listening at [::]:50051
    2024/04/29 09:28:54 Received: bar server
    ```

   Now we'll send a request to `foo.bar.com`

    ```shell
    grpcurl -plaintext -proto grpc.proto -authority foo.bar.com -d '{"name": "foo bar server"}' ${GW_IP}:${GW_PORT} helloworld.Greeter/SayHello
    ```

    ```text
    {
        "message": "Hello foo bar server"
    }
    ```

   This time, we'll check the POD_V2 logs:

    ```shell
    kubectl logs ${POD_V2}
    ```

    ```text
    2024/04/29 09:26:55 server listening at [::]:50051
    2024/04/29 09:29:46 Received: foo bar server
    ```

3. Clean up the Gateway and GRPCRoute resources:

    ```shell
    kubectl delete -f hostname.yaml
    ```

### 3c. Configure headers based routing

1. Create the Gateway and GRPCRoute resources:

    ```shell
    kubectl apply -f headers.yaml
    ```

2. Test the Application:

    ```shell
    grpcurl -plaintext -proto grpc.proto -authority bar.com -d '{"name": "version one"}' -H 'version: one' ${GW_IP}:${GW_PORT} helloworld.Greeter/SayHello
    ```

    ```text
    {
        "message": "Hello version one"
    }
    ```

   To make sure this came from the correct server, we can check the application server logs:

    ```shell
    kubectl logs ${POD_V1}
    ```

    ```text
    <...>
    2024/04/29 09:30:27 Received: version one
    ```

   Now we'll send a request with the header `version: two`

    ```shell
    grpcurl -plaintext -proto grpc.proto -authority bar.com -d '{"name": "version two"}' -H 'version: two' ${GW_IP}:${GW_PORT} helloworld.Greeter/SayHello
    ```

    ```text
    {
        "message": "Hello version two"
    }
    ```

   This time, we'll check the POD_V2 logs:

    ```shell
    kubectl logs ${POD_V2}
    ```

    ```text
    <...>
    2024/04/29 09:32:46 Received: version two
    ```

   Finally, we'll send a request with the headers `version: two` and `color: orange`

    ```shell
    grpcurl -plaintext -proto grpc.proto -authority bar.com -d '{"name": "version two orange"}' -H 'version: two' -H 'color: orange' ${GW_IP}:${GW_PORT} helloworld.Greeter/SayHello
    ```

   ```text
   {
     "message": "Hello version two orange"
   }
   ```

   Now check the POD_V1 logs again:

   ```shell
   kubectl logs ${POD_V1}
   ```

   ```text
   <...>
   2024/04/29 09:30:27 Received: version one
   2024/04/29 09:33:26 Received: version two orange
   ```

3. Clean up the Gateway and GRPCRoute resources:

   ```shell
   kubectl delete -f headers.yaml
   ```
