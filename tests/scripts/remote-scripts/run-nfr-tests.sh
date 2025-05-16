#!/usr/bin/env bash

set -e

source "${HOME}"/vars.env

if [ "${START_LONGEVITY}" == "true" ]; then
    GINKGO_LABEL="longevity-setup"
elif [ "${STOP_LONGEVITY}" == "true" ]; then
    GINKGO_LABEL="longevity-teardown"
fi

cd nginx-gateway-fabric/tests && make .vm-nfr-test CI=${CI} TAG="${TAG}" PREFIX="${PREFIX}" NGINX_PREFIX="${NGINX_PREFIX}" NGINX_PLUS_PREFIX="${NGINX_PLUS_PREFIX}" PLUS_ENABLED="${PLUS_ENABLED}" GINKGO_LABEL=${GINKGO_LABEL} GINKGO_FLAGS="${GINKGO_FLAGS}" PULL_POLICY=Always GW_SERVICE_TYPE=LoadBalancer NGF_VERSION="${NGF_VERSION}" PLUS_USAGE_ENDPOINT="${PLUS_USAGE_ENDPOINT}" GKE_PROJECT="${GKE_PROJECT}"

if [ "${START_LONGEVITY}" == "true" ]; then
    suite/scripts/longevity-wrk.sh
fi
