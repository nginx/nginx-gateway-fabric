#!/usr/bin/env bash
# Live view of the LOCAL data plane pods. These are the BIG-IP pool members for
# cluster-1; as you scale, watch them appear/disappear and mirror onto BIG-IP.
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

exec watch -n 2 $KUBECTL -n "$DEMO_NS" get pods \
  -l gateway.networking.k8s.io/gateway-name="$GATEWAY_NAME" -o wide
