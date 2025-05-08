#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

docker tag nginx-crossplane:latest us-docker.pkg.dev/$GKE_PROJECT/nginx-gateway-fabric/nginx-crossplane:latest
docker push us-docker.pkg.dev/$GKE_PROJECT/nginx-gateway-fabric/nginx-crossplane:latest
