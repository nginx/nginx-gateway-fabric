#!/bin/bash -e

# NB: UPDATE THESE VARIABLES TO MATCH YOUR ENVIRONMENT!
NAP_CHART_DIRECTORY_PATH=/Users/c.stacke/Desktop/
NGF_NAP_REGISTRY=eu.gcr.io/f5-gcs-7899-ptg-ingrss-ctlr/dev/nginx-gateway-fabric/nap
NGF_CHART_PATH=/Users/c.stacke/workspace/github/nginx/nginx-gateway-fabric/charts/nginx-gateway-fabric
NGINX_IMAGE_REPO=eu.gcr.io/f5-gcs-7899-ptg-ingrss-ctlr/dev/nginx-gateway-fabric/nginx-plus
NGINX_IMAGE_TAG=nap-waf
NGF_IMAGE_REPO=eu.gcr.io/f5-gcs-7899-ptg-ingrss-ctlr/dev/nginx-gateway-fabric
NGF_IMAGE_TAG=ciara
BUILD_AND_PUSH_NAP_IMAGES=false

### --- WAF Variables, Images and Tags ---
NAP_NAMESPACE="ngf-nginx-app-protect"
NAP_HELM_CHART_VERSION=5.12.0-plm-branch-2.60.0
ARTIFACTORY_HOST="sea.artifactory.f5net.com"
export APP_PROTECT_VERSION=5.591.0-r1
export NAP_WAF_REPO_URL=https://${ARTIFACTORY_HOST}/artifactory/f5-waf_on_nginx-alpine
WAF_CONFIG_MGR_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/config-mgr/ubuntu/jammy"
WAF_ENFORCER_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/enforcer/ubuntu/jammy"
WAF_COMPILER_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/nap-x-compiler-image"
WAF_CONTROLLER_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/policy-controller"
WAF_REDIS_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/waf-redis"
WAF_SEAWEEDFS_OPERATOR_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/seaweedfs-operator"
WAF_IMAGE_TAG=v11.625.0
WAF_CONTROLLER_IMAGE_TAG=v2.60.0
WAF_SEAWEEDFS_OPERATOR_IMAGE_TAG=v1.4.0
WAF_REDIS_IMAGE_TAG=8.4.0
WAF_SEAWEEDFS_OPERATOR_CHART_VERSION=5.9.0
WAF_SEAWEEDFS_IMAGE="${ARTIFACTORY_HOST}/f5-waf-docker/seaweedfs"
WAF_SEAWEEDFS_IMAGE_TAG=v1.1.0

# If BUILD_AND_PUSH_NAP_IMAGES=true
if [ "$BUILD_AND_PUSH_NAP_IMAGES" = "true" ]; then

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

fi

# helm repo add f5-nap-helm https://sea.artifactory.f5net.com/artifactory/api/helm/f5-nap-helm
# helm repo update
# helm pull f5-nap-helm/nginx-app-protect --version ${NAP_HELM_CHART_VERSION} --untardir ${NAP_CHART_DIRECTORY_PATH} --untar
# helm dependency update ${NAP_CHART_DIRECTORY_PATH}/nginx-app-protect

helm install nginx-app-protect ${NAP_CHART_DIRECTORY_PATH}/nginx-app-protect  --namespace ${NAP_NAMESPACE} --create-namespace \
    --set appprotect.policyController.enable=true \
    --set appprotect.enable=false \
    --set appprotect.policyController.image.repository=${WAF_CONTROLLER_IMAGE} \
    --set appprotect.policyController.image.tag=${WAF_CONTROLLER_IMAGE_TAG} \
    --set appprotect.policyController.s3Endpoint="http://nginx-app-protect-seaweed-filer.$NAP_NAMESPACE.svc.cluster.local:8333" \
    --set appprotect.policyController.wafCompiler.image.repository=${WAF_COMPILER_IMAGE} \
    --set appprotect.policyController.wafCompiler.image.tag=${WAF_IMAGE_TAG} \
    --set appprotect.policyController.wafRedis.image.repository=${WAF_REDIS_IMAGE} \
    --set appprotect.policyController.wafRedis.image.tag=${WAF_REDIS_IMAGE_TAG} \
    --set seaweedfs-operator.image.repository=${WAF_SEAWEEDFS_OPERATOR_IMAGE} \
    --set seaweedfs-operator.image.tag=${WAF_SEAWEEDFS_OPERATOR_IMAGE_TAG} \
    --set seaweedfsOperatorConfig.enabled=true \
    --set seaweedfsOperatorConfig.seaweedfs.image.repository=$WAF_SEAWEEDFS_IMAGE \
    --set seaweedfsOperatorConfig.seaweedfs.image.tag=$WAF_SEAWEEDFS_IMAGE_TAG

# make build-ngf-image GOARCH=amd64 PREFIX=$NGF_IMAGE_REPO TAG=$NGF_IMAGE_TAG
# docker push $NGF_IMAGE_REPO:$NGF_IMAGE_TAG

# make build-nginx-plus-image-with-nap-waf-dev NGINX_PLUS_PREFIX=$NGINX_IMAGE_REPO TAG=$NGINX_IMAGE_TAG
# docker push $NGINX_IMAGE_REPO:$NGINX_IMAGE_TAG

make install-gateway-crds
kubectl -n nginx-gateway create secret generic nplus-license --from-file license.jwt || true

helm install nginx-gateway $NGF_CHART_PATH \
  --wait \
  --set nginx.image.repository=$NGINX_IMAGE_REPO \
  --set nginxGateway.image.pullPolicy=Always \
  --set nginx.service.type=LoadBalancer \
  --set nginxGateway.image.repository=$NGF_IMAGE_REPO \
  --set nginxGateway.image.tag=$NGF_IMAGE_TAG \
  --set nginx.image.tag=$NGINX_IMAGE_TAG \
  --set nginx.image.pullPolicy=Always \
  --set nginx.usage.endpoint=product.connect.nginxlab.net \
  --set nginx.plus=true \
  --set nginxGateway.plmStorage.url=nginx-app-protect-seaweed-filer.${NAP_NAMESPACE}.svc.cluster.local:8333 \
  --set nginxGateway.plmStorage.credentialsSecretName=${NAP_NAMESPACE}/nginx-app-protect-seaweedfs-auth \
  -n nginx-gateway --create-namespace
