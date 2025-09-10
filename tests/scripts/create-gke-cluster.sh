#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

ip_random_digit=$((1 + RANDOM % 250))

IS_CI=${1:-false}

IPV6_ENABLE=${2:-false}

if [ -z "$GKE_MACHINE_TYPE" ]; then
    # If the environment variable is not set, use a default value
    GKE_MACHINE_TYPE="e2-medium"
fi

if [ -z "$GKE_NUM_NODES" ]; then
    # If the environment variable is not set, use a default value
    GKE_NUM_NODES="3"
fi

if [ "${IPV6_ENABLE}" = "true" ]; then
    echo "Creating IPv6 Network interface for the GKE cluster"
    gcloud compute networks create ${GKE_CLUSTER_NAME}-network --subnet-mode=custom --bgp-routing-mode=regional --mtu=1460
    gcloud compute networks subnets create ${GKE_CLUSTER_NAME}-subnet \
        --network=${GKE_CLUSTER_NAME}-network \
        --stack-type=IPV6_ONLY \
        --ipv6-access-type=EXTERNAL \
        --region=${GKE_CLUSTER_REGION}

    gcloud compute firewall-rules create ${GKE_CLUSTER_NAME}-firewall --network ${GKE_CLUSTER_NAME}-network --allow tcp:22,tcp:3389,icmp
fi

gcloud container clusters create "${GKE_CLUSTER_NAME}" \
    --project "${GKE_PROJECT}" \
    --zone "${GKE_CLUSTER_ZONE}" \
    --enable-master-authorized-networks \
    --enable-ip-alias \
    --service-account "${GKE_NODES_SERVICE_ACCOUNT}" \
    --enable-private-nodes \
    --master-ipv4-cidr 172.16.${ip_random_digit}.32/28 \
    --metadata=block-project-ssh-keys=TRUE \
    --monitoring=SYSTEM,POD,DEPLOYMENT \
    --logging=SYSTEM,WORKLOAD \
    --machine-type "${GKE_MACHINE_TYPE}" \
    --num-nodes "${GKE_NUM_NODES}" \
    --no-enable-insecure-kubelet-readonly-port \
    --subnetwork=${GKE_CLUSTER_NAME}-subnet

# Add current IP to GKE master control node access, if this script is not invoked during a CI run.
if [ "${IS_CI}" = "false" ]; then
    SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
    "${SCRIPT_DIR}"/add-local-ip-auth-networks.sh
fi
