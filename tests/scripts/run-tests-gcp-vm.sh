#!/usr/bin/env bash

set -eo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

source scripts/vars.env

gcloud compute scp --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" "${SCRIPT_DIR}"/vars.env username@"${RESOURCE_NAME}":~

## Create a timestamp marker on the VM so we can identify only new/modified results files after the test run.
gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
    --command="touch ~/results-marker"

gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
    --command="export START_LONGEVITY=${START_LONGEVITY} &&\
        export STOP_LONGEVITY=${STOP_LONGEVITY} &&\
        export CI=${CI} &&\
        bash -s" <"${SCRIPT_DIR}"/remote-scripts/run-nfr-tests.sh
retcode=$?

## Download only new/modified results files from the VM (not the entire historical results directory).
## We use the timestamp marker created before the test run to identify changed files,
## tar them up preserving directory structure, and extract locally.
mkdir -p results

gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
    --command="cd ~/nginx-gateway-fabric/tests && \
        find results -newer ~/results-marker -type f > /tmp/changed-results.txt && \
        if [ -s /tmp/changed-results.txt ]; then \
            tar cf /tmp/changed-results.tar -T /tmp/changed-results.txt; \
        fi"

## Copy the tar back and extract if it exists (it won't exist if no results were generated).
if gcloud compute ssh --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" username@"${RESOURCE_NAME}" \
    --command="test -f /tmp/changed-results.tar"; then
    gcloud compute scp --zone "${GKE_CLUSTER_ZONE}" --project="${GKE_PROJECT}" \
        username@"${RESOURCE_NAME}":/tmp/changed-results.tar /tmp/changed-results.tar
    tar xf /tmp/changed-results.tar -C .
    rm -f /tmp/changed-results.tar
fi

## Exit with error code after downloading results if tests failed
if [ ${retcode} -ne 0 ]; then
    echo "Error running tests on VM"
    exit 1
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
    ngfErrText=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${RESOURCE_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx-gateway AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND severity=ERROR AND SEARCH("error")' --format "value(textPayload)")
    ngfErrJSON=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${RESOURCE_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx-gateway AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND severity=ERROR AND SEARCH("error")' --format "value(jsonPayload)")
    printf "### nginx-gateway\n%s\n%s\n\n" "${ngfErrText}" "${ngfErrJSON}" >>"${results}"

    ## nginx error logs
    ngxErr=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${RESOURCE_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND severity=ERROR AND SEARCH("`[warn]`") OR SEARCH("`[error]`") OR SEARCH("`[emerg]`")' --format "value(textPayload)")
    printf "### nginx\n%s\n\n" "${ngxErr}" >>"${results}"

    ## nginx non-200 responses (also filter out 499 since wrk cancels connections)
    ngxNon200=$(gcloud logging read --project="${GKE_PROJECT}" 'resource.labels.cluster_name='"${RESOURCE_NAME}"' AND resource.type=k8s_container AND resource.labels.container_name=nginx AND labels."k8s-pod/app_kubernetes_io/instance"=ngf-longevity AND "GET" "HTTP/1.1" -"200" -"499" -"client prematurely closed connection"' --format "value(textPayload)")
    printf "%s\n\n" "${ngxNon200}" >>"${results}"
fi
