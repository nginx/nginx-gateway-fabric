#!/usr/bin/env bash

set -eo pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <pod-name>"
    exit 1
fi

POD_NAME=$1

CODE=$(kubectl get pod "${POD_NAME}" -o jsonpath='{.status.containerStatuses[].state.terminated.exitCode}')
if [ "${CODE}" -ne 0 ]; then
    exit 2
fi
