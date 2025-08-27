#!/usr/bin/env bash

set -eo pipefail

source scripts/vars.env

gcloud compute firewall-rules update ${NETWORK_TAGS} --source-ranges ${SOURCE_IP_RANGE}
