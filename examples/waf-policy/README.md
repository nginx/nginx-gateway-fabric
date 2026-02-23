# Deploy PLM and NGF and protect traffic with NAP Policy

## Variables - update to your env

```bash
REGISTRY_ROOT=my-registry
ARTIFACTORY_HOST=my.artifactory.com
NGF_IMAGE_TAG=plm-nap
PLM_NS=plm
PLM_HELM_INSTALL_NAME=my-plm
NGF_NAP_REGISTRY=${REGISTRY_ROOT}/nap

export APP_PROTECT_VERSION=5.591.0-r1
WAF_CONFIG_MGR_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/config-mgr/ubuntu/jammy"
WAF_ENFORCER_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/enforcer/ubuntu/jammy"
WAF_COMPILER_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/nap-x-compiler-image"
WAF_CONTROLLER_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/policy-controller"
WAF_REDIS_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/waf-redis"
WAF_SEAWEEDFS_OPERATOR_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/seaweedfs-operator"
WAF_IMAGE_TAG=v11.625.0
WAF_CONTROLLER_IMAGE_TAG=v2.65.0
WAF_SEAWEEDFS_OPERATOR_IMAGE_TAG=v1.9.1
WAF_REDIS_IMAGE_TAG=8.4.0
WAF_SEAWEEDFS_OPERATOR_CHART_VERSION=5.9.0
WAF_SEAWEEDFS_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/seaweedfs"
WAF_SEAWEEDFS_IMAGE_TAG=v1.1.0
```

## Build and push NAP images to your own registry if needed

```bash
docker pull ${WAF_CONTROLLER_IMAGE}:${WAF_CONTROLLER_IMAGE_TAG}
docker pull $WAF_SEAWEEDFS_IMAGE:$WAF_SEAWEEDFS_IMAGE_TAG
docker pull ${WAF_SEAWEEDFS_OPERATOR_IMAGE}:${WAF_SEAWEEDFS_OPERATOR_IMAGE_TAG}
docker pull ${WAF_REDIS_IMAGE}:${WAF_REDIS_IMAGE_TAG}
docker pull ${WAF_COMPILER_IMAGE}:${WAF_IMAGE_TAG}
docker pull ${WAF_ENFORCER_IMAGE}:${WAF_IMAGE_TAG}
docker pull ${WAF_CONFIG_MGR_IMAGE}:${WAF_IMAGE_TAG}

docker tag ${WAF_CONTROLLER_IMAGE}:${WAF_CONTROLLER_IMAGE_TAG} ${NGF_NAP_REGISTRY}/policy-controller:${WAF_CONTROLLER_IMAGE_TAG}
docker tag ${WAF_COMPILER_IMAGE}:${WAF_IMAGE_TAG} ${NGF_NAP_REGISTRY}/nap-x-compiler-image:${WAF_IMAGE_TAG}
docker tag ${WAF_ENFORCER_IMAGE}:${WAF_IMAGE_TAG} ${NGF_NAP_REGISTRY}/waf-enforcer:${WAF_IMAGE_TAG}
docker tag ${WAF_CONFIG_MGR_IMAGE}:${WAF_IMAGE_TAG} ${NGF_NAP_REGISTRY}/config-mgr:${WAF_IMAGE_TAG}

docker push ${NGF_NAP_REGISTRY}/policy-controller:${WAF_CONTROLLER_IMAGE_TAG}
docker push ${NGF_NAP_REGISTRY}/nap-x-compiler-image:${WAF_IMAGE_TAG}
docker push ${NGF_NAP_REGISTRY}/waf-enforcer:${WAF_IMAGE_TAG}
docker push ${NGF_NAP_REGISTRY}/config-mgr:${WAF_IMAGE_TAG}

WAF_CONTROLLER_IMAGE=${NGF_NAP_REGISTRY}/policy-controller
WAF_COMPILER_IMAGE=${NGF_NAP_REGISTRY}/nap-x-compiler-image
WAF_REDIS_IMAGE=${NGF_NAP_REGISTRY}/waf-redis
WAF_SEAWEEDFS_OPERATOR_IMAGE=${NGF_NAP_REGISTRY}/seaweed-operator
WAF_SEAWEEDFS_IMAGE=${NGF_NAP_REGISTRY}/seaweedfs
```

## Deploy PLM

```bash
helm repo add f5-nap-helm https://${ARTIFACTORY_HOST}/artifactory/api/helm/f5-nap-helm
helm repo update
helm pull f5-nap-helm/f5-waf-policy-controller --version 2.65.0 --untar

helm install ${PLM_HELM_INSTALL_NAME} ./f5-waf-policy-controller \
  --namespace ${PLM_NS} \
  --create-namespace \
  --set policyController.image.repository=${WAF_CONTROLLER_IMAGE} \
  --set policyController.image.tag=${WAF_CONTROLLER_IMAGE_TAG} \
  --set compiler.image.repository=${WAF_COMPILER_IMAGE} \
  --set compiler.image.tag=${WAF_IMAGE_TAG} \
  --set redis.image.repository=${WAF_REDIS_IMAGE} \
  --set redis.image.tag=${WAF_REDIS_IMAGE_TAG} \
  --set seaweedfs-operator.image.repository=${WAF_SEAWEEDFS_OPERATOR_IMAGE} \
  --set seaweedfs-operator.image.tag=${WAF_SEAWEEDFS_OPERATOR_IMAGE_TAG} \
  --set seaweedfsOperatorConfig.seaweedfs.image.repository=${WAF_SEAWEEDFS_IMAGE} \
  --set seaweedfsOperatorConfig.seaweedfs.image.tag=${WAF_SEAWEEDFS_IMAGE_TAG}
```

## Build (and push) the NGF images

```bash
git clone git@github.com:nginx/nginx-gateway-fabric.git && cd nginx-gateway-fabric
git checkout docs/waf-example

NGINX_IMAGE_REPO=${REGISTRY_ROOT}/nginx-plus
```

> **Note:** Ensure you have the `nginx-repo.crt` and `nginx-repo.key` files in the directory.

```bash
export NAP_WAF_REPO_URL=https://${ARTIFACTORY_HOST}/artifactory/f5-waf_on_nginx-alpine
make build-nginx-plus-image-with-nap-waf-dev NGINX_PLUS_PREFIX=$NGINX_IMAGE_REPO TAG=$NGF_IMAGE_TAG
docker push $NGINX_IMAGE_REPO:$NGF_IMAGE_TAG

make build-ngf-image GOARCH=amd64 PREFIX=${REGISTRY_ROOT} TAG=$NGF_IMAGE_TAG
docker push ${REGISTRY_ROOT}:$NGF_IMAGE_TAG
```

## Create the Plus license key

> **Note:** Ensure you have the `license.jwt` file in the directory.

```bash
kubectl create namespace nginx-gateway || true
kubectl -n nginx-gateway create secret generic nplus-license --from-file license.jwt
```

## Deploy NGF with PLM args

```bash
helm install nginx-gateway charts/nginx-gateway-fabric \
  --wait \
  --set nginx.image.repository=${REGISTRY_ROOT}/nginx-plus \
  --set nginxGateway.image.pullPolicy=Always \
  --set nginx.service.type=LoadBalancer \
  --set nginxGateway.image.repository=${REGISTRY_ROOT} \
  --set nginxGateway.image.tag=${NGF_IMAGE_TAG} \
  --set nginx.image.tag=${NGF_IMAGE_TAG} \
  --set nginx.image.pullPolicy=Always \
  --set nginx.usage.endpoint=product.connect.nginxlab.net \
  --set nginx.plus=true \
  --set nginxGateway.plmStorage.url=${PLM_HELM_INSTALL_NAME}-seaweed-filer.${PLM_NS}.svc.cluster.local:8333 \
  --set nginxGateway.plmStorage.credentialsSecretName=${PLM_NS}/${PLM_HELM_INSTALL_NAME}-seaweedfs-auth \
  -n nginx-gateway --create-namespace
```

## Create the resources

## Step 1 - Deploy the NginxProxy

Create the `NginxProxy` resource that enables WAF and configures the WAF sidecar container images.
Update the image repositories and tags in `nginx-proxy.yaml` to match your environment before applying:

```console
kubectl apply -f nginx-proxy.yaml
```

## Step 2 - Deploy a Web Application

Create the application deployments and services:

```console
kubectl apply -f cafe.yaml
```

## Step 3 - Deploy the AP Policy

1. Create the syslog service and pod for the App Protect security logs:

    ```console
    kubectl apply -f syslog.yaml
    ```

1. Create the App Protect policy and log configuration:

    ```console
    kubectl apply -f appolicy.yaml
    kubectl apply -f ap-logconf.yaml
    ```

## Step 4 - Deploy the Gateway

Create the Gateway resource. It references the `NginxProxy` created in Step 1 via `infrastructure.parametersRef`,
which enables WAF:

```console
kubectl apply -f gateway.yaml
```

## Step 5 - Deploy the WAFGatewayBindingPolicy

Create the `WAFGatewayBindingPolicy` to bind the App Protect policy to the Gateway:

```console
kubectl apply -f wgbpolicy.yaml
```

## Step 6 - Configure Routes

Create the HTTPRoutes for the coffee and tea services:

```console
kubectl apply -f cafe-routes.yaml
```

## Step 7 - Test the Application

To access the application, you will need the external IP address and port of the Gateway service. Save them into shell
variables:

```console
GW_IP=<gateway-external-ip>
GW_PORT=<gateway-port>
```

1. Send a request to the coffee service:

    ```console
    curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/coffee
    ```

    ```text
    <html><head><title>Request Rejected</title></head><body>
    ...
    ```

    Note that the response is blocked by the DataGuard WAF policy due to the sensitive data (credit card number and SSN).

1. Send a request to the tea service:

    ```console
    curl --resolve cafe.example.com:$GW_PORT:$GW_IP http://cafe.example.com:$GW_PORT/tea
    ```

1. Now, send a request with a suspicious URL to verify WAF blocking:

    ```console
    curl --resolve cafe.example.com:$GW_PORT:$GW_IP "http://cafe.example.com:$GW_PORT/<script>"
    ```

    ```text
    <html><head><title>Request Rejected</title></head><body>
    ...
    ```

1. To check the security logs in the syslog pod:

    ```console
    kubectl exec -it <SYSLOG_POD> -- cat /var/log/messages
    ```
