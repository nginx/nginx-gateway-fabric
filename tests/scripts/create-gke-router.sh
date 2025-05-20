#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

gcloud compute routers create "${RESOURCE_NAME}" \
    --region "${GKE_CLUSTER_REGION}" \
    --network default

gcloud compute routers nats create "${RESOURCE_NAME}" \
    --router-region "${GKE_CLUSTER_REGION}" \
    --router "${RESOURCE_NAME}" \
    --nat-all-subnet-ip-ranges \
    --auto-allocate-nat-external-ips
