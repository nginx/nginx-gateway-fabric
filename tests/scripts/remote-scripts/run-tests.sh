#!/bin/bash

set -e

source ~/vars.env

echo "export PATH=$PATH:/usr/local/go/bin" >> $HOME/.profile && . $HOME/.profile

cd nginx-gateway-fabric/tests && make test TAG=${TAG} PREFIX=${PREFIX} NGINX_PREFIX=${NGINX_PREFIX} GINKGO_LABEL=${GINKGO_LABEL} GINKGO_FLAGS=${GINKGO_FLAGS} PULL_POLICY=Always GW_SERVICE_TYPE=LoadBalancer GW_SVC_GKE_INTERNAL=true
