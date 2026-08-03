#!/usr/bin/env bash

# Deletes the BIG-IP VE instance and its firewall rule created by create-bigip-vm.sh.

set -o pipefail

source scripts/vars.env

# Delete each independently so a missing resource never leaves the others orphaned. The instance
# carries the IPAM alias IP, so that is removed with it. The pods rule is the egress rule added for
# cluster pool mode.
gcloud compute instances delete "${BIGIP_RESOURCE_NAME}" --quiet --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" || true
gcloud compute firewall-rules delete "${BIGIP_RESOURCE_NAME}" --quiet --project="${GKE_PROJECT}" || true
gcloud compute firewall-rules delete "${BIGIP_RESOURCE_NAME}-pods" --quiet --project="${GKE_PROJECT}" || true
