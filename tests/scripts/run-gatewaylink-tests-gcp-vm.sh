#!/usr/bin/env bash

# Drives the GatewayLink integration test on the bastion VM over SSH. Modeled on
# run-tests-gcp-vm.sh, but this is a pass/fail functional test so it does not collect results files.

set -eo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

source "${SCRIPT_DIR}/vars.env"

gcloud compute scp --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" "${SCRIPT_DIR}"/vars.env username@"${RESOURCE_NAME}":~

gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
    --command="export CI=${CI} GATEWAYLINK_LABEL='${GATEWAYLINK_LABEL:-gatewaylink}' && bash -s" \
    <"${SCRIPT_DIR}"/remote-scripts/run-gatewaylink-tests.sh
