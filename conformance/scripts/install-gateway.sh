#!/bin/bash

if [ -z $1 ]; then
    echo "gateway API version argument not set; exiting"
    exit 1
fi

if [ $1 == "main" ]; then
    temp_dir=$(mktemp -d)
    cd ${temp_dir}
    curl -s https://codeload.github.com/kubernetes-sigs/gateway-api/tar.gz/main | tar -xz --strip=2 gateway-api-main/config
    kubectl apply -f webhook
    kubectl apply -f crd/standard
    rm -rf ${temp_dir}
else
    kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v$1/standard-install.yaml
fi
