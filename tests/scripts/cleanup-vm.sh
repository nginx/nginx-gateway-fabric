#!/usr/bin/env bash

set -o pipefail

source scripts/vars.env

skip_gke_master_control_node_access="${1:-false}"

# Remove VM IP from GKE master control node access, if required
if [ "${ADD_VM_IP_AUTH_NETWORKS}" = "true" ] && [ "${skip_gke_master_control_node_access}" != "true" ]; then
    CURRENT_AUTH_NETWORK=$(gcloud container clusters describe "${GKE_CLUSTER_NAME}" --zone "${GKE_CLUSTER_ZONE}" \
        --format="value(masterAuthorizedNetworksConfig.cidrBlocks[0])" | sed 's/cidrBlock=//')
    gcloud container clusters update "${GKE_CLUSTER_NAME}" --zone "${GKE_CLUSTER_ZONE}" --enable-master-authorized-networks --master-authorized-networks="${CURRENT_AUTH_NETWORK}"
fi

gcloud compute instances delete "${RESOURCE_NAME}" --quiet --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}"

gcloud compute firewall-rules delete "${RESOURCE_NAME}" --quiet --project="${GKE_PROJECT}"

# Clean up the custom network and subnet if IPv6 was enabled
if [ "${IPV6_ENABLED}" = "true" ]; then
    gcloud compute networks subnets delete ${RESOURCE_NAME} --region=${GKE_CLUSTER_REGION} --quiet || true
    gcloud compute networks delete ${RESOURCE_NAME} --quiet || true
fi
