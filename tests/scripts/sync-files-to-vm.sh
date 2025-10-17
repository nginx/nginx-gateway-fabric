#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

NGF_DIR=$(dirname "$PWD")/

gcloud compute config-ssh --ssh-config-file ngf-gcp.ssh >/dev/null

rsync -ave 'ssh -F ngf-gcp.ssh' --exclude '.git' "${NGF_DIR}" username@"${RESOURCE_NAME}"."${GKE_CLUSTER_ZONE}"."${GKE_PROJECT}":~/nginx-gateway-fabric
