#!/usr/bin/env bash

set -o pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
REPO_DIR=$(dirname $(dirname "$SCRIPT_DIR"))

# Default network settings
NETWORK=default
STACK_TYPE="IPV4_ONLY"
NETWORK_TIER="network-tier=PREMIUM"

source scripts/vars.env

# Create custom network and subnet if IPv6 is enabled
if [ "${IPV6_ENABLED}" = "true" ]; then
    echo "Creating IPv6 Network interface for the GKE cluster"
    gcloud compute networks create ${RESOURCE_NAME} --subnet-mode=custom --bgp-routing-mode=regional --mtu=1460 --quiet
    gcloud compute networks subnets create ${RESOURCE_NAME} \
        --network=${RESOURCE_NAME} \
        --stack-type=IPV6_ONLY \
        --ipv6-access-type=EXTERNAL \
        --region=${GKE_CLUSTER_REGION}

    NETWORK=${RESOURCE_NAME}
    NETWORK_TIER="ipv6-network-tier=PREMIUM"
    STACK_TYPE="IPV6_ONLY"
fi

gcloud compute firewall-rules create "${RESOURCE_NAME}" \
    --project="${GKE_PROJECT}" \
    --direction=INGRESS \
    --priority=1000 \
    --network=${NETWORK} \
    --action=ALLOW \
    --rules=tcp:22 \
    --source-ranges="${SOURCE_IP_RANGE}" \
    --target-tags="${NETWORK_TAGS}"

gcloud compute instances create "${RESOURCE_NAME}" --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" --machine-type=n2-standard-2 \
    --network-interface=${NETWORK_TIER},stack-type=${STACK_TYPE},subnet=${NETWORK} --maintenance-policy=MIGRATE \
    --provisioning-model=STANDARD --service-account="${GKE_SVC_ACCOUNT}" \
    --scopes=https://www.googleapis.com/auth/devstorage.read_only,https://www.googleapis.com/auth/logging.write,https://www.googleapis.com/auth/monitoring.write,https://www.googleapis.com/auth/servicecontrol,https://www.googleapis.com/auth/service.management.readonly,https://www.googleapis.com/auth/trace.append,https://www.googleapis.com/auth/cloud-platform \
    --tags="${NETWORK_TAGS}" --create-disk=auto-delete=yes,boot=yes,device-name="${RESOURCE_NAME}",image-family=projects/"${GKE_PROJECT}"/global/images/ngf-debian,mode=rw,size=20 --no-shielded-secure-boot --shielded-vtpm --shielded-integrity-monitoring --labels=goog-ec-src=vm_add-gcloud --reservation-affinity=any

# Add VM IP to GKE master control node access, if required
if [ "${ADD_VM_IP_AUTH_NETWORKS}" = "true" ]; then

    if [ "${IPV6_ENABLED}" = "true" ]; then
        echo "IPv6 is enabled, fetching the external IPv6 address"
        EXTERNAL_IP=$(gcloud compute instances describe "${RESOURCE_NAME}" --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" \
            --format='value(networkInterfaces[0].ipv6AccessConfigs[0].externalIpv6)')
    else
        echo "IPv6 is not enabled, fetching the external IPv4 address"
        EXTERNAL_IP=$(gcloud compute instances describe "${RESOURCE_NAME}" --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" \
            --format='value(networkInterfaces[0].accessConfigs[0].natIP)')
    fi
    
    echo "External IP of the VM is: ${EXTERNAL_IP}"

    CURRENT_AUTH_NETWORK=$(gcloud container clusters describe "${GKE_CLUSTER_NAME}" --zone="${GKE_CLUSTER_ZONE}" \
        --format="value(masterAuthorizedNetworksConfig.cidrBlocks[0])" | sed 's/cidrBlock=//')

    TMP_CURRENT_AUTH_NETWORK=$(gcloud container clusters describe "${GKE_CLUSTER_NAME}" --zone="${GKE_CLUSTER_ZONE}"
    echo "TMP Current GKE master authorized networks: ${TMP_CURRENT_AUTH_NETWORK}"
    gcloud container clusters update "${GKE_CLUSTER_NAME}" --zone="${GKE_CLUSTER_ZONE}" --enable-master-authorized-networks --master-authorized-networks="${EXTERNAL_IP}"/32,"${CURRENT_AUTH_NETWORK}"
fi

# Poll for SSH connectivity
MAX_RETRIES=10
RETRY_INTERVAL=5
for ((i = 1; i <= MAX_RETRIES; i++)); do
    echo "Attempt $i to connect to the VM..."
    gcloud compute ssh username@"${RESOURCE_NAME}" --zone="${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" --quiet --command="echo 'VM is ready'"
    if [ $? -eq 0 ]; then
        echo "SSH connection successful. VM is ready."
        break
    fi
    echo "Waiting for ${RETRY_INTERVAL} seconds before the next attempt..."
    sleep ${RETRY_INTERVAL}
done

gcloud compute scp --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" "${SCRIPT_DIR}"/vars.env username@"${RESOURCE_NAME}":~

gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" --command="bash -s" <"${SCRIPT_DIR}"/remote-scripts/install-deps.sh

if [ -n "${NGF_REPO}" ] && [ "${NGF_REPO}" != "nginx" ]; then
    gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
        --command="bash -i <<EOF
rm -rf nginx-gateway-fabric
git clone https://github.com/${NGF_REPO}/nginx-gateway-fabric.git
EOF" -- -t
fi

gcloud compute scp --quiet --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" "${REPO_DIR}"/license.jwt username@"${RESOURCE_NAME}":~/nginx-gateway-fabric/

gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
    --command="bash -i <<EOF
cd nginx-gateway-fabric/tests
git fetch -pP --all
git checkout ${NGF_BRANCH}
git pull
gcloud container clusters get-credentials ${GKE_CLUSTER_NAME} --zone ${GKE_CLUSTER_ZONE} --project=${GKE_PROJECT} --quiet
EOF" -- -t
