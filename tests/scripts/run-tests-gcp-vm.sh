#!/usr/bin/env bash

set -eo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

GKE_CLUSTER_ZONE=$(tofu -chdir=tofu output -raw k8s_cluster_zone)
GKE_CLUSTER_NAME=$(tofu -chdir=tofu output -raw k8s_cluster_name)
GKE_PROJECT=$(tofu -chdir=tofu output -raw project_id)
VM_NAME=$(tofu -chdir=tofu output -raw vm_name)

gcloud compute scp --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" "${SCRIPT_DIR}"/vars.env username@"${VM_NAME}":~

gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${VM_NAME}" \
    --command="export START_LONGEVITY=${START_LONGEVITY} &&\
        export STOP_LONGEVITY=${STOP_LONGEVITY} &&\
        export CI=${CI} &&\
        bash -s" <"${SCRIPT_DIR}"/remote-scripts/run-nfr-tests.sh
retcode=$?

if [ ${retcode} -ne 0 ]; then
    echo "Error running tests on VM"
    exit 1
fi

## Use rsync if running locally (faster); otherwise if in the pipeline don't download an SSH config
if [ "${CI}" = "false" ]; then
    gcloud compute config-ssh --ssh-config-file ngf-gcp.ssh >/dev/null
    rsync -ave 'ssh -F ngf-gcp.ssh' username@"${VM_NAME}"."${GKE_CLUSTER_ZONE}"."${GKE_PROJECT}":~/nginx-gateway-fabric/tests/results .
else
    gcloud compute scp --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" --recurse username@"${VM_NAME}":~/nginx-gateway-fabric/tests/results .
fi

## If tearing down the longevity test, we need to collect logs from gcloud and add to the results
if [ "${STOP_LONGEVITY}" = "true" ]; then
    version=${NGF_VERSION}
    if [ "${version}" = "" ]; then
        version=${TAG}
    fi

    runType=oss
    if [ "${PLUS_ENABLED}" = "true" ]; then
        runType=plus
    fi

    results="${SCRIPT_DIR}/../results/longevity/$version/$version-$runType.md"
    printf "\n## Error Logs\n\n" >>"${results}"

    ## ngf error logs
    ngfErrText=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${GKE_CLUSTER_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx-gateway AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND severity=ERROR AND SEARCH("error")' --format "value(textPayload)")
    ngfErrJSON=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${GKE_CLUSTER_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx-gateway AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND severity=ERROR AND SEARCH("error")' --format "value(jsonPayload)")
    printf "### nginx-gateway\n%s\n%s\n\n" "${ngfErrText}" "${ngfErrJSON}" >>"${results}"

    ## nginx error logs
    ngxErr=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${GKE_CLUSTER_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND severity=ERROR AND SEARCH("`[warn]`") OR SEARCH("`[error]`") OR SEARCH("`[emerg]`")' --format "value(textPayload)")
    printf "### nginx\n%s\n\n" "${ngxErr}" >>"${results}"

    ## nginx non-200 responses (also filter out 499 since wrk cancels connections)
    ngxNon200=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${GKE_CLUSTER_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND "GET" "HTTP/1.1" -"200" -"499" -"client prematurely closed connection"' --format "value(textPayload)")
    printf "%s\n\n" "${ngxNon200}" >>"${results}"
fi
