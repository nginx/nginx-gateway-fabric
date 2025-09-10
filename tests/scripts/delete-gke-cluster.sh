#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

gcloud container clusters delete "${GKE_CLUSTER_NAME}" --zone "${GKE_CLUSTER_ZONE}" --project "${GKE_PROJECT}" --quiet

echo "Deleting firewall rule ${GKE_CLUSTER_NAME}-firewall (if exists)..."
gcloud compute firewall-rules delete ${GKE_CLUSTER_NAME}-firewall --quiet || true
echo "Deleting subnet ${GKE_CLUSTER_NAME}-subnet (if exists)..."
gcloud compute networks subnets delete ${GKE_CLUSTER_NAME}-subnet --region=${GKE_CLUSTER_REGION} --quiet || true
echo "Deleting network ${GKE_CLUSTER_NAME}-network (if exists)..."
gcloud compute networks delete ${GKE_CLUSTER_NAME}-network --quiet || true
