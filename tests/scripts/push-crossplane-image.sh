#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

docker tag nginx-crossplane:latest gcr.io/$GKE_PROJECT/nginx-crossplane:latest
docker push gcr.io/$GKE_PROJECT/nginx-crossplane:latest
