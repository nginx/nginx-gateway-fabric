# Example

In this example we will deploy NGINX Gateway Fabric and configure traffic splitting for a simple cafe application.
We will use HTTPRoute resources to split traffic between two versions of the application -- `coffee-v1`
and `coffee-v2`.


## Running the Example

## 1. Deploy NGINX Gateway Fabric

1. Follow the [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/installation/) to deploy NGINX Gateway Fabric.

## 2. Deploy the Coffee Application

1. Create the Cafe Deployments and Services:

   ```shell
   kubectl apply -f cafe.yaml
   ```

1. Check that the Pods are running in the `default` Namespace:

   ```shell
   kubectl -n default get pods
   ```

   ```text
   NAME                         READY   STATUS    RESTARTS   AGE
   coffee-v1-7c57c576b-rfjsh    1/1     Running   0          21m
   coffee-v2-698f66dc46-vcb6r   1/1     Running   0          21m
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
   kubectl apply -f cafe-route.yaml
   ```

This HTTPRoute resource defines a route for the path `/coffee` that sends 80% of the requests to `coffee-v1` and 20%
to `coffee-v2`. In this example, we use 80 and 20; however, the weights are calculated proportionally and do not need to
sum to 100. For example, the weights of 8 and 2, 16 and 4, or 32 and 8 all evaluate to the same relative proportions.

## 4. Test the Application

To access the application, we will use `curl` to send requests to `/coffee`:

```shell
curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
```

80% of the responses will come from `coffee-v1`:

```text
Server address: 10.12.0.18:80
Server name: coffee-v1-7c57c576b-rfjsh
```

20% of the responses will come from `coffee-v2`:

```text
Server address: 10.12.0.19:80
Server name: coffee-v2-698f66dc46-vcb6r
```

### 5. Modify the Traffic Split Configuration

Let's shift more of the traffic to `coffee-v2`. To do this we will update the HTTPRoute resource and change the weight
of the `coffee-v2` backend to 80. Backends with equal weights will receive an equal share of traffic.

1. Apply the updated HTTPRoute resource:

   ```shell
   kubectl apply -f cafe-route-equal-weight.yaml
   ```

2. Test the application again:

   ```shell
   curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
   ```

The responses will now be split evenly between `coffee-v1` and `coffee-v2`.

We can continue modifying the weights of the backends to shift more and more traffic to `coffee-v2`. If there's an issue
with `coffee-v2`, we can quickly shift traffic back to `coffee-v1`.
