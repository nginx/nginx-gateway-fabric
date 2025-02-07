#!/usr/bin/env bash

set -eo pipefail

NGF_DIR=$(dirname "${PWD}")/
GKE_CLUSTER_ZONE=$(tofu -chdir=tofu output -raw k8s_cluster_zone)
GKE_PROJECT=$(tofu -chdir=tofu output -raw project_id)
VM_NAME=$(tofu -chdir=tofu output -raw vm_name)

gcloud compute config-ssh --ssh-config-file ngf-gcp.ssh >/dev/null

rsync -Putae 'ssh -F ngf-gcp.ssh' "${NGF_DIR}" username@"${VM_NAME}"."${GKE_CLUSTER_ZONE}"."${GKE_PROJECT}":~/nginx-gateway-fabric
